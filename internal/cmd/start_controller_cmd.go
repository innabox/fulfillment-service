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
	"io"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/innabox/fulfillment-service/internal"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	"github.com/innabox/fulfillment-service/internal/controllers/cluster"
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
	runner.addVMaaSFlags(flags)
	return command
}

// startControllerRunner contains the data and logic needed to run the `start controllers` command.
type startControllerRunner struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	client *grpc.ClientConn
}

// addVMaaSFlags adds VMaaS-specific command line flags.
func (r *startControllerRunner) addVMaaSFlags(flags *pflag.FlagSet) {
	flags.String(
		"vmaas-hub-kubeconfig",
		"",
		"Path to the kubeconfig file for the VMaaS hub cluster. If specified, this hub will be used for all virtual machine requests.",
	)
	flags.String(
		"vmaas-hub-namespace", 
		"vmaas-system",
		"Namespace in the VMaaS hub where VirtualMachine CRDs will be created.",
	)
	flags.String(
		"vmaas-hub-id",
		"vmaas-primary",
		"Identifier for the VMaaS hub. This will be used as the hub ID when registering the VMaaS hub.",
	)
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

	// Register VMaaS hub if configured:
	err = r.registerVMaaSHub(ctx)
	if err != nil {
		return fmt.Errorf("failed to register VMaaS hub: %w", err)
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

// registerVMaaSHub registers a VMaaS hub in the database if VMaaS configuration is provided.
func (r *startControllerRunner) registerVMaaSHub(ctx context.Context) error {
	// Get VMaaS configuration from flags
	vmaasHubKubeconfig, _ := r.flags.GetString("vmaas-hub-kubeconfig")
	vmaasHubNamespace, _ := r.flags.GetString("vmaas-hub-namespace")
	vmaasHubID, _ := r.flags.GetString("vmaas-hub-id")

	// If no VMaaS kubeconfig is provided, skip registration
	if vmaasHubKubeconfig == "" {
		r.logger.InfoContext(ctx, "No VMaaS hub kubeconfig provided, skipping VMaaS hub registration")
		return nil
	}

	r.logger.InfoContext(ctx, "Registering VMaaS hub",
		slog.String("hub_id", vmaasHubID),
		slog.String("namespace", vmaasHubNamespace),
		slog.String("kubeconfig", vmaasHubKubeconfig),
	)

	// Read the kubeconfig file
	kubeconfigFile, err := os.Open(vmaasHubKubeconfig)
	if err != nil {
		return fmt.Errorf("failed to open VMaaS kubeconfig file %s: %w", vmaasHubKubeconfig, err)
	}
	defer kubeconfigFile.Close()

	kubeconfigBytes, err := io.ReadAll(kubeconfigFile)
	if err != nil {
		return fmt.Errorf("failed to read VMaaS kubeconfig file %s: %w", vmaasHubKubeconfig, err)
	}

	// Validate and ensure the namespace exists in the VMaaS hub cluster
	err = r.ensureVMaaSNamespace(ctx, kubeconfigBytes, vmaasHubNamespace)
	if err != nil {
		return fmt.Errorf("failed to ensure VMaaS namespace %s exists: %w", vmaasHubNamespace, err)
	}

	// Create the hub object
	hub := privatev1.Hub_builder{
		Id:        vmaasHubID,
		Metadata:  &privatev1.Metadata{},
		Kubeconfig: kubeconfigBytes,
		Namespace: vmaasHubNamespace,
		Capabilities: []string{"virtualmachines"},
		HubType:   "vmaas",
	}.Build()

	// Create or update the hub via the Hubs API
	hubsClient := privatev1.NewHubsClient(r.client)
	
	// Try to get the existing hub first
	getResponse, err := hubsClient.Get(ctx, privatev1.HubsGetRequest_builder{Id: vmaasHubID}.Build())
	if err != nil {
		// Hub doesn't exist, create it
		r.logger.InfoContext(ctx, "Creating new VMaaS hub", slog.String("id", vmaasHubID))
		_, err = hubsClient.Create(ctx, privatev1.HubsCreateRequest_builder{
			Object: hub,
		}.Build())
		if err != nil {
			return fmt.Errorf("failed to create VMaaS hub: %w", err)
		}
		r.logger.InfoContext(ctx, "Successfully created VMaaS hub", slog.String("id", vmaasHubID))
	} else {
		// Hub exists, update it
		existingHub := getResponse.GetObject()
		existingHub.SetKubeconfig(kubeconfigBytes)
		existingHub.SetNamespace(vmaasHubNamespace)
		existingHub.SetCapabilities([]string{"virtualmachines"})
		existingHub.SetHubType("vmaas")
		
		r.logger.InfoContext(ctx, "Updating existing VMaaS hub", slog.String("id", vmaasHubID))
		_, err = hubsClient.Update(ctx, privatev1.HubsUpdateRequest_builder{
			Object: existingHub,
		}.Build())
		if err != nil {
			return fmt.Errorf("failed to update VMaaS hub: %w", err)
		}
		r.logger.InfoContext(ctx, "Successfully updated VMaaS hub", slog.String("id", vmaasHubID))
	}

	return nil
}

// ensureVMaaSNamespace validates that the specified namespace exists in the VMaaS hub cluster,
// and creates it if it doesn't exist.
func (r *startControllerRunner) ensureVMaaSNamespace(ctx context.Context, kubeconfigBytes []byte, namespace string) error {
	// Create a REST config from the kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
	if err != nil {
		return fmt.Errorf("failed to create REST config from kubeconfig: %w", err)
	}

	// Create a Kubernetes client
	client, err := clnt.New(config, clnt.Options{})
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	r.logger.InfoContext(ctx, "Checking if VMaaS namespace exists", slog.String("namespace", namespace))

	// Check if the namespace exists
	ns := &corev1.Namespace{}
	err = client.Get(ctx, clnt.ObjectKey{Name: namespace}, ns)
	if err == nil {
		r.logger.InfoContext(ctx, "VMaaS namespace already exists", slog.String("namespace", namespace))
		return nil
	}

	// If the error is not "not found", return it
	if !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to check if namespace %s exists: %w", namespace, err)
	}

	// Namespace doesn't exist, create it
	r.logger.InfoContext(ctx, "Creating VMaaS namespace", slog.String("namespace", namespace))
	newNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"created-by": "fulfillment-controller",
				"purpose":    "vmaas-hub",
			},
		},
	}

	err = client.Create(ctx, newNamespace)
	if err != nil {
		// Check if the namespace was created by another process concurrently
		if k8serrors.IsAlreadyExists(err) {
			r.logger.InfoContext(ctx, "VMaaS namespace was created concurrently", slog.String("namespace", namespace))
			return nil
		}
		return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
	}

	r.logger.InfoContext(ctx, "Successfully created VMaaS namespace", slog.String("namespace", namespace))
	return nil
}
