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
	"fmt"
)

// ErrNotFound is an error type that indicates that a requested object doesn't exist.
type ErrNotFound struct {
	// ID is the identifier of the object that was not found.
	ID string
}

// Error returns the error message.
func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("object with identifier '%s' not found", e.ID)
}
