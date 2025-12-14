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
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// Compute returns a field mask of fields that changed between before and after.
// This is a convenience function that creates a Calculator and calls its Compute method.
// For more control, use NewCalculator().Build().Compute() directly.
func Compute(before, after proto.Message) *fieldmaskpb.FieldMask {
	calculator := NewCalculator().Build()
	return calculator.Compute(before, after)
}
