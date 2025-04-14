/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

// This file contains a custom type provider that simplifies use of the authentication and authorization types (for
// example the Subject type) in CEL expressions.

package auth

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// celSubjectTypeName is the name of the subject type used in CEL expressions.
const celSubjectTypeName = "auth.Subject"

// celSubjectType is the subject type used in CEL expressions.
var celSubjectType = cel.ObjectType(celSubjectTypeName)

// celSubjectNameField describes the `name` field of the subject type.
var celSubjectNameFieldType = &types.FieldType{
	Type: types.StringType,
	IsSet: func(target any) bool {
		subject := target.(*Subject)
		return subject.Name != ""
	},
	GetFrom: func(target any) (value any, err error) {
		subject := target.(*Subject)
		value = subject.Name
		return
	},
}

// celSubjectClaimsFieldType describes the `claims` field of the subject type.
var celSubjectClaimsFieldType = &types.FieldType{
	Type: types.NewMapType(types.StringType, types.AnyType),
	IsSet: func(target any) bool {
		subject := target.(*Subject)
		return subject.Claims != nil
	},
	GetFrom: func(target any) (value any, err error) {
		subject := target.(*Subject)
		value = subject.Claims
		return
	},
}

// celSubject wraps a subject so that we can use it as a value in CEL expressions.
type celSubject struct {
	subject *Subject
}

func (s *celSubject) ConvertToNative(typeDesc reflect.Type) (result any, err error) {
	err = fmt.Errorf("unsupported type conversion from 'subject' to %v", typeDesc)
	return
}

func (s *celSubject) ConvertToType(typeValue ref.Type) ref.Val {
	switch typeValue {
	case types.StringType:
		return types.String(s.subject.Name)
	}
	return types.NewErr("type conversion error from '%s' to '%s'", celSubjectType, typeValue)
}

func (s *celSubject) Equal(other ref.Val) ref.Val {
	return types.Bool(reflect.DeepEqual(s.subject, other.(*celSubject).subject))
}

func (s *celSubject) Type() ref.Type {
	return celSubjectType
}

func (s *celSubject) Value() any {
	return s.subject
}

// celTypeProvider is an implementation of the CEL type provider interface that contains the knowledge about the types
// that we want to use in CEL expressions, in particular the subject type.
type celTypeProvider struct {
	registry *types.Registry
}

func (p *celTypeProvider) EnumValue(enumName string) ref.Val {
	return p.registry.EnumValue(enumName)
}

func (p *celTypeProvider) FindIdent(identName string) (result ref.Val, ok bool) {
	result, ok = p.registry.FindIdent(identName)
	return
}

func (p *celTypeProvider) FindStructFieldNames(structType string) (result []string, ok bool) {
	switch structType {
	case celSubjectTypeName:
		result = []string{
			"name",
			"claims",
		}
		ok = true
	default:
		result, ok = p.registry.FindStructFieldNames(structType)
	}
	return
}

func (p *celTypeProvider) FindStructFieldType(structType string, fieldName string) (result *types.FieldType, ok bool) {
	switch structType {
	case celSubjectTypeName:
		switch fieldName {
		case "name":
			result, ok = celSubjectNameFieldType, true
		case "claims":
			result, ok = celSubjectClaimsFieldType, true
		default:
			result, ok = nil, false
		}
	default:
		result, ok = p.registry.FindStructFieldType(structType, fieldName)
	}
	return
}

func (p *celTypeProvider) FindStructType(structType string) (result *types.Type, found bool) {
	switch structType {
	case celSubjectTypeName:
		result, found = celSubjectType, true
	default:
		result, found = p.registry.FindStructType(structType)
	}
	return
}

func (p *celTypeProvider) NewValue(structType string, fields map[string]ref.Val) ref.Val {
	return p.registry.NewValue(structType, fields)
}

func (p *celTypeProvider) RegisterDescriptor(descriptor protoreflect.FileDescriptor) error {
	return p.registry.RegisterDescriptor(descriptor)
}

func (p *celTypeProvider) RegisterType(types ...ref.Type) error {
	return p.registry.RegisterType(types...)
}
