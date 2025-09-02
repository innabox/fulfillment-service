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
)

// DefaultOwnershipLogicBuilder contains the data and logic needed to create default ownership logic.
type DefaultOwnershipLogicBuilder struct {
	logger *slog.Logger
}

// DefaultOwnershipLogic is the default implementation of OwnershipLogic that extracts the subject from the context
// and returns the subject name as the owner.
type DefaultOwnershipLogic struct {
	logger *slog.Logger
}

// NewDefaultOwnershipLogic creates a new builder for default ownership logic.
func NewDefaultOwnershipLogic() *DefaultOwnershipLogicBuilder {
	return &DefaultOwnershipLogicBuilder{}
}

// SetLogger sets the logger that will be used by the ownership logic.
func (b *DefaultOwnershipLogicBuilder) SetLogger(value *slog.Logger) *DefaultOwnershipLogicBuilder {
	b.logger = value
	return b
}

// Build creates the default ownership logic that extracts the subject from the auth context and returns the name as
// the owner.
func (b *DefaultOwnershipLogicBuilder) Build() (result *DefaultOwnershipLogic, err error) {
	// Check that the logger has been set:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}

	// Create the ownership logic:
	result = &DefaultOwnershipLogic{
		logger: b.logger,
	}
	return
}

// DetermineAssignedOwners extracts the subject from the auth context and returns the subject name as the owner.
func (p *DefaultOwnershipLogic) DetermineAssignedOwners(ctx context.Context) (result []string, err error) {
	subject := SubjectFromContext(ctx)
	result = []string{
		subject.Name,
	}
	return
}
