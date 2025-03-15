/*
Copyright (c) 2025 Red Hat, Inc.

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

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/ginkgo/v2/dsl/decorators"
	. "github.com/onsi/gomega"
)

var _ = Describe("gRPC all authorization function", func() {
	var (
		ctx      context.Context
		function GrpcAuthzFunc
	)

	BeforeEach(func() {
		var err error

		ctx = context.Background()

		function, err = NewGrpcAllAuthzFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())
	})

	It("Can't be built without a logger", func() {
		_, err := NewGrpcAllAuthzFunc().
			Build()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("logger"))
		Expect(err.Error()).To(ContainSubstring("mandatory"))
	})

	It("Accepts any subject", MustPassRepeatedly(10), func() {
		subject := &Subject{
			Name: uuid.NewString(),
			Claims: map[string]any{
				"sub": uuid.NewString(),
			},
		}
		ctx = ContextWithSubject(ctx, subject)
		err := function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})
})
