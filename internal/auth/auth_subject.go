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

// SubjectSource represents the kind of sources from which subjects are extracted.
type SubjectSource string

const (
	// SubjectSourceJwt indicates the subject was extracted from a JWT token.
	SubjectSourceJwt SubjectSource = "jwt"

	// SubjectSourceServiceAccount indicates the subject was extracted from a Kubernetes service account.
	SubjectSourceServiceAccount SubjectSource = "serviceaccount"

	// SubjectSourceNone indicates the subject represents an unauthenticated guest.
	SubjectSourceNone SubjectSource = "none"
)

// Subject represents an entity, such as person or a service account.
type Subject struct {
	// Source is the source from which the subject was extracted, service account, JSON web token, etc.
	Source SubjectSource `json:"source"`

	// User is the name of the user.
	User string `json:"user"`

	// Groups are the names of the groups that the subject belongs to.
	Groups []string `json:"groups"`
}
