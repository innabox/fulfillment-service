/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package utils

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

const (
	// maxLabelValueLength is the maximum length for a Kubernetes label value
	maxLabelValueLength = 63
	// HashSuffixLength is the length reserved for the hash suffix to differentiate strings
	HashSuffixLength = 8
	// maxNormalizedLength is the maximum length for the normalized part before appending the hash
	maxNormalizedLength = maxLabelValueLength - HashSuffixLength
)

// AsDNSName normalizes a string as a DNS name that is compatible with Kubernetes naming constraints.
// The result conforms to Kubernetes label value requirements:
// - Must be 63 characters or less
// - Must be empty or consist of alphanumeric characters, '-', '_' or '.'
// - Must start and end with an alphanumeric character (if not empty)
//
// Invalid characters are replaced with '-', and the result is truncated to leave space for a hash suffix.
// A hash suffix is always appended to differentiate strings, even if they have the same normalized prefix.
func AsDNSName(input string) string {
	if input == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(input))

	// First pass: replace invalid characters and build the string
	for _, r := range input {
		if unicode.IsLetter(r) {
			result.WriteRune(unicode.ToLower(r))
		} else if unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			result.WriteRune(r)
		} else {
			// Replace invalid characters with '-'
			result.WriteRune('-')
		}
	}

	normalized := result.String()

	// Remove leading non-alphanumeric characters
	normalized = strings.TrimLeftFunc(normalized, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	// Always calculate hash of original input for differentiation
	hash := sha256.Sum256([]byte(input))
	hashSuffix := fmt.Sprintf("%x", hash)[:HashSuffixLength]

	// If after trimming we have an empty string, use default value
	if normalized == "" {
		normalized = "x"
	}

	// Truncate to leave space for hash suffix (always append hash)
	if len(normalized) > maxNormalizedLength {
		normalized = normalized[:maxNormalizedLength]
	}

	// Always append hash suffix
	normalized = normalized + hashSuffix

	return normalized
}

// ListAsDNSName normalizes a list of strings as a DNS name that is compatible with Kubernetes naming constraints.
// The input list is sorted (without mutation), joined together, and then normalized using AsDNSName.
// Returns a single normalized DNS name string.
func ListAsDNSName(inputs []string) string {
	if len(inputs) == 0 {
		return ""
	}

	// Create a copy and sort it without mutating the input
	sorted := make([]string, len(inputs))
	copy(sorted, inputs)
	slices.Sort(sorted)

	// Join the sorted strings with a dot
	joined := strings.Join(sorted, ".")

	// Normalize the joined string
	return AsDNSName(joined)
}
