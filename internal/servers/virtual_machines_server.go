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
)

type VirtualMachinesServerBuilder struct {
	logger  *slog.Logger
	private privatev1.VirtualMachinesServer
}

var _ ffv1.VirtualMachinesServer = (*VirtualMachinesServer)(nil)

type VirtualMachinesServer struct {
	ffv1.UnimplementedVirtualMachinesServer

	logger    *slog.Logger
	private   privatev1.VirtualMachinesServer
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

// SetPrivate sets the private server to use. This is mandatory.
func (b *VirtualMachinesServerBuilder) SetPrivate(value privatev1.VirtualMachinesServer) *VirtualMachinesServerBuilder {
	b.private = value
	return b
}

func (b *VirtualMachinesServerBuilder) Build() (result *VirtualMachinesServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.private == nil {
		err = errors.New("private server is mandatory")
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

	// Create and populate the object:
	result = &VirtualMachinesServer{
		logger:    b.logger,
		private:   b.private,
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
	privateResponse, err := s.private.List(ctx, privateRequest)
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
	privateResponse, err := s.private.Get(ctx, privateRequest)
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
	privateResponse, err := s.private.Create(ctx, privateRequest)
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
	getResponse, err := s.private.Get(ctx, getRequest)
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
	privateResponse, err := s.private.Update(ctx, privateRequest)
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
	_, err = s.private.Delete(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Create the public response:
	response = &ffv1.VirtualMachinesDeleteResponse{}
	return
}
