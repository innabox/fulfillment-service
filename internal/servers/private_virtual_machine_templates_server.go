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
	"github.com/innabox/fulfillment-service/internal/database"
)

type PrivateVirtualMachineTemplatesServerBuilder struct {
	logger   *slog.Logger
	notifier *database.Notifier
}

var _ privatev1.VirtualMachineTemplatesServer = (*PrivateVirtualMachineTemplatesServer)(nil)

type PrivateVirtualMachineTemplatesServer struct {
	privatev1.UnimplementedVirtualMachineTemplatesServer
	logger  *slog.Logger
	generic *GenericServer[*privatev1.VirtualMachineTemplate]
}

func NewPrivateVirtualMachineTemplatesServer() *PrivateVirtualMachineTemplatesServerBuilder {
	return &PrivateVirtualMachineTemplatesServerBuilder{}
}

func (b *PrivateVirtualMachineTemplatesServerBuilder) SetLogger(value *slog.Logger) *PrivateVirtualMachineTemplatesServerBuilder {
	b.logger = value
	return b
}

func (b *PrivateVirtualMachineTemplatesServerBuilder) SetNotifier(value *database.Notifier) *PrivateVirtualMachineTemplatesServerBuilder {
	b.notifier = value
	return b
}

func (b *PrivateVirtualMachineTemplatesServerBuilder) Build() (result *PrivateVirtualMachineTemplatesServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*privatev1.VirtualMachineTemplate]().
		SetLogger(b.logger).
		SetService(privatev1.VirtualMachineTemplates_ServiceDesc.ServiceName).
		SetTable("virtual_machine_templates").
		SetNotifier(b.notifier).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &PrivateVirtualMachineTemplatesServer{
		logger:  b.logger,
		generic: generic,
	}
	return
}

func (s *PrivateVirtualMachineTemplatesServer) List(ctx context.Context,
	request *privatev1.VirtualMachineTemplatesListRequest) (response *privatev1.VirtualMachineTemplatesListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *PrivateVirtualMachineTemplatesServer) Get(ctx context.Context,
	request *privatev1.VirtualMachineTemplatesGetRequest) (response *privatev1.VirtualMachineTemplatesGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *PrivateVirtualMachineTemplatesServer) Create(ctx context.Context,
	request *privatev1.VirtualMachineTemplatesCreateRequest) (response *privatev1.VirtualMachineTemplatesCreateResponse, err error) {
	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *PrivateVirtualMachineTemplatesServer) Update(ctx context.Context,
	request *privatev1.VirtualMachineTemplatesUpdateRequest) (response *privatev1.VirtualMachineTemplatesUpdateResponse, err error) {
	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *PrivateVirtualMachineTemplatesServer) Delete(ctx context.Context,
	request *privatev1.VirtualMachineTemplatesDeleteRequest) (response *privatev1.VirtualMachineTemplatesDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}
