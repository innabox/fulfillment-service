/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package servers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GenericMapperBuilder contains the data and logic neede to create new generic mapper.
type GenericMapperBuilder[From, To proto.Message] struct {
	logger *slog.Logger
	strict bool
}

// GenericMapper is knows how to map the private representation of an object to the corresponding public representation,
// and the other way around.
type GenericMapper[From, To proto.Message] struct {
	logger    *slog.Logger
	strict    bool
	identical bool
}

// NewGenericMapper creates a builder that can then be used to configure and create a new generic mapper.
func NewGenericMapper[From, To proto.Message]() *GenericMapperBuilder[From, To] {
	return &GenericMapperBuilder[From, To]{}
}

// SetLogger sets the logger. This is mandatory.
func (b *GenericMapperBuilder[From, To]) SetLogger(value *slog.Logger) *GenericMapperBuilder[From, To] {
	b.logger = value
	return b
}

// SetStrict sets the flag if the copy should be strict, meaning that it will fail if there are fields in the `from`
// object that don't exist in the `to` object. This is optional and the default is `false`.
func (b *GenericMapperBuilder[From, To]) SetStrict(value bool) *GenericMapperBuilder[From, To] {
	b.strict = value
	return b
}

// Build uses the configuration stored in the builder to create and configure a new generic mapper.
func (b *GenericMapperBuilder[From, To]) Build() (result *GenericMapper[From, To], err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// If the from and to types are the same, then we can accelerate the operations:
	var (
		from From
		to   To
	)
	fromDesc := from.ProtoReflect().Descriptor()
	toDesc := to.ProtoReflect().Descriptor()
	identical := fromDesc == toDesc

	// Create and populate the object:
	result = &GenericMapper[From, To]{
		logger:    b.logger,
		identical: identical,
	}
	return
}

// Map creates a new `to` object and copies the data from the `from` object.
func (m *GenericMapper[From, To]) Map(ctx context.Context, from From) (to To, err error) {
	if m.identical {
		to = any(from).(To)
		return
	}
	fromReflect := from.ProtoReflect()
	if !fromReflect.IsValid() {
		return
	}
	toReflect := to.ProtoReflect().New()
	err = m.copyMessage(fromReflect, toReflect)
	if err != nil {
		return
	}
	to = toReflect.Interface().(To)
	return
}

// Merge copies the data from the `from` object into the `to` object.
func (m *GenericMapper[From, To]) Merge(ctx context.Context, from From, to To) error {
	if m.identical {
		proto.Merge(to, from)
		return nil
	}
	fromReflect := from.ProtoReflect()
	if !fromReflect.IsValid() {
		return nil
	}
	toReflect := to.ProtoReflect()
	return m.copyMessage(fromReflect, toReflect)
}

func (m *GenericMapper[From, To]) copyMessage(from, to protoreflect.Message) (err error) {
	toFields := to.Descriptor().Fields()
	from.Range(func(fromField protoreflect.FieldDescriptor, fromValue protoreflect.Value) bool {
		toField := toFields.ByName(fromField.Name())
		if toField == nil {
			if m.strict {
				err = fmt.Errorf(
					"type '%s' doesn't have a '%s' field",
					to.Descriptor().FullName(),
					fromField.Name(),
				)
			}
			return false
		}
		switch {
		case fromField.IsList():
			fromList := fromValue.List()
			toList := to.Mutable(toField).List()
			err = m.copyList(fromList, toList, fromField)
			if err != nil {
				return false
			}
		case fromField.IsMap():
			fromMap := fromValue.Map()
			toMap := to.Mutable(toField).Map()
			err = m.copyMap(fromMap, toMap, fromField.MapValue())
			if err != nil {
				return false
			}
		case fromField.Message() != nil:
			fromMessage := fromValue.Message()
			toMessage := to.Mutable(toField).Message()
			err = m.copyMessage(fromMessage, toMessage)
			if err != nil {
				return false
			}
		case fromField.Kind() == protoreflect.BytesKind:
			to.Set(toField, m.cloneBytes(fromValue))
		default:
			to.Set(toField, fromValue)
		}
		return true
	})
	return
}

func (m *GenericMapper[From, To]) copyList(to, from protoreflect.List, field protoreflect.FieldDescriptor) error {
	for i := range from.Len() {
		switch fromValue := from.Get(i); {
		case field.Message() != nil:
			toValue := to.NewElement()
			err := m.copyMessage(toValue.Message(), fromValue.Message())
			if err != nil {
				return err
			}
			to.Append(toValue)
		case field.Kind() == protoreflect.BytesKind:
			to.Append(m.cloneBytes(fromValue))
		default:
			to.Append(fromValue)
		}
	}
	return nil
}

func (m *GenericMapper[From, To]) copyMap(from, to protoreflect.Map, field protoreflect.FieldDescriptor) (err error) {
	from.Range(func(fromKey protoreflect.MapKey, fromValue protoreflect.Value) bool {
		switch {
		case field.Message() != nil:
			toValue := to.NewValue()
			err = m.copyMessage(fromValue.Message(), toValue.Message())
			if err != nil {
				return false
			}
			to.Set(fromKey, toValue)
		case field.Kind() == protoreflect.BytesKind:
			to.Set(fromKey, m.cloneBytes(fromValue))
		default:
			to.Set(fromKey, fromValue)
		}
		return true
	})
	return
}

func (m *GenericMapper[From, To]) cloneBytes(v protoreflect.Value) protoreflect.Value {
	return protoreflect.ValueOfBytes(slices.Clone(v.Bytes()))
}
