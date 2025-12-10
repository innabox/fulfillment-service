/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package masks

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// Compute returns a field mask of fields that changed between before and after.
// This prevents updates from overwriting concurrent changes to fields not being modified.
// Uses deep recursive comparison to detect changes at any level of nesting.
func Compute(before, after proto.Message) *fieldmaskpb.FieldMask {
	paths := compareMessages(before.ProtoReflect(), after.ProtoReflect(), "")
	return &fieldmaskpb.FieldMask{Paths: paths}
}

// compareMessages recursively compares two protobuf messages and returns paths of changed fields.
func compareMessages(before, after protoreflect.Message, prefix string) []string {
	var paths []string

	// Iterate through all fields in the 'before' message
	before.Range(func(fd protoreflect.FieldDescriptor, beforeVal protoreflect.Value) bool {
		afterVal := after.Get(fd)

		// Build the field path (e.g., "status.state" or "metadata.finalizers")
		fieldPath := string(fd.Name())
		if prefix != "" {
			fieldPath = prefix + "." + fieldPath
		}

		// Handle different field types
		switch {
		case fd.IsMap():
			// Compare map fields
			if !compareMaps(beforeVal.Map(), afterVal.Map(), fd) {
				paths = append(paths, fieldPath)
			}

		case fd.IsList():
			// Compare list/repeated fields
			if !compareLists(beforeVal.List(), afterVal.List(), fd) {
				paths = append(paths, fieldPath)
			}

		case fd.Message() != nil:
			// Recursively compare message fields for granularity
			subPaths := compareMessages(beforeVal.Message(), afterVal.Message(), fieldPath)
			if len(subPaths) > 0 {
				paths = append(paths, subPaths...)
			}

		default:
			// Compare scalar fields (string, int32, int64, bool, enum, bytes, float, double, etc.)
			if !beforeVal.Equal(afterVal) {
				paths = append(paths, fieldPath)
			}
		}

		return true
	})

	// Check for fields that exist in 'after' but not in 'before' (newly set fields)
	after.Range(func(fd protoreflect.FieldDescriptor, afterVal protoreflect.Value) bool {
		if !before.Has(fd) {
			fieldPath := string(fd.Name())
			if prefix != "" {
				fieldPath = prefix + "." + fieldPath
			}

			// For message fields, recursively get all changed subfields
			if fd.Message() != nil && !fd.IsMap() && !fd.IsList() {
				// Create an empty message of the same type
				emptyMsg := afterVal.Message().New()
				subPaths := compareMessages(emptyMsg, afterVal.Message(), fieldPath)
				if len(subPaths) > 0 {
					paths = append(paths, subPaths...)
				} else {
					paths = append(paths, fieldPath)
				}
			} else {
				paths = append(paths, fieldPath)
			}
		}
		return true
	})

	return paths
}

// compareLists compares two protobuf list fields element by element.
// Handles both scalar and message element types.
func compareLists(a, b protoreflect.List, fd protoreflect.FieldDescriptor) bool {
	if a.Len() != b.Len() {
		return false
	}

	for i := 0; i < a.Len(); i++ {
		aVal := a.Get(i)
		bVal := b.Get(i)

		// For message elements, use proto.Equal for deep comparison
		if fd.Message() != nil {
			if !proto.Equal(aVal.Message().Interface(), bVal.Message().Interface()) {
				return false
			}
		} else {
			// For scalar elements (string, int, bool, bytes, etc.), use direct comparison
			if !aVal.Equal(bVal) {
				return false
			}
		}
	}

	return true
}

// compareMaps compares two protobuf map fields key by key.
// Handles both scalar and message value types.
func compareMaps(a, b protoreflect.Map, fd protoreflect.FieldDescriptor) bool {
	if a.Len() != b.Len() {
		return false
	}

	equal := true
	a.Range(func(k protoreflect.MapKey, aVal protoreflect.Value) bool {
		// Check if key exists in both maps
		if !b.Has(k) {
			equal = false
			return false
		}

		bVal := b.Get(k)

		// For message values, use proto.Equal for deep comparison
		if fd.MapValue().Message() != nil {
			if !proto.Equal(aVal.Message().Interface(), bVal.Message().Interface()) {
				equal = false
				return false
			}
		} else {
			// For scalar values (string, int, bool, bytes, etc.), use direct comparison
			if !aVal.Equal(bVal) {
				equal = false
				return false
			}
		}

		return true
	})

	return equal
}
