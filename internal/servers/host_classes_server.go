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

type HostClassesServerBuilder struct {
	logger  *slog.Logger
	private privatev1.HostClassesServer
}

var _ ffv1.HostClassesServer = (*HostClassesServer)(nil)

type HostClassesServer struct {
	ffv1.UnimplementedHostClassesServer

	logger    *slog.Logger
	private   privatev1.HostClassesServer
	inMapper  *GenericMapper[*ffv1.HostClass, *privatev1.HostClass]
	outMapper *GenericMapper[*privatev1.HostClass, *ffv1.HostClass]
}

func NewHostClassesServer() *HostClassesServerBuilder {
	return &HostClassesServerBuilder{}
}

// SetLogger sets the logger to use. This is mandatory.
func (b *HostClassesServerBuilder) SetLogger(value *slog.Logger) *HostClassesServerBuilder {
	b.logger = value
	return b
}

// SetPrivate sets the private server to use. This is mandatory.
func (b *HostClassesServerBuilder) SetPrivate(value privatev1.HostClassesServer) *HostClassesServerBuilder {
	b.private = value
	return b
}

func (b *HostClassesServerBuilder) Build() (result *HostClassesServer, err error) {
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
	inMapper, err := NewGenericMapper[*ffv1.HostClass, *privatev1.HostClass]().
		SetLogger(b.logger).
		SetStrict(true).
		Build()
	if err != nil {
		return
	}
	outMapper, err := NewGenericMapper[*privatev1.HostClass, *ffv1.HostClass]().
		SetLogger(b.logger).
		SetStrict(false).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &HostClassesServer{
		logger:    b.logger,
		private:   b.private,
		inMapper:  inMapper,
		outMapper: outMapper,
	}
	return
}

func (s *HostClassesServer) List(ctx context.Context,
	request *ffv1.HostClassesListRequest) (response *ffv1.HostClassesListResponse, err error) {
	// Create private request with same parameters:
	privateRequest := &privatev1.HostClassesListRequest{}
	privateRequest.SetOffset(request.GetOffset())
	privateRequest.SetLimit(request.GetLimit())
	privateRequest.SetFilter(request.GetFilter())
	privateRequest.SetOrder(request.GetOrder())

	// Delegate to private server:
	privateResponse, err := s.private.List(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map private response to public format:
	privateItems := privateResponse.GetItems()
	publicItems := make([]*ffv1.HostClass, len(privateItems))
	for i, privateItem := range privateItems {
		publicItem := &ffv1.HostClass{}
		err = s.outMapper.Copy(ctx, privateItem, publicItem)
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to map private host class to public",
				slog.Any("error", err),
			)
			return nil, grpcstatus.Errorf(grpccodes.Internal, "failed to process host classes")
		}
		publicItems[i] = publicItem
	}

	// Create the public response:
	response = &ffv1.HostClassesListResponse{}
	response.SetSize(privateResponse.GetSize())
	response.SetTotal(privateResponse.GetTotal())
	response.SetItems(publicItems)
	return
}

func (s *HostClassesServer) Get(ctx context.Context,
	request *ffv1.HostClassesGetRequest) (response *ffv1.HostClassesGetResponse, err error) {
	// Create private request:
	privateRequest := &privatev1.HostClassesGetRequest{}
	privateRequest.SetId(request.GetId())

	// Delegate to private server:
	privateResponse, err := s.private.Get(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map private response to public format:
	privateHostClass := privateResponse.GetObject()
	publicHostClass := &ffv1.HostClass{}
	err = s.outMapper.Copy(ctx, privateHostClass, publicHostClass)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private host class to public",
			slog.Any("error", err),
		)
		return nil, grpcstatus.Errorf(grpccodes.Internal, "failed to process host class")
	}

	// Create the public response:
	response = &ffv1.HostClassesGetResponse{}
	response.SetObject(publicHostClass)
	return
}

func (s *HostClassesServer) Create(ctx context.Context,
	request *ffv1.HostClassesCreateRequest) (response *ffv1.HostClassesCreateResponse, err error) {
	// Map the public host class to private format:
	publicHostClass := request.GetObject()
	if publicHostClass == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
		return
	}
	privateHostClass := &privatev1.HostClass{}
	err = s.inMapper.Copy(ctx, publicHostClass, privateHostClass)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map public host class to private",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process host class")
		return
	}

	// Delegate to the private server:
	privateRequest := &privatev1.HostClassesCreateRequest{}
	privateRequest.SetObject(privateHostClass)
	privateResponse, err := s.private.Create(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map the private response back to public format:
	createdPrivateHostClass := privateResponse.GetObject()
	createdPublicHostClass := &ffv1.HostClass{}
	err = s.outMapper.Copy(ctx, createdPrivateHostClass, createdPublicHostClass)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private host class to public",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process host class")
		return
	}

	// Create the public response:
	response = &ffv1.HostClassesCreateResponse{}
	response.SetObject(createdPublicHostClass)
	return
}

func (s *HostClassesServer) Update(ctx context.Context,
	request *ffv1.HostClassesUpdateRequest) (response *ffv1.HostClassesUpdateResponse, err error) {
	// Validate the request:
	publicHostClass := request.GetObject()
	if publicHostClass == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
		return
	}
	id := publicHostClass.GetId()
	if id == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object identifier is mandatory")
		return
	}

	// Get the existing object from the private server:
	getRequest := &privatev1.HostClassesGetRequest{}
	getRequest.SetId(id)
	getResponse, err := s.private.Get(ctx, getRequest)
	if err != nil {
		return nil, err
	}
	existingPrivateHostClass := getResponse.GetObject()

	// Map the public changes to the existing private object (preserving private data):
	err = s.inMapper.Copy(ctx, publicHostClass, existingPrivateHostClass)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map public host class to private",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process host class")
		return
	}

	// Delegate to the private server with the merged object:
	privateRequest := &privatev1.HostClassesUpdateRequest{}
	privateRequest.SetObject(existingPrivateHostClass)
	privateResponse, err := s.private.Update(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map the private response back to public format:
	updatedPrivateHostClass := privateResponse.GetObject()
	updatedPublicHostClass := &ffv1.HostClass{}
	err = s.outMapper.Copy(ctx, updatedPrivateHostClass, updatedPublicHostClass)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private host class to public",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process host class")
		return
	}

	// Create the public response:
	response = &ffv1.HostClassesUpdateResponse{}
	response.SetObject(updatedPublicHostClass)
	return
}

func (s *HostClassesServer) Delete(ctx context.Context,
	request *ffv1.HostClassesDeleteRequest) (response *ffv1.HostClassesDeleteResponse, err error) {
	// Create private request:
	privateRequest := &privatev1.HostClassesDeleteRequest{}
	privateRequest.SetId(request.GetId())

	// Delegate to private server:
	_, err = s.private.Delete(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Create the public response:
	response = &ffv1.HostClassesDeleteResponse{}
	return
}
