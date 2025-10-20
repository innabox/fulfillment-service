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
	"github.com/innabox/fulfillment-service/internal/auth"
	"github.com/innabox/fulfillment-service/internal/database"
)

type VirtualMachinesServerBuilder struct {
	logger           *slog.Logger
	notifier         *database.Notifier
	attributionLogic auth.AttributionLogic
	tenancyLogic     auth.TenancyLogic
}

var _ ffv1.VirtualMachinesServer = (*VirtualMachinesServer)(nil)

type VirtualMachinesServer struct {
	ffv1.UnimplementedVirtualMachinesServer

	logger    *slog.Logger
	delegate  privatev1.VirtualMachinesServer
	inMapper  *GenericMapper[*ffv1.VirtualMachine, *privatev1.VirtualMachine]
	outMapper *GenericMapper[*privatev1.VirtualMachine, *ffv1.VirtualMachine]
}

func NewVirtualMachinesServer() *VirtualMachinesServerBuilder {
	return &VirtualMachinesServerBuilder{}
}

// SetLogger sets the logger to use. This is mandatory.
func (b *VirtualMachinesServerBuilder) SetLogger(value *slog.Logger) *VirtualMachinesServerBuilder {
	b.logger = value
	return b
}

// SetNotifier sets the notifier to use. This is optional.
func (b *VirtualMachinesServerBuilder) SetNotifier(value *database.Notifier) *VirtualMachinesServerBuilder {
	b.notifier = value
	return b
}

// SetAttributionLogic sets the attribution logic to use. This is optional.
func (b *VirtualMachinesServerBuilder) SetAttributionLogic(value auth.AttributionLogic) *VirtualMachinesServerBuilder {
	b.attributionLogic = value
	return b
}

// SetTenancyLogic sets the tenancy logic to use. This is mandatory.
func (b *VirtualMachinesServerBuilder) SetTenancyLogic(value auth.TenancyLogic) *VirtualMachinesServerBuilder {
	b.tenancyLogic = value
	return b
}

func (b *VirtualMachinesServerBuilder) Build() (result *VirtualMachinesServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.tenancyLogic == nil {
		err = errors.New("tenancy logic is mandatory")
		return
	}

	// Create the mappers:
	inMapper, err := NewGenericMapper[*ffv1.VirtualMachine, *privatev1.VirtualMachine]().
		SetLogger(b.logger).
		SetStrict(true).
		Build()
	if err != nil {
		return
	}
	outMapper, err := NewGenericMapper[*privatev1.VirtualMachine, *ffv1.VirtualMachine]().
		SetLogger(b.logger).
		SetStrict(false).
		Build()
	if err != nil {
		return
	}

	// Create the private server to delegate to:
	delegate, err := NewPrivateVirtualMachinesServer().
		SetLogger(b.logger).
		SetNotifier(b.notifier).
		SetAttributionLogic(b.attributionLogic).
		SetTenancyLogic(b.tenancyLogic).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &VirtualMachinesServer{
		logger:    b.logger,
		delegate:  delegate,
		inMapper:  inMapper,
		outMapper: outMapper,
	}
	return
}

func (s *VirtualMachinesServer) List(ctx context.Context,
	request *ffv1.VirtualMachinesListRequest) (response *ffv1.VirtualMachinesListResponse, err error) {
	// Create private request with same parameters:
	privateRequest := &privatev1.VirtualMachinesListRequest{}
	privateRequest.SetOffset(request.GetOffset())
	privateRequest.SetLimit(request.GetLimit())
	privateRequest.SetFilter(request.GetFilter())

	// Delegate to private server:
	privateResponse, err := s.delegate.List(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map private response to public format:
	privateItems := privateResponse.GetItems()
	publicItems := make([]*ffv1.VirtualMachine, len(privateItems))
	for i, privateItem := range privateItems {
		publicItem := &ffv1.VirtualMachine{}
		err = s.outMapper.Copy(ctx, privateItem, publicItem)
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to map private virtual machine to public",
				slog.Any("error", err),
			)
			return nil, grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machines")
		}
		publicItems[i] = publicItem
	}

	// Create the public response:
	response = &ffv1.VirtualMachinesListResponse{}
	response.SetSize(privateResponse.GetSize())
	response.SetTotal(privateResponse.GetTotal())
	response.SetItems(publicItems)
	return
}

func (s *VirtualMachinesServer) Get(ctx context.Context,
	request *ffv1.VirtualMachinesGetRequest) (response *ffv1.VirtualMachinesGetResponse, err error) {
	// Create private request:
	privateRequest := &privatev1.VirtualMachinesGetRequest{}
	privateRequest.SetId(request.GetId())

	// Delegate to private server:
	privateResponse, err := s.delegate.Get(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map private response to public format:
	privateVirtualMachine := privateResponse.GetObject()
	publicVirtualMachine := &ffv1.VirtualMachine{}
	err = s.outMapper.Copy(ctx, privateVirtualMachine, publicVirtualMachine)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private virtual machine to public",
			slog.Any("error", err),
		)
		return nil, grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine")
	}

	// Create the public response:
	response = &ffv1.VirtualMachinesGetResponse{}
	response.SetObject(publicVirtualMachine)
	return
}

func (s *VirtualMachinesServer) Create(ctx context.Context,
	request *ffv1.VirtualMachinesCreateRequest) (response *ffv1.VirtualMachinesCreateResponse, err error) {
	// Map the public virtual machine to private format:
	publicVirtualMachine := request.GetObject()
	if publicVirtualMachine == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
		return
	}
	privateVirtualMachine := &privatev1.VirtualMachine{}
	err = s.inMapper.Copy(ctx, publicVirtualMachine, privateVirtualMachine)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map public virtual machine to private",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine")
		return
	}

	// Delegate to the private server:
	privateRequest := &privatev1.VirtualMachinesCreateRequest{}
	privateRequest.SetObject(privateVirtualMachine)
	privateResponse, err := s.delegate.Create(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map the private response back to public format:
	createdPrivateVirtualMachine := privateResponse.GetObject()
	createdPublicVirtualMachine := &ffv1.VirtualMachine{}
	err = s.outMapper.Copy(ctx, createdPrivateVirtualMachine, createdPublicVirtualMachine)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private virtual machine to public",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine")
		return
	}

	// Create the public response:
	response = &ffv1.VirtualMachinesCreateResponse{}
	response.SetObject(createdPublicVirtualMachine)
	return
}

func (s *VirtualMachinesServer) Update(ctx context.Context,
	request *ffv1.VirtualMachinesUpdateRequest) (response *ffv1.VirtualMachinesUpdateResponse, err error) {
	// Validate the request:
	publicVirtualMachine := request.GetObject()
	if publicVirtualMachine == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
		return
	}
	id := publicVirtualMachine.GetId()
	if id == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object identifier is mandatory")
		return
	}

	// Get the existing object from the private server:
	getRequest := &privatev1.VirtualMachinesGetRequest{}
	getRequest.SetId(id)
	getResponse, err := s.delegate.Get(ctx, getRequest)
	if err != nil {
		return nil, err
	}
	existingPrivateVirtualMachine := getResponse.GetObject()

	// Map the public changes to the existing private object (preserving private data):
	err = s.inMapper.Copy(ctx, publicVirtualMachine, existingPrivateVirtualMachine)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map public virtual machine to private",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine")
		return
	}

	// Delegate to the private server with the merged object:
	privateRequest := &privatev1.VirtualMachinesUpdateRequest{}
	privateRequest.SetObject(existingPrivateVirtualMachine)
	privateResponse, err := s.delegate.Update(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map the private response back to public format:
	updatedPrivateVirtualMachine := privateResponse.GetObject()
	updatedPublicVirtualMachine := &ffv1.VirtualMachine{}
	err = s.outMapper.Copy(ctx, updatedPrivateVirtualMachine, updatedPublicVirtualMachine)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private virtual machine to public",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine")
		return
	}

	// Create the public response:
	response = &ffv1.VirtualMachinesUpdateResponse{}
	response.SetObject(updatedPublicVirtualMachine)
	return
}

func (s *VirtualMachinesServer) Delete(ctx context.Context,
	request *ffv1.VirtualMachinesDeleteRequest) (response *ffv1.VirtualMachinesDeleteResponse, err error) {
	// Create private request:
	privateRequest := &privatev1.VirtualMachinesDeleteRequest{}
	privateRequest.SetId(request.GetId())

	// Delegate to private server:
	_, err = s.delegate.Delete(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Create the public response:
	response = &ffv1.VirtualMachinesDeleteResponse{}
	return
}
