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

type VirtualMachineTemplatesServerBuilder struct {
	logger  *slog.Logger
	private privatev1.VirtualMachineTemplatesServer
}

var _ ffv1.VirtualMachineTemplatesServer = (*VirtualMachineTemplatesServer)(nil)

type VirtualMachineTemplatesServer struct {
	ffv1.UnimplementedVirtualMachineTemplatesServer

	logger    *slog.Logger
	private   privatev1.VirtualMachineTemplatesServer
	inMapper  *GenericMapper[*ffv1.VirtualMachineTemplate, *privatev1.VirtualMachineTemplate]
	outMapper *GenericMapper[*privatev1.VirtualMachineTemplate, *ffv1.VirtualMachineTemplate]
}

func NewVirtualMachineTemplatesServer() *VirtualMachineTemplatesServerBuilder {
	return &VirtualMachineTemplatesServerBuilder{}
}

// SetLogger sets the logger to use. This is mandatory.
func (b *VirtualMachineTemplatesServerBuilder) SetLogger(value *slog.Logger) *VirtualMachineTemplatesServerBuilder {
	b.logger = value
	return b
}

// SetPrivate sets the private server to use. This is mandatory.
func (b *VirtualMachineTemplatesServerBuilder) SetPrivate(value privatev1.VirtualMachineTemplatesServer) *VirtualMachineTemplatesServerBuilder {
	b.private = value
	return b
}

func (b *VirtualMachineTemplatesServerBuilder) Build() (result *VirtualMachineTemplatesServer, err error) {
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
	inMapper, err := NewGenericMapper[*ffv1.VirtualMachineTemplate, *privatev1.VirtualMachineTemplate]().
		SetLogger(b.logger).
		SetStrict(true).
		Build()
	if err != nil {
		return
	}
	outMapper, err := NewGenericMapper[*privatev1.VirtualMachineTemplate, *ffv1.VirtualMachineTemplate]().
		SetLogger(b.logger).
		SetStrict(false).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &VirtualMachineTemplatesServer{
		logger:    b.logger,
		private:   b.private,
		inMapper:  inMapper,
		outMapper: outMapper,
	}
	return
}

func (s *VirtualMachineTemplatesServer) List(ctx context.Context,
	request *ffv1.VirtualMachineTemplatesListRequest) (response *ffv1.VirtualMachineTemplatesListResponse, err error) {
	// Create private request with same parameters:
	privateRequest := &privatev1.VirtualMachineTemplatesListRequest{}
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
	publicItems := make([]*ffv1.VirtualMachineTemplate, len(privateItems))
	for i, privateItem := range privateItems {
		publicItem := &ffv1.VirtualMachineTemplate{}
		err = s.outMapper.Copy(ctx, privateItem, publicItem)
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to map private virtual machine template to public",
				slog.Any("error", err),
			)
			return nil, grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine templates")
		}
		publicItems[i] = publicItem
	}

	// Create the public response:
	response = &ffv1.VirtualMachineTemplatesListResponse{}
	response.SetSize(privateResponse.GetSize())
	response.SetTotal(privateResponse.GetTotal())
	response.SetItems(publicItems)
	return
}

func (s *VirtualMachineTemplatesServer) Get(ctx context.Context,
	request *ffv1.VirtualMachineTemplatesGetRequest) (response *ffv1.VirtualMachineTemplatesGetResponse, err error) {
	// Create private request:
	privateRequest := &privatev1.VirtualMachineTemplatesGetRequest{}
	privateRequest.SetId(request.GetId())

	// Delegate to private server:
	privateResponse, err := s.private.Get(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map private response to public format:
	privateVirtualMachineTemplate := privateResponse.GetObject()
	publicVirtualMachineTemplate := &ffv1.VirtualMachineTemplate{}
	err = s.outMapper.Copy(ctx, privateVirtualMachineTemplate, publicVirtualMachineTemplate)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private virtual machine template to public",
			slog.Any("error", err),
		)
		return nil, grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine template")
	}

	// Create the public response:
	response = &ffv1.VirtualMachineTemplatesGetResponse{}
	response.SetObject(publicVirtualMachineTemplate)
	return
}

func (s *VirtualMachineTemplatesServer) Create(ctx context.Context,
	request *ffv1.VirtualMachineTemplatesCreateRequest) (response *ffv1.VirtualMachineTemplatesCreateResponse, err error) {
	// Map the public virtual machine template to private format:
	publicVirtualMachineTemplate := request.GetObject()
	if publicVirtualMachineTemplate == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
		return
	}
	privateVirtualMachineTemplate := &privatev1.VirtualMachineTemplate{}
	err = s.inMapper.Copy(ctx, publicVirtualMachineTemplate, privateVirtualMachineTemplate)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map public virtual machine template to private",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine template")
		return
	}

	// Delegate to the private server:
	privateRequest := &privatev1.VirtualMachineTemplatesCreateRequest{}
	privateRequest.SetObject(privateVirtualMachineTemplate)
	privateResponse, err := s.private.Create(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map the private response back to public format:
	createdPrivateVirtualMachineTemplate := privateResponse.GetObject()
	createdPublicVirtualMachineTemplate := &ffv1.VirtualMachineTemplate{}
	err = s.outMapper.Copy(ctx, createdPrivateVirtualMachineTemplate, createdPublicVirtualMachineTemplate)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private virtual machine template to public",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine template")
		return
	}

	// Create the public response:
	response = &ffv1.VirtualMachineTemplatesCreateResponse{}
	response.SetObject(createdPublicVirtualMachineTemplate)
	return
}

func (s *VirtualMachineTemplatesServer) Update(ctx context.Context,
	request *ffv1.VirtualMachineTemplatesUpdateRequest) (response *ffv1.VirtualMachineTemplatesUpdateResponse, err error) {
	// Validate the request:
	publicVirtualMachineTemplate := request.GetObject()
	if publicVirtualMachineTemplate == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
		return
	}
	id := publicVirtualMachineTemplate.GetId()
	if id == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object identifier is mandatory")
		return
	}

	// Get the existing object from the private server:
	getRequest := &privatev1.VirtualMachineTemplatesGetRequest{}
	getRequest.SetId(id)
	getResponse, err := s.private.Get(ctx, getRequest)
	if err != nil {
		return nil, err
	}
	existingPrivateVirtualMachineTemplate := getResponse.GetObject()

	// Map the public changes to the existing private object (preserving private data):
	err = s.inMapper.Copy(ctx, publicVirtualMachineTemplate, existingPrivateVirtualMachineTemplate)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map public virtual machine template to private",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine template")
		return
	}

	// Delegate to the private server with the merged object:
	privateRequest := &privatev1.VirtualMachineTemplatesUpdateRequest{}
	privateRequest.SetObject(existingPrivateVirtualMachineTemplate)
	privateResponse, err := s.private.Update(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map the private response back to public format:
	updatedPrivateVirtualMachineTemplate := privateResponse.GetObject()
	updatedPublicVirtualMachineTemplate := &ffv1.VirtualMachineTemplate{}
	err = s.outMapper.Copy(ctx, updatedPrivateVirtualMachineTemplate, updatedPublicVirtualMachineTemplate)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private virtual machine template to public",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process virtual machine template")
		return
	}

	// Create the public response:
	response = &ffv1.VirtualMachineTemplatesUpdateResponse{}
	response.SetObject(updatedPublicVirtualMachineTemplate)
	return
}

func (s *VirtualMachineTemplatesServer) Delete(ctx context.Context,
	request *ffv1.VirtualMachineTemplatesDeleteRequest) (response *ffv1.VirtualMachineTemplatesDeleteResponse, err error) {
	// Create private request:
	privateRequest := &privatev1.VirtualMachineTemplatesDeleteRequest{}
	privateRequest.SetId(request.GetId())

	// Delegate to private server:
	_, err = s.private.Delete(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Create the public response:
	response = &ffv1.VirtualMachineTemplatesDeleteResponse{}
	return
}
