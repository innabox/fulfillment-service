/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package clients

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/innabox/fulfillment-common/network"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	eventsv1 "github.com/innabox/fulfillment-service/internal/api/events/v1"
	api "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
)

// ClientBuilder contains the data and logic needed to create an object that simplifies use of the API for clients.
// Don't create instances of this directly, use the NewClient function instead.
type ClientBuilder struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
}

// Client simplifies use of the API for clients.
type Client struct {
	logger                 *slog.Logger
	grpcConn               *grpc.ClientConn
	clusterTemplatesClient api.ClusterTemplatesClient
	clustersClient         api.ClustersClient
	hubsClient             privatev1.HubsClient
	eventsClient           eventsv1.EventsClient
}

// NewClient creates a builder that can then be used to configure and create an API client.
func NewClient() *ClientBuilder {
	return &ClientBuilder{}
}

// SetLogger sets the logger. This is mandatory.
func (b *ClientBuilder) SetLogger(value *slog.Logger) *ClientBuilder {
	b.logger = value
	return b
}

// SetFlags sets the command line flags that should be used to configure the client.
//
// The name is used to select the options when there are multiple clients. For example, if it is 'API' then it will only
// take into accounts the flags starting with '--api'.
//
// This is optional.
func (b *ClientBuilder) SetFlags(flags *pflag.FlagSet, name string) *ClientBuilder {
	b.flags = flags
	return b
}

// Build uses the data stored in the buider to create a new API client.
func (b *ClientBuilder) Build() (result *Client, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the gRPC client:
	grpcConn, err := network.NewClient().
		SetLogger(b.logger).
		SetFlags(b.flags, network.GrpcClientName).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create gRPC client: %w", err)
		return
	}

	// Create the clients for specific services:
	clusterTemplatesClient := api.NewClusterTemplatesClient(grpcConn)
	clustersClient := api.NewClustersClient(grpcConn)
	managementClustersClient := privatev1.NewHubsClient(grpcConn)
	eventsClient := &eventsClient{
		delegate: eventsv1.NewEventsClient(grpcConn),
	}

	// Create and populate the object:
	result = &Client{
		logger:                 b.logger,
		grpcConn:               grpcConn,
		clusterTemplatesClient: clusterTemplatesClient,
		clustersClient:         clustersClient,
		eventsClient:           eventsClient,
		hubsClient:             managementClustersClient,
	}
	return
}

func (c *Client) ClusterTemplates() api.ClusterTemplatesClient {
	return c.clusterTemplatesClient
}

func (c *Client) Clusters() api.ClustersClient {
	return c.clustersClient
}

func (c *Client) Events() eventsv1.EventsClient {
	return c.eventsClient
}

func (c *Client) Hubs() privatev1.HubsClient {
	return c.hubsClient
}

func (c *Client) Close() error {
	if c.grpcConn != nil {
		return c.grpcConn.Close()
	}
	return nil
}

var _ eventsv1.EventsClient = (*eventsClient)(nil)

type eventsClient struct {
	delegate eventsv1.EventsClient
}

func (c *eventsClient) Watch(ctx context.Context, request *eventsv1.EventsWatchRequest,
	opts ...grpc.CallOption) (stream grpc.ServerStreamingClient[eventsv1.EventsWatchResponse], err error) {
	stream, err = c.delegate.Watch(ctx, request, opts...)
	return
}
