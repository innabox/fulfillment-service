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

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/spf13/pflag"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

// GrpcAclAuthzType is the name of the guest authentication function.
const GrpcAclAuthzType = "acl"

// GrpcAclAuthzFuncBuilder contains the data and logic needed to create an authorization function that checks if the
// subject from the context matches a set of rules in a simple access control list.
type GrpcAclAuthzFuncBuilder struct {
	logger        *slog.Logger
	publicMethods []string
	aclFiles      []string
}

type grpcAclAuthzFunc struct {
	logger        *slog.Logger
	publicMethods []*regexp.Regexp
	aclItems      []grpcAclAuthzItem
}

type grpcAclAuthzItem struct {
	Claim   string
	Pattern *regexp.Regexp
}

// NewGrpcAclAuthzFunc creates a builder that can then be configured and used to create a new authorization function.
func NewGrpcAclAuthzFunc() *GrpcAclAuthzFuncBuilder {
	return &GrpcAclAuthzFuncBuilder{}
}

// SetLogger sets the logger that the function will use to write messages to the log. This is mandatory.
func (b *GrpcAclAuthzFuncBuilder) SetLogger(value *slog.Logger) *GrpcAclAuthzFuncBuilder {
	b.logger = value
	return b
}

// AddPublicMethodRegex adds a regular expression that describes a sets of methods that are considered public, and
// therefore require no authorization. The regular expression will be matched against to the full gRPC method name,
// including the leading slash. For example, to consider public all the methods of the `example.v1.Products` service
// the regular expression could be `^/example\.v1\.Products/.*$`.
//
// This method may be called multiple times to add multiple regular expressions. A method will be considered public if
// it matches at least one of them.
func (b *GrpcAclAuthzFuncBuilder) AddPublicMethodRegex(value string) *GrpcAclAuthzFuncBuilder {
	b.publicMethods = append(b.publicMethods, value)
	return b
}

// SetFlags sets the command line flags that should be used to configure the function. This is optional.
func (b *GrpcAclAuthzFuncBuilder) SetFlags(flags *pflag.FlagSet) *GrpcAclAuthzFuncBuilder {
	if flags == nil {
		return b
	}

	if flags.Changed(grpcAclAuthzFileFlagName) {
		values, err := flags.GetStringArray(grpcAclAuthzFileFlagName)
		if err == nil {
			for _, value := range values {
				b.AddAclFile(value)
			}
		}
	}

	return b
}

// AddAclFile adds a file that contains items of the access control list. This should be a YAML file with the following
// format:
//
//   - claim: email
//     pattern: ^.*@redhat\.com$
//
//   - claim: sub
//     pattern: ^f:b3f7b485-7184-43c8-8169-37bd6d1fe4aa:myuser$
//
// The claim field is the name of the claim of the subject that will be checked. The pattern field is a regular
// expression. If the claim matches at least one of the regular expression then access will be allowed, otherwise it
// will be denied.
func (b *GrpcAclAuthzFuncBuilder) AddAclFile(value string) *GrpcAclAuthzFuncBuilder {
	if value != "" {
		b.aclFiles = append(b.aclFiles, value)
	}
	return b
}

// Build uses the data stored in the builder to create a new authorization function.
func (b *GrpcAclAuthzFuncBuilder) Build() (result GrpcAuthzFunc, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}

	// Load the ACL files:
	var aclItems []grpcAclAuthzItem
	for _, file := range b.aclFiles {
		var fileItems []grpcAclAuthzItem
		fileItems, err = b.loadAclFile(file)
		if err != nil {
			return
		}
		aclItems = append(aclItems, fileItems...)
	}

	// Try to compile the regular expressions that define the set of public methods:
	publicMethods := make([]*regexp.Regexp, len(b.publicMethods))
	for i, expr := range b.publicMethods {
		publicMethods[i], err = regexp.Compile(expr)
		if err != nil {
			return
		}
	}

	// Create and populate the object:
	object := &grpcAclAuthzFunc{
		logger:        b.logger,
		publicMethods: publicMethods,
		aclItems:      aclItems,
	}
	result = object.call

	return
}

// grpcAclAuthzItemYaml is the type used to read a single ACL item from a YAML document.
type grpcAclAuthzItemYaml struct {
	Claim   string `yaml:"claim"`
	Pattern string `yaml:"pattern"`
}

// loadAclFile loads the given ACL file into the given map of ACL items.
func (b *GrpcAclAuthzFuncBuilder) loadAclFile(file string) (items []grpcAclAuthzItem, err error) {
	// Load the YAML data:
	yamlData, err := os.ReadFile(file)
	if err != nil {
		return
	}

	// Parse the YAML data:
	var listData []grpcAclAuthzItemYaml
	err = yaml.Unmarshal(yamlData, &listData)
	if err != nil {
		return
	}

	// Process the items:
	for _, itemData := range listData {
		var itemRe *regexp.Regexp
		itemRe, err = regexp.Compile(itemData.Pattern)
		if err != nil {
			return
		}
		items = append(items, grpcAclAuthzItem{
			Claim:   itemData.Claim,
			Pattern: itemRe,
		})
	}

	return
}

func (f *grpcAclAuthzFunc) call(ctx context.Context, method string) error {
	if f.isPublicMethod(method) {
		return nil
	}
	subject := SubjectFromContext(ctx)
	if !f.checkAcl(subject.Claims) {
		f.logger.InfoContext(
			ctx,
			"Access denied",
			slog.String("subject", subject.Name),
			slog.Any("claims", subject.Claims),
			slog.String("method", method),
		)
		return grpcstatus.Errorf(grpccodes.PermissionDenied, "Access denied")
	}
	return nil
}

// checkAcl checks if the given set of claims match at least one of the items of the access control list.
func (f *grpcAclAuthzFunc) checkAcl(claims map[string]any) bool {
	for _, aclItem := range f.aclItems {
		value, ok := claims[aclItem.Claim]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		if aclItem.Pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func (f *grpcAclAuthzFunc) isPublicMethod(method string) bool {
	for _, publicMethod := range f.publicMethods {
		if publicMethod.MatchString(method) {
			return true
		}
	}
	return false
}
