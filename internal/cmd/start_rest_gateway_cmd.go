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
	"github.com/innabox/fulfillment-common/network"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/innabox/fulfillment-service/internal"
	api "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privateapi "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// NewStartRestGatewayCommand creates and returns the `start rest-gateway` command.
func NewStartRestGatewayCommand() *cobra.Command {
	runner := &startRestGatewayCommandRunner{}
	command := &cobra.Command{
		Use:   "rest-gateway",
		Short: "Starts the REST gateway",
		Args:  cobra.NoArgs,
		RunE:  runner.run,
	}
	flags := command.Flags()
	network.AddListenerFlags(flags, network.HttpListenerName, network.DefaultHttpAddress)
	network.AddCorsFlags(flags, network.HttpListenerName)
	network.AddGrpcClientFlags(flags, network.GrpcClientName, network.DefaultGrpcAddress)
	flags.StringArrayVar(
		&runner.args.caFiles,
		"ca-file",
		[]string{},
		"File or directory containing trusted CA certificates.",
	)
	return command
}

// startRestGatewayCommandRunner contains the data and logic needed to run the `start rest-gateway` command.
type startRestGatewayCommandRunner struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	args   struct {
		caFiles   []string
		tokenFile string
	}
}

// run runs the `start rest-gateway` command.
func (c *startRestGatewayCommandRunner) run(cmd *cobra.Command, argv []string) error {
	// Get the context:
	ctx := cmd.Context()

	// Get the dependencies from the context:
	c.logger = internal.LoggerFromContext(ctx)

	// Save the flags:
	c.flags = cmd.Flags()

	// Create the network listener:
	c.logger.InfoContext(ctx, "Creating REST gateway listener")
	gwListener, err := network.NewListener().
		SetLogger(c.logger).
		SetFlags(c.flags, network.HttpListenerName).
		Build()
	if err != nil {
		return err
	}

	// Load the trusted CA certificates:
	caPool, err := network.NewCertPool().
		SetLogger(c.logger).
		AddFiles(c.args.caFiles...).
		Build()
	if err != nil {
		return fmt.Errorf("failed to load trusted CA certificates: %w", err)
	}

	// Create the gRPC client:
	c.logger.InfoContext(ctx, "Creating gRPC client")
	grpcClient, err := network.NewGrpcClient().
		SetLogger(c.logger).
		SetFlags(c.flags, network.GrpcClientName).
		SetCaPool(caPool).
		Build()
	if err != nil {
		return err
	}

	// Create the gateway multiplexer:
	c.logger.InfoContext(ctx, "Creating REST gateway server")
	gatewayMarshaller := &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames: true,
		},
	}
	gatewayMux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, gatewayMarshaller),
	)

	// Register the service handlers:
	err = api.RegisterClusterTemplatesHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = api.RegisterClustersHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = api.RegisterHostClassesHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = api.RegisterHostsHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = api.RegisterHostPoolsHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = api.RegisterVirtualMachineTemplatesHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = api.RegisterVirtualMachinesHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}

	// Register the private API service handlers:
	err = privateapi.RegisterClusterTemplatesHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = privateapi.RegisterClustersHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = privateapi.RegisterEventsHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = privateapi.RegisterHostClassesHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = privateapi.RegisterHostPoolsHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = privateapi.RegisterHostsHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = privateapi.RegisterHubsHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = privateapi.RegisterVirtualMachineTemplatesHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}
	err = privateapi.RegisterVirtualMachinesHandler(ctx, gatewayMux, grpcClient)
	if err != nil {
		return err
	}

	// Add the health endpoint:
	err = gatewayMux.HandlePath(
		http.MethodGet,
		"/healthz",
		func(w http.ResponseWriter, r *http.Request, p map[string]string) {
			w.WriteHeader(http.StatusOK)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register health endpoint: %w", err)
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
