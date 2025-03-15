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
	"fmt"
	"log/slog"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/innabox/fulfillment-service/internal"
	api "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/innabox/fulfillment-service/internal/auth"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
	"github.com/innabox/fulfillment-service/internal/logging"
	"github.com/innabox/fulfillment-service/internal/network"
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
	auth.AddGrpcAclAuthzFlags(flags)
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
			auth.GrpcAllAuthzType, auth.GrpcAclAuthzType,
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

	// Create the data access objects:
	c.logger.InfoContext(ctx, "Creating data access objects")
	daos, err := dao.NewSet().
		SetLogger(c.logger).
		SetPool(dbPool).
		Build()
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
	case auth.GrpcAclAuthzType:
		authzFunc, err = auth.NewGrpcAclAuthzFunc().
			SetLogger(c.logger).
			SetFlags(c.flags).
			AddPublicMethodRegex(publicMethodRegex).
			Build()
		if err != nil {
			return fmt.Errorf("failed to create gRPC ACL authorization function: %w", err)
		}
	default:
		return fmt.Errorf(
			"unknown gRPC authorization type '%s', valid values are '%s' and '%s'",
			c.grpcAuthzType, auth.GrpcAllAuthzType, auth.GrpcAclAuthzType,
		)
	}
	authzInterceptor, err := auth.NewGrpcAuthzInterceptor().
		SetLogger(c.logger).
		SetFunction(authzFunc).
		Build()
	if err != nil {
		return err
	}

	// Create the gRPC server:
	c.logger.InfoContext(ctx, "Creating gRPC server")
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor.UnaryServer,
			authnInterceptor.UnaryServer,
			authzInterceptor.UnaryServer,
		),
		grpc.ChainStreamInterceptor(
			loggingInterceptor.StreamServer,
			authnInterceptor.StreamServer,
			authzInterceptor.StreamServer,
		),
	)

	// Register the reflection server:
	c.logger.InfoContext(ctx, "Registering gRPC reflection server")
	reflection.RegisterV1(grpcServer)

	// Create the cluster templates server:
	c.logger.InfoContext(ctx, "Creating cluster templates server")
	clusterTemplatesServer, err := servers.NewClusterTemplatesServer().
		SetLogger(c.logger).
		SetFlags(c.flags).
		SetDAOs(daos).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create cluster templates server")
	}
	api.RegisterClusterTemplatesServer(grpcServer, clusterTemplatesServer)

	// Create the cluster orders server:
	c.logger.InfoContext(ctx, "Creating cluster orders server")
	clusterOrdersServer, err := servers.NewClusterOrdersServer().
		SetLogger(c.logger).
		SetFlags(c.flags).
		SetDAOs(daos).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create cluster orders server")
	}
	api.RegisterClusterOrdersServer(grpcServer, clusterOrdersServer)

	// Create the clusters server:
	c.logger.InfoContext(ctx, "Creating clusters server")
	clustersServer, err := servers.NewClustersServer().
		SetLogger(c.logger).
		SetFlags(c.flags).
		SetDAOs(daos).
		Build()
	if err != nil {
		return errors.Wrapf(err, "failed to create clusters server")
	}
	api.RegisterClustersServer(grpcServer, clustersServer)

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

// publicMethodRegex is regular expression for the methods that are considered public, including the reflection methods.
// These will skip authentication and authorization.
const publicMethodRegex = `^/grpc\.reflection\.v1\..*$`
