/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-logr/logr"
	"github.com/innabox/fulfillment-common/network"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/innabox/fulfillment-common/logging"
	"github.com/innabox/fulfillment-service/internal"
	eventsv1 "github.com/innabox/fulfillment-service/internal/api/events/v1"
	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	metadatav1 "github.com/innabox/fulfillment-service/internal/api/metadata/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/auth"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/recovery"
	"github.com/innabox/fulfillment-service/internal/servers"
)

// NewStartServerCommand creates and returns the `start server` command.
func NewStartServerCommand() *cobra.Command {
	runner := &startServerCommandRunner{}
	command := &cobra.Command{
		Use:   "server",
		Short: "Starts the gRPC server",
		Args:  cobra.NoArgs,
		RunE:  runner.run,
	}
	flags := command.Flags()
	network.AddListenerFlags(flags, network.GrpcListenerName, network.DefaultGrpcAddress)
	database.AddFlags(flags)
	flags.StringVar(
		&runner.args.grpcAuthnType,
		"grpc-authn-type",
		auth.GrpcGuestAuthnType,
		fmt.Sprintf(
			"Type of gRPC authentication. Valid values are \"%s\" and \"%s\"",
			auth.GrpcGuestAuthnType, auth.GrpcExternalAuthnType,
		),
	)
	flags.StringSliceVar(
		&runner.args.grpcAuthnTrustedTokenIssuers,
		"grpc-authn-trusted-token-issuers",
		[]string{},
		"Comma separated list of token issuers that are adversised as trusted by the gRPC server.",
	)
	flags.StringVar(
		&runner.args.tenancyLogic,
		"tenancy-logic",
		"default",
		"Type of tenancy logic to use. Valid values are 'default' and 'serviceaccount'.",
	)
	return command
}

// startServerCommandRunner contains the data and logic needed to run the `start server` command.
type startServerCommandRunner struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	args   struct {
		caFiles                      []string
		grpcAuthnType                string
		grpcAuthnTrustedTokenIssuers []string
		tenancyLogic                 string
	}
}

// run runs the `start server` command.
func (c *startServerCommandRunner) run(cmd *cobra.Command, argv []string) error {
	// Get the context:
	ctx := cmd.Context()

	// Get the dependencies from the context:
	c.logger = internal.LoggerFromContext(ctx)

	// Configure the Kubernetes libraries to use the logger:
	logrLogger := logr.FromSlogHandler(c.logger.Handler())
	crlog.SetLogger(logrLogger)
	klog.SetLogger(logrLogger)

	// Save the flags:
	c.flags = cmd.Flags()

	// Wait till the database is available:
	dbTool, err := database.NewTool().
		SetLogger(c.logger).
		SetFlags(c.flags).
		Build()
	if err != nil {
		return err
	}
	c.logger.InfoContext(ctx, "Waiting for database")
	err = dbTool.Wait(ctx)
	if err != nil {
		return err
	}

	// Run the migrations:
	c.logger.InfoContext(ctx, "Running database migrations")
	err = dbTool.Migrate(ctx)
	if err != nil {
		return err
	}

	// Create the database connection pool:
	c.logger.InfoContext(ctx, "Creating database connection pool")
	dbPool, err := dbTool.Pool(ctx)
	if err != nil {
		return err
	}

	// Create the network listener:
	listener, err := network.NewListener().
		SetLogger(c.logger).
		SetFlags(c.flags, network.GrpcListenerName).
		Build()
	if err != nil {
		return err
	}

	// Prepare the logging interceptor:
	c.logger.InfoContext(ctx, "Creating logging interceptor")
	loggingInterceptor, err := logging.NewInterceptor().
		SetLogger(c.logger).
		SetFlags(c.flags).
		Build()
	if err != nil {
		return err
	}

	// Prepare the authentication interceptor:
	c.logger.InfoContext(
		ctx,
		"Creating authentication interceptor",
		slog.String("type", c.args.grpcAuthnType),
	)
	var authnFunc auth.GrpcAuthnFunc
	switch strings.ToLower(c.args.grpcAuthnType) {
	case auth.GrpcGuestAuthnType:
		authnFunc, err = auth.NewGrpcGuestAuthnFunc().
			SetLogger(c.logger).
			SetFlags(c.flags).
			Build()
		if err != nil {
			return fmt.Errorf("failed to create gRPC guest authentication function: %w", err)
		}
	case auth.GrpcExternalAuthnType:
		authnFunc, err = auth.NewGrpcExternalAuthnFunc().
			SetLogger(c.logger).
			SetFlags(c.flags).
			AddPublicMethodRegex(publicMethodRegex).
			Build()
		if err != nil {
			return fmt.Errorf("failed to create gRPC external authentication function: %w", err)
		}
	default:
		return fmt.Errorf(
			"unknown gRPC authentication type '%s', valid values are '%s' and '%s'",
			c.args.grpcAuthnType, auth.GrpcGuestAuthnType, auth.GrpcExternalAuthnType,
		)
	}
	authnInterceptor, err := auth.NewGrpcAuthnInterceptor().
		SetLogger(c.logger).
		SetFunction(authnFunc).
		Build()
	if err != nil {
		return err
	}

	// Prepare the panic interceptor:
	c.logger.InfoContext(ctx, "Creating panic interceptor")
	panicInterceptor, err := recovery.NewGrpcPanicInterceptor().
		SetLogger(c.logger).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create panic interceptor: %w", err)
	}

	// Prepare the transactions interceptor:
	c.logger.InfoContext(ctx, "Creating transactions interceptor")
	txManager, err := database.NewTxManager().
		SetLogger(c.logger).
		SetPool(dbPool).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create transactions manager: %w", err)
	}
	txInterceptor, err := database.NewTxInterceptor().
		SetLogger(c.logger).
		SetManager(txManager).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create transactions interceptor: %w", err)
	}

	// Create the gRPC server:
	c.logger.InfoContext(ctx, "Creating gRPC server")
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			panicInterceptor.UnaryServer,
			loggingInterceptor.UnaryServer,
			authnInterceptor.UnaryServer,
			txInterceptor.UnaryServer,
		),
		grpc.ChainStreamInterceptor(
			panicInterceptor.StreamServer,
			loggingInterceptor.StreamServer,
			authnInterceptor.StreamServer,
		),
	)

	// Register the reflection server:
	c.logger.InfoContext(ctx, "Registering gRPC reflection server")
	reflection.RegisterV1(grpcServer)

	// Register the health server:
	c.logger.InfoContext(ctx, "Registering gRPC health server")
	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(grpcServer, healthServer)

	// Create the notifier:
	c.logger.InfoContext(ctx, "Creating notifier")
	notifier, err := database.NewNotifier().
		SetLogger(c.logger).
		SetChannel("events").
		Build()
	if err != nil {
		return fmt.Errorf("failed to create notifier: %w", err)
	}

	// Create the public attribution logic:
	c.logger.InfoContext(ctx, "Creating public attribution logic")
	publicAttributionLogic, err := auth.NewDefaultAttributionLogic().
		SetLogger(c.logger).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create public attribution logic: %w", err)
	}

	// Create the public tenancy logic:
	c.logger.InfoContext(
		ctx,
		"Creating public tenancy logic",
		slog.String("type", c.args.tenancyLogic),
	)
	var publicTenancyLogic auth.TenancyLogic
	switch strings.ToLower(c.args.tenancyLogic) {
	case "default":
		publicTenancyLogic, err = auth.NewDefaultTenancyLogic().
			SetLogger(c.logger).
			Build()
		if err != nil {
			return fmt.Errorf("failed to create default tenancy logic: %w", err)
		}
	case "serviceaccount":
		publicTenancyLogic, err = auth.NewServiceAccountTenancyLogic().
			SetLogger(c.logger).
			Build()
		if err != nil {
			return fmt.Errorf("failed to create service account tenancy logic: %w", err)
		}
	default:
		return fmt.Errorf(
			"unknown tenancy logic '%s', valid values are 'default' and 'serviceaccount'",
			c.args.tenancyLogic,
		)
	}

	// Create the private attribution logic:
	c.logger.InfoContext(ctx, "Creating private attribution logic")
	privateAttributionLogic, err := auth.NewSystemAttributionLogic().
		SetLogger(c.logger).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create system attribution logic: %w", err)
	}

	// Create the system tenancy logic:
	c.logger.InfoContext(ctx, "Creating private tenancy logic")
	privateTenancyLogic, err := auth.NewSystemTenancyLogic().
		SetLogger(c.logger).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create private tenancy logic: %w", err)
	}

	// Create the metadata server:
	c.logger.InfoContext(ctx, "Creating metadata server")
	metadataServer, err := servers.NewMetadataServer().
		SetLogger(c.logger).
		AddAutnTrustedTokenIssuers(c.args.grpcAuthnTrustedTokenIssuers...).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create metadata server: %w", err)
	}
	metadatav1.RegisterMetadataServer(grpcServer, metadataServer)

	// Create the cluster templates server:
	c.logger.InfoContext(ctx, "Creating cluster templates server")
	clusterTemplatesServer, err := servers.NewClusterTemplatesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(publicAttributionLogic).
		SetTenancyLogic(publicTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create cluster templates server: %w", err)
	}
	ffv1.RegisterClusterTemplatesServer(grpcServer, clusterTemplatesServer)

	// Create the private cluster templates server:
	c.logger.InfoContext(ctx, "Creating private cluster templates server")
	privateClusterTemplatesServer, err := servers.NewPrivateClusterTemplatesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(privateAttributionLogic).
		SetTenancyLogic(privateTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create private cluster templates server: %w", err)
	}
	privatev1.RegisterClusterTemplatesServer(grpcServer, privateClusterTemplatesServer)

	// Create the clusters server:
	c.logger.InfoContext(ctx, "Creating clusters server")
	clustersServer, err := servers.NewClustersServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(publicAttributionLogic).
		SetTenancyLogic(publicTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create clusters server: %w", err)
	}
	ffv1.RegisterClustersServer(grpcServer, clustersServer)

	// Create the private clusters server:
	c.logger.InfoContext(ctx, "Creating private clusters server")
	privateClustersServer, err := servers.NewPrivateClustersServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(privateAttributionLogic).
		SetTenancyLogic(privateTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create private clusters server: %w", err)
	}
	privatev1.RegisterClustersServer(grpcServer, privateClustersServer)

	// Create the host classes server:
	c.logger.InfoContext(ctx, "Creating host classes server")
	hostClassesServer, err := servers.NewHostClassesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(publicAttributionLogic).
		SetTenancyLogic(publicTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create host classes server: %w", err)
	}
	ffv1.RegisterHostClassesServer(grpcServer, hostClassesServer)

	// Create the private host classes server:
	c.logger.InfoContext(ctx, "Creating private host classes server")
	privateHostClassesServer, err := servers.NewPrivateHostClassesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(privateAttributionLogic).
		SetTenancyLogic(privateTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create private host classes server: %w", err)
	}
	privatev1.RegisterHostClassesServer(grpcServer, privateHostClassesServer)

	// Create the hosts server:
	c.logger.InfoContext(ctx, "Creating hosts server")
	hostsServer, err := servers.NewHostsServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(publicAttributionLogic).
		SetTenancyLogic(publicTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create hosts server: %w", err)
	}
	ffv1.RegisterHostsServer(grpcServer, hostsServer)

	// Create the private hosts server:
	c.logger.InfoContext(ctx, "Creating private hosts server")
	privateHostsServer, err := servers.NewPrivateHostsServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(privateAttributionLogic).
		SetTenancyLogic(privateTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create private hosts server: %w", err)
	}
	privatev1.RegisterHostsServer(grpcServer, privateHostsServer)

	// Create the host pools server:
	c.logger.InfoContext(ctx, "Creating host pools server")
	hostPoolsServer, err := servers.NewHostPoolsServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(publicAttributionLogic).
		SetTenancyLogic(publicTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create host pools server: %w", err)
	}
	ffv1.RegisterHostPoolsServer(grpcServer, hostPoolsServer)

	// Create the private host pools server:
	c.logger.InfoContext(ctx, "Creating private host pools server")
	privateHostPoolsServer, err := servers.NewPrivateHostPoolsServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(privateAttributionLogic).
		SetTenancyLogic(privateTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create private host pools server: %w", err)
	}
	privatev1.RegisterHostPoolsServer(grpcServer, privateHostPoolsServer)

	// Create the virtual machine templates server:
	c.logger.InfoContext(ctx, "Creating virtual machine templates server")
	virtualMachineTemplatesServer, err := servers.NewVirtualMachineTemplatesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(publicAttributionLogic).
		SetTenancyLogic(publicTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create virtual machine templates server: %w", err)
	}
	ffv1.RegisterVirtualMachineTemplatesServer(grpcServer, virtualMachineTemplatesServer)

	// Create the private virtual machine templates server:
	c.logger.InfoContext(ctx, "Creating private virtual machine templates server")
	privateVirtualMachineTemplatesServer, err := servers.NewPrivateVirtualMachineTemplatesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(privateAttributionLogic).
		SetTenancyLogic(privateTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create private virtual machine templates server: %w", err)
	}
	privatev1.RegisterVirtualMachineTemplatesServer(grpcServer, privateVirtualMachineTemplatesServer)

	// Create the virtual machines server:
	c.logger.InfoContext(ctx, "Creating virtual machines server")
	virtualMachinesServer, err := servers.NewVirtualMachinesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(publicAttributionLogic).
		SetTenancyLogic(publicTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create virtual machines server: %w", err)
	}
	ffv1.RegisterVirtualMachinesServer(grpcServer, virtualMachinesServer)

	// Create the private virtual machines server:
	c.logger.InfoContext(ctx, "Creating private virtual machines server")
	privateVirtualMachinesServer, err := servers.NewPrivateVirtualMachinesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(privateAttributionLogic).
		SetTenancyLogic(privateTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create private virtual machines server: %w", err)
	}
	privatev1.RegisterVirtualMachinesServer(grpcServer, privateVirtualMachinesServer)

	// Create the private hubs server:
	c.logger.InfoContext(ctx, "Creating hubs server")
	privateHubsServer, err := servers.NewPrivateHubsServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		SetAttributionLogic(privateAttributionLogic).
		SetTenancyLogic(privateTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create hubs server: %w", err)
	}
	privatev1.RegisterHubsServer(grpcServer, privateHubsServer)

	// Create the events server:
	c.logger.InfoContext(ctx, "Creating events server")
	eventsServer, err := servers.NewEventsServer().
		SetLogger(c.logger).
		SetFlags(c.flags).
		SetDbUrl(dbTool.URL()).
		SetTenancyLogic(publicTenancyLogic).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create events server: %w", err)
	}
	go func() {
		err := eventsServer.Start(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			c.logger.InfoContext(ctx, "Events server finished")
		} else {
			c.logger.ErrorContext(
				ctx,
				"Events server finished",
				slog.Any("error", err),
			)
		}
	}()
	eventsv1.RegisterEventsServer(grpcServer, eventsServer)

	// Create the private events server:
	c.logger.InfoContext(ctx, "Creating private events server")
	privateEventsServer, err := servers.NewPrivateEventsServer().
		SetLogger(c.logger).
		SetFlags(c.flags).
		SetDbUrl(dbTool.URL()).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create private events server: %w", err)
	}
	go func() {
		err := privateEventsServer.Start(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			c.logger.InfoContext(ctx, "Private events server finished")
		} else {
			c.logger.ErrorContext(
				ctx,
				"Private events server finished",
				slog.Any("error", err),
			)
		}
	}()
	privatev1.RegisterEventsServer(grpcServer, privateEventsServer)

	// Start serving:
	c.logger.InfoContext(
		ctx,
		"Start serving",
		slog.String("address", listener.Addr().String()),
	)
	go func() {
		defer grpcServer.GracefulStop()
		<-ctx.Done()
	}()
	return grpcServer.Serve(listener)
}

// publicMethodRegex is regular expression for the methods that are considered public, including the metadata, and
// reflection and health methods. These will skip authentication and authorization.
const publicMethodRegex = `^/(metadata|grpc\.(reflection|health))\..*$`
