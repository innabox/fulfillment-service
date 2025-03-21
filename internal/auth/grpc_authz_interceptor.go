/*
Copyright (c) 2025 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package auth

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc"
)

// GrpcAuthzInterceptorBuilder contains the data and logic needed to build an interceptor that checks authorization.
// Don't create instances of this type directly, use the NewGrpcAuthzInterceptor function instead.
type GrpcAuthzInterceptorBuilder struct {
	logger   *slog.Logger
	function GrpcAuthzFunc
}

// GrpcAuthzInterceptor contains the data needed by the interceptor.
type GrpcAuthzInterceptor struct {
	logger   *slog.Logger
	function GrpcAuthzFunc
}

// NewGrpcAuthzInterceptor creates a builder that can then be used to configure and create an authorization interceptor.
func NewGrpcAuthzInterceptor() *GrpcAuthzInterceptorBuilder {
	return &GrpcAuthzInterceptorBuilder{}
}

// SetLogger sets the logger that will be used to write to the log. This is mandatory.
func (b *GrpcAuthzInterceptorBuilder) SetLogger(value *slog.Logger) *GrpcAuthzInterceptorBuilder {
	b.logger = value
	return b
}

// SetFunction sets the authorization function. This is mandatory.
func (b *GrpcAuthzInterceptorBuilder) SetFunction(value GrpcAuthzFunc) *GrpcAuthzInterceptorBuilder {
	b.function = value
	return b
}

// Build uses the data stored in the builder to create and configure a new interceptor.
func (b *GrpcAuthzInterceptorBuilder) Build() (result *GrpcAuthzInterceptor, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.function == nil {
		err = errors.New("authorization function is mandatory")
		return
	}

	// Create and populate the object:
	result = &GrpcAuthzInterceptor{
		logger:   b.logger,
		function: b.function,
	}
	return
}

// UnaryServer is the unary server interceptor function.
func (i *GrpcAuthzInterceptor) UnaryServer(ctx context.Context, request any, info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (response any, err error) {
	err = i.function(ctx, info.FullMethod)
	if err != nil {
		return
	}
	response, err = handler(ctx, request)
	return
}

// StreamServer is the stream server interceptor function.
func (i *GrpcAuthzInterceptor) StreamServer(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {
	err := i.function(stream.Context(), info.FullMethod)
	if err != nil {
		return err
	}
	return handler(server, stream)
}
