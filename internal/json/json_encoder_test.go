/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package json

import (
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/ginkgo/v2/dsl/table"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	sharedv1 "github.com/innabox/fulfillment-service/internal/api/shared/v1"
	testsv1 "github.com/innabox/fulfillment-service/internal/api/tests/v1"
)

var _ = Describe("Encoder", func() {
	Describe("Creation", func() {
		It("Can be created with all the mandatory parameters", func() {
			encoder, err := NewEncoder().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(encoder).ToNot(BeNil())
		})

		It("Can't be created without a logger", func() {
			encoder, err := NewEncoder().
				Build()
			Expect(err).To(MatchError("logger is mandatory"))
			Expect(encoder).To(BeNil())
		})

		It("Can't be created with ignored field of wrong type", func() {
			encoder, err := NewEncoder().
				SetLogger(logger).
				AddIgnoredFields(123).
				Build()
			Expect(err).To(MatchError(
				"ignored fields should be strings or protocol buffers field names, but value 0 is of " +
					"type 'int'",
			))
			Expect(encoder).To(BeNil())
		})
	})

	Describe("Regular encoding", func() {
		var encoder *Encoder

		BeforeEach(func() {
			var err error
			encoder, err = NewEncoder().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
		})

		type Case struct {
			Input    proto.Message
			Expected string
		}

		DescribeTable(
			"Encoding success",
			func(c Case) {
				actual, err := encoder.Marshal(c.Input)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual).To(MatchJSON(c.Expected))
			},
			Entry(
				"Nil",
				Case{
					Input:    nil,
					Expected: `{}`,
				},
			),
			Entry(
				"Empty",
				Case{
					Input:    testsv1.Object_builder{}.Build(),
					Expected: `{}`,
				},
			),
			Entry(
				"True field",
				Case{
					Input: testsv1.Object_builder{
						MyBool: true,
					}.Build(),
					Expected: `{
						"my_bool": true
					}`,
				},
			),
			Entry(
				"False field",
				Case{
					Input: testsv1.Object_builder{
						MyBool: false,
					}.Build(),
					Expected: `{}`,
				},
			),
			Entry(
				"Empty string",
				Case{
					Input: testsv1.Object_builder{
						MyString: "",
					}.Build(),
					Expected: `{}`,
				},
			),
			Entry(
				"Non empty string",
				Case{
					Input: testsv1.Object_builder{
						MyString: "my value",
					}.Build(),
					Expected: `{
						"my_string": "my value"
					}`,
				},
			),
			Entry(
				"Nested string",
				Case{
					Input: testsv1.Object_builder{
						Spec: testsv1.Spec_builder{
							SpecInt32: 123,
						}.Build(),
					}.Build(),
					Expected: `{
						"spec": {
							"spec_int32": 123
						}
					}`,
				},
			),
		)
	})

	Describe("Ignore fields", func() {
		type Case struct {
			Input    proto.Message
			Ignored  []any
			Expected string
		}

		DescribeTable(
			"Encoding success",
			func(c Case) {
				encoder, err := NewEncoder().
					SetLogger(logger).
					AddIgnoredFields(c.Ignored...).
					Build()
				Expect(err).ToNot(HaveOccurred())
				actual, err := encoder.Marshal(c.Input)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual).To(MatchJSON(c.Expected))
			},
			Entry(
				"Ignore one field",
				Case{
					Input: testsv1.Object_builder{
						Metadata: sharedv1.Metadata_builder{
							CreationTimestamp: timestamppb.Now(),
						}.Build(),
					}.Build(),
					Ignored: []any{
						"metadata",
					},
					Expected: `{}`,
				},
			),
			Entry(
				"Ignore two fields",
				Case{
					Input: testsv1.Object_builder{
						Id: "123",
						Metadata: sharedv1.Metadata_builder{
							CreationTimestamp: timestamppb.Now(),
						}.Build(),
					}.Build(),
					Ignored: []any{
						"id",
						"metadata",
					},
					Expected: `{}`,
				},
			),
			Entry(
				"Mix ignored and non ignored fields",
				Case{
					Input: testsv1.Object_builder{
						Id:       "123",
						MyString: "my value",
						MyBool:   true,
					}.Build(),
					Ignored: []any{
						"id",
					},
					Expected: `{
						"my_string": "my value",
						"my_bool": true
					}`,
				},
			),
			Entry(
				"Ignore field by protoreflect.Name",
				Case{
					Input: testsv1.Object_builder{
						Id: "123",
					}.Build(),
					Ignored: []any{
						protoreflect.Name("id"),
					},
					Expected: `{}`,
				},
			),
			Entry(
				"Ignore field by protoreflect.FullName",
				Case{
					Input: testsv1.Object_builder{
						Id: "123",
					}.Build(),
					Ignored: []any{
						protoreflect.FullName("tests.v1.Object.id"),
					},
					Expected: `{}`,
				},
			),
			Entry(
				"Doesn't ignore field if it oesn't match full name",
				Case{
					Input: testsv1.Object_builder{
						Ignore: "don't ignore",
						Spec: testsv1.Spec_builder{
							Ignore: "do ignore",
						}.Build(),
					}.Build(),
					Ignored: []any{
						protoreflect.FullName("tests.v1.Spec.ignore"),
					},
					Expected: `{
						"ignore": "don't ignore",
						"spec": {}
					}`,
				},
			),
			Entry(
				"Converts string to full name if it contains dots",
				Case{
					Input: testsv1.Object_builder{
						Ignore: "don't ignore",
						Spec: testsv1.Spec_builder{
							Ignore: "do ignore",
						}.Build(),
					}.Build(),
					Ignored: []any{
						"tests.v1.Spec.ignore",
					},
					Expected: `{
						"ignore": "don't ignore",
						"spec": {}
					}`,
				},
			),
		)
	})
})
