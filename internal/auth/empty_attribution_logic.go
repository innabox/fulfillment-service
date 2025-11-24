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

	"github.com/innabox/fulfillment-service/internal/collections"
)

// EmptyAttributionLogicBuilder contains the data and logic needed to create empty attribution logic.
type EmptyAttributionLogicBuilder struct {
	logger *slog.Logger
}

// EmptyAttributionLogic is a minimal implementation that returns no creators. This is used as a fallback when no
// attribution logic is configured.
type EmptyAttributionLogic struct {
	logger *slog.Logger
}

// NewEmptyAttributionLogic creates a new builder for empty attribution logic.
func NewEmptyAttributionLogic() *EmptyAttributionLogicBuilder {
	return &EmptyAttributionLogicBuilder{}
}

// SetLogger sets the logger that will be used by the attribution logic.
func (b *EmptyAttributionLogicBuilder) SetLogger(value *slog.Logger) *EmptyAttributionLogicBuilder {
	b.logger = value
	return b
}

// Build creates the empty attribution logic.
func (b *EmptyAttributionLogicBuilder) Build() (result *EmptyAttributionLogic, err error) {
	result = &EmptyAttributionLogic{
		logger: b.logger,
	}
	return
}

// DetermineAssignedCreators returns an empty list of creators.
func (l *EmptyAttributionLogic) DetermineAssignedCreators(_ context.Context) (result collections.Set[string], err error) {
	result = emptyCreators
	return
}

var emptyCreators = collections.NewEmptySet[string]()
