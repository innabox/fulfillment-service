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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/klog/v2"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/innabox/fulfillment-service/internal"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	"github.com/innabox/fulfillment-service/internal/controllers/cluster"
	"github.com/innabox/fulfillment-service/internal/controllers/host"
	"github.com/innabox/fulfillment-service/internal/controllers/hostpool"
	"github.com/innabox/fulfillment-service/internal/controllers/vm"
	"github.com/innabox/fulfillment-service/internal/network"
	"google.golang.org/grpc"
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
	client *grpc.ClientConn
}

// run runs the `start controllers` command.
func (r *startControllerRunner) run(cmd *cobra.Command, argv []string) error {
	var err error

	// Get the context:
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Get the dependencies from the context:
	r.logger = internal.LoggerFromContext(ctx)

	// Configure the Kubernetes libraries to use the logger:
	logrLogger := logr.FromSlogHandler(r.logger.Handler())
	crlog.SetLogger(logrLogger)
	klog.SetLogger(logrLogger)

	// Save the flags:
	r.flags = cmd.Flags()

	// Create the gRPC client:
	r.client, err = network.NewClient().
		SetLogger(r.logger).
		SetFlags(r.flags, network.GrpcClientName).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Wait for the server to be ready:
	err = r.waitForServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for server: %w", err)
	}

	// Create the hub cache:
	r.logger.InfoContext(ctx, "Creating hub cache")
	hubCache, err := controllers.NewHubCache().
		SetLogger(r.logger).
		SetConnection(r.client).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create hub cache: %w", err)
	}

	// Create the cluster reconciler:
	r.logger.InfoContext(ctx, "Creating cluster reconciler")
	clusterReconcilerFunction, err := cluster.NewFunction().
		SetLogger(r.logger).
		SetConnection(r.client).
		SetHubCache(hubCache).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create cluster reconciler function: %w", err)
	}
	clusterReconciler, err := controllers.NewReconciler[*privatev1.Cluster]().
		SetLogger(r.logger).
		SetClient(r.client).
		SetFunction(clusterReconcilerFunction).
		SetEventFilter("has(event.cluster) || (has(event.hub) && event.type == EVENT_TYPE_OBJECT_CREATED)").
		Build()
	if err != nil {
		return fmt.Errorf("failed to create cluster reconciler: %w", err)
	}

	// Start the cluster reconciler:
	r.logger.InfoContext(ctx, "Starting cluster reconciler")
	go func() {
		err := clusterReconciler.Start(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			r.logger.InfoContext(ctx, "Cluster reconciler finished")
		} else {
			r.logger.InfoContext(
				ctx,
				"Cluster reconciler failed",
				slog.Any("error", err),
			)
		}
	}()

	// Create the virtual machine reconciler:
	r.logger.InfoContext(ctx, "Creating virtual machine reconciler")
	vmReconcilerFunction, err := vm.NewFunction().
		SetLogger(r.logger).
		SetConnection(r.client).
		SetHubCache(hubCache).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create virtual machine reconciler function: %w", err)
	}
	vmReconciler, err := controllers.NewReconciler[*privatev1.VirtualMachine]().
		SetLogger(r.logger).
		SetClient(r.client).
		SetFunction(vmReconcilerFunction).
		SetEventFilter("has(event.virtual_machine) || (has(event.hub) && event.type == EVENT_TYPE_OBJECT_CREATED)").
		Build()
	if err != nil {
		return fmt.Errorf("failed to create virtual machine reconciler: %w", err)
	}

	// Start the virtual machine reconciler:
	r.logger.InfoContext(ctx, "Starting virtual machine reconciler")
	go func() {
		err := vmReconciler.Start(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			r.logger.InfoContext(ctx, "Virtual machine reconciler finished")
		} else {
			r.logger.InfoContext(
				ctx,
				"Virtual machine reconciler failed",
				slog.Any("error", err),
			)
		}
	}()

	// Create the host reconciler:
	r.logger.InfoContext(ctx, "Creating host reconciler")
	hostReconcilerFunction, err := host.NewFunction().
		SetLogger(r.logger).
		SetConnection(r.client).
		SetHubCache(hubCache).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create host reconciler function: %w", err)
	}
	hostReconciler, err := controllers.NewReconciler[*privatev1.Host]().
		SetLogger(r.logger).
		SetClient(r.client).
		SetFunction(hostReconcilerFunction).
		SetEventFilter("has(event.host) || (has(event.hub) && event.type == EVENT_TYPE_OBJECT_CREATED)").
		Build()
	if err != nil {
		return fmt.Errorf("failed to create host reconciler: %w", err)
	}

	// Start the host reconciler:
	r.logger.InfoContext(ctx, "Starting host reconciler")
	go func() {
		err := hostReconciler.Start(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			r.logger.InfoContext(ctx, "Host reconciler finished")
		} else {
			r.logger.InfoContext(
				ctx,
				"Host reconciler failed",
				slog.Any("error", err),
			)
		}
	}()

	// Create the host pool reconciler:
	r.logger.InfoContext(ctx, "Creating host pool reconciler")
	hostPoolReconcilerFunction, err := hostpool.NewFunction().
		SetLogger(r.logger).
		SetConnection(r.client).
		SetHubCache(hubCache).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create host pool reconciler function: %w", err)
	}
	hostPoolReconciler, err := controllers.NewReconciler[*privatev1.HostPool]().
		SetLogger(r.logger).
		SetClient(r.client).
		SetFunction(hostPoolReconcilerFunction).
		SetEventFilter("has(event.host_pool) || (has(event.hub) && event.type == EVENT_TYPE_OBJECT_CREATED)").
		Build()
	if err != nil {
		return fmt.Errorf("failed to create host pool reconciler: %w", err)
	}

	// Start the host pool reconciler:
	r.logger.InfoContext(ctx, "Starting host pool reconciler")
	go func() {
		err := hostPoolReconciler.Start(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			r.logger.InfoContext(ctx, "Host pool reconciler finished")
		} else {
			r.logger.InfoContext(
				ctx,
				"Host pool reconciler failed",
				slog.Any("error", err),
			)
		}
	}()

	// Wait for a signal:
	r.logger.InfoContext(ctx, "Waiting for signal")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	r.logger.InfoContext(ctx, "Signal received, shutting down")
	return nil
}

// waitForServer waits for the server to be ready using the health service.
func (r *startControllerRunner) waitForServer(ctx context.Context) error {
	r.logger.InfoContext(ctx, "Waiting for server")
	client := healthv1.NewHealthClient(r.client)
	request := &healthv1.HealthCheckRequest{}
	const max = time.Minute
	const interval = time.Second
	start := time.Now()
	for {
		response, err := client.Check(ctx, request)
		if err == nil && response.Status == healthv1.HealthCheckResponse_SERVING {
			r.logger.InfoContext(ctx, "Server is ready")
			return nil
		}
		if time.Since(start) >= max {
			return fmt.Errorf("server did not become ready after waiting for %s: %w", max, err)
		}
		r.logger.InfoContext(
			ctx,
			"Server not yet ready",
			slog.Duration("elapsed", time.Since(start)),
			slog.Any("error", err),
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
