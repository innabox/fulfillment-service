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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/innabox/fulfillment-service/internal"
	api "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/innabox/fulfillment-service/internal/network"
)

// NewExampleCommand creates and returns the `create` command.
func NewExampleCommand() *cobra.Command {
	runner := &createCommandRunner{}
	command := &cobra.Command{
		Use:   "example",
		Short: "Example",
		RunE:  runner.run,
	}
	flags := command.Flags()
	network.AddGrpcClientFlags(flags, network.GrpcClientName, network.DefaultGrpcAddress)
	return command
}

// createCommandRunner contains the data and logic needed to run the `create` command.
type createCommandRunner struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	client api.ClusterTemplatesClient
}

// run runs the `create` command.
func (c *createCommandRunner) run(cmd *cobra.Command, argv []string) error {
	// Get the context:
	ctx := cmd.Context()

	// Get the dependencies from the context:
	c.logger = internal.LoggerFromContext(ctx)

	// Save the flags:
	c.flags = cmd.Flags()

	// Create the gRPC client:
	grpcClient, err := network.NewClient().
		SetLogger(c.logger).
		SetFlags(c.flags, network.GrpcClientName).
		Build()
	if err != nil {
		return err
	}

	// Create the client:
	c.client = api.NewClusterTemplatesClient(grpcClient)

	// Get the list of templates:
	response, err := c.client.List(ctx, &api.ClusterTemplatesListRequest{})
	if err != nil {
		return err
	}
	items := response.Items
	for _, item := range items {
		fmt.Printf("%s - %s - %s\n", item.Id, item.Title, item.Description)
	}

	// Get all the templates individually:
	for _, item := range items {
		response, err := c.client.Get(ctx, &api.ClusterTemplatesGetRequest{
			TemplateId: item.Id,
		})
		if err != nil {
			return err
		}
		fmt.Printf("%s - %s - %s\n", response.Template.Id, response.Template.Title, response.Template.Description)
	}

	return nil
}
