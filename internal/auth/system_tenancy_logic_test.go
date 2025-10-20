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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("System tenancy logic", func() {
	var (
		ctx   context.Context
		logic *SystemTenancyLogic
	)

	BeforeEach(func() {
		var err error

		// Create the context:
		ctx = context.Background()

		// Create the tenancy logic:
		logic, err = NewSystemTenancyLogic().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())
		Expect(logic).ToNot(BeNil())
	})

	It("Returns the shared tenant for assigned tenants", func() {
		result, err := logic.DetermineAssignedTenants(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(ConsistOf("shared"))
	})

	It("Returns an empty list of visible tenants to disable filtering", func() {
		result, err := logic.DetermineVisibleTenants(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(BeEmpty())
	})
})
