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
// source: private/v1/cluster_orders_service.proto

//go:build protoopaque

package privatev1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ClusterOrdersListRequest struct {
	state                  protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Offset      int32                  `protobuf:"varint,1,opt,name=offset,proto3,oneof"`
	xxx_hidden_Limit       int32                  `protobuf:"varint,2,opt,name=limit,proto3,oneof"`
	XXX_raceDetectHookData protoimpl.RaceDetectHookData
	XXX_presence           [1]uint32
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *ClusterOrdersListRequest) Reset() {
	*x = ClusterOrdersListRequest{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersListRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersListRequest) ProtoMessage() {}

func (x *ClusterOrdersListRequest) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterOrdersListRequest) GetOffset() int32 {
	if x != nil {
		return x.xxx_hidden_Offset
	}
	return 0
}

func (x *ClusterOrdersListRequest) GetLimit() int32 {
	if x != nil {
		return x.xxx_hidden_Limit
	}
	return 0
}

func (x *ClusterOrdersListRequest) SetOffset(v int32) {
	x.xxx_hidden_Offset = v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 0, 2)
}

func (x *ClusterOrdersListRequest) SetLimit(v int32) {
	x.xxx_hidden_Limit = v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 1, 2)
}

func (x *ClusterOrdersListRequest) HasOffset() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 0)
}

func (x *ClusterOrdersListRequest) HasLimit() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 1)
}

func (x *ClusterOrdersListRequest) ClearOffset() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 0)
	x.xxx_hidden_Offset = 0
}

func (x *ClusterOrdersListRequest) ClearLimit() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 1)
	x.xxx_hidden_Limit = 0
}

type ClusterOrdersListRequest_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Offset *int32
	Limit  *int32
}

func (b0 ClusterOrdersListRequest_builder) Build() *ClusterOrdersListRequest {
	m0 := &ClusterOrdersListRequest{}
	b, x := &b0, m0
	_, _ = b, x
	if b.Offset != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 0, 2)
		x.xxx_hidden_Offset = *b.Offset
	}
	if b.Limit != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 1, 2)
		x.xxx_hidden_Limit = *b.Limit
	}
	return m0
}

type ClusterOrdersListResponse struct {
	state                  protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Size        int32                  `protobuf:"varint,1,opt,name=size,proto3,oneof"`
	xxx_hidden_Total       int32                  `protobuf:"varint,2,opt,name=total,proto3,oneof"`
	xxx_hidden_Items       *[]*ClusterOrder       `protobuf:"bytes,3,rep,name=items,proto3"`
	XXX_raceDetectHookData protoimpl.RaceDetectHookData
	XXX_presence           [1]uint32
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *ClusterOrdersListResponse) Reset() {
	*x = ClusterOrdersListResponse{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersListResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersListResponse) ProtoMessage() {}

func (x *ClusterOrdersListResponse) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterOrdersListResponse) GetSize() int32 {
	if x != nil {
		return x.xxx_hidden_Size
	}
	return 0
}

func (x *ClusterOrdersListResponse) GetTotal() int32 {
	if x != nil {
		return x.xxx_hidden_Total
	}
	return 0
}

func (x *ClusterOrdersListResponse) GetItems() []*ClusterOrder {
	if x != nil {
		if x.xxx_hidden_Items != nil {
			return *x.xxx_hidden_Items
		}
	}
	return nil
}

func (x *ClusterOrdersListResponse) SetSize(v int32) {
	x.xxx_hidden_Size = v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 0, 3)
}

func (x *ClusterOrdersListResponse) SetTotal(v int32) {
	x.xxx_hidden_Total = v
	protoimpl.X.SetPresent(&(x.XXX_presence[0]), 1, 3)
}

func (x *ClusterOrdersListResponse) SetItems(v []*ClusterOrder) {
	x.xxx_hidden_Items = &v
}

func (x *ClusterOrdersListResponse) HasSize() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 0)
}

func (x *ClusterOrdersListResponse) HasTotal() bool {
	if x == nil {
		return false
	}
	return protoimpl.X.Present(&(x.XXX_presence[0]), 1)
}

func (x *ClusterOrdersListResponse) ClearSize() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 0)
	x.xxx_hidden_Size = 0
}

func (x *ClusterOrdersListResponse) ClearTotal() {
	protoimpl.X.ClearPresent(&(x.XXX_presence[0]), 1)
	x.xxx_hidden_Total = 0
}

type ClusterOrdersListResponse_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Size  *int32
	Total *int32
	Items []*ClusterOrder
}

func (b0 ClusterOrdersListResponse_builder) Build() *ClusterOrdersListResponse {
	m0 := &ClusterOrdersListResponse{}
	b, x := &b0, m0
	_, _ = b, x
	if b.Size != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 0, 3)
		x.xxx_hidden_Size = *b.Size
	}
	if b.Total != nil {
		protoimpl.X.SetPresentNonAtomic(&(x.XXX_presence[0]), 1, 3)
		x.xxx_hidden_Total = *b.Total
	}
	x.xxx_hidden_Items = &b.Items
	return m0
}

type ClusterOrdersGetRequest struct {
	state         protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Id string                 `protobuf:"bytes,1,opt,name=id,proto3"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClusterOrdersGetRequest) Reset() {
	*x = ClusterOrdersGetRequest{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersGetRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersGetRequest) ProtoMessage() {}

func (x *ClusterOrdersGetRequest) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterOrdersGetRequest) GetId() string {
	if x != nil {
		return x.xxx_hidden_Id
	}
	return ""
}

func (x *ClusterOrdersGetRequest) SetId(v string) {
	x.xxx_hidden_Id = v
}

type ClusterOrdersGetRequest_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Id string
}

func (b0 ClusterOrdersGetRequest_builder) Build() *ClusterOrdersGetRequest {
	m0 := &ClusterOrdersGetRequest{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Id = b.Id
	return m0
}

type ClusterOrdersGetResponse struct {
	state             protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Object *ClusterOrder          `protobuf:"bytes,1,opt,name=object,proto3"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *ClusterOrdersGetResponse) Reset() {
	*x = ClusterOrdersGetResponse{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersGetResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersGetResponse) ProtoMessage() {}

func (x *ClusterOrdersGetResponse) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterOrdersGetResponse) GetObject() *ClusterOrder {
	if x != nil {
		return x.xxx_hidden_Object
	}
	return nil
}

func (x *ClusterOrdersGetResponse) SetObject(v *ClusterOrder) {
	x.xxx_hidden_Object = v
}

func (x *ClusterOrdersGetResponse) HasObject() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_Object != nil
}

func (x *ClusterOrdersGetResponse) ClearObject() {
	x.xxx_hidden_Object = nil
}

type ClusterOrdersGetResponse_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Object *ClusterOrder
}

func (b0 ClusterOrdersGetResponse_builder) Build() *ClusterOrdersGetResponse {
	m0 := &ClusterOrdersGetResponse{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Object = b.Object
	return m0
}

type ClusterOrdersCreateRequest struct {
	state             protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Object *ClusterOrder          `protobuf:"bytes,1,opt,name=object,proto3"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *ClusterOrdersCreateRequest) Reset() {
	*x = ClusterOrdersCreateRequest{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersCreateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersCreateRequest) ProtoMessage() {}

func (x *ClusterOrdersCreateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterOrdersCreateRequest) GetObject() *ClusterOrder {
	if x != nil {
		return x.xxx_hidden_Object
	}
	return nil
}

func (x *ClusterOrdersCreateRequest) SetObject(v *ClusterOrder) {
	x.xxx_hidden_Object = v
}

func (x *ClusterOrdersCreateRequest) HasObject() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_Object != nil
}

func (x *ClusterOrdersCreateRequest) ClearObject() {
	x.xxx_hidden_Object = nil
}

type ClusterOrdersCreateRequest_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Object *ClusterOrder
}

func (b0 ClusterOrdersCreateRequest_builder) Build() *ClusterOrdersCreateRequest {
	m0 := &ClusterOrdersCreateRequest{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Object = b.Object
	return m0
}

type ClusterOrdersCreateResponse struct {
	state             protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Object *ClusterOrder          `protobuf:"bytes,1,opt,name=object,proto3"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *ClusterOrdersCreateResponse) Reset() {
	*x = ClusterOrdersCreateResponse{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersCreateResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersCreateResponse) ProtoMessage() {}

func (x *ClusterOrdersCreateResponse) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterOrdersCreateResponse) GetObject() *ClusterOrder {
	if x != nil {
		return x.xxx_hidden_Object
	}
	return nil
}

func (x *ClusterOrdersCreateResponse) SetObject(v *ClusterOrder) {
	x.xxx_hidden_Object = v
}

func (x *ClusterOrdersCreateResponse) HasObject() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_Object != nil
}

func (x *ClusterOrdersCreateResponse) ClearObject() {
	x.xxx_hidden_Object = nil
}

type ClusterOrdersCreateResponse_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Object *ClusterOrder
}

func (b0 ClusterOrdersCreateResponse_builder) Build() *ClusterOrdersCreateResponse {
	m0 := &ClusterOrdersCreateResponse{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Object = b.Object
	return m0
}

type ClusterOrdersDeleteRequest struct {
	state         protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Id string                 `protobuf:"bytes,1,opt,name=id,proto3"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClusterOrdersDeleteRequest) Reset() {
	*x = ClusterOrdersDeleteRequest{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersDeleteRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersDeleteRequest) ProtoMessage() {}

func (x *ClusterOrdersDeleteRequest) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterOrdersDeleteRequest) GetId() string {
	if x != nil {
		return x.xxx_hidden_Id
	}
	return ""
}

func (x *ClusterOrdersDeleteRequest) SetId(v string) {
	x.xxx_hidden_Id = v
}

type ClusterOrdersDeleteRequest_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Id string
}

func (b0 ClusterOrdersDeleteRequest_builder) Build() *ClusterOrdersDeleteRequest {
	m0 := &ClusterOrdersDeleteRequest{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Id = b.Id
	return m0
}

type ClusterOrdersDeleteResponse struct {
	state         protoimpl.MessageState `protogen:"opaque.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClusterOrdersDeleteResponse) Reset() {
	*x = ClusterOrdersDeleteResponse{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersDeleteResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersDeleteResponse) ProtoMessage() {}

func (x *ClusterOrdersDeleteResponse) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

type ClusterOrdersDeleteResponse_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

}

func (b0 ClusterOrdersDeleteResponse_builder) Build() *ClusterOrdersDeleteResponse {
	m0 := &ClusterOrdersDeleteResponse{}
	b, x := &b0, m0
	_, _ = b, x
	return m0
}

type ClusterOrdersUpdateRequest struct {
	state             protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Object *ClusterOrder          `protobuf:"bytes,1,opt,name=object,proto3"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *ClusterOrdersUpdateRequest) Reset() {
	*x = ClusterOrdersUpdateRequest{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersUpdateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersUpdateRequest) ProtoMessage() {}

func (x *ClusterOrdersUpdateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterOrdersUpdateRequest) GetObject() *ClusterOrder {
	if x != nil {
		return x.xxx_hidden_Object
	}
	return nil
}

func (x *ClusterOrdersUpdateRequest) SetObject(v *ClusterOrder) {
	x.xxx_hidden_Object = v
}

func (x *ClusterOrdersUpdateRequest) HasObject() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_Object != nil
}

func (x *ClusterOrdersUpdateRequest) ClearObject() {
	x.xxx_hidden_Object = nil
}

type ClusterOrdersUpdateRequest_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Object *ClusterOrder
}

func (b0 ClusterOrdersUpdateRequest_builder) Build() *ClusterOrdersUpdateRequest {
	m0 := &ClusterOrdersUpdateRequest{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Object = b.Object
	return m0
}

type ClusterOrdersUpdateResponse struct {
	state             protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Object *ClusterOrder          `protobuf:"bytes,1,opt,name=object,proto3"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *ClusterOrdersUpdateResponse) Reset() {
	*x = ClusterOrdersUpdateResponse{}
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterOrdersUpdateResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterOrdersUpdateResponse) ProtoMessage() {}

func (x *ClusterOrdersUpdateResponse) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_orders_service_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterOrdersUpdateResponse) GetObject() *ClusterOrder {
	if x != nil {
		return x.xxx_hidden_Object
	}
	return nil
}

func (x *ClusterOrdersUpdateResponse) SetObject(v *ClusterOrder) {
	x.xxx_hidden_Object = v
}

func (x *ClusterOrdersUpdateResponse) HasObject() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_Object != nil
}

func (x *ClusterOrdersUpdateResponse) ClearObject() {
	x.xxx_hidden_Object = nil
}

type ClusterOrdersUpdateResponse_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Object *ClusterOrder
}

func (b0 ClusterOrdersUpdateResponse_builder) Build() *ClusterOrdersUpdateResponse {
	m0 := &ClusterOrdersUpdateResponse{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Object = b.Object
	return m0
}

var File_private_v1_cluster_orders_service_proto protoreflect.FileDescriptor

var file_private_v1_cluster_orders_service_proto_rawDesc = string([]byte{
	0x0a, 0x27, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x31, 0x2f, 0x63, 0x6c, 0x75,
	0x73, 0x74, 0x65, 0x72, 0x5f, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x5f, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0a, 0x70, 0x72, 0x69, 0x76, 0x61,
	0x74, 0x65, 0x2e, 0x76, 0x31, 0x1a, 0x23, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2f, 0x76,
	0x31, 0x2f, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x5f, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x5f,
	0x74, 0x79, 0x70, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x67, 0x0a, 0x18, 0x43, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x4c, 0x69, 0x73, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b, 0x0a, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x48, 0x00, 0x52, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74,
	0x88, 0x01, 0x01, 0x12, 0x19, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x05, 0x48, 0x01, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x88, 0x01, 0x01, 0x42, 0x09,
	0x0a, 0x07, 0x5f, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x6c, 0x69,
	0x6d, 0x69, 0x74, 0x22, 0x92, 0x01, 0x0a, 0x19, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f,
	0x72, 0x64, 0x65, 0x72, 0x73, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x17, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x48,
	0x00, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x88, 0x01, 0x01, 0x12, 0x19, 0x0a, 0x05, 0x74, 0x6f,
	0x74, 0x61, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x48, 0x01, 0x52, 0x05, 0x74, 0x6f, 0x74,
	0x61, 0x6c, 0x88, 0x01, 0x01, 0x12, 0x2e, 0x0a, 0x05, 0x69, 0x74, 0x65, 0x6d, 0x73, 0x18, 0x03,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76,
	0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x52, 0x05,
	0x69, 0x74, 0x65, 0x6d, 0x73, 0x42, 0x07, 0x0a, 0x05, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x42, 0x08,
	0x0a, 0x06, 0x5f, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x22, 0x29, 0x0a, 0x17, 0x43, 0x6c, 0x75, 0x73,
	0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x47, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x02, 0x69, 0x64, 0x22, 0x4c, 0x0a, 0x18, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72,
	0x64, 0x65, 0x72, 0x73, 0x47, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x30, 0x0a, 0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x18, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75,
	0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x52, 0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63,
	0x74, 0x22, 0x4e, 0x0a, 0x1a, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65,
	0x72, 0x73, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x30, 0x0a, 0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x18, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75,
	0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x52, 0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63,
	0x74, 0x22, 0x4f, 0x0a, 0x1b, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65,
	0x72, 0x73, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x30, 0x0a, 0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x18, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x52, 0x06, 0x6f, 0x62, 0x6a, 0x65,
	0x63, 0x74, 0x22, 0x2c, 0x0a, 0x1a, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64,
	0x65, 0x72, 0x73, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64,
	0x22, 0x1d, 0x0a, 0x1b, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72,
	0x73, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x4e, 0x0a, 0x1a, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x30, 0x0a,
	0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e,
	0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74,
	0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x52, 0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22,
	0x4f, 0x0a, 0x1b, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x30,
	0x0a, 0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18,
	0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73,
	0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x52, 0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74,
	0x32, 0xd1, 0x03, 0x0a, 0x0d, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65,
	0x72, 0x73, 0x12, 0x55, 0x0a, 0x04, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x24, 0x2e, 0x70, 0x72, 0x69,
	0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f,
	0x72, 0x64, 0x65, 0x72, 0x73, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x25, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x4c, 0x69, 0x73, 0x74, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x52, 0x0a, 0x03, 0x47, 0x65, 0x74,
	0x12, 0x23, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x47, 0x65, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x24, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e,
	0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73,
	0x47, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x5b, 0x0a,
	0x06, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x12, 0x26, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74,
	0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65,
	0x72, 0x73, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x27, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75,
	0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x5b, 0x0a, 0x06, 0x44, 0x65,
	0x6c, 0x65, 0x74, 0x65, 0x12, 0x26, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76,
	0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x44,
	0x65, 0x6c, 0x65, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x27, 0x2e, 0x70,
	0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65,
	0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x5b, 0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x12, 0x26, 0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43,
	0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x55, 0x70, 0x64, 0x61,
	0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x27, 0x2e, 0x70, 0x72, 0x69, 0x76,
	0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4f, 0x72,
	0x64, 0x65, 0x72, 0x73, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x00, 0x42, 0xc0, 0x01, 0x0a, 0x0e, 0x63, 0x6f, 0x6d, 0x2e, 0x70, 0x72, 0x69,
	0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x42, 0x19, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x50, 0x72, 0x6f,
	0x74, 0x6f, 0x50, 0x01, 0x5a, 0x48, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x69, 0x6e, 0x6e, 0x61, 0x62, 0x6f, 0x78, 0x2f, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c,
	0x6d, 0x65, 0x6e, 0x74, 0x2d, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x69, 0x6e, 0x74,
	0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74,
	0x65, 0x2f, 0x76, 0x31, 0x3b, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x76, 0x31, 0xa2, 0x02,
	0x03, 0x50, 0x58, 0x58, 0xaa, 0x02, 0x0a, 0x50, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x56,
	0x31, 0xca, 0x02, 0x0b, 0x50, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x5f, 0x5c, 0x56, 0x31, 0xe2,
	0x02, 0x17, 0x50, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x5f, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50,
	0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x0b, 0x50, 0x72, 0x69, 0x76,
	0x61, 0x74, 0x65, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var file_private_v1_cluster_orders_service_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_private_v1_cluster_orders_service_proto_goTypes = []any{
	(*ClusterOrdersListRequest)(nil),    // 0: private.v1.ClusterOrdersListRequest
	(*ClusterOrdersListResponse)(nil),   // 1: private.v1.ClusterOrdersListResponse
	(*ClusterOrdersGetRequest)(nil),     // 2: private.v1.ClusterOrdersGetRequest
	(*ClusterOrdersGetResponse)(nil),    // 3: private.v1.ClusterOrdersGetResponse
	(*ClusterOrdersCreateRequest)(nil),  // 4: private.v1.ClusterOrdersCreateRequest
	(*ClusterOrdersCreateResponse)(nil), // 5: private.v1.ClusterOrdersCreateResponse
	(*ClusterOrdersDeleteRequest)(nil),  // 6: private.v1.ClusterOrdersDeleteRequest
	(*ClusterOrdersDeleteResponse)(nil), // 7: private.v1.ClusterOrdersDeleteResponse
	(*ClusterOrdersUpdateRequest)(nil),  // 8: private.v1.ClusterOrdersUpdateRequest
	(*ClusterOrdersUpdateResponse)(nil), // 9: private.v1.ClusterOrdersUpdateResponse
	(*ClusterOrder)(nil),                // 10: private.v1.ClusterOrder
}
var file_private_v1_cluster_orders_service_proto_depIdxs = []int32{
	10, // 0: private.v1.ClusterOrdersListResponse.items:type_name -> private.v1.ClusterOrder
	10, // 1: private.v1.ClusterOrdersGetResponse.object:type_name -> private.v1.ClusterOrder
	10, // 2: private.v1.ClusterOrdersCreateRequest.object:type_name -> private.v1.ClusterOrder
	10, // 3: private.v1.ClusterOrdersCreateResponse.object:type_name -> private.v1.ClusterOrder
	10, // 4: private.v1.ClusterOrdersUpdateRequest.object:type_name -> private.v1.ClusterOrder
	10, // 5: private.v1.ClusterOrdersUpdateResponse.object:type_name -> private.v1.ClusterOrder
	0,  // 6: private.v1.ClusterOrders.List:input_type -> private.v1.ClusterOrdersListRequest
	2,  // 7: private.v1.ClusterOrders.Get:input_type -> private.v1.ClusterOrdersGetRequest
	4,  // 8: private.v1.ClusterOrders.Create:input_type -> private.v1.ClusterOrdersCreateRequest
	6,  // 9: private.v1.ClusterOrders.Delete:input_type -> private.v1.ClusterOrdersDeleteRequest
	8,  // 10: private.v1.ClusterOrders.Update:input_type -> private.v1.ClusterOrdersUpdateRequest
	1,  // 11: private.v1.ClusterOrders.List:output_type -> private.v1.ClusterOrdersListResponse
	3,  // 12: private.v1.ClusterOrders.Get:output_type -> private.v1.ClusterOrdersGetResponse
	5,  // 13: private.v1.ClusterOrders.Create:output_type -> private.v1.ClusterOrdersCreateResponse
	7,  // 14: private.v1.ClusterOrders.Delete:output_type -> private.v1.ClusterOrdersDeleteResponse
	9,  // 15: private.v1.ClusterOrders.Update:output_type -> private.v1.ClusterOrdersUpdateResponse
	11, // [11:16] is the sub-list for method output_type
	6,  // [6:11] is the sub-list for method input_type
	6,  // [6:6] is the sub-list for extension type_name
	6,  // [6:6] is the sub-list for extension extendee
	0,  // [0:6] is the sub-list for field type_name
}

func init() { file_private_v1_cluster_orders_service_proto_init() }
func file_private_v1_cluster_orders_service_proto_init() {
	if File_private_v1_cluster_orders_service_proto != nil {
		return
	}
	file_private_v1_cluster_order_type_proto_init()
	file_private_v1_cluster_orders_service_proto_msgTypes[0].OneofWrappers = []any{}
	file_private_v1_cluster_orders_service_proto_msgTypes[1].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_private_v1_cluster_orders_service_proto_rawDesc), len(file_private_v1_cluster_orders_service_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_private_v1_cluster_orders_service_proto_goTypes,
		DependencyIndexes: file_private_v1_cluster_orders_service_proto_depIdxs,
		MessageInfos:      file_private_v1_cluster_orders_service_proto_msgTypes,
	}.Build()
	File_private_v1_cluster_orders_service_proto = out.File
	file_private_v1_cluster_orders_service_proto_goTypes = nil
	file_private_v1_cluster_orders_service_proto_depIdxs = nil
}
