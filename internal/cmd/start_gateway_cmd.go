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
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/innabox/fulfillment-service/internal"
	api "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/innabox/fulfillment-service/internal/network"
)

// NewStartGatewayCommand creates and returns the `start server` command.
func NewStartGatewayCommand() *cobra.Command {
	runner := &startGatewayCommandRunner{}
	command := &cobra.Command{
		Use:   "gateway",
		Short: "Starts the gRPC gateway",
		Args:  cobra.NoArgs,
		RunE:  runner.run,
	}
	flags := command.Flags()
	network.AddListenerFlags(flags, network.HttpListenerName, network.DefaultHttpAddress)
	network.AddCorsFlags(flags, network.HttpListenerName)
	network.AddGrpcClientFlags(flags, network.GrpcClientName, network.DefaultGrpcAddress)
	return command
}

// startGatewayCommandRunner contains the data and logic needed to run the `start gateway` command.
type startGatewayCommandRunner struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
}

// run runs the `start gateway` command.
func (c *startGatewayCommandRunner) run(cmd *cobra.Command, argv []string) error {
	// Get the context:
	ctx := cmd.Context()

	// Get the dependencies from the context:
	c.logger = internal.LoggerFromContext(ctx)

	// Save the flags:
	c.flags = cmd.Flags()

	// Create the network listener:
	c.logger.InfoContext(ctx, "Creating gateway listener")
	gwListener, err := network.NewListener().
		SetLogger(c.logger).
		SetFlags(c.flags, network.HttpListenerName).
		Build()
	if err != nil {
		return err
	}

	// Create the network client:
	c.logger.InfoContext(ctx, "Creating network client")
	grpcClient, err := network.NewClient().
		SetLogger(c.logger).
		SetFlags(c.flags, network.GrpcClientName).
		Build()
	if err != nil {
		return err
	}

	// Create the gateway multiplexer:
	c.logger.InfoContext(ctx, "Creating gateway server")
	gatewayMux := runtime.NewServeMux()

	// Register the service handlers:
	err = api.RegisterClusterTemplatesHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = api.RegisterClusterOrdersHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = api.RegisterClustersHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}

	// Add the CORS support:
	corsMiddleware, err := network.NewCorsMiddleware().
		SetLogger(c.logger).
		SetFlags(c.flags, network.HttpListenerName).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create CORS middleware: %w", err)
	}
	handler := corsMiddleware(gatewayMux)

	// Start serving:
	c.logger.InfoContext(
		ctx,
		"Start serving",
		slog.String("address", gwListener.Addr().String()),
	)
	http2Server := &http2.Server{}
	http1Server := &http.Server{
		Addr:    gwListener.Addr().String(),
		Handler: h2c.NewHandler(handler, http2Server),
	}
	return http1Server.Serve(gwListener)
}
