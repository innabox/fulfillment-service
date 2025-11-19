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

	"github.com/innabox/fulfillment-common/auth"
	"github.com/innabox/fulfillment-common/network"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/innabox/fulfillment-service/internal"
	"github.com/innabox/fulfillment-service/internal/bcm"
)

// NewStartBcmSyncCommand creates and returns the `start bcm-sync` command.
func NewStartBcmSyncCommand() *cobra.Command {
	runner := &startBcmSyncRunner{}
	command := &cobra.Command{
		Use:   "bcm-sync",
		Short: "Starts the inventory synchronizer",
		Args:  cobra.NoArgs,
		RunE:  runner.run,
	}
	flags := command.Flags()
	network.AddGrpcClientFlags(flags, network.GrpcClientName, network.DefaultGrpcAddress)
	flags.StringVar(
		&runner.args.bcmUrl,
		"bcm-url",
		"",
		"URL of the BCM API endpoint.",
	)
	flags.StringVar(
		&runner.args.bcmCertFile,
		"bcm-crt",
		"",
		"Path to the BCM client certificate file.",
	)
	flags.StringVar(
		&runner.args.bcmKeyFile,
		"bcm-key",
		"",
		"Path to the BCM client key file.",
	)
	flags.StringArrayVar(
		&runner.args.caFiles,
		"ca-file",
		[]string{},
		"File or directory containing trusted CA certificates.",
	)
	flags.DurationVar(
		&runner.args.syncInterval,
		"sync-interval",
		10*time.Second,
		"Interval between synchronization cycles.",
	)
	flags.StringVar(
		&runner.args.tokenFile,
		"token-file",
		"",
		"File containing the token to use for authentication.",
	)
	return command
}

// startBcmSyncRunner contains the data and logic needed to run the `start bcm-sync`
// command.
type startBcmSyncRunner struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	args   struct {
		bcmUrl       string
		bcmCertFile  string
		bcmKeyFile   string
		caFiles      []string
		syncInterval time.Duration
		tokenFile    string
	}
}

// run runs the `start bcm-sync` command.
func (r *startBcmSyncRunner) run(cmd *cobra.Command, argv []string) error {
	var err error

	// Get the context:
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Get the dependencies from the context:
	r.logger = internal.LoggerFromContext(ctx)

	// Save the flags:
	r.flags = cmd.Flags()

	// Validate required parameters:
	if r.args.bcmUrl == "" {
		return fmt.Errorf("--bcm-url is required")
	}
	if r.args.bcmCertFile == "" {
		return fmt.Errorf("--bcm-crt is required")
	}
	if r.args.bcmKeyFile == "" {
		return fmt.Errorf("--bcm-key is required")
	}

	// Load the trusted CA certificates:
	caPool, err := network.NewCertPool().
		SetLogger(r.logger).
		AddFiles(r.args.caFiles...).
		Build()
	if err != nil {
		return fmt.Errorf("failed to load trusted CA certificates: %w", err)
	}

	// Create the token source:
	tokenSource, err := auth.NewFileTokenSource().
		SetLogger(r.logger).
		SetFile(r.args.tokenFile).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create token source: %w", err)
	}

	// Create the BCM client:
	r.logger.InfoContext(ctx, "Creating BCM client")
	bcmClient, err := bcm.NewClient().
		SetLogger(r.logger).
		SetUrl(r.args.bcmUrl).
		SetCertFile(r.args.bcmCertFile).
		SetKeyFile(r.args.bcmKeyFile).
		SetCaPool(caPool).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create BCM client: %w", err)
	}

	// Create the gRPC client:
	r.logger.InfoContext(ctx, "Creating gRPC client")
	grpcClient, err := network.NewGrpcClient().
		SetLogger(r.logger).
		SetFlags(r.flags, network.GrpcClientName).
		SetCaPool(caPool).
		SetTokenSource(tokenSource).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Create the synchronizer:
	r.logger.InfoContext(ctx, "Creating synchronizer")
	synchronizer, err := bcm.NewSynchronizer().
		SetLogger(r.logger).
		SetBcmClient(bcmClient).
		SetGrpcClient(grpcClient).
		SetInterval(r.args.syncInterval).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create inventory synchronizer: %w", err)
	}

	// Set up signal handling:
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		r.logger.InfoContext(ctx, "Received interrupt signal, shutting down")
		cancel()
	}()

	// Run the synchronizer:
	r.logger.InfoContext(ctx, "Starting inventory synchronizer")
	err = synchronizer.Run(ctx)
	if err != nil && err != context.Canceled {
		return fmt.Errorf("synchronizer failed: %w", err)
	}

	r.logger.InfoContext(ctx, "Inventory synchronizer stopped")
	return nil
}
