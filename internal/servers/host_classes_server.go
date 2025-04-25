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
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
)

type HostClassesServerBuilder struct {
	logger *slog.Logger
}

var _ ffv1.HostClassesServer = (*HostClassesServer)(nil)

type HostClassesServer struct {
	ffv1.UnimplementedHostClassesServer

	logger  *slog.Logger
	generic *GenericServer[*ffv1.HostClass, *privatev1.HostClass]
}

func NewHostClassesServer() *HostClassesServerBuilder {
	return &HostClassesServerBuilder{}
}

func (b *HostClassesServerBuilder) SetLogger(value *slog.Logger) *HostClassesServerBuilder {
	b.logger = value
	return b
}

func (b *HostClassesServerBuilder) Build() (result *HostClassesServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*ffv1.HostClass, *privatev1.HostClass]().
		SetLogger(b.logger).
		SetService(ffv1.HostClasses_ServiceDesc.ServiceName).
		SetTable("host_classes").
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &HostClassesServer{
		logger:  b.logger,
		generic: generic,
	}
	return
}

func (s *HostClassesServer) List(ctx context.Context,
	request *ffv1.HostClassesListRequest) (response *ffv1.HostClassesListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *HostClassesServer) Get(ctx context.Context,
	request *ffv1.HostClassesGetRequest) (response *ffv1.HostClassesGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *HostClassesServer) Create(ctx context.Context,
	request *ffv1.HostClassesCreateRequest) (response *ffv1.HostClassesCreateResponse, err error) {
	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *HostClassesServer) Update(ctx context.Context,
	request *ffv1.HostClassesUpdateRequest) (response *ffv1.HostClassesUpdateResponse, err error) {
	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *HostClassesServer) Delete(ctx context.Context,
	request *ffv1.HostClassesDeleteRequest) (response *ffv1.HostClassesDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}
