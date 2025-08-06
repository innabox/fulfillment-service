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
	"fmt"
	"log/slog"

	"github.com/innabox/fulfillment-service/internal/auth"
)

// OwnershipFuncBuilder contains the data and logic needed to create ownership functions.
type OwnershipFuncBuilder struct {
	logger *slog.Logger
}

type ownershipFunc struct {
	logger *slog.Logger
}

// NewOwnershipFunc creates a new builder for ownership functions.
func NewOwnershipFunc() *OwnershipFuncBuilder {
	return &OwnershipFuncBuilder{}
}

// SetLogger sets the logger that will be used by the ownership function.
func (b *OwnershipFuncBuilder) SetLogger(value *slog.Logger) *OwnershipFuncBuilder {
	b.logger = value
	return b
}

// Build creates the ownership function that extracts the subject from the auth context and returns the name.
func (b *OwnershipFuncBuilder) Build() (result func(ctx context.Context) string, err error) {
	// Check that the logger has been set:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}

	// Create the ownership function:
	function := &ownershipFunc{
		logger: b.logger,
	}
	result = function.call
	return
}

func (f *ownershipFunc) call(ctx context.Context) string {
	subject := auth.SubjectFromContext(ctx)
	return subject.Name
}
