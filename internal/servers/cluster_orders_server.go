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

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/database/dao"
)

type ClusterOrdersServerBuilder struct {
	logger *slog.Logger
}

var _ ffv1.ClusterOrdersServer = (*ClusterOrdersServer)(nil)

type ClusterOrdersServer struct {
	ffv1.UnimplementedClusterOrdersServer
	logger              *slog.Logger
	clusterTemplatesDao *dao.GenericDAO[*ffv1.ClusterTemplate]
	genericServer       *GenericServer[*ffv1.ClusterOrder, *privatev1.ClusterOrder]
}

func NewClusterOrdersServer() *ClusterOrdersServerBuilder {
	return &ClusterOrdersServerBuilder{}
}

func (b *ClusterOrdersServerBuilder) SetLogger(value *slog.Logger) *ClusterOrdersServerBuilder {
	b.logger = value
	return b
}

func (b *ClusterOrdersServerBuilder) Build() (result *ClusterOrdersServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the required DAOs:
	clusterTemplatesDao, err := dao.NewGenericDAO[*ffv1.ClusterTemplate]().
		SetLogger(b.logger).
		SetTable("cluster_templates").
		Build()
	if err != nil {
		return
	}

	// Create the genericServer server:
	genericServer, err := NewGenericServer[*ffv1.ClusterOrder, *privatev1.ClusterOrder]().
		SetLogger(b.logger).
		SetService(ffv1.ClusterOrders_ServiceDesc.ServiceName).
		SetTable("cluster_orders").
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &ClusterOrdersServer{
		logger:              b.logger,
		clusterTemplatesDao: clusterTemplatesDao,
		genericServer:       genericServer,
	}
	return
}

func (s *ClusterOrdersServer) List(ctx context.Context,
	request *ffv1.ClusterOrdersListRequest) (response *ffv1.ClusterOrdersListResponse, err error) {
	err = s.genericServer.List(ctx, request, &response)
	return
}

func (s *ClusterOrdersServer) Get(ctx context.Context,
	request *ffv1.ClusterOrdersGetRequest) (response *ffv1.ClusterOrdersGetResponse, err error) {
	err = s.genericServer.Get(ctx, request, &response)
	return
}

func (s *ClusterOrdersServer) Create(ctx context.Context,
	request *ffv1.ClusterOrdersCreateRequest) (response *ffv1.ClusterOrdersCreateResponse, err error) {
	err = s.validateId(ctx, request.Object)
	if err != nil {
		return
	}
	err = s.validateTemplateId(ctx, request.Object)
	if err != nil {
		return
	}
	err = s.genericServer.Create(ctx, request, &response)
	return
}

func (s *ClusterOrdersServer) Update(ctx context.Context,
	request *ffv1.ClusterOrdersUpdateRequest) (response *ffv1.ClusterOrdersUpdateResponse, err error) {
	err = s.validateTemplateId(ctx, request.Object)
	if err != nil {
		return
	}
	err = s.genericServer.Update(ctx, request, &response)
	return
}

func (s *ClusterOrdersServer) Delete(ctx context.Context,
	request *ffv1.ClusterOrdersDeleteRequest) (response *ffv1.ClusterOrdersDeleteResponse, err error) {
	err = s.genericServer.Delete(ctx, request, &response)
	return
}

func (s *ClusterOrdersServer) validateId(ctx context.Context, object *ffv1.ClusterOrder) error {
	if object == nil {
		return grpcstatus.Error(
			grpccodes.InvalidArgument,
			"object is mandatory",
		)
	}
	id := object.Id
	if id != "" {
		return grpcstatus.Error(
			grpccodes.InvalidArgument,
			"identifier isn't allowed",
		)
	}
	return nil
}

func (s *ClusterOrdersServer) validateTemplateId(ctx context.Context, object *ffv1.ClusterOrder) error {
	if object == nil {
		return grpcstatus.Error(
			grpccodes.InvalidArgument,
			"object is mandatory",
		)
	}
	spec := object.Spec
	if spec == nil {
		return grpcstatus.Error(
			grpccodes.InvalidArgument,
			"spec is mandatory",
		)
	}
	id := spec.TemplateId
	if id == "" {
		return grpcstatus.Error(
			grpccodes.InvalidArgument,
			"cluster template identifier is mandatory",
		)
	}
	exists, err := s.clusterTemplatesDao.Exists(ctx, id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to check if cluster template exists",
			slog.Any("id", id),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to check if template with identifier '%s' exists",
			id,
		)
	}
	if !exists {
		return grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"cluster template with identifier '%s' doesn't exist",
			object.Spec.TemplateId,
		)
	}
	return nil
}
