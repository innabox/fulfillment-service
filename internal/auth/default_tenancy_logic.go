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

// DefaultTenancyLogicBuilder contains the data and logic needed to create default tenancy logic.
type DefaultTenancyLogicBuilder struct {
	logger *slog.Logger
}

// DefaultTenancyLogic is the default implementation of TenancyLogic that extracts the subject from the context
// and returns the subject name as the tenant.
type DefaultTenancyLogic struct {
	logger *slog.Logger
}

// NewDefaultTenancyLogic creates a new builder for default tenancy logic.
func NewDefaultTenancyLogic() *DefaultTenancyLogicBuilder {
	return &DefaultTenancyLogicBuilder{}
}

// SetLogger sets the logger that will be used by the tenancy logic.
func (b *DefaultTenancyLogicBuilder) SetLogger(value *slog.Logger) *DefaultTenancyLogicBuilder {
	b.logger = value
	return b
}

// Build creates the default tenancy logic that extracts the subject from the auth context and returns the identifiers
// of the tenants.
func (b *DefaultTenancyLogicBuilder) Build() (result *DefaultTenancyLogic, err error) {
	// Check that the logger has been set:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}

	// Create the tenancy logic:
	result = &DefaultTenancyLogic{
		logger: b.logger,
	}
	return
}

// DetermineAssignedTenants extracts the subject from the auth context and returns the identifiers of the tenants.
func (p *DefaultTenancyLogic) DetermineAssignedTenants(ctx context.Context) (result []string, err error) {
	// TODO: This should be extracted from the subject. For now, assign objects to the 'shared' tenant, which
	// represents resources accessible to all users.
	result = defaultTenants
	return
}

// DetermineVisibleTenants extracts the subject from the auth context and returns the identifiers of the tenants
// that the current user has permission to see.
func (p *DefaultTenancyLogic) DetermineVisibleTenants(ctx context.Context) (result []string, err error) {
	// TODO: This should be extracted from the subject. For now, allow users to see the 'shared' tenant, which
	// represents resources accessible to all users.
	result = defaultTenants
	return
}

var defaultTenants = []string{
	"shared",
}
