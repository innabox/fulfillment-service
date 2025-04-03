/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package dev

import (
	"log/slog"
	"math/rand/v2"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/proto"

	"github.com/innabox/fulfillment-service/internal"
	eventsv1 "github.com/innabox/fulfillment-service/internal/api/events/v1"
	fulfillmentv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	apiclient "github.com/innabox/fulfillment-service/internal/clients/api"
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
}

// run runs the `create` command.
func (c *createCommandRunner) run(cmd *cobra.Command, argv []string) error {
	// Get the context:
	ctx := cmd.Context()

	// Get the dependencies from the context:
	c.logger = internal.LoggerFromContext(ctx)

	// Save the flags:
	c.flags = cmd.Flags()

	// Create the client:
	client, err := apiclient.NewClient().
		SetLogger(c.logger).
		SetFlags(c.flags, network.GrpcClientName).
		Build()
	if err != nil {
		return err
	}
	defer func() {
		err := client.Close()
		if err != nil {
			c.logger.ErrorContext(
				ctx,
				"Failed to close client",
				slog.Any("error", err),
			)
		}
	}()

	// Get the list of templates:
	listTemplatesResponse, err := client.ClusterTemplates().List(ctx, &fulfillmentv1.ClusterTemplatesListRequest{})
	if err != nil {
		return err
	}
	templates := listTemplatesResponse.Items
	if len(templates) == 0 {
		return errors.New("there re no templates")
	}

	// Select a template randomly:
	template := templates[rand.IntN(len(templates))]
	c.logger.InfoContext(
		ctx,
		"Selected template",
		slog.String("id", template.Id),
		slog.String("title", template.Title),
	)

	// Place the order:
	order := &fulfillmentv1.ClusterOrder{
		Spec: &fulfillmentv1.ClusterOrderSpec{
			TemplateId: template.Id,
		},
	}
	placeOrderResponse, err := client.ClusterOrders().Create(ctx, &fulfillmentv1.ClusterOrdersCreateRequest{
		Object: order,
	})
	if err != nil {
		return err
	}
	order = placeOrderResponse.Object
	c.logger.InfoContext(
		ctx,
		"Placed order",
		slog.Any("order", order),
	)

	// Wait for the order to be fulfilled:
	stream, err := client.Events().Watch(ctx, &eventsv1.EventsWatchRequest{
		Filter: proto.String(`
			event.type == EVENT_TYPE_OBJECT_UPDATED &&
			event.cluster_order.status.state == CLUSTER_ORDER_STATE_FULFILLED
		`),
	})
	if err != nil {
		return err
	}
	response, err := stream.Recv()
	if err != nil {
		return err
	}
	order = response.Event.GetClusterOrder()
	c.logger.InfoContext(
		ctx,
		"Order is fulfilled",
		slog.Any("order", order),
	)

	// Get the admin Kubeconfig of the cluster:
	getKubeconfigResponse, err := client.Clusters().GetKubeconfig(ctx, &fulfillmentv1.ClustersGetKubeconfigRequest{
		Id: order.Status.ClusterId,
	})
	if err != nil {
		return err
	}
	kubeconfig := getKubeconfigResponse.Kubeconfig
	c.logger.InfoContext(
		ctx,
		"Got admin Kubeconfig",
		slog.String("length", len(kubeconfig)),
	)

	return nil
}
