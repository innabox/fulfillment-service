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

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/auth"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
	"github.com/innabox/fulfillment-service/internal/utils"
)

type PrivateVirtualMachinesServerBuilder struct {
	logger           *slog.Logger
	notifier         *database.Notifier
	attributionLogic auth.AttributionLogic
	tenancyLogic     auth.TenancyLogic
}

var _ privatev1.VirtualMachinesServer = (*PrivateVirtualMachinesServer)(nil)

type PrivateVirtualMachinesServer struct {
	privatev1.UnimplementedVirtualMachinesServer

	logger       *slog.Logger
	generic      *GenericServer[*privatev1.VirtualMachine]
	templatesDao *dao.GenericDAO[*privatev1.VirtualMachineTemplate]
}

func NewPrivateVirtualMachinesServer() *PrivateVirtualMachinesServerBuilder {
	return &PrivateVirtualMachinesServerBuilder{}
}

func (b *PrivateVirtualMachinesServerBuilder) SetLogger(value *slog.Logger) *PrivateVirtualMachinesServerBuilder {
	b.logger = value
	return b
}

func (b *PrivateVirtualMachinesServerBuilder) SetNotifier(value *database.Notifier) *PrivateVirtualMachinesServerBuilder {
	b.notifier = value
	return b
}

func (b *PrivateVirtualMachinesServerBuilder) SetAttributionLogic(value auth.AttributionLogic) *PrivateVirtualMachinesServerBuilder {
	b.attributionLogic = value
	return b
}

func (b *PrivateVirtualMachinesServerBuilder) SetTenancyLogic(value auth.TenancyLogic) *PrivateVirtualMachinesServerBuilder {
	b.tenancyLogic = value
	return b
}

func (b *PrivateVirtualMachinesServerBuilder) Build() (result *PrivateVirtualMachinesServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.tenancyLogic == nil {
		err = errors.New("tenancy logic is mandatory")
		return
	}

	// Create the templates DAO:
	templatesDao, err := dao.NewGenericDAO[*privatev1.VirtualMachineTemplate]().
		SetLogger(b.logger).
		SetTable("virtual_machine_templates").
		SetAttributionLogic(b.attributionLogic).
		SetTenancyLogic(b.tenancyLogic).
		Build()
	if err != nil {
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*privatev1.VirtualMachine]().
		SetLogger(b.logger).
		SetService(privatev1.VirtualMachines_ServiceDesc.ServiceName).
		SetTable("virtual_machines").
		SetNotifier(b.notifier).
		SetAttributionLogic(b.attributionLogic).
		SetTenancyLogic(b.tenancyLogic).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &PrivateVirtualMachinesServer{
		logger:       b.logger,
		generic:      generic,
		templatesDao: templatesDao,
	}
	return
}

func (s *PrivateVirtualMachinesServer) List(ctx context.Context,
	request *privatev1.VirtualMachinesListRequest) (response *privatev1.VirtualMachinesListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *PrivateVirtualMachinesServer) Get(ctx context.Context,
	request *privatev1.VirtualMachinesGetRequest) (response *privatev1.VirtualMachinesGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *PrivateVirtualMachinesServer) Create(ctx context.Context,
	request *privatev1.VirtualMachinesCreateRequest) (response *privatev1.VirtualMachinesCreateResponse, err error) {
	// Validate template:
	err = s.validateTemplate(ctx, request.GetObject())
	if err != nil {
		return
	}

	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *PrivateVirtualMachinesServer) Update(ctx context.Context,
	request *privatev1.VirtualMachinesUpdateRequest) (response *privatev1.VirtualMachinesUpdateResponse, err error) {
	// Validate template:
	err = s.validateTemplate(ctx, request.GetObject())
	if err != nil {
		return
	}

	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *PrivateVirtualMachinesServer) Delete(ctx context.Context,
	request *privatev1.VirtualMachinesDeleteRequest) (response *privatev1.VirtualMachinesDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}

func (s *PrivateVirtualMachinesServer) Signal(ctx context.Context,
	request *privatev1.VirtualMachinesSignalRequest) (response *privatev1.VirtualMachinesSignalResponse, err error) {
	err = s.generic.Signal(ctx, request, &response)
	return
}

// validateTemplate validates the template ID and parameters in the virtual machine spec.
func (s *PrivateVirtualMachinesServer) validateTemplate(ctx context.Context, vm *privatev1.VirtualMachine) error {
	if vm == nil {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "virtual machine is mandatory")
	}

	spec := vm.GetSpec()
	if spec == nil {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "virtual machine spec is mandatory")
	}

	templateID := spec.GetTemplate()
	if templateID == "" {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "template ID is mandatory")
	}

	// Get the template:
	getTemplateResponse, err := s.templatesDao.Get().
		SetId(templateID).
		Do(ctx)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Template retrieval failed",
			slog.String("template_id", templateID),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to retrieve template '%s'",
			templateID,
		)
	}
	template := getTemplateResponse.GetObject()
	if template == nil {
		return grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"template '%s' does not exist",
			templateID,
		)
	}

	// Validate template parameters:
	vmParameters := spec.GetTemplateParameters()
	err = utils.ValidateVirtualMachineTemplateParameters(template, vmParameters)
	if err != nil {
		return err
	}

	// Set default values for template parameters:
	actualVmParameters := utils.ProcessTemplateParametersWithDefaults(
		utils.VirtualMachineTemplateAdapter{VirtualMachineTemplate: template},
		vmParameters,
	)
	spec.SetTemplateParameters(actualVmParameters)

	return nil
}
