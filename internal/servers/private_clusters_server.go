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

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
)

type PrivateClustersServerBuilder struct {
	logger *slog.Logger
}

var _ privatev1.ClustersServer = (*PrivateClustersServer)(nil)

type PrivateClustersServer struct {
	privatev1.UnimplementedClustersServer
	logger  *slog.Logger
	generic *GenericServer[*privatev1.Cluster]
}

func NewPrivateClustersServer() *PrivateClustersServerBuilder {
	return &PrivateClustersServerBuilder{}
}

func (b *PrivateClustersServerBuilder) SetLogger(value *slog.Logger) *PrivateClustersServerBuilder {
	b.logger = value
	return b
}

func (b *PrivateClustersServerBuilder) Build() (result *PrivateClustersServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*privatev1.Cluster]().
		SetLogger(b.logger).
		SetService(privatev1.Clusters_ServiceDesc.ServiceName).
		SetTable("private.clusters").
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &PrivateClustersServer{
		logger:  b.logger,
		generic: generic,
	}
	return
}

func (s *PrivateClustersServer) List(ctx context.Context,
	request *privatev1.ClustersListRequest) (response *privatev1.ClustersListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *PrivateClustersServer) Get(ctx context.Context,
	request *privatev1.ClustersGetRequest) (response *privatev1.ClustersGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *PrivateClustersServer) Create(ctx context.Context,
	request *privatev1.ClustersCreateRequest) (response *privatev1.ClustersCreateResponse, err error) {
	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *PrivateClustersServer) Update(ctx context.Context,
	request *privatev1.ClustersUpdateRequest) (response *privatev1.ClustersUpdateResponse, err error) {
	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *PrivateClustersServer) Delete(ctx context.Context,
	request *privatev1.ClustersDeleteRequest) (response *privatev1.ClustersDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}
