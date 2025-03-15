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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

var _ = Describe("gRPC ACL authorization", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Can't be built without a logger", func() {
		_, err := NewGrpcAclAuthzFunc().
			Build()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("logger"))
		Expect(err.Error()).To(ContainSubstring("mandatory"))
	})

	It("Can't be built with an ACL file that doesn't exist", func() {
		// Create a file name that doesn't exist:
		dir, err := os.MkdirTemp("", "*.acls")
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.RemoveAll(dir)
			Expect(err).ToNot(HaveOccurred())
		}()
		file := filepath.Join(dir, "bad.yaml")

		// Try to create the wrapper:
		_, err = NewGrpcAclAuthzFunc().
			SetLogger(logger).
			AddAclFile(file).
			Build()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no such file"))
	})

	It("Accepts request that matches ACL", func() {
		// Prepare the ACL:
		acl, err := os.CreateTemp("", "acl-*.yaml")
		Expect(err).ToNot(HaveOccurred())
		_, err = acl.WriteString(`[{
			"claim": "sub",
			"pattern: ^mysubject$"
		}]`)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.Remove(acl.Name())
			Expect(err).ToNot(HaveOccurred())
		}()
		err = acl.Close()
		Expect(err).ToNot(HaveOccurred())

		// Create the function:
		function, err := NewGrpcAclAuthzFunc().
			SetLogger(logger).
			AddAclFile(acl.Name()).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "mysubject",
			Claims: map[string]any{
				"sub": "mysubject",
			},
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Accepts request that matches second of two ACL items", func() {
		// Prepare the ACL:
		acl, err := os.CreateTemp("", "acl-*.yaml")
		Expect(err).ToNot(HaveOccurred())
		_, err = acl.WriteString((`[
			{
				"claim": "sub",
				"pattern": "^mysubject$"
			},
			{
				"claim": "sub",
				"pattern": "^yoursubject$"
			}
		]`))
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.Remove(acl.Name())
			Expect(err).ToNot(HaveOccurred())
		}()
		err = acl.Close()
		Expect(err).ToNot(HaveOccurred())

		// Create the function:
		function, err := NewGrpcAclAuthzFunc().
			SetLogger(logger).
			AddAclFile(acl.Name()).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "yoursubject",
			Claims: map[string]any{
				"sub": "yoursubject",
			},
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Accepts request that matches second of two ACL files", func() {
		// Prepare the ACL files:
		dir, err := os.MkdirTemp("", "acls.*")
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.RemoveAll(dir)
			Expect(err).ToNot(HaveOccurred())
		}()
		file1 := filepath.Join(dir, "acl1.yaml")
		err = os.WriteFile(
			file1,
			[]byte(`[{
				"claim": "sub",
				"pattern": "^mysubject$"
			}]`),
			0600,
		)
		Expect(err).ToNot(HaveOccurred())
		file2 := filepath.Join(dir, "acl2.yaml")
		err = os.WriteFile(
			file2,
			[]byte(`[{
				"claim": "sub",
				"pattern": "^yoursubject$"
			}]`),
			0600,
		)
		Expect(err).ToNot(HaveOccurred())

		// Create the function:
		function, err := NewGrpcAclAuthzFunc().
			SetLogger(logger).
			AddAclFile(file1).
			AddAclFile(file2).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "yoursubject",
			Claims: map[string]any{
				"sub": "yoursubject",
			},
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Rejects request if there is no ACL", func() {
		// Create the function:
		function, err := NewGrpcAclAuthzFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "mysubject",
			Claims: map[string]any{
				"sub": "mysubject",
			},
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_request/MyMethod")
		Expect(err).To(HaveOccurred())
		status, ok := grpcstatus.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(status.Code()).To(Equal(grpccodes.PermissionDenied))
		Expect(status.Message()).To(Equal("Access denied"))
	})

	It("Rejects request if there are ACL items but none matches", func() {
		// Prepare the ACL:
		acl, err := os.CreateTemp("", "acl-*.yaml")
		Expect(err).ToNot(HaveOccurred())
		_, err = acl.WriteString(`[{
			"claim": "sub",
			"pattern": "^mysubject$"
		}]`)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.Remove(acl.Name())
			Expect(err).ToNot(HaveOccurred())
		}()
		err = acl.Close()
		Expect(err).ToNot(HaveOccurred())

		// Create the function:
		function, err := NewGrpcAclAuthzFunc().
			SetLogger(logger).
			AddAclFile(acl.Name()).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "yoursubject",
			Claims: map[string]any{
				"sub": "yoursubject",
			},
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(HaveOccurred())
		status, ok := grpcstatus.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(status.Code()).To(Equal(grpccodes.PermissionDenied))
		Expect(status.Message()).To(Equal("Access denied"))
	})
})
