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

	"github.com/spf13/pflag"
)

// GrpcGuestAuthnType is the name of the guest authentication function.
const GrpcGuestAuthnType = "guest"

// GrpcGuestAuthnFuncBuilder is a gRPC authentication function that ignores all authentication flags and always adds to
// the context the guest subject, effectively disabling authentication.
type GrpcGuestAuthnFuncBuilder struct {
	logger *slog.Logger
}

// grpcGuestAuthnFunc contains the data needed by the function.
type grpcGuestAuthnFunc struct {
	logger *slog.Logger
}

// NewGrpcGuestAuthnFunc creates a builder that can then be used to configure and create a new gRPMC authentication
// function.
func NewGrpcGuestAuthnFunc() *GrpcGuestAuthnFuncBuilder {
	return &GrpcGuestAuthnFuncBuilder{}
}

// SetLogger sets the logger that will be used to write to the log. This is mandatory.
func (b *GrpcGuestAuthnFuncBuilder) SetLogger(value *slog.Logger) *GrpcGuestAuthnFuncBuilder {
	b.logger = value
	return b
}

// SetFlags sets the command line flags that should be used to configure the function. This is optional.
func (b *GrpcGuestAuthnFuncBuilder) SetFlags(flags *pflag.FlagSet) *GrpcGuestAuthnFuncBuilder {
	// There are no flags for this function currently.
	return b
}

// Build uses the data stored in the builder to create and configure a new gRPC guest authentication function.
func (b *GrpcGuestAuthnFuncBuilder) Build() (result GrpcAuthnFunc, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create and populate the object:
	object := &grpcGuestAuthnFunc{
		logger: b.logger,
	}
	result = object.call
	return
}

// call is the implementation of the `GrpcAuthnFunc` type.
func (f *grpcGuestAuthnFunc) call(ctx context.Context, method string) (result context.Context, err error) {
	result = ContextWithSubject(ctx, Guest)
	return
}
