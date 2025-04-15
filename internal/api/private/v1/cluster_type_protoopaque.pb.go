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
// source: private/v1/cluster_type.proto

//go:build protoopaque

package privatev1

import (
	v1 "github.com/innabox/fulfillment-service/internal/api/shared/v1"
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

// Contains the details about the cluster that are available only for the system.
type Cluster struct {
	state               protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Id       string                 `protobuf:"bytes,1,opt,name=id,proto3"`
	xxx_hidden_Metadata *v1.Metadata           `protobuf:"bytes,2,opt,name=metadata,proto3"`
	xxx_hidden_HubId    string                 `protobuf:"bytes,3,opt,name=hub_id,json=hubId,proto3"`
	xxx_hidden_OrderId  string                 `protobuf:"bytes,4,opt,name=order_id,json=orderId,proto3"`
	unknownFields       protoimpl.UnknownFields
	sizeCache           protoimpl.SizeCache
}

func (x *Cluster) Reset() {
	*x = Cluster{}
	mi := &file_private_v1_cluster_type_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Cluster) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Cluster) ProtoMessage() {}

func (x *Cluster) ProtoReflect() protoreflect.Message {
	mi := &file_private_v1_cluster_type_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *Cluster) GetId() string {
	if x != nil {
		return x.xxx_hidden_Id
	}
	return ""
}

func (x *Cluster) GetMetadata() *v1.Metadata {
	if x != nil {
		return x.xxx_hidden_Metadata
	}
	return nil
}

func (x *Cluster) GetHubId() string {
	if x != nil {
		return x.xxx_hidden_HubId
	}
	return ""
}

func (x *Cluster) GetOrderId() string {
	if x != nil {
		return x.xxx_hidden_OrderId
	}
	return ""
}

func (x *Cluster) SetId(v string) {
	x.xxx_hidden_Id = v
}

func (x *Cluster) SetMetadata(v *v1.Metadata) {
	x.xxx_hidden_Metadata = v
}

func (x *Cluster) SetHubId(v string) {
	x.xxx_hidden_HubId = v
}

func (x *Cluster) SetOrderId(v string) {
	x.xxx_hidden_OrderId = v
}

func (x *Cluster) HasMetadata() bool {
	if x == nil {
		return false
	}
	return x.xxx_hidden_Metadata != nil
}

func (x *Cluster) ClearMetadata() {
	x.xxx_hidden_Metadata = nil
}

type Cluster_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	// Unique identifier of the cluster.
	Id string
	// Metadata of the object.
	Metadata *v1.Metadata
	// Identifier of the hub that was selected for this cluster.
	HubId string
	// Identifier of the cluster order that was originally used to create this cluster.
	//
	// This is needed because the way to find the Kubernetes `ClusterOrder` object corresponding to this cluster is to
	// use the `.../clusterorder-uuid` label that was set when it was created, and that contains the identifier of the
	// cluster order because at that point there is no cluster yet.
	OrderId string
}

func (b0 Cluster_builder) Build() *Cluster {
	m0 := &Cluster{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Id = b.Id
	x.xxx_hidden_Metadata = b.Metadata
	x.xxx_hidden_HubId = b.HubId
	x.xxx_hidden_OrderId = b.OrderId
	return m0
}

var File_private_v1_cluster_type_proto protoreflect.FileDescriptor

var file_private_v1_cluster_type_proto_rawDesc = string([]byte{
	0x0a, 0x1d, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x31, 0x2f, 0x63, 0x6c, 0x75,
	0x73, 0x74, 0x65, 0x72, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x0a, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x1a, 0x1d, 0x73, 0x68, 0x61,
	0x72, 0x65, 0x64, 0x2f, 0x76, 0x31, 0x2f, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x5f,
	0x74, 0x79, 0x70, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x7c, 0x0a, 0x07, 0x43, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x2f, 0x0a, 0x08, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74,
	0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x73, 0x68, 0x61, 0x72, 0x65, 0x64,
	0x2e, 0x76, 0x31, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x52, 0x08, 0x6d, 0x65,
	0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x15, 0x0a, 0x06, 0x68, 0x75, 0x62, 0x5f, 0x69, 0x64,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x68, 0x75, 0x62, 0x49, 0x64, 0x12, 0x19, 0x0a,
	0x08, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x5f, 0x69, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x07, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x49, 0x64, 0x42, 0xb7, 0x01, 0x0a, 0x0e, 0x63, 0x6f, 0x6d,
	0x2e, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x76, 0x31, 0x42, 0x10, 0x43, 0x6c, 0x75,
	0x73, 0x74, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a,
	0x48, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x69, 0x6e, 0x6e, 0x61,
	0x62, 0x6f, 0x78, 0x2f, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2d,
	0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c,
	0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x31, 0x3b,
	0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x76, 0x31, 0xa2, 0x02, 0x03, 0x50, 0x58, 0x58, 0xaa,
	0x02, 0x0a, 0x50, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x0b, 0x50,
	0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x5f, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x17, 0x50, 0x72, 0x69,
	0x76, 0x61, 0x74, 0x65, 0x5f, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61,
	0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x0b, 0x50, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x3a, 0x3a,
	0x56, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var file_private_v1_cluster_type_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_private_v1_cluster_type_proto_goTypes = []any{
	(*Cluster)(nil),     // 0: private.v1.Cluster
	(*v1.Metadata)(nil), // 1: shared.v1.Metadata
}
var file_private_v1_cluster_type_proto_depIdxs = []int32{
	1, // 0: private.v1.Cluster.metadata:type_name -> shared.v1.Metadata
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_private_v1_cluster_type_proto_init() }
func file_private_v1_cluster_type_proto_init() {
	if File_private_v1_cluster_type_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_private_v1_cluster_type_proto_rawDesc), len(file_private_v1_cluster_type_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_private_v1_cluster_type_proto_goTypes,
		DependencyIndexes: file_private_v1_cluster_type_proto_depIdxs,
		MessageInfos:      file_private_v1_cluster_type_proto_msgTypes,
	}.Build()
	File_private_v1_cluster_type_proto = out.File
	file_private_v1_cluster_type_proto_goTypes = nil
	file_private_v1_cluster_type_proto_depIdxs = nil
}
