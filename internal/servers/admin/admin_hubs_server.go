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

type HubsServerBuilder struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	daos   dao.AdminSet
}

var _ adminv1.HubsServer = (*HubsServer)(nil)

type HubsServer struct {
	adminv1.UnimplementedHubsServer

	logger *slog.Logger
	daos   dao.AdminSet
}

func NewHubsServer() *HubsServerBuilder {
	return &HubsServerBuilder{}
}

func (b *HubsServerBuilder) SetLogger(value *slog.Logger) *HubsServerBuilder {
	b.logger = value
	return b
}

func (b *HubsServerBuilder) SetDAOs(value dao.AdminSet) *HubsServerBuilder {
	b.daos = value
	return b
}

func (b *HubsServerBuilder) SetFlags(value *pflag.FlagSet) *HubsServerBuilder {
	b.flags = value
	return b
}

func (b *HubsServerBuilder) Build() (result *HubsServer, err error) {
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
	result = &HubsServer{
		logger: b.logger,
		daos:   b.daos,
	}
	return
}

func (s *HubsServer) List(ctx context.Context,
	request *adminv1.HubsListRequest) (response *adminv1.HubsListResponse, err error) {
	items, err := s.daos.Hubs().List(ctx, dao.ListRequest{})
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to list hubs",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to list hubs")
		return
	}
	response = &adminv1.HubsListResponse{
		Items: items.Items,
	}
	return
}

func (s *HubsServer) Get(ctx context.Context,
	request *adminv1.HubsGetRequest) (response *adminv1.HubsGetResponse, err error) {
	object, err := s.daos.Hubs().Get(ctx, request.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get hub",
			slog.String("id", request.Id),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to get hub with identifier '%s'", request.Id)
		return
	}
	if object == nil {
		err = grpcstatus.Errorf(grpccodes.NotFound, "hub with id '%s' not found", request.Id)
		return
	}
	response = &adminv1.HubsGetResponse{
		Object: object,
	}
	return
}

func (s *HubsServer) Create(ctx context.Context,
	request *adminv1.HubsCreateRequest) (response *adminv1.HubsCreateResponse, err error) {
	if request.Object == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "hub is required")
		return
	}
	object := request.Object
	if object.Id == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "hub identifier is required")
		return
	}
	if object.Kubeconfig == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "kubeconfig is required")
		return
	}
	if object.Namespace == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "namespace is required")
		return
	}
	_, err = s.daos.Hubs().Insert(ctx, object)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to save hub",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to create hub")
		return
	}
	response = &adminv1.HubsCreateResponse{
		Object: object,
	}
	return
}

func (s *HubsServer) Delete(ctx context.Context,
	request *adminv1.HubsDeleteRequest) (response *adminv1.HubsDeleteResponse, err error) {
	exists, err := s.daos.Hubs().Exists(ctx, request.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to chec if hub exists",
			slog.String("id", request.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to check if hub with identifier '%s' exists",
			request.Id,
		)
		return
	}
	if !exists {
		err = grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"hub with identifier '%s' doesn't exist",
			request.Id,
		)
		return
	}
	err = s.daos.Hubs().Delete(ctx, request.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to delete hub",
			slog.String("id", request.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to delete hub with identifier '%s'",
			request.Id,
		)
		return
	}
	response = &adminv1.HubsDeleteResponse{}
	return
}

func (s *HubsServer) Update(ctx context.Context,
	request *adminv1.HubsUpdateRequest) (response *adminv1.HubsUpdateResponse, err error) {
	object := request.Object
	exists, err := s.daos.Hubs().Exists(ctx, object.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to check if hub exists",
			slog.String("id", object.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to check if hub with identifier '%s' exists",
			object.Id,
		)
		return
	}
	if !exists {
		err = grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"hub with identifier '%s' doesn't exist",
			object.Id,
		)
		return
	}
	err = s.daos.Hubs().Update(ctx, object.Id, object)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to update hub",
			slog.String("id", object.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to update hub with identifier '%s'",
			object.Id,
		)
		return
	}
	response = &adminv1.HubsUpdateResponse{
		Object: object,
	}
	return
}
