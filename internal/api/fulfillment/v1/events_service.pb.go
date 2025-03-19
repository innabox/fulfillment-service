//
// Copyright (c) 2025 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
// specific language governing permissions and limitations under the License.
//

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        (unknown)
// source: fulfillment/v1/events_service.proto

package fulfillmentv1

import (
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type EventsWatchRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// When this parameter is used the server will only send the events that are newer than the event with this
	// identifier. Note that the event with this identifier will *not* be sent.
	//
	// When this parameter isn't used the server will send only the events that are generated after the request is
	// received.
	From *string `protobuf:"bytes,1,opt,name=from,proto3,oneof" json:"from,omitempty"`
	// Filter criteria.
	//
	// The syntax of this parameter is similar to the syntax of the _where_ clause of a SQL statement, but using the names
	// of the attributes of the event instead of the names of the columns of a table. For example, in order to get only
	// the events of type `CREATED` the value should be:
	//
	//	type = 'CREATED'
	//
	// If this isn't provided, or if the value is empty, then all the events that the user has permission to see will be
	// returned.
	Filter        *string `protobuf:"bytes,2,opt,name=filter,proto3,oneof" json:"filter,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *EventsWatchRequest) Reset() {
	*x = EventsWatchRequest{}
	mi := &file_fulfillment_v1_events_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *EventsWatchRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventsWatchRequest) ProtoMessage() {}

func (x *EventsWatchRequest) ProtoReflect() protoreflect.Message {
	mi := &file_fulfillment_v1_events_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventsWatchRequest.ProtoReflect.Descriptor instead.
func (*EventsWatchRequest) Descriptor() ([]byte, []int) {
	return file_fulfillment_v1_events_service_proto_rawDescGZIP(), []int{0}
}

func (x *EventsWatchRequest) GetFrom() string {
	if x != nil && x.From != nil {
		return *x.From
	}
	return ""
}

func (x *EventsWatchRequest) GetFilter() string {
	if x != nil && x.Filter != nil {
		return *x.Filter
	}
	return ""
}

type EventsWatchResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Event         *Event                 `protobuf:"bytes,1,opt,name=event,proto3" json:"event,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *EventsWatchResponse) Reset() {
	*x = EventsWatchResponse{}
	mi := &file_fulfillment_v1_events_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *EventsWatchResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventsWatchResponse) ProtoMessage() {}

func (x *EventsWatchResponse) ProtoReflect() protoreflect.Message {
	mi := &file_fulfillment_v1_events_service_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventsWatchResponse.ProtoReflect.Descriptor instead.
func (*EventsWatchResponse) Descriptor() ([]byte, []int) {
	return file_fulfillment_v1_events_service_proto_rawDescGZIP(), []int{1}
}

func (x *EventsWatchResponse) GetEvent() *Event {
	if x != nil {
		return x.Event
	}
	return nil
}

var File_fulfillment_v1_events_service_proto protoreflect.FileDescriptor

var file_fulfillment_v1_events_service_proto_rawDesc = string([]byte{
	0x0a, 0x23, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2f, 0x76, 0x31,
	0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0e, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65,
	0x6e, 0x74, 0x2e, 0x76, 0x31, 0x1a, 0x1f, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65,
	0x6e, 0x74, 0x2f, 0x76, 0x31, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x5f, 0x74, 0x79, 0x70, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61,
	0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0x5e, 0x0a, 0x12, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x57, 0x61,
	0x74, 0x63, 0x68, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x17, 0x0a, 0x04, 0x66, 0x72,
	0x6f, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x04, 0x66, 0x72, 0x6f, 0x6d,
	0x88, 0x01, 0x01, 0x12, 0x1b, 0x0a, 0x06, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x48, 0x01, 0x52, 0x06, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x88, 0x01, 0x01,
	0x42, 0x07, 0x0a, 0x05, 0x5f, 0x66, 0x72, 0x6f, 0x6d, 0x42, 0x09, 0x0a, 0x07, 0x5f, 0x66, 0x69,
	0x6c, 0x74, 0x65, 0x72, 0x22, 0x42, 0x0a, 0x13, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x57, 0x61,
	0x74, 0x63, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2b, 0x0a, 0x05, 0x65,
	0x76, 0x65, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x66, 0x75, 0x6c,
	0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x45, 0x76, 0x65, 0x6e,
	0x74, 0x52, 0x05, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x32, 0x80, 0x01, 0x0a, 0x06, 0x45, 0x76, 0x65,
	0x6e, 0x74, 0x73, 0x12, 0x76, 0x0a, 0x05, 0x57, 0x61, 0x74, 0x63, 0x68, 0x12, 0x22, 0x2e, 0x66,
	0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x45, 0x76,
	0x65, 0x6e, 0x74, 0x73, 0x57, 0x61, 0x74, 0x63, 0x68, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x23, 0x2e, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x76,
	0x31, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x57, 0x61, 0x74, 0x63, 0x68, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x22, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x1c, 0x12, 0x1a, 0x2f,
	0x61, 0x70, 0x69, 0x2f, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2f,
	0x76, 0x31, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x30, 0x01, 0x42, 0xd3, 0x01, 0x0a, 0x12,
	0x63, 0x6f, 0x6d, 0x2e, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2e,
	0x76, 0x31, 0x42, 0x12, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x50, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x69, 0x6e, 0x6e, 0x61, 0x62, 0x6f, 0x78, 0x2f, 0x66, 0x75, 0x6c,
	0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2d, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x66, 0x75,
	0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2f, 0x76, 0x31, 0x3b, 0x66, 0x75, 0x6c,
	0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x76, 0x31, 0xa2, 0x02, 0x03, 0x46, 0x58, 0x58,
	0xaa, 0x02, 0x0e, 0x46, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x56,
	0x31, 0xca, 0x02, 0x0e, 0x46, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x5c,
	0x56, 0x31, 0xe2, 0x02, 0x1a, 0x46, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74,
	0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea,
	0x02, 0x0f, 0x46, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x3a, 0x3a, 0x56,
	0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_fulfillment_v1_events_service_proto_rawDescOnce sync.Once
	file_fulfillment_v1_events_service_proto_rawDescData []byte
)

func file_fulfillment_v1_events_service_proto_rawDescGZIP() []byte {
	file_fulfillment_v1_events_service_proto_rawDescOnce.Do(func() {
		file_fulfillment_v1_events_service_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_fulfillment_v1_events_service_proto_rawDesc), len(file_fulfillment_v1_events_service_proto_rawDesc)))
	})
	return file_fulfillment_v1_events_service_proto_rawDescData
}

var file_fulfillment_v1_events_service_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_fulfillment_v1_events_service_proto_goTypes = []any{
	(*EventsWatchRequest)(nil),  // 0: fulfillment.v1.EventsWatchRequest
	(*EventsWatchResponse)(nil), // 1: fulfillment.v1.EventsWatchResponse
	(*Event)(nil),               // 2: fulfillment.v1.Event
}
var file_fulfillment_v1_events_service_proto_depIdxs = []int32{
	2, // 0: fulfillment.v1.EventsWatchResponse.event:type_name -> fulfillment.v1.Event
	0, // 1: fulfillment.v1.Events.Watch:input_type -> fulfillment.v1.EventsWatchRequest
	1, // 2: fulfillment.v1.Events.Watch:output_type -> fulfillment.v1.EventsWatchResponse
	2, // [2:3] is the sub-list for method output_type
	1, // [1:2] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_fulfillment_v1_events_service_proto_init() }
func file_fulfillment_v1_events_service_proto_init() {
	if File_fulfillment_v1_events_service_proto != nil {
		return
	}
	file_fulfillment_v1_event_type_proto_init()
	file_fulfillment_v1_events_service_proto_msgTypes[0].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_fulfillment_v1_events_service_proto_rawDesc), len(file_fulfillment_v1_events_service_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_fulfillment_v1_events_service_proto_goTypes,
		DependencyIndexes: file_fulfillment_v1_events_service_proto_depIdxs,
		MessageInfos:      file_fulfillment_v1_events_service_proto_msgTypes,
	}.Build()
	File_fulfillment_v1_events_service_proto = out.File
	file_fulfillment_v1_events_service_proto_goTypes = nil
	file_fulfillment_v1_events_service_proto_depIdxs = nil
}
