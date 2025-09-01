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

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
)

var _ = Describe("Empty ownership logic", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Creation", func() {
		It("Succeeds without logger", func() {
			logic, err := NewEmptyOwnershipLogic().
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(logic).ToNot(BeNil())
		})

		It("Succeeds with logger", func() {
			logic, err := NewEmptyOwnershipLogic().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(logic).ToNot(BeNil())
		})
	})

	Describe("Behavior", func() {
		It("Returns empty list of owners", func() {
			logic, err := NewEmptyOwnershipLogic().
				Build()
			Expect(err).ToNot(HaveOccurred())
			owners, err := logic.DetermineAssignedOwners(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(owners).To(BeEmpty())
		})
	})
})
