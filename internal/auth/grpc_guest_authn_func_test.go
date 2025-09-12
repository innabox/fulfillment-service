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
	"time"

	. "github.com/innabox/fulfillment-common/testing"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/metadata"
)

var _ = Describe("gRPC guest authentication function", func() {
	var (
		ctx      context.Context
		function GrpcAuthnFunc
	)

	BeforeEach(func() {
		var err error

		ctx = context.Background()

		function, err = NewGrpcGuestAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())
	})

	It("Can't be built without a logger", func() {
		_, err := NewGrpcGuestAuthnFunc().
			Build()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("logger"))
		Expect(err.Error()).To(ContainSubstring("mandatory"))
	})

	It("Adds the guest subject to the context", func() {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs())
		ctx, err := function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).To(BeIdenticalTo(Guest))
		}).ToNot(Panic())
	})

	It("Ignores good authorization header", func() {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"Authorization", "Bad junk",
		))
		_, err := function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Ignores bad authorization header", func() {
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"Authorization", "Bearer "+bearer,
		))
		_, err := function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})
})
