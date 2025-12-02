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
	"encoding/json"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/metadata"
)

var _ = Describe("gRPC external authentication function", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Building", func() {
		It("Can be built with all the mandatory parameters", func() {
			function, err := NewGrpcExternalAuthnFunc().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(function).ToNot(BeNil())
		})

		It("Can't be built without a logger", func() {
			_, err := NewGrpcExternalAuthnFunc().
				Build()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("logger"))
			Expect(err.Error()).To(ContainSubstring("mandatory"))
		})
	})

	It("Adds subject from the header to the context", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Set the subject header:
		subject := &Subject{
			User: "my_user",
			Groups: []string{
				"my_first_group",
				"my_second_group",
			},
			Source: SubjectSourceJwt,
		}
		value, err := json.Marshal(subject)
		Expect(err).ToNot(HaveOccurred())
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"X-Subject", string(value),
		))

		// Verify that the subject is added to the context:
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).ToNot(BeNil())
			Expect(subject.User).To(Equal("my_user"))
			Expect(subject.Groups).To(ConsistOf("my_first_group", "my_second_group"))
			Expect(subject.Source).To(Equal(SubjectSourceJwt))
		}).ToNot(Panic())
	})

	It("Doesn't require header for public method", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			AddPublicMethodRegex("^/my_package/.*$").
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the response:
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs())
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Combines multiple public methods", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			AddPublicMethodRegex("^/my_package/.*$").
			AddPublicMethodRegex("^/your_package/.*$").
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the results:
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs())
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		ctx, err = function(ctx, "/your_package/YourMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Adds subject to the context for public methods", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			AddPublicMethodRegex("^/my_package/.*$").
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Set the subject header:
		subject := &Subject{
			User:   "my_user",
			Groups: []string{"my_group"},
		}
		value, err := json.Marshal(subject)
		Expect(err).ToNot(HaveOccurred())
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"X-Subject", string(value),
		))

		// Verify the results:
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).ToNot(BeNil())
			Expect(subject.User).To(Equal("my_user"))
		}).ToNot(Panic())
	})

	It("Fails if there is no header", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the results:
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(MatchError("missing authentication header"))
	})

	It("Fails if header is missing", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the results:
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(MatchError("missing authentication header"))
	})

	It("Fails header has multiple values", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the results:
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"X-Subject", "first value",
			"X-Subject", "second value",
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(MatchError("too many values for authentication header"))
	})

	It("Fails if header can't be decoded", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the results:
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"X-Subject", "junk",
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(MatchError("failed to decode authentication header"))
	})

	It("Fails if subject name is empty", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Set the subject header:
		subject := &Subject{
			User: "",
		}
		value, err := json.Marshal(subject)
		Expect(err).ToNot(HaveOccurred())
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"X-Subject", string(value),
		))

		// Verify the results:
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(MatchError("subject name is empty"))
	})

	It("Trims space from subject name", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Set the subject header:
		subject := &Subject{
			User:   "  \t  my_user\n",
			Groups: []string{"my_group"},
		}
		value, err := json.Marshal(subject)
		Expect(err).ToNot(HaveOccurred())
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"X-Subject", string(value),
		))

		// Verify the results:
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).ToNot(BeNil())
			Expect(subject.User).To(Equal("my_user"))
		}).ToNot(Panic())
	})

	It("Trims space from group name", func() {
		// Create the function:
		function, err := NewGrpcExternalAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Set the subject header:
		subject := &Subject{
			User: "my_user",
			Groups: []string{
				" first_group\n",
				"\tsecond_group ",
			},
		}
		value, err := json.Marshal(subject)
		Expect(err).ToNot(HaveOccurred())
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"X-Subject", string(value),
		))

		// Verify the results:
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).ToNot(BeNil())
			Expect(subject.Groups).To(ConsistOf("first_group", "second_group"))
		}).ToNot(Panic())
	})
})
