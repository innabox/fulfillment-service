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

	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
)

type ClusterTemplatesServerBuilder struct {
	logger *slog.Logger
}

var _ ffv1.ClusterTemplatesServer = (*ClusterTemplatesServer)(nil)

type ClusterTemplatesServer struct {
	ffv1.UnimplementedClusterTemplatesServer

	logger  *slog.Logger
	generic *GenericServer[*ffv1.ClusterTemplate]
}

func NewClusterTemplatesServer() *ClusterTemplatesServerBuilder {
	return &ClusterTemplatesServerBuilder{}
}

func (b *ClusterTemplatesServerBuilder) SetLogger(value *slog.Logger) *ClusterTemplatesServerBuilder {
	b.logger = value
	return b
}

func (b *ClusterTemplatesServerBuilder) Build() (result *ClusterTemplatesServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*ffv1.ClusterTemplate]().
		SetLogger(b.logger).
		SetService(ffv1.ClusterTemplates_ServiceDesc.ServiceName).
		SetTable("cluster_templates").
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &ClusterTemplatesServer{
		logger:  b.logger,
		generic: generic,
	}
	return
}

func (s *ClusterTemplatesServer) List(ctx context.Context,
	request *ffv1.ClusterTemplatesListRequest) (response *ffv1.ClusterTemplatesListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *ClusterTemplatesServer) Get(ctx context.Context,
	request *ffv1.ClusterTemplatesGetRequest) (response *ffv1.ClusterTemplatesGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *ClusterTemplatesServer) Create(ctx context.Context,
	request *ffv1.ClusterTemplatesCreateRequest) (response *ffv1.ClusterTemplatesCreateResponse, err error) {
	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *ClusterTemplatesServer) Update(ctx context.Context,
	request *ffv1.ClusterTemplatesUpdateRequest) (response *ffv1.ClusterTemplatesUpdateResponse, err error) {
	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *ClusterTemplatesServer) Delete(ctx context.Context,
	request *ffv1.ClusterTemplatesDeleteRequest) (response *ffv1.ClusterTemplatesDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}
