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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/innabox/fulfillment-service/internal"
	eventsv1 "github.com/innabox/fulfillment-service/internal/api/events/v1"
	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/auth"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/logging"
	"github.com/innabox/fulfillment-service/internal/network"
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
	auth.AddGrpcJwksAuthnFlags(flags)
	auth.AddGrpcRulesAuthzFlags(flags)
	flags.StringVar(
		&runner.grpcAuthnType,
		"grpc-authn-type",
		auth.GrpcGuestAuthnType,
		fmt.Sprintf(
			"Type of gRPC authentication. Valid values are \"%s\" and \"%s\"",
			auth.GrpcGuestAuthnType, auth.GrpcJwksAuthnType,
		),
	)
	flags.StringVar(
		&runner.grpcAuthzType,
		"grpc-authz-type",
		auth.GrpcAllAuthzType,
		fmt.Sprintf(
			"Type of gRPC authorization. Valid values are \"%s\" and \"%s\"",
			auth.GrpcAllAuthzType, auth.GrpcRulesAuthzType,
		),
	)
	return command
}

// startServerCommandRunner contains the data and logic needed to run the `start server` command.
type startServerCommandRunner struct {
	logger        *slog.Logger
	flags         *pflag.FlagSet
	grpcAuthnType string
	grpcAuthzType string
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
		slog.String("type", c.grpcAuthnType),
	)
	var authnFunc auth.GrpcAuthnFunc
	switch strings.ToLower(c.grpcAuthnType) {
	case auth.GrpcGuestAuthnType:
		authnFunc, err = auth.NewGrpcGuestAuthnFunc().
			SetLogger(c.logger).
			SetFlags(c.flags).
			Build()
		if err != nil {
			return fmt.Errorf("failed to create gRPC guest authentication function: %w", err)
		}
	case auth.GrpcJwksAuthnType:
		authnFunc, err = auth.NewGrpcJwksAuthnFunc().
			SetLogger(c.logger).
			SetFlags(c.flags).
			AddPublicMethodRegex(publicMethodRegex).
			Build()
		if err != nil {
			return fmt.Errorf("failed to create gRPC JWKS authentication function: %w", err)
		}
	default:
		return fmt.Errorf(
			"unknown gRPC authentication type '%s', valid values are '%s' and '%s'",
			c.grpcAuthnType, auth.GrpcGuestAuthnType, auth.GrpcJwksAuthnType,
		)
	}
	authnInterceptor, err := auth.NewGrpcAuthnInterceptor().
		SetLogger(c.logger).
		SetFunction(authnFunc).
		Build()
	if err != nil {
		return err
	}

	// Prepare the authorization interceptor:
	c.logger.InfoContext(
		ctx,
		"Creating authorization interceptor",
		slog.String("type", c.grpcAuthzType),
	)
	var authzFunc auth.GrpcAuthzFunc
	switch strings.ToLower(c.grpcAuthzType) {
	case auth.GrpcAllAuthzType:
		authzFunc, err = auth.NewGrpcAllAuthzFunc().
			SetLogger(c.logger).
			SetFlags(c.flags).
			Build()
		if err != nil {
			return fmt.Errorf("failed to create gRPC all authorization function: %w", err)
		}
	case auth.GrpcRulesAuthzType:
		authzFunc, err = auth.NewGrpcRulesAuthzFunc().
			SetLogger(c.logger).
			SetFlags(c.flags).
			Build()
		if err != nil {
			return fmt.Errorf("failed to create gRPC rules authorization function: %w", err)
		}
	default:
		return fmt.Errorf(
			"unknown gRPC authorization type '%s', valid values are '%s' and '%s'",
			c.grpcAuthzType, auth.GrpcAllAuthzType, auth.GrpcRulesAuthzType,
		)
	}
	authzInterceptor, err := auth.NewGrpcAuthzInterceptor().
		SetLogger(c.logger).
		SetFunction(authzFunc).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create gRPC authorization interceptor: %w", err)
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
			authzInterceptor.UnaryServer,
			txInterceptor.UnaryServer,
		),
		grpc.ChainStreamInterceptor(
			panicInterceptor.StreamServer,
			loggingInterceptor.StreamServer,
			authnInterceptor.StreamServer,
			authzInterceptor.StreamServer,
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

	// Create the private cluster templates server:
	c.logger.InfoContext(ctx, "Creating private cluster templates server")
	privateClusterTemplatesServer, err := servers.NewPrivateClusterTemplatesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create private cluster templates server")
	}
	privatev1.RegisterClusterTemplatesServer(grpcServer, privateClusterTemplatesServer)

	// Create the cluster templates server:
	c.logger.InfoContext(ctx, "Creating cluster templates server")
	clusterTemplatesServer, err := servers.NewClusterTemplatesServer().
		SetLogger(c.logger).
		SetPrivate(privateClusterTemplatesServer).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create cluster templates server")
	}
	ffv1.RegisterClusterTemplatesServer(grpcServer, clusterTemplatesServer)

	// Create the private clusters server:
	c.logger.InfoContext(ctx, "Creating private clusters server")
	privateClustersServer, err := servers.NewPrivateClustersServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create private clusters server")
	}
	privatev1.RegisterClustersServer(grpcServer, privateClustersServer)

	// Create the clusters server:
	c.logger.InfoContext(ctx, "Creating clusters server")
	clustersServer, err := servers.NewClustersServer().
		SetLogger(c.logger).
		SetPrivate(privateClustersServer).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create clusters server")
	}
	ffv1.RegisterClustersServer(grpcServer, clustersServer)

	// Create the private host classes server:
	c.logger.InfoContext(ctx, "Creating private host classes server")
	privateHostClassesServer, err := servers.NewPrivateHostClassesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create private host classes server")
	}
	privatev1.RegisterHostClassesServer(grpcServer, privateHostClassesServer)

	// Create the host classes server:
	c.logger.InfoContext(ctx, "Creating host classes server")
	hostClassesServer, err := servers.NewHostClassesServer().
		SetLogger(c.logger).
		SetPrivate(privateHostClassesServer).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create host classes server")
	}
	ffv1.RegisterHostClassesServer(grpcServer, hostClassesServer)

	// Create the private virtual machine templates server:
	c.logger.InfoContext(ctx, "Creating private virtual machine templates server")
	privateVirtualMachineTemplatesServer, err := servers.NewPrivateVirtualMachineTemplatesServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create private virtual machine templates server")
	}
	privatev1.RegisterVirtualMachineTemplatesServer(grpcServer, privateVirtualMachineTemplatesServer)

	// Create the virtual machine templates server:
	c.logger.InfoContext(ctx, "Creating virtual machine templates server")
	virtualMachineTemplatesServer, err := servers.NewVirtualMachineTemplatesServer().
		SetLogger(c.logger).
		SetPrivate(privateVirtualMachineTemplatesServer).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create virtual machine templates server")
	}
	ffv1.RegisterVirtualMachineTemplatesServer(grpcServer, virtualMachineTemplatesServer)

	// Create the private hubs server:
	c.logger.InfoContext(ctx, "Creating hubs server")
	privateHubsServer, err := servers.NewPrivateHubsServer().
		SetLogger(c.logger).
		SetNotifier(notifier).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create hubs server")
	}
	privatev1.RegisterHubsServer(grpcServer, privateHubsServer)

	// Create the events server:
	c.logger.InfoContext(ctx, "Creating events server")
	eventsServer, err := servers.NewEventsServer().
		SetLogger(c.logger).
		SetFlags(c.flags).
		SetDbUrl(dbTool.URL()).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create events server")
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

// publicMethodRegex is regular expression for the methods that are considered public, including the reflection and
// health methods. These will skip authentication and authorization.
const publicMethodRegex = `^/grpc\.(reflection|health)\..*$`
