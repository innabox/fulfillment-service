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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/innabox/fulfillment-service/internal/jq"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	// We need to use these deprecated package because Authorino currently uses version 0 of the Rego language,
	// which has some significant differences. For more details see here:
	//
	// https://github.com/Kuadrant/authorino/issues/546
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

var _ = Describe("Authorization rules", func() {
	var (
		ctx   context.Context
		rules string
	)

	BeforeEach(func() {
		// Create a context:
		ctx = context.Background()

		// Read the Authorino config that contains the rules:
		file := filepath.Join("..", "..", "manifests", "base", "service", "authconfig.yaml")
		bytes, err := os.ReadFile(file)
		Expect(err).ToNot(HaveOccurred())
		var data map[string]any
		err = yaml.Unmarshal(bytes, &data)
		Expect(err).ToNot(HaveOccurred())

		// Extract the rules field:
		tool, err := jq.NewTool().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())
		err = tool.Evaluate(
			".spec.authorization | to_entries[] | .value.opa.rego",
			data, &rules,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(rules).ToNot(BeEmpty())

		// Authorino adds the package name and sets the default value for the allow variable, so we need
		// to do the same.
		buffer := &strings.Builder{}
		fmt.Fprintf(buffer, "package authz\n")
		fmt.Fprintf(buffer, "default allow = false\n")
		buffer.WriteString(rules)
		rules = buffer.String()
	})

	It("Can compile the rules", func() {
		module, err := ast.ParseModule("authz.rego", rules)
		Expect(err).ToNot(HaveOccurred())
		compiler := ast.NewCompiler()
		compiler.Compile(map[string]*ast.Module{
			"authz.rego": module,
		})
		Expect(compiler.Errors).To(BeEmpty())
		Expect(module.Rules).ToNot(BeEmpty())
	})

	It("Allows service account clients to use the public API", func() {
		// Create a rego query to evaluate the rules:
		query, err := rego.New(
			rego.Query("data.authz.allow"),
			rego.Module("authz.rego", rules),
		).PrepareForEval(ctx)
		Expect(err).ToNot(HaveOccurred())

		// Create input that simulates a request from the client service account to create a cluster:
		input := map[string]any{
			"context": map[string]any{
				"request": map[string]any{
					"http": map[string]any{
						"path": "/fulfillment.v1.Clusters/Create",
					},
				},
			},
			"auth": map[string]any{
				"identity": map[string]any{
					"authnMethod": "serviceaccount",
					"user": map[string]any{
						"username": "system:serviceaccount:innabox:client",
					},
				},
			},
		}

		// Evaluate the rules:
		results, err := query.Eval(ctx, rego.EvalInput(input))
		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(HaveLen(1))

		// Check that the result allows the request:
		allow, ok := results[0].Expressions[0].Value.(bool)
		Expect(ok).To(BeTrue())
		Expect(allow).To(BeTrue())
	})

	It("Allows JWT clients to use the public API", func() {
		// Create a rego query to evaluate the rules:
		query, err := rego.New(
			rego.Query("data.authz.allow"),
			rego.Module("authz.rego", rules),
		).PrepareForEval(ctx)
		Expect(err).ToNot(HaveOccurred())

		// Create input that simulates a request from the JWT client to create a cluster:
		input := map[string]any{
			"context": map[string]any{
				"request": map[string]any{
					"http": map[string]any{
						"path": "/fulfillment.v1.Clusters/Create",
					},
				},
			},
			"auth": map[string]any{
				"identity": map[string]any{
					"authnMethod": "jwt",
					"user": map[string]any{
						"preferred_username": "my_user",
					},
				},
			},
		}

		// Evaluate the rules:
		results, err := query.Eval(ctx, rego.EvalInput(input))
		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(HaveLen(1))

		// Check that the result allows the request:
		allow, ok := results[0].Expressions[0].Value.(bool)
		Expect(ok).To(BeTrue())
		Expect(allow).To(BeTrue())
	})

	It("Doesn't allow service account clients to use the private API", func() {
		// Create a rego query to evaluate the rules:
		query, err := rego.New(
			rego.Query("data.authz.allow"),
			rego.Module("authz.rego", rules),
		).PrepareForEval(ctx)
		Expect(err).ToNot(HaveOccurred())

		// Create input that simulates a request from the client service account to create a cluster:
		input := map[string]any{
			"context": map[string]any{
				"request": map[string]any{
					"http": map[string]any{
						"path": "/private.v1.Clusters/Create",
					},
				},
			},
			"auth": map[string]any{
				"identity": map[string]any{
					"authnMethod": "serviceaccount",
					"user": map[string]any{
						"username": "system:serviceaccount:innabox:client",
					},
				},
			},
		}

		// Evaluate the rules:
		results, err := query.Eval(ctx, rego.EvalInput(input))
		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(HaveLen(1))

		// Check that the result doesn't allow the request:
		allow, ok := results[0].Expressions[0].Value.(bool)
		Expect(ok).To(BeTrue())
		Expect(allow).To(BeFalse())
	})

	It("Doesn't allow JWT clients to use the private API", func() {
		// Create a rego query to evaluate the rules:
		query, err := rego.New(
			rego.Query("data.authz.allow"),
			rego.Module("authz.rego", rules),
		).PrepareForEval(ctx)
		Expect(err).ToNot(HaveOccurred())

		// Create input that simulates a request from the JWT client to create a cluster:
		input := map[string]any{
			"context": map[string]any{
				"request": map[string]any{
					"http": map[string]any{
						"path": "/private.v1.Clusters/Create",
					},
				},
			},
			"auth": map[string]any{
				"identity": map[string]any{
					"authnMethod": "jwt",
					"user": map[string]any{
						"preferred_username": "my_user",
					},
				},
			},
		}

		// Evaluate the rules:
		results, err := query.Eval(ctx, rego.EvalInput(input))
		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(HaveLen(1))

		// Check that the result doesn't allow the request:
		allow, ok := results[0].Expressions[0].Value.(bool)
		Expect(ok).To(BeTrue())
		Expect(allow).To(BeFalse())
	})
})
