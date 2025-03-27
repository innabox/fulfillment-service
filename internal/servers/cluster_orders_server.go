/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package servers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/spf13/pflag"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	api "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/innabox/fulfillment-service/internal/database/dao"
)

type ClusterOrdersServerBuilder struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	daos   dao.Set
}

var _ api.ClusterOrdersServer = (*ClusterOrdersServer)(nil)

type ClusterOrdersServer struct {
	api.UnimplementedClusterOrdersServer

	logger *slog.Logger
	daos   dao.Set
}

func NewClusterOrdersServer() *ClusterOrdersServerBuilder {
	return &ClusterOrdersServerBuilder{}
}

func (b *ClusterOrdersServerBuilder) SetLogger(value *slog.Logger) *ClusterOrdersServerBuilder {
	b.logger = value
	return b
}

func (b *ClusterOrdersServerBuilder) SetDAOs(value dao.Set) *ClusterOrdersServerBuilder {
	b.daos = value
	return b
}

func (b *ClusterOrdersServerBuilder) SetFlags(value *pflag.FlagSet) *ClusterOrdersServerBuilder {
	b.flags = value
	return b
}

func (b *ClusterOrdersServerBuilder) Build() (result *ClusterOrdersServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.daos == nil {
		err = errors.New("data access objects are mandatory")
		return
	}

	// Create and populate the object:
	result = &ClusterOrdersServer{
		logger: b.logger,
		daos:   b.daos,
	}
	return
}

func (s *ClusterOrdersServer) List(ctx context.Context,
	request *api.ClusterOrdersListRequest) (response *api.ClusterOrdersListResponse, err error) {
	orders, err := s.daos.ClusterOrders().List(ctx, dao.ListRequest{
		Offset: request.GetOffset(),
		Limit:  request.GetLimit(),
	})
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to list cluster orders",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to list cluster orders")
		return
	}
	response = &api.ClusterOrdersListResponse{
		Size:  proto.Int32(orders.Size),
		Total: proto.Int32(orders.Total),
		Items: orders.Items,
	}
	return
}

func (s *ClusterOrdersServer) Get(ctx context.Context,
	request *api.ClusterOrdersGetRequest) (response *api.ClusterOrdersGetResponse, err error) {
	order, err := s.daos.ClusterOrders().Get(ctx, request.OrderId)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster order",
			slog.String("order_id", request.OrderId),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to get cluster order with identifier '%s'",
			request.OrderId,
		)
		return
	}
	if order == nil {
		err = grpcstatus.Errorf(
			grpccodes.NotFound,
			"cluster order with identifier '%s' not found",
			request.OrderId,
		)
		return
	}
	response = &api.ClusterOrdersGetResponse{
		Order: order,
	}
	return
}

func (s *ClusterOrdersServer) Place(ctx context.Context,
	request *api.ClusterOrdersPlaceRequest) (response *api.ClusterOrdersPlaceResponse, err error) {
	// Validate the request:
	if request.Order == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "order is required")
		return
	}
	if request.Order.Id != "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "order identifier isn't allowed")
		return
	}
	if request.Order.Spec == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "order spec is required")
		return
	}
	if request.Order.Spec.TemplateId == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "template identifier is required")
		return
	}
	if request.Order.Status != nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "status isn't allowed")
		return
	}

	// Check that the requested template exists:
	templateId := request.Order.Spec.TemplateId
	template, err := s.daos.ClusterTemplates().Get(ctx, templateId)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster template",
			slog.String("template_id", templateId),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to get cluster template with identifier '%s'",
			templateId,
		)
		return
	}
	if template == nil {
		err = grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"cluster template with identifier '%s' doesn't exist",
			templateId,
		)
		return
	}

	// Insert the new order:
	order := &api.ClusterOrder{
		Spec: &api.ClusterOrderSpec{
			TemplateId: templateId,
		},
		Status: &api.ClusterOrderStatus{
			State: api.ClusterOrderState_CLUSTER_ORDER_STATE_ACCEPTED,
		},
	}
	_, err = s.daos.ClusterOrders().Insert(ctx, order)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to insert cluster order",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to create order")
		return
	}

	// Return the result:
	response = &api.ClusterOrdersPlaceResponse{
		Order: order,
	}
	return
}

func (s *ClusterOrdersServer) Cancel(ctx context.Context,
	request *api.ClusterOrdersCancelRequest) (response *api.ClusterOrdersCancelResponse, err error) {
	// Fetch the order:
	order, err := s.daos.ClusterOrders().Get(ctx, request.OrderId)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster order",
			slog.String("order_id", request.OrderId),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to get cluster order '%s'", request.OrderId)
		return
	}
	if order == nil {
		err = grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"cluster order with identifier '%s' doesn't exist",
			request.OrderId,
		)
		return
	}

	// Update the state:
	if order.Status == nil {
		order.Status = &api.ClusterOrderStatus{}
	}
	order.Status.State = api.ClusterOrderState_CLUSTER_ORDER_STATE_CANCELED
	err = s.daos.ClusterOrders().Update(ctx, order.Id, order)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to update cluster order state",
			slog.String("order_id", request.OrderId),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to update state for cluster order with identifier '%s'",
			request.OrderId,
		)
		return
	}
	response = &api.ClusterOrdersCancelResponse{}
	return
}
