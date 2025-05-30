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

type PrivateClusterOrdersServerBuilder struct {
	logger *slog.Logger
}

var _ privatev1.ClusterOrdersServer = (*PrivateClusterOrdersServer)(nil)

type PrivateClusterOrdersServer struct {
	privatev1.UnimplementedClusterOrdersServer
	logger  *slog.Logger
	generic *GenericServer[*privatev1.ClusterOrder, *privatev1.ClusterOrder]
}

func NewPrivateClusterOrdersServer() *PrivateClusterOrdersServerBuilder {
	return &PrivateClusterOrdersServerBuilder{}
}

func (b *PrivateClusterOrdersServerBuilder) SetLogger(value *slog.Logger) *PrivateClusterOrdersServerBuilder {
	b.logger = value
	return b
}

func (b *PrivateClusterOrdersServerBuilder) Build() (result *PrivateClusterOrdersServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*privatev1.ClusterOrder, *privatev1.ClusterOrder]().
		SetLogger(b.logger).
		SetService(privatev1.ClusterOrders_ServiceDesc.ServiceName).
		SetTable("private.cluster_orders").
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &PrivateClusterOrdersServer{
		logger:  b.logger,
		generic: generic,
	}
	return
}

func (s *PrivateClusterOrdersServer) List(ctx context.Context,
	request *privatev1.ClusterOrdersListRequest) (response *privatev1.ClusterOrdersListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *PrivateClusterOrdersServer) Get(ctx context.Context,
	request *privatev1.ClusterOrdersGetRequest) (response *privatev1.ClusterOrdersGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *PrivateClusterOrdersServer) Create(ctx context.Context,
	request *privatev1.ClusterOrdersCreateRequest) (response *privatev1.ClusterOrdersCreateResponse, err error) {
	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *PrivateClusterOrdersServer) Update(ctx context.Context,
	request *privatev1.ClusterOrdersUpdateRequest) (response *privatev1.ClusterOrdersUpdateResponse, err error) {
	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *PrivateClusterOrdersServer) Delete(ctx context.Context,
	request *privatev1.ClusterOrdersDeleteRequest) (response *privatev1.ClusterOrdersDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}
