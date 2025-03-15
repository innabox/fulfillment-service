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
	"google.golang.org/grpc/metadata"
)

// GrpcAuthnInterceptorBuilder contains the data and logic needed to build an interceptor that checks authentication.
// Don't create instances of this type directly, use the NewGrpcAuthnInterceptor function instead.
type GrpcAuthnInterceptorBuilder struct {
	logger   *slog.Logger
	function GrpcAuthnFunc
}

// GrpcAuthnInterceptor contains the data needed by the interceptor.
type GrpcAuthnInterceptor struct {
	logger   *slog.Logger
	function GrpcAuthnFunc
}

// NewGrpcAuthnInterceptor creates a builder that can then be used to configure and create an authentication
// interceptor.
func NewGrpcAuthnInterceptor() *GrpcAuthnInterceptorBuilder {
	return &GrpcAuthnInterceptorBuilder{}
}

// SetLogger sets the logger that will be used to write to the log. This is mandatory.
func (b *GrpcAuthnInterceptorBuilder) SetLogger(value *slog.Logger) *GrpcAuthnInterceptorBuilder {
	b.logger = value
	return b
}

// SetFunction sets the authentication function. This is mandatory.
func (b *GrpcAuthnInterceptorBuilder) SetFunction(value GrpcAuthnFunc) *GrpcAuthnInterceptorBuilder {
	b.function = value
	return b
}

// Build uses the data stored in the builder to create and configure a new interceptor.
func (b *GrpcAuthnInterceptorBuilder) Build() (result *GrpcAuthnInterceptor, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.function == nil {
		err = errors.New("authentication function is mandatory")
		return
	}

	// Create and populate the object:
	result = &GrpcAuthnInterceptor{
		logger:   b.logger,
		function: b.function,
	}
	return
}

// UnaryServer is the unary server interceptor function.
func (i *GrpcAuthnInterceptor) UnaryServer(ctx context.Context, request any, info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (response any, err error) {
	ctx, err = i.function(ctx, info.FullMethod)
	if err != nil {
		return
	}
	response, err = handler(ctx, request)
	return
}

// StreamServer is the stream server interceptor function.
func (i *GrpcAuthnInterceptor) StreamServer(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {
	ctx, err := i.function(stream.Context(), info.FullMethod)
	if err != nil {
		return err
	}
	stream = &grpcAuthnInterceptorStream{
		logger:  i.logger,
		context: ctx,
		stream:  stream,
	}
	return handler(server, stream)
}

type grpcAuthnInterceptorStream struct {
	logger  *slog.Logger
	context context.Context
	stream  grpc.ServerStream
}

func (s *grpcAuthnInterceptorStream) Context() context.Context {
	return s.context
}

func (s *grpcAuthnInterceptorStream) RecvMsg(message any) error {
	return s.stream.RecvMsg(message)
}

func (s *grpcAuthnInterceptorStream) SendHeader(md metadata.MD) error {
	return s.stream.SendHeader(md)
}

func (s *grpcAuthnInterceptorStream) SendMsg(message any) error {
	return s.stream.SendMsg(message)
}

func (s *grpcAuthnInterceptorStream) SetHeader(md metadata.MD) error {
	return s.stream.SendHeader(md)
}

func (s *grpcAuthnInterceptorStream) SetTrailer(md metadata.MD) {
	s.stream.SetTrailer(md)
}
