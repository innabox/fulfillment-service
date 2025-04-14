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
	"time"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/ginkgo/v2/dsl/table"
	. "github.com/onsi/gomega"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

var _ = Describe("gRPC rules authorization", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Can't be built without a logger", func() {
		_, err := NewGrpcRulesAuthzFunc().
			Build()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("logger"))
		Expect(err.Error()).To(ContainSubstring("mandatory"))
	})

	It("Can't be built with a rules file that doesn't exist", func() {
		// Create a file name that doesn't exist:
		dir, err := os.MkdirTemp("", "*.acls")
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.RemoveAll(dir)
			Expect(err).ToNot(HaveOccurred())
		}()
		file := filepath.Join(dir, "bad.yaml")

		// Verify that creation fails:
		_, err = NewGrpcRulesAuthzFunc().
			SetLogger(logger).
			AddRulesFile(file).
			Build()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no such file"))
	})

	It("Allows access that matches rule", func() {
		// Prepare the rules file:
		rulesFile, err := os.CreateTemp("", "rules-*.yaml")
		Expect(err).ToNot(HaveOccurred())
		_, err = rulesFile.WriteString(`[{
			"name": "My rule",
			"condition": "true",
			"action": "allow"
		}]`)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.Remove(rulesFile.Name())
			Expect(err).ToNot(HaveOccurred())
		}()
		err = rulesFile.Close()
		Expect(err).ToNot(HaveOccurred())

		// Create the function:
		function, err := NewGrpcRulesAuthzFunc().
			SetLogger(logger).
			AddRulesFile(rulesFile.Name()).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "mysubject",
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_package.my_service/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Allows access that matches second of two rules", func() {
		// Prepare the rules:
		rulesFile, err := os.CreateTemp("", "rules-*.yaml")
		Expect(err).ToNot(HaveOccurred())
		_, err = rulesFile.WriteString((`[
			{
				"name": "First rule",
				"condition": "false",
				"action": "deny"
			},
			{
				"name": "Second rule",
				"condition": "true",
				"action": "allow"
			}
		]`))
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.Remove(rulesFile.Name())
			Expect(err).ToNot(HaveOccurred())
		}()
		err = rulesFile.Close()
		Expect(err).ToNot(HaveOccurred())

		// Create the function:
		function, err := NewGrpcRulesAuthzFunc().
			SetLogger(logger).
			AddRulesFile(rulesFile.Name()).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "yoursubject",
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_package.my_service/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Allows access that matches second of two rule files", func() {
		// Prepare the rule files:
		dir, err := os.MkdirTemp("", "rules.*")
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.RemoveAll(dir)
			Expect(err).ToNot(HaveOccurred())
		}()
		ruleFile1 := filepath.Join(dir, "rules.yaml")
		err = os.WriteFile(
			ruleFile1,
			[]byte(`[{
				"name": "Rule 1",
				"condition": "false",
				"action": "deny"
			}]`),
			0600,
		)
		Expect(err).ToNot(HaveOccurred())
		ruleFile2 := filepath.Join(dir, "rules.yaml")
		err = os.WriteFile(
			ruleFile2,
			[]byte(`[{
				"name": "Rule 2",
				"condition": "true",
				"action": "allow"
			}]`),
			0600,
		)
		Expect(err).ToNot(HaveOccurred())

		// Create the function:
		function, err := NewGrpcRulesAuthzFunc().
			SetLogger(logger).
			AddRulesFile(ruleFile1).
			AddRulesFile(ruleFile2).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "yoursubject",
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_package.my_service/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Denies access if there are no rules", func() {
		// Create the function:
		function, err := NewGrpcRulesAuthzFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "mysubject",
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_request.my_service/MyMethod")
		Expect(err).To(HaveOccurred())
		status, ok := grpcstatus.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(status.Code()).To(Equal(grpccodes.PermissionDenied))
		Expect(status.Message()).To(Equal("Access denied"))
	})

	It("Denies access if there are rules but none matches", func() {
		// Prepare the rules:
		rulesFile, err := os.CreateTemp("", "rules-*.yaml")
		Expect(err).ToNot(HaveOccurred())
		_, err = rulesFile.WriteString(`[{
			"name": "My rule",
			"condition": "false",
			"action": "allow"
		}]`)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.Remove(rulesFile.Name())
			Expect(err).ToNot(HaveOccurred())
		}()
		err = rulesFile.Close()
		Expect(err).ToNot(HaveOccurred())

		// Create the function:
		function, err := NewGrpcRulesAuthzFunc().
			SetLogger(logger).
			AddRulesFile(rulesFile.Name()).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		subject := &Subject{
			Name: "yoursubject",
		}
		ctx = ContextWithSubject(ctx, subject)
		err = function(ctx, "/my_package.my_service/MyMethod")
		Expect(err).To(HaveOccurred())
		status, ok := grpcstatus.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(status.Code()).To(Equal(grpccodes.PermissionDenied))
		Expect(status.Message()).To(Equal("Access denied"))
	})

	DescribeTable(
		"Use of variables",
		func(condition string) {
			// Prepare the rules:
			rulesData := []grpcAuthzRuleData{{
				Name:      "My rule",
				Condition: condition,
				Action:    "allow",
			}}
			rulesBytes, err := yaml.Marshal(rulesData)
			Expect(err).ToNot(HaveOccurred())
			rulesFile, err := os.CreateTemp("", "rules-*.yaml")
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				err := os.Remove(rulesFile.Name())
				Expect(err).ToNot(HaveOccurred())
			}()
			_, err = rulesFile.Write(rulesBytes)
			Expect(err).ToNot(HaveOccurred())
			err = rulesFile.Close()
			Expect(err).ToNot(HaveOccurred())

			// Create the function:
			function, err := NewGrpcRulesAuthzFunc().
				SetLogger(logger).
				AddRulesFile(rulesFile.Name()).
				Build()
			Expect(err).ToNot(HaveOccurred())

			// Verify the result:
			subject := &Subject{
				Name: "mysubject",
				Claims: map[string]any{
					"sub": "mysubject",
					"exp": time.Now().Add(5 * time.Minute),
				},
			}
			ctx = ContextWithSubject(ctx, subject)
			err = function(ctx, "/my_package.my_service/MyMethod")
			Expect(err).ToNot(HaveOccurred())
		},
		Entry(
			"Can use subject name",
			"subject.name == 'mysubject'",
		),
		Entry(
			"Can use string claim",
			"subject.claims['sub'] == 'mysubject'",
		),
		Entry(
			"Can use time claim",
			"timestamp(subject.claims['exp']) > now",
		),
		Entry(
			"Can use string claim",
			"subject.claims['sub'] == 'mysubject'",
		),
		Entry(
			"Can use method",
			"method.endsWith('/MyMethod')",
		),
		Entry(
			"Can use subject name and method simultaneously",
			"method.startsWith('/my_package.') && subject.name == 'mysubject'",
		),
		Entry(
			"Can check if subject has name",
			"has(subject.name)",
		),
		Entry(
			"Can check if subject has claims",
			"has(subject.claims)",
		),
		Entry(
			"Can check if subject has specific claim",
			"'sub' in subject.claims",
		),
	)
})
