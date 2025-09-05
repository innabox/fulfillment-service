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
	"log/slog"
)

// EmptyOwnershipLogicBuilder contains the data and logic needed to create empty ownership logic.
type EmptyOwnershipLogicBuilder struct {
	logger *slog.Logger
}

// EmptyOwnershipLogic is a minimal implementation that returns no owners. This is used as a fallback when no
// ownership logic is configured.
type EmptyOwnershipLogic struct {
	logger *slog.Logger
}

// NewEmptyOwnershipLogic creates a new builder for empty ownership logic.
func NewEmptyOwnershipLogic() *EmptyOwnershipLogicBuilder {
	return &EmptyOwnershipLogicBuilder{}
}

// SetLogger sets the logger that will be used by the ownership logic.
func (b *EmptyOwnershipLogicBuilder) SetLogger(value *slog.Logger) *EmptyOwnershipLogicBuilder {
	b.logger = value
	return b
}

// Build creates the empty ownership logic.
func (b *EmptyOwnershipLogicBuilder) Build() (result *EmptyOwnershipLogic, err error) {
	// Create the ownership logic:
	result = &EmptyOwnershipLogic{
		logger: b.logger,
	}
	return
}

// DetermineAssignedOwners returns an empty list of owners.
func (p *EmptyOwnershipLogic) DetermineAssignedOwners(_ context.Context) (result []string, err error) {
	return
}
