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

	"github.com/innabox/fulfillment-service/internal/collections"
)

// TenancyLogic defines the logic for determining object tenancy and access control.
//
//go:generate mockgen -destination=tenancy_logic_mock.go -package=auth . TenancyLogic
type TenancyLogic interface {
	// DetermineAssignedTenants calculates and returns the list of tenant names that should be assigned to an object
	// being created. The context will contain authentication and authorization information that can be used to
	// determine the appropriate tenants.
	DetermineAssignedTenants(ctx context.Context) (collections.Set[string], error)

	// DetermineVisibleTenants calculates and returns the list of tenant names that the current user has permission
	// to see. Database queries will be filtered to only return objects where the tenants column has a non-empty
	// intersection with the values returned by this method.
	DetermineVisibleTenants(ctx context.Context) (collections.Set[string], error)
}

// SharedTenants is the set of tenants that are always visible to all users.
var SharedTenants = collections.NewSet("shared")

// AllTenants is the set of all tenants that are possible.
var AllTenants = collections.NewUniversal[string]()
