/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package admin

import (
	"context"
	"errors"
	"log/slog"

	"github.com/spf13/pflag"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	adminv1 "github.com/innabox/fulfillment-service/internal/api/admin/v1"
	"github.com/innabox/fulfillment-service/internal/database/dao"
)

type ClusterOrdersServerBuilder struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	daos   dao.AdminSet
}

var _ adminv1.ClusterOrdersServer = (*ClusterOrdersServer)(nil)

type ClusterOrdersServer struct {
	adminv1.UnimplementedClusterOrdersServer

	logger *slog.Logger
	daos   dao.AdminSet
}

func NewClusterOrdersServer() *ClusterOrdersServerBuilder {
	return &ClusterOrdersServerBuilder{}
}

func (b *ClusterOrdersServerBuilder) SetLogger(value *slog.Logger) *ClusterOrdersServerBuilder {
	b.logger = value
	return b
}

func (b *ClusterOrdersServerBuilder) SetDAOs(value dao.AdminSet) *ClusterOrdersServerBuilder {
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

func (s *ClusterOrdersServer) Get(ctx context.Context,
	request *adminv1.ClusterOrdersGetRequest) (response *adminv1.ClusterOrdersGetResponse, err error) {
	object, err := s.daos.ClusterOrders().Get(ctx, request.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster order",
			slog.String("id", request.Id),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to get cluster order with identifier '%s'",
			request.Id,
		)
		return
	}
	if object == nil {
		err = grpcstatus.Errorf(
			grpccodes.NotFound,
			"cluster order with identifier '%s' not found",
			request.Id,
		)
		return
	}
	response = &adminv1.ClusterOrdersGetResponse{
		Object: object,
	}
	return
}

func (s *ClusterOrdersServer) Create(ctx context.Context,
	request *adminv1.ClusterOrdersCreateRequest) (response *adminv1.ClusterOrdersCreateResponse, err error) {
	if request.Object == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "cluster order is required")
		return
	}
	object := request.Object
	if object.Id == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "cluster order identifier is required")
		return
	}
	_, err = s.daos.ClusterOrders().Insert(ctx, object)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to save cluster order",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to create cluster order")
		return
	}
	response = &adminv1.ClusterOrdersCreateResponse{
		Object: object,
	}
	return
}

func (s *ClusterOrdersServer) Delete(ctx context.Context,
	request *adminv1.ClusterOrdersDeleteRequest) (response *adminv1.ClusterOrdersDeleteResponse, err error) {
	exists, err := s.daos.ClusterOrders().Exists(ctx, request.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to check if cluster order exists",
			slog.String("id", request.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to get if cluster order with identifier '%s'",
			request.Id,
		)
		return
	}
	if !exists {
		err = grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"cluster order with identifier '%s' doesn't exist",
			request.Id,
		)
		return
	}
	err = s.daos.ClusterOrders().Delete(ctx, request.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to delete cluster order",
			slog.String("id", request.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to delete cluster order with identifier '%s'",
			request.Id,
		)
		return
	}
	response = &adminv1.ClusterOrdersDeleteResponse{}
	return
}

func (s *ClusterOrdersServer) Update(ctx context.Context,
	request *adminv1.ClusterOrdersUpdateRequest) (response *adminv1.ClusterOrdersUpdateResponse, err error) {
	object := request.Object
	exists, err := s.daos.ClusterOrders().Exists(ctx, object.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get check if cluster order exists",
			slog.String("id", object.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to get check if cluster order with identifier '%s' exists",
			object.Id,
		)
		return
	}
	if !exists {
		err = grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"cluster order with identifier '%s' doesn't exist",
			object.Id,
		)
		return
	}
	err = s.daos.ClusterOrders().Update(ctx, object.Id, object)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to update cluster order",
			slog.String("id", object.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to update cluster order with identifier '%s'",
			object.Id,
		)
		return
	}
	response = &adminv1.ClusterOrdersUpdateResponse{
		Object: object,
	}
	return
}
