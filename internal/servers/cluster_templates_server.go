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
	"github.com/innabox/fulfillment-service/internal/database/models"
)

type ClusterTemplatesServerBuilder struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	daos   dao.Set
}

var _ api.ClusterTemplatesServer = (*ClusterTemplatesServer)(nil)

type ClusterTemplatesServer struct {
	api.UnimplementedClusterTemplatesServer

	logger *slog.Logger
	daos   dao.Set
}

func NewClusterTemplatesServer() *ClusterTemplatesServerBuilder {
	return &ClusterTemplatesServerBuilder{}
}

func (b *ClusterTemplatesServerBuilder) SetLogger(value *slog.Logger) *ClusterTemplatesServerBuilder {
	b.logger = value
	return b
}

func (b *ClusterTemplatesServerBuilder) SetDAOs(value dao.Set) *ClusterTemplatesServerBuilder {
	b.daos = value
	return b
}

func (b *ClusterTemplatesServerBuilder) SetFlags(value *pflag.FlagSet) *ClusterTemplatesServerBuilder {
	b.flags = value
	return b
}

func (b *ClusterTemplatesServerBuilder) Build() (result *ClusterTemplatesServer, err error) {
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
	result = &ClusterTemplatesServer{
		logger: b.logger,
		daos:   b.daos,
	}
	return
}

func (s *ClusterTemplatesServer) List(ctx context.Context,
	request *api.ClusterTemplatesListRequest) (response *api.ClusterTemplatesListResponse, err error) {
	templates, err := s.daos.ClusterTemplates().List(ctx)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to list cluster templates",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to list cluster templates")
		return
	}
	results := make([]*api.ClusterTemplate, len(templates))
	for i, template := range templates {
		results[i] = &api.ClusterTemplate{}
		err = s.mapOutbound(template, results[i])
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to map outbound cluster template",
				slog.Any("error", err),
			)
			return
		}
	}
	response = &api.ClusterTemplatesListResponse{
		Size:  proto.Int32(int32(len(results))),
		Total: proto.Int32(int32(len(results))),
		Items: results,
	}
	return
}

func (s *ClusterTemplatesServer) Get(ctx context.Context,
	request *api.ClusterTemplatesGetRequest) (response *api.ClusterTemplatesGetResponse, err error) {
	template, err := s.daos.ClusterTemplates().Get(ctx, request.TemplateId)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster template",
			slog.String("template_id", request.TemplateId),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to get cluster template with identifier '%s'",
			request.TemplateId,
		)
		return
	}
	if template == nil {
		err = grpcstatus.Errorf(
			grpccodes.NotFound,
			"cluster template with identifier '%s' not found",
			request.TemplateId,
		)
		return
	}
	result := &api.ClusterTemplate{}
	err = s.mapOutbound(template, result)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map outbound cluster template",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to map outbound template")
		return
	}
	response = &api.ClusterTemplatesGetResponse{
		Template: result,
	}
	return
}

func (s *ClusterTemplatesServer) mapOutbound(from *models.ClusterTemplate, to *api.ClusterTemplate) error {
	to.Id = from.ID
	to.Title = from.Title
	to.Description = from.Description
	return nil
}
