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
	"os"
	"os/signal"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/innabox/fulfillment-service/internal"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	"github.com/innabox/fulfillment-service/internal/controllers/cluster"
	"github.com/innabox/fulfillment-service/internal/network"
)

// NewStartControllerCommand creates and returns the `start controllers` command.
func NewStartControllerCommand() *cobra.Command {
	runner := &startControllerRunner{}
	command := &cobra.Command{
		Use:   "controller",
		Short: "Starts the controller",
		Args:  cobra.NoArgs,
		RunE:  runner.run,
	}
	flags := command.Flags()
	network.AddGrpcClientFlags(flags, network.GrpcClientName, network.DefaultGrpcAddress)
	return command
}

// startControllerRunner contains the data and logic needed to run the `start controllers` command.
type startControllerRunner struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
}

// run runs the `start controllers` command.
func (c *startControllerRunner) run(cmd *cobra.Command, argv []string) error {
	// Get the context:
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Get the dependencies from the context:
	c.logger = internal.LoggerFromContext(ctx)

	// Configure the Kubernetes libraries to use the logger:
	logrLogger := logr.FromSlogHandler(c.logger.Handler())
	crlog.SetLogger(logrLogger)
	klog.SetLogger(logrLogger)

	// Save the flags:
	c.flags = cmd.Flags()

	// Create the gRPC client:
	client, err := network.NewClient().
		SetLogger(c.logger).
		SetFlags(c.flags, network.GrpcClientName).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Create the hub cache:
	c.logger.InfoContext(ctx, "Creating hub cache")
	hubCache, err := controllers.NewHubCache().
		SetLogger(c.logger).
		SetConnection(client).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create hub cache: %w", err)
	}

	// Create the cluster reconciler:
	c.logger.InfoContext(ctx, "Creating cluster reconciler")
	clusterReconcilerFunction, err := cluster.NewFunction().
		SetLogger(c.logger).
		SetConnection(client).
		SetHubCache(hubCache).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create cluster reconciler function: %w", err)
	}
	clusterReconciler, err := controllers.NewReconciler[*privatev1.Cluster]().
		SetLogger(c.logger).
		SetClient(client).
		SetFunction(clusterReconcilerFunction).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create cluster reconciler: %w", err)
	}

	// Start the cluster reconciler:
	c.logger.InfoContext(ctx, "Starting cluster reconciler")
	go func() {
		err := clusterReconciler.Start(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			c.logger.InfoContext(ctx, "Cluster reconciler finished")
		} else {
			c.logger.InfoContext(
				ctx,
				"Cluster reconciler failed",
				slog.Any("error", err),
			)
		}
	}()

	// Wait for a signal:
	c.logger.InfoContext(ctx, "Waiting for signal")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	c.logger.InfoContext(ctx, "Signal received, shutting down")
	return nil
}
