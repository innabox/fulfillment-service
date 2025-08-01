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

	"github.com/innabox/fulfillment-service/internal/auth"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ownership function", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Creation", func() {
		It("Succeeds with valid logger", func() {
			function, err := NewOwnershipFunc().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(function).ToNot(BeNil())
		})

		It("Returns error when logger is not set", func() {
			function, err := NewOwnershipFunc().
				Build()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("logger is mandatory"))
			Expect(function).To(BeNil())
		})
	})

	Describe("Behavior", func() {
		It("Returns subject name when subject exists in context", func() {
			function, err := NewOwnershipFunc().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
			subject := &auth.Subject{
				Name: "my_owner",
			}
			ctx = auth.ContextWithSubject(ctx, subject)
			owner := function(ctx)
			Expect(owner).To(Equal("my_owner"))
		})

		It("Panics when there is no subject in the context", func() {
			function, err := NewOwnershipFunc().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(func() {
				function(ctx)
			}).To(Panic())
		})
	})
})
