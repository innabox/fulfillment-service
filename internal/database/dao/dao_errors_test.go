/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package dao

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Errors", func() {
	Describe("ErrNotFound", func() {
		It("Implements the error interface", func() {
			var err error = &ErrNotFound{ID: "123"}
			Expect(err).ToNot(BeNil())
		})

		It("Returns expected error message", func() {
			err := &ErrNotFound{ID: "my-id"}
			Expect(err.Error()).To(Equal("object with identifier 'my-id' not found"))
		})
	})

	Describe("ErrAlreadyExists", func() {
		It("Implements the error interface", func() {
			var err error = &ErrAlreadyExists{ID: "123"}
			Expect(err).ToNot(BeNil())
		})

		It("Returns expected error message", func() {
			err := &ErrAlreadyExists{ID: "my-id"}
			Expect(err.Error()).To(Equal("object with identifier 'my-id' already exists"))
		})
	})

	Describe("ErrDenied", func() {
		It("Implements the error interface", func() {
			var err error = &ErrDenied{Reason: "not allowed"}
			Expect(err).ToNot(BeNil())
		})

		It("Returns the Reason field as the error message", func() {
			err := &ErrDenied{Reason: "operation not permitted"}
			Expect(err.Error()).To(Equal("operation not permitted"))
		})

		It("Returns empty string when Reason is empty", func() {
			err := &ErrDenied{}
			Expect(err.Error()).To(BeEmpty())
		})
	})
})
