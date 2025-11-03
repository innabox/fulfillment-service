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
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"

	"github.com/spf13/pflag"
	"google.golang.org/grpc/metadata"
)

// GrpcExternalAuthnType is the name of the external authentication function.
const GrpcExternalAuthnType = "external"

// GrpcExternalAuthnFuncBuilder is a gRPC authentication function that gets the subject from the `X-Subject` header,
// which should contain a JSON document with the user name and groups, like this:
//
//	{
//		"user": "jane.doe"
//		"groups": [
//			"accounting",
//			"admin"
//		]
//	}
type GrpcExternalAuthnFuncBuilder struct {
	logger        *slog.Logger
	publicMethods []string
}

// grpcExternalAuthnFunc contains the data needed by the function.
type grpcExternalAuthnFunc struct {
	logger        *slog.Logger
	publicMethods []*regexp.Regexp
}

// NewGrpcExternalAuthnFunc creates a builder that can then be used to configure and create a new gRPC authentication
// function.
func NewGrpcExternalAuthnFunc() *GrpcExternalAuthnFuncBuilder {
	return &GrpcExternalAuthnFuncBuilder{}
}

// SetLogger sets the logger that will be used to write to the log. This is mandatory.
func (b *GrpcExternalAuthnFuncBuilder) SetLogger(value *slog.Logger) *GrpcExternalAuthnFuncBuilder {
	b.logger = value
	return b
}

// AddPublicMethodRegex adds a regular expression that describes a sets of methods that are considered public, and
// therefore require no authentication. The regular expression will be matched against to the full gRPC method name,
// including the leading slash. For example, to consider public all the methods of the `example.v1.Products` service
// the regular expression could be `^/example\.v1\.Products/.*$`.
//
// This method may be called multiple times to add multiple regular expressions. A method will be considered public if
// it matches at least one of them.
func (b *GrpcExternalAuthnFuncBuilder) AddPublicMethodRegex(value string) *GrpcExternalAuthnFuncBuilder {
	b.publicMethods = append(b.publicMethods, value)
	return b
}

// SetFlags sets the command line flags that should be used to configure the function. This is optional.
func (b *GrpcExternalAuthnFuncBuilder) SetFlags(flags *pflag.FlagSet) *GrpcExternalAuthnFuncBuilder {
	// There are no flags for this function currently.
	return b
}

// Build uses the data stored in the builder to create and configure a new gRPC guest authentication function.
func (b *GrpcExternalAuthnFuncBuilder) Build() (result GrpcAuthnFunc, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Add the name of the header to the logger:
	logger := b.logger.With(slog.String("header", grpcExternalAuthnSubjectHeader))

	// Try to compile the regular expressions that define the set of public methods:
	publicMethods := make([]*regexp.Regexp, len(b.publicMethods))
	for i, expr := range b.publicMethods {
		publicMethods[i], err = regexp.Compile(expr)
		if err != nil {
			return
		}
	}

	// Create and populate the object:
	object := &grpcExternalAuthnFunc{
		logger:        logger,
		publicMethods: publicMethods,
	}
	result = object.call
	return
}

// call is the implementation of the `GrpcAuthnFunc` type.
func (f *grpcExternalAuthnFunc) call(ctx context.Context, method string) (result context.Context, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		f.logger.ErrorContext(ctx, "Context doesn't contain metadata")
		err = errors.New("missing authentication header")
		return
	}
	values := md.Get(grpcExternalAuthnSubjectHeader)
	count := len(values)
	if count == 0 && f.isPublicMethod(method) {
		result = ContextWithSubject(ctx, Guest)
		return
	}
	if count != 1 {
		f.logger.ErrorContext(
			ctx,
			"Expected exactly one value for the subject header",
			slog.Any("values", values),
		)
		if count == 0 {
			err = errors.New("missing authentication header")
		} else {
			err = errors.New("too many values for authentication header")
		}
		return
	}
	value := values[0]
	subject := &Subject{}
	err = json.Unmarshal([]byte(value), subject)
	if err != nil {
		f.logger.ErrorContext(
			ctx,
			"Failed to umarshal subject header",
			slog.String("value", value),
			slog.Any("error", err),
		)
		err = errors.New("failed to decode authentication header")
		return
	}
	subject.User = strings.TrimSpace(subject.User)
	for i, group := range subject.Groups {
		subject.Groups[i] = strings.TrimSpace(group)
	}
	if len(subject.User) == 0 {
		f.logger.ErrorContext(
			ctx,
			"Subject name from authentication header is empty",
			slog.String("value", value),
			slog.Any("subject", subject),
		)
		err = errors.New("subject name is empty")
		return
	}
	f.logger.DebugContext(
		ctx,
		"Extraced subject from header",
		slog.Any("subject", subject),
	)
	result = ContextWithSubject(ctx, subject)
	return
}

func (f *grpcExternalAuthnFunc) isPublicMethod(method string) bool {
	for _, publicMethod := range f.publicMethods {
		if publicMethod.MatchString(method) {
			return true
		}
	}
	return false
}

// grpcExternalAuthnSubjectHeader is the name of the header that should contain the subject data.
const grpcExternalAuthnSubjectHeader = "x-subject"
