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

// GrpcGuestAuthType is the name of the guest authentication type.
const GrpcGuestAuthType = "guest"

// GrpcGuestAuthInterceptorBuilder contains the data and logic needed to build an interceptor that ignores all
// authentication and always adds the guest subject to the context.
type GrpcGuestAuthInterceptorBuilder struct {
	logger *slog.Logger
}

// GrpcGuestAuthInterceptor is an interceptor that ignores all authentication and always adds the guest subject to
// the context, effectively disabling authentication. This is intended for development and testing purposes only.
type GrpcGuestAuthInterceptor struct {
	logger *slog.Logger
}

// NewGrpcGuestAuthInterceptor creates a builder that can then be used to configure and create a guest auth
// interceptor.
func NewGrpcGuestAuthInterceptor() *GrpcGuestAuthInterceptorBuilder {
	return &GrpcGuestAuthInterceptorBuilder{}
}

// SetLogger sets the logger that will be used to write to the log. This is mandatory.
func (b *GrpcGuestAuthInterceptorBuilder) SetLogger(value *slog.Logger) *GrpcGuestAuthInterceptorBuilder {
	b.logger = value
	return b
}

// Build uses the data stored in the builder to create and configure a new interceptor.
func (b *GrpcGuestAuthInterceptorBuilder) Build() (result *GrpcGuestAuthInterceptor, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create and populate the object:
	result = &GrpcGuestAuthInterceptor{
		logger: b.logger,
	}
	return
}

// UnaryServer is the unary server interceptor function.
func (i *GrpcGuestAuthInterceptor) UnaryServer(ctx context.Context, request any, info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (response any, err error) {
	ctx = ContextWithSubject(ctx, Guest)
	return handler(ctx, request)
}

// StreamServer is the stream server interceptor function.
func (i *GrpcGuestAuthInterceptor) StreamServer(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {
	ctx := ContextWithSubject(stream.Context(), Guest)
	stream = &grpcGuestAuthInterceptorStream{
		context: ctx,
		stream:  stream,
	}
	return handler(server, stream)
}

// grpcGuestAuthInterceptorStream wraps a gRPC server stream with a modified context.
type grpcGuestAuthInterceptorStream struct {
	context context.Context
	stream  grpc.ServerStream
}

func (s *grpcGuestAuthInterceptorStream) Context() context.Context {
	return s.context
}

func (s *grpcGuestAuthInterceptorStream) RecvMsg(message any) error {
	return s.stream.RecvMsg(message)
}

func (s *grpcGuestAuthInterceptorStream) SendHeader(md metadata.MD) error {
	return s.stream.SendHeader(md)
}

func (s *grpcGuestAuthInterceptorStream) SendMsg(message any) error {
	return s.stream.SendMsg(message)
}

func (s *grpcGuestAuthInterceptorStream) SetHeader(md metadata.MD) error {
	return s.stream.SetHeader(md)
}

func (s *grpcGuestAuthInterceptorStream) SetTrailer(md metadata.MD) {
	s.stream.SetTrailer(md)
}
