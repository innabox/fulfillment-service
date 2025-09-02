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

// EmptyTenancyLogicBuilder contains the data and logic needed to create empty tenancy logic.
type EmptyTenancyLogicBuilder struct {
	logger *slog.Logger
}

// EmptyTenancyLogic is a minimal implementation that returns no tenants. This is used as a fallback when no
// tenancy logic is configured.
type EmptyTenancyLogic struct {
	logger *slog.Logger
}

// NewEmptyTenancyLogic creates a new builder for empty tenancy logic.
func NewEmptyTenancyLogic() *EmptyTenancyLogicBuilder {
	return &EmptyTenancyLogicBuilder{}
}

// SetLogger sets the logger that will be used by the tenancy logic.
func (b *EmptyTenancyLogicBuilder) SetLogger(value *slog.Logger) *EmptyTenancyLogicBuilder {
	b.logger = value
	return b
}

// Build creates the empty tenancy logic.
func (b *EmptyTenancyLogicBuilder) Build() (result *EmptyTenancyLogic, err error) {
	// Create the tenancy logic:
	result = &EmptyTenancyLogic{
		logger: b.logger,
	}
	return
}

// DetermineAssignedTenants returns an empty list of tenants.
func (p *EmptyTenancyLogic) DetermineAssignedTenants(_ context.Context) (result []string, err error) {
	return
}

// DetermineVisibleTenants returns an empty list of tenants, which means no tenant filtering will be applied.
// This allows access to all objects regardless of their tenant assignment.
func (p *EmptyTenancyLogic) DetermineVisibleTenants(_ context.Context) (result []string, err error) {
	return
}
