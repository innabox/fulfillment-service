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
	"encoding/json"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/innabox/fulfillment-service/internal"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/network"
)

// NewStartPublisherCommand creates and returns the `start server` command.
func NewStartPublisherCommand() *cobra.Command {
	runner := &startPublisherCommandRunner{}
	command := &cobra.Command{
		Use:   "publisher",
		Short: "Starts the change publisher",
		Args:  cobra.NoArgs,
		RunE:  runner.run,
	}
	flags := command.Flags()
	network.AddListenerFlags(flags, network.GrpcListenerName, network.DefaultGrpcAddress)
	database.AddFlags(flags)
	return command
}

// startPublisherCommandRunner contains the data and logic needed to run the `start server` command.
type startPublisherCommandRunner struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
}

// run runs the `start server` command.
func (c *startPublisherCommandRunner) run(cmd *cobra.Command, argv []string) error {
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

	// Create the change detector:
	c.logger.InfoContext(ctx, "Creating change queue")
	changeDetector, err := database.NewChangeDetector().
		SetLogger(c.logger).
		SetURL(dbTool.URL()).
		SetCallback(c.changeCallback).
		Build()
	if err != nil {
		return err
	}

	// Start the change detector:
	c.logger.InfoContext(ctx, "Starting the change detector")
	return changeDetector.Start(ctx)
}

func (c *startPublisherCommandRunner) changeCallback(ctx context.Context, change *database.Change) {
	bytes, err := json.Marshal(change)
	if err != nil {
		c.logger.ErrorContext(
			ctx,
			"Failed to marshal change",
			slog.String("error", err.Error()),
		)
		return
	}
	var data any
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		c.logger.ErrorContext(
			ctx,
			"Failed to unmarshal change",
			slog.String("error", err.Error()),
		)
		return
	}
	c.logger.InfoContext(
		ctx,
		"Change detected",
		slog.Any("change", data),
	)
}
