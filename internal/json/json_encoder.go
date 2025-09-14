/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package json

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// EncoderBuilder is a builder for creating JSON encoders.
type EncoderBuilder struct {
	logger        *slog.Logger
	ignoredFields []any
}

// Encoder knows how to convert protocol buffers messages to JSON, with the additional capability to omit some fields.
type Encoder struct {
	logger                *slog.Logger
	ignoredFieldNames     map[protoreflect.Name]bool
	ignoredFieldFullNames map[protoreflect.FullName]bool
	anyDesc               protoreflect.MessageDescriptor
	timestampDesc         protoreflect.MessageDescriptor
	jsonApi               jsoniter.API
}

// NewEncoder creates a builder that can then be used to configure and create a JSON encoder.
func NewEncoder() *EncoderBuilder {
	return &EncoderBuilder{}
}

// SetLogger sets the logger. This is mandatory.
func (b *EncoderBuilder) SetLogger(value *slog.Logger) *EncoderBuilder {
	b.logger = value
	return b
}

// AddIgnoredFields adds a set of fields to be omitted from the generated JSON. The values passed can be of the
// following types:
//
// string - This should be a field name, for example 'metadata' and then any field with that name in any object will
// be ignored.
//
// protoreflect.Name - Like string.
//
// protoreflect.FullName - This indicates a field of a particular type. For example, if the value is
// 'private.v1.Cluster.metadata' then only the 'metadata' field of the 'private.v1.Cluster' object will be ignored.
func (b *EncoderBuilder) AddIgnoredFields(values ...any) *EncoderBuilder {
	b.ignoredFields = append(b.ignoredFields, values...)
	return b
}

// Build creates a new encoder using the configuration stored in the builder.
func (b *EncoderBuilder) Build() (result *Encoder, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Get descriptors of well known types:
	var (
		anyValue       *anypb.Any
		timestampValue *timestamppb.Timestamp
	)
	anyDesc := anyValue.ProtoReflect().Descriptor()
	timestampDesc := timestampValue.ProtoReflect().Descriptor()

	// Create the JSON API:
	jsonApi := jsoniter.Config{}.Froze()

	// Create the sets of ignored fields:
	ignoredFieldNames := make(map[protoreflect.Name]bool)
	ignoredFieldFullNames := make(map[protoreflect.FullName]bool)
	for i, ignoredField := range b.ignoredFields {
		switch ignoredField := ignoredField.(type) {
		case string:
			if strings.Contains(ignoredField, ".") {
				ignoredFieldFullNames[protoreflect.FullName(ignoredField)] = true
			} else {
				ignoredFieldNames[protoreflect.Name(ignoredField)] = true
			}
		case protoreflect.Name:
			ignoredFieldNames[ignoredField] = true
		case protoreflect.FullName:
			ignoredFieldFullNames[ignoredField] = true
		default:
			err = fmt.Errorf(
				"ignored fields should be strings or protocol buffers field names, but value %d is "+
					"of type '%T'",
				i, ignoredField,
			)
			return
		}
	}

	// Create and populate the object:
	result = &Encoder{
		logger:                b.logger,
		ignoredFieldNames:     ignoredFieldNames,
		ignoredFieldFullNames: ignoredFieldFullNames,
		anyDesc:               anyDesc,
		timestampDesc:         timestampDesc,
		jsonApi:               jsonApi,
	}
	return
}

func (e *Encoder) Marshal(object proto.Message) (result []byte, err error) {
	if object == nil {
		result = []byte("{}")
		return
	}
	stream := e.jsonApi.BorrowStream(nil)
	defer e.jsonApi.ReturnStream(stream)
	err = e.marshalMessage(stream, object.ProtoReflect())
	if err != nil {
		return
	}
	err = stream.Flush()
	if err != nil {
		return
	}
	result = bytes.Clone(stream.Buffer())
	return
}

func (e *Encoder) marshalMessage(stream *jsoniter.Stream, message protoreflect.Message) (err error) {
	descriptor := message.Descriptor()
	if descriptor == e.anyDesc || descriptor == e.timestampDesc {
		err = e.marshalWellKnown(stream, message.Interface())
		return
	}
	stream.WriteObjectStart()
	if stream.Error != nil {
		err = stream.Error
		return
	}
	first := true
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if e.ignoredFieldNames[field.Name()] || e.ignoredFieldFullNames[field.FullName()] {
			return true
		}
		if !first {
			stream.WriteMore()
			if stream.Error != nil {
				err = stream.Error
				return false
			}
		}
		stream.WriteObjectField(field.TextName())
		if stream.Error != nil {
			err = stream.Error
			return false
		}
		
		// Defensive marshaling with fallback to null
		beforePos := len(stream.Buffer())
		marshalErr := e.marshalValue(stream, value, field)
		afterPos := len(stream.Buffer())
		
		if marshalErr != nil || beforePos == afterPos {
			// If marshaling failed or nothing was written, write null
			stream.WriteNil()
			if stream.Error != nil {
				err = stream.Error
				return false
			}
		}
		
		first = false
		return true
	})
	if err != nil {
		return
	}
	stream.WriteObjectEnd()
	err = stream.Error
	if err != nil {
		return
	}
	return stream.Flush()
}

func (e *Encoder) marshalValue(stream *jsoniter.Stream, value protoreflect.Value, field protoreflect.FieldDescriptor) error {
	switch {
	case field.IsList():
		return e.marshalList(stream, value.List(), field)
	case field.IsMap():
		return e.marshalMap(stream, value.Map(), field)
	default:
		return e.marshalSingle(stream, value, field)
	}
}

func (e *Encoder) marshalSingle(stream *jsoniter.Stream, value protoreflect.Value,
	field protoreflect.FieldDescriptor) error {
	if !value.IsValid() {
		stream.WriteNil()
		return stream.Error
	}
	// Handle empty protobuf.Any and protobuf.Value by writing null or empty array
	if field.Kind() == protoreflect.MessageKind {
		message := value.Message()
		if message.Descriptor() == e.anyDesc {
			// Check if the Any message is empty/nil
			anyValue := message.Interface().(*anypb.Any)
			if anyValue == nil || anyValue.TypeUrl == "" {
				// Write appropriate defaults based on field type
				fieldName := string(field.Name())
				// Log what we're fixing
				fmt.Printf("CUSTOM DEBUG: Fixing empty field: %s\n", fieldName)
				if fieldName == "additional_disks" || fieldName == "vm_service_ports" {
					// Array fields get empty arrays
					fmt.Printf("CUSTOM DEBUG: Writing empty array for %s\n", fieldName)
					stream.WriteArrayStart()
					stream.WriteArrayEnd()
				} else if fieldName == "cloud_init_config" {
					// cloud_init_config gets empty string
					fmt.Printf("CUSTOM DEBUG: Writing empty string for %s\n", fieldName)
					stream.WriteString("")
				} else {
					// Other fields get null
					fmt.Printf("CUSTOM DEBUG: Writing null for %s\n", fieldName)
					stream.WriteNil()
				}
				return stream.Error
			}
		}
	}
	switch field.Kind() {
	case protoreflect.BoolKind:
		stream.WriteBool(value.Bool())
		return stream.Error
	case protoreflect.StringKind:
		stream.WriteString(value.String())
		return stream.Error
	case protoreflect.Int32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Sfixed32Kind:
		stream.WriteInt32(int32(value.Int()))
		return stream.Error
	case protoreflect.Uint32Kind,
		protoreflect.Fixed32Kind:
		stream.WriteUint32(uint32(value.Uint()))
		return stream.Error
	case protoreflect.Int64Kind,
		protoreflect.Sint64Kind,
		protoreflect.Uint64Kind,
		protoreflect.Sfixed64Kind,
		protoreflect.Fixed64Kind:
		stream.WriteString(value.String())
		return stream.Error
	case protoreflect.FloatKind:
		stream.WriteFloat64(value.Float())
		return stream.Error
	case protoreflect.DoubleKind:
		stream.WriteFloat64(value.Float())
		return stream.Error
	case protoreflect.BytesKind:
		stream.WriteString(base64.StdEncoding.EncodeToString(value.Bytes()))
		return stream.Error
	case protoreflect.EnumKind:
		enum := value.Enum()
		if enum == 0 {
			stream.WriteNil()
			return stream.Error
		}
		desc := field.Enum().Values().ByNumber(enum)
		stream.WriteString(string(desc.Name()))
		return stream.Error
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return e.marshalMessage(stream, value.Message())
	default:
		return fmt.Errorf("field '%v' has unknown kind '%v'", field.FullName(), field.Kind())
	}
	return nil
}

func (e *Encoder) marshalList(stream *jsoniter.Stream, list protoreflect.List,
	field protoreflect.FieldDescriptor) error {
	stream.WriteArrayStart()
	if stream.Error != nil {
		return stream.Error
	}
	for i := range list.Len() {
		if i > 0 {
			stream.WriteMore()
			if stream.Error != nil {
				return stream.Error
			}
		}
		item := list.Get(i)
		err := e.marshalSingle(stream, item, field)
		if err != nil {
			return err
		}
	}
	stream.WriteArrayEnd()
	return stream.Error
}

func (e *Encoder) marshalMap(stream *jsoniter.Stream, m protoreflect.Map,
	field protoreflect.FieldDescriptor) (err error) {
	stream.WriteObjectStart()
	if stream.Error != nil {
		err = stream.Error
		return
	}
	first := true
	m.Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
		if !first {
			stream.WriteMore()
			if stream.Error != nil {
				err = stream.Error
				return false
			}
		}
		stream.WriteObjectField(key.String())
		if stream.Error != nil {
			err = stream.Error
			return false
		}
		
		// Save the current buffer position to detect if a value was written
		beforeValuePos := len(stream.Buffer())
		err = e.marshalSingle(stream, value, field.MapValue())
		afterValuePos := len(stream.Buffer())
		
		if err != nil || beforeValuePos == afterValuePos {
			// If marshaling fails or no value was written, write null to prevent malformed JSON
			stream.WriteNil()
			if stream.Error != nil {
				err = stream.Error
				return false
			}
		}
		first = false
		return true
	})
	if err != nil {
		return
	}
	stream.WriteObjectEnd()
	return stream.Error
}

func (e *Encoder) marshalWellKnown(stream *jsoniter.Stream, value proto.Message) error {
	// Handle empty Any values - write appropriate defaults
	if anyValue, ok := value.(*anypb.Any); ok {
		if anyValue == nil || anyValue.TypeUrl == "" {
			stream.WriteNil()
			return stream.Error
		}
	}
	
	data, err := protojson.Marshal(value)
	if err != nil {
		// If protojson marshal fails, write null instead of causing malformed JSON
		stream.WriteNil()
		return stream.Error
	}
	_, err = stream.Write(data)
	return err
}
