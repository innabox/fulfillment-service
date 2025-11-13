/*
Copyright (c) 2025 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package utils

import (
	"strings"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation"
)

var _ = Describe("AsDNSName", func() {
	Context("with valid input", func() {
		It("should append hash suffix to lowercase alphanumeric strings", func() {
			result := AsDNSName("abc123")
			Expect(result).To(HavePrefix("abc123"))
			Expect(len(result)).To(BeNumerically("<=", 63))
			// Hash is always appended, so result will be longer than input if input is short
			if len("abc123")+HashSuffixLength <= 63 {
				Expect(len(result)).To(Equal(len("abc123") + HashSuffixLength))
			}

			result2 := AsDNSName("my-label-value")
			Expect(result2).To(HavePrefix("my-label-value"))
			Expect(len(result2)).To(BeNumerically("<=", 63))
		})

		It("should convert uppercase to lowercase and append hash", func() {
			result := AsDNSName("ABC123")
			Expect(result).To(HavePrefix("abc123"))
			Expect(len(result)).To(BeNumerically("<=", 63))

			result2 := AsDNSName("My-Label-Value")
			Expect(result2).To(HavePrefix("my-label-value"))
			Expect(len(result2)).To(BeNumerically("<=", 63))
		})

		It("should handle mixed valid characters and append hash", func() {
			result := AsDNSName("abc-123_def.456")
			Expect(result).To(HavePrefix("abc-123_def.456"))
			Expect(len(result)).To(BeNumerically("<=", 63))
		})
	})

	Context("with invalid characters", func() {
		It("should replace invalid characters with hyphens and append hash", func() {
			result := AsDNSName("abc@123")
			Expect(result).To(HavePrefix("abc-123"))
			Expect(len(result)).To(BeNumerically("<=", 63))

			result2 := AsDNSName("abc#123")
			Expect(result2).To(HavePrefix("abc-123"))
			Expect(len(result2)).To(BeNumerically("<=", 63))
		})

		It("should handle multiple consecutive invalid characters and append hash", func() {
			result := AsDNSName("abc@@@123")
			Expect(result).To(HavePrefix("abc---123"))
			Expect(len(result)).To(BeNumerically("<=", 63))
		})
	})

	Context("with leading and trailing invalid characters", func() {
		It("should trim leading non-alphanumeric characters and append hash", func() {
			result := AsDNSName("-abc123")
			Expect(result).To(HavePrefix("abc123"))
			Expect(len(result)).To(BeNumerically("<=", 63))

			result2 := AsDNSName("_abc123")
			Expect(result2).To(HavePrefix("abc123"))
		})

		It("should trim trailing non-alphanumeric characters and append hash", func() {
			result := AsDNSName("abc123-")
			Expect(result).To(HavePrefix("abc123"))
			Expect(len(result)).To(BeNumerically("<=", 63))

			result2 := AsDNSName("abc123_")
			Expect(result2).To(HavePrefix("abc123"))
		})

		It("should trim both leading and trailing non-alphanumeric characters and append hash", func() {
			result := AsDNSName("-abc123-")
			Expect(result).To(HavePrefix("abc123"))
			Expect(len(result)).To(BeNumerically("<=", 63))

			result2 := AsDNSName("@abc123@")
			Expect(result2).To(HavePrefix("abc123"))
		})
	})

	Context("with length constraints", func() {
		It("should always append hash suffix", func() {
			shortString := strings.Repeat("a", 10)
			result := AsDNSName(shortString)
			Expect(result).To(HavePrefix(shortString))
			Expect(len(result)).To(Equal(len(shortString) + HashSuffixLength))
		})

		It("should truncate long strings and append hash", func() {
			longString := strings.Repeat("a", 100)
			result := AsDNSName(longString)
			Expect(len(result)).To(Equal(63))
			Expect(result).To(HavePrefix(strings.Repeat("a", 55)))
			hashSuffix := result[55:]
			Expect(hashSuffix).To(MatchRegexp("^[a-z0-9]{8}$"))
		})

		It("should differentiate long strings with same prefix using hash", func() {
			string1 := strings.Repeat("a", 100) + "different-suffix-1"
			string2 := strings.Repeat("a", 100) + "different-suffix-2"

			result1 := AsDNSName(string1)
			result2 := AsDNSName(string2)

			Expect(result1).To(HavePrefix(strings.Repeat("a", 55)))
			Expect(result2).To(HavePrefix(strings.Repeat("a", 55)))
			Expect(result1[55:]).ToNot(Equal(result2[55:]))
			Expect(len(result1)).To(Equal(63))
			Expect(len(result2)).To(Equal(63))
		})

		It("should generate different DNS names for strings with same first 63 characters", func() {
			// Two strings that have the same first 63 characters but differ after
			prefix := strings.Repeat("a", 63)
			string1 := prefix + "suffix1"
			string2 := prefix + "suffix2"

			result1 := AsDNSName(string1)
			result2 := AsDNSName(string2)

			// Both should be truncated to 55 chars + 8 char hash = 63 total
			Expect(len(result1)).To(Equal(63))
			Expect(len(result2)).To(Equal(63))
			// Both should have the same normalized prefix (55 'a's)
			Expect(result1).To(HavePrefix(strings.Repeat("a", 55)))
			Expect(result2).To(HavePrefix(strings.Repeat("a", 55)))
			// But different hash suffixes because the original inputs differ
			Expect(result1[55:]).ToNot(Equal(result2[55:]), "Strings with same first 63 chars but different overall should have different hash suffixes")
		})
	})

	Context("with edge cases", func() {
		It("should return empty string for empty input", func() {
			Expect(AsDNSName("")).To(Equal(""))
		})

		It("should handle strings that become empty after trimming", func() {
			result := AsDNSName("@@@")
			Expect(result).To(HavePrefix("x"))
			Expect(len(result)).To(Equal(1 + HashSuffixLength))
		})

		It("should handle single character inputs", func() {
			result := AsDNSName("a")
			Expect(result).To(HavePrefix("a"))
			Expect(len(result)).To(Equal(1 + HashSuffixLength))

			result2 := AsDNSName("1")
			Expect(result2).To(HavePrefix("1"))
			Expect(len(result2)).To(Equal(1 + HashSuffixLength))

			result3 := AsDNSName("-")
			Expect(result3).To(HavePrefix("x"))
			Expect(len(result3)).To(Equal(1 + HashSuffixLength))
		})
	})

	Context("with real-world examples", func() {
		It("should normalize UUIDs and append hash", func() {
			uuid := "550e8400-e29b-41d4-a716-446655440000"
			result := AsDNSName(uuid)
			Expect(result).To(HavePrefix("550e8400-e29b-41d4-a716-446655440000"))
			// UUID is 36 chars, so result should be 36 + 8 = 44 chars
			Expect(len(result)).To(Equal(36 + HashSuffixLength))
		})

		It("should normalize tenant IDs with special characters and append hash", func() {
			tenantID := "tenant@example.com"
			result := AsDNSName(tenantID)
			// '@' is replaced with '-', but '.' is valid and preserved
			Expect(result).To(HavePrefix("tenant-example.com"))
			Expect(len(result)).To(BeNumerically("<=", 63))
		})

		It("should normalize paths and append hash", func() {
			path := "/path/to/resource"
			result := AsDNSName(path)
			Expect(result).To(HavePrefix("path-to-resource"))
			Expect(len(result)).To(BeNumerically("<=", 63))
		})

		It("should normalize strings with spaces and append hash", func() {
			spaced := "my resource name"
			result := AsDNSName(spaced)
			Expect(result).To(HavePrefix("my-resource-name"))
			Expect(len(result)).To(BeNumerically("<=", 63))
		})
	})

	Context("with Unicode characters", func() {
		It("should handle Unicode letters and append hash", func() {
			// Unicode letters should be converted to lowercase
			result := AsDNSName("Café")
			Expect(result).To(HavePrefix("café"))
			Expect(len(result)).To(Equal(len("café") + HashSuffixLength))
		})

		It("should replace non-ASCII invalid characters and append hash", func() {
			result := AsDNSName("test©value")
			Expect(result).To(HavePrefix("test-value"))
			Expect(len(result)).To(BeNumerically("<=", 63))
		})
	})

	Context("validation against Kubernetes standards", func() {
		It("should produce values that pass Kubernetes validation", func() {
			testCases := []string{
				"abc123",
				"ABC123",
				"my-label-value",
				"my_label_value",
				"my.label.value",
				"abc@123",
				"tenant@example.com",
				"/path/to/resource",
				"my resource name",
				strings.Repeat("a", 100),
				"---",
				"@@@",
			}

			for _, input := range testCases {
				result := AsDNSName(input)
				// Verify the result passes Kubernetes validation
				errs := validation.IsValidLabelValue(result)
				Expect(errs).To(BeEmpty(), "AsDNSName(%q) = %q should be valid, but got errors: %v", input, result, errs)
			}
		})
	})
})

var _ = Describe("ListAsDNSName", func() {
	Context("with empty list", func() {
		It("should return empty string", func() {
			result := ListAsDNSName([]string{})
			Expect(result).To(Equal(""))
		})

		It("should return empty string for nil input", func() {
			result := ListAsDNSName(nil)
			Expect(result).To(Equal(""))
		})
	})

	Context("with single element", func() {
		It("should normalize the single element", func() {
			result := ListAsDNSName([]string{"Test@String"})
			Expect(result).To(HavePrefix("test-string"))
		})
	})

	Context("with multiple elements", func() {
		It("should sort, join, and normalize", func() {
			input := []string{"zebra", "alpha", "beta"}
			result := ListAsDNSName(input)

			// Verify input is not mutated
			Expect(input).To(Equal([]string{"zebra", "alpha", "beta"}))

			// Result should be the normalized joined string: "alphabetazebra" (sorted)
			// The exact result depends on the hash, but it should contain the normalized parts
			Expect(len(result)).To(BeNumerically("<=", 63))
			// Verify it's a valid DNS name
			errs := validation.IsValidLabelValue(result)
			Expect(errs).To(BeEmpty())
		})

		It("should handle strings with special characters", func() {
			input := []string{"test@example.com", "user#123", "path/to/resource"}
			result := ListAsDNSName(input)

			// After sorting and joining: "path/to/resourcetest@example.comuser#123"
			// Then normalized
			Expect(len(result)).To(BeNumerically("<=", 63))
			errs := validation.IsValidLabelValue(result)
			Expect(errs).To(BeEmpty())
		})

		It("should produce consistent results for same input", func() {
			input := []string{"test", "alpha", "test", "beta"}
			result1 := ListAsDNSName(input)
			result2 := ListAsDNSName(input)

			Expect(result1).To(Equal(result2))
			Expect(len(result1)).To(BeNumerically("<=", 63))
		})

		It("should handle empty strings in list", func() {
			input := []string{"test", "", "alpha"}
			result := ListAsDNSName(input)

			// After sorting: "", "alpha", "test"
			// Joined: "alphatest" (empty string doesn't contribute)
			Expect(len(result)).To(BeNumerically("<=", 63))
			errs := validation.IsValidLabelValue(result)
			Expect(errs).To(BeEmpty())
		})

		It("should produce same result regardless of input order", func() {
			input1 := []string{"alpha", "beta"}
			input2 := []string{"beta", "alpha"}

			result1 := ListAsDNSName(input1)
			result2 := ListAsDNSName(input2)

			Expect(result1).To(Equal(result2))
		})

		It("should handle strings that exceed 63 characters when joined", func() {
			// Create strings that when joined with dots exceed 63 characters
			input := []string{
				strings.Repeat("a", 30),
				strings.Repeat("b", 30),
				strings.Repeat("c", 30),
			}
			// Joined: "aaa...bbb...ccc..." = 90+ chars, then normalized
			result := ListAsDNSName(input)

			// Result should be truncated to 63 characters (55 + 8 char hash)
			Expect(len(result)).To(Equal(63))
			errs := validation.IsValidLabelValue(result)
			Expect(errs).To(BeEmpty())
		})
	})
})
