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
)

// GrpcAuthnFunc is a function that checks authentication for gRPC calls using the context and the method name. Returns
// a potentially modified context containing the authenticated subject if authentication succeeds, or an error suitable
// for sending directly to the caller if authentication fails.
type GrpcAuthnFunc func(ctx context.Context, method string) (result context.Context, err error)
