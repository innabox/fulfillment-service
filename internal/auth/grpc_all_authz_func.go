/*
Copyright (c) 2025 Red Hat Inc.

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
	"fmt"
	"log/slog"

	"github.com/spf13/pflag"
)

// GrpcAllAuthzType is the name of the authentication function that authorizes everything unconditionally.
const GrpcAllAuthzType = "all"

// GrpcAllAuthzFuncBuilder contains the data and logic needed to create an authorization function.
type GrpcAllAuthzFuncBuilder struct {
	logger *slog.Logger
}

type grpcAllAuthzFunc struct {
	logger *slog.Logger
}

// NewGrpcAllAuthzFunc creates a builder that can then be configured and used to create a new authorization function.
func NewGrpcAllAuthzFunc() *GrpcAllAuthzFuncBuilder {
	return &GrpcAllAuthzFuncBuilder{}
}

// SetLogger sets the logger that the function will use to write messages to the log. This is mandatory.
func (b *GrpcAllAuthzFuncBuilder) SetLogger(value *slog.Logger) *GrpcAllAuthzFuncBuilder {
	b.logger = value
	return b
}

// SetFlags sets the command line flags that should be used to configure the function. This is optional.
func (b *GrpcAllAuthzFuncBuilder) SetFlags(flags *pflag.FlagSet) *GrpcAllAuthzFuncBuilder {
	// No flags defined yet.
	return b
}

// Build uses the data stored in the builder to create a new authorization function.
func (b *GrpcAllAuthzFuncBuilder) Build() (result GrpcAuthzFunc, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}

	// Create and populate the object:
	object := &grpcAllAuthzFunc{
		logger: b.logger,
	}
	result = object.call

	return
}

func (f *grpcAllAuthzFunc) call(ctx context.Context, method string) error {
	return nil
}
