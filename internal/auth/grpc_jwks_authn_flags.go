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

import "github.com/spf13/pflag"

// AddGrpcJwksAuthnFlags adds the flags related to authentication to the given flag set.
func AddGrpcJwksAuthnFlags(set *pflag.FlagSet) {
	_ = set.StringArray(
		grpcJwksAuthnFileFlagName,
		[]string{},
		"File containing the JSON web key set.",
	)
	_ = set.StringArray(
		grpcJwksAutnUrlFlagName,
		[]string{},
		"URL of the JSON web key set.",
	)
	_ = set.String(
		grpcJwksAuthnTokenFlagName,
		"",
		"Bearer token used to download the JSON web key set.",
	)
	_ = set.String(
		grpcjwksAuthnTokenFileFlagName,
		"",
		"File containing the bearer token used to download the JSON web key set.",
	)
	_ = set.String(
		grpcJwksAuthnCaFileFlagName,
		"",
		"File containing the CA used to verify the TLS certificate of the JSON web "+
			"key set server.",
	)
}

// Names of the flags:
const (
	grpcJwksAuthnFileFlagName      = "grpc-authn-jwks-file"
	grpcJwksAutnUrlFlagName        = "grpc-authn-jwks-url"
	grpcJwksAuthnTokenFlagName     = "grpc-authn-jwks-token"
	grpcjwksAuthnTokenFileFlagName = "grpc-authn-jwks-token-file"
	grpcJwksAuthnCaFileFlagName    = "grpc-authn-jwks-ca-file"
)
