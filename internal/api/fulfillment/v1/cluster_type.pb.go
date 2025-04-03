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
// source: fulfillment/v1/cluster_type.proto

//go:build !protoopaque

package fulfillmentv1

import (
	v1 "github.com/innabox/fulfillment-service/internal/api/shared/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Represents the overall state of a cluster.
type ClusterState int32

const (
	// Unspecified indicates that the state is unknown.
	ClusterState_CLUSTER_STATE_UNSPECIFIED ClusterState = 0
	// Indicates that the cluster isn't ready yet.
	ClusterState_CLUSTER_STATE_PROGRESSING ClusterState = 1
	// Indicates indicates that the cluster is ready.
	ClusterState_CLUSTER_STATE_READY ClusterState = 2
	// Indicates indicates that the cluster is unusable.
	ClusterState_CLUSTER_STATE_FAILED ClusterState = 3
)

// Enum value maps for ClusterState.
var (
	ClusterState_name = map[int32]string{
		0: "CLUSTER_STATE_UNSPECIFIED",
		1: "CLUSTER_STATE_PROGRESSING",
		2: "CLUSTER_STATE_READY",
		3: "CLUSTER_STATE_FAILED",
	}
	ClusterState_value = map[string]int32{
		"CLUSTER_STATE_UNSPECIFIED": 0,
		"CLUSTER_STATE_PROGRESSING": 1,
		"CLUSTER_STATE_READY":       2,
		"CLUSTER_STATE_FAILED":      3,
	}
)

func (x ClusterState) Enum() *ClusterState {
	p := new(ClusterState)
	*p = x
	return p
}

func (x ClusterState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ClusterState) Descriptor() protoreflect.EnumDescriptor {
	return file_fulfillment_v1_cluster_type_proto_enumTypes[0].Descriptor()
}

func (ClusterState) Type() protoreflect.EnumType {
	return &file_fulfillment_v1_cluster_type_proto_enumTypes[0]
}

func (x ClusterState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Types of conditions used to describe the status of cluster.
type ClusterConditionType int32

const (
	// Unspecified indicates that the condition is unknown.
	//
	// This will never be appear in the `spec.conditions` field of a cluster.
	ClusterConditionType_CLUSTER_CONDITION_TYPE_UNSPECIFIED ClusterConditionType = 0
	// Indicates that the cluster isn't completely ready yet.
	//
	// Currently there are no `reason` values defined.
	ClusterConditionType_CLUSTER_CONDITION_TYPE_PROGRESSING ClusterConditionType = 1
	// Indicates that the cluster is ready to use.
	//
	// Currently there are no `reason` values defined.
	ClusterConditionType_CLUSTER_CONDITION_TYPE_READY ClusterConditionType = 2
	// Indicates that the cluster is unusable.
	//
	// Currently there are no `reason` values defined.
	ClusterConditionType_CLUSTER_CONDITION_TYPE_FAILED ClusterConditionType = 3
)

// Enum value maps for ClusterConditionType.
var (
	ClusterConditionType_name = map[int32]string{
		0: "CLUSTER_CONDITION_TYPE_UNSPECIFIED",
		1: "CLUSTER_CONDITION_TYPE_PROGRESSING",
		2: "CLUSTER_CONDITION_TYPE_READY",
		3: "CLUSTER_CONDITION_TYPE_FAILED",
	}
	ClusterConditionType_value = map[string]int32{
		"CLUSTER_CONDITION_TYPE_UNSPECIFIED": 0,
		"CLUSTER_CONDITION_TYPE_PROGRESSING": 1,
		"CLUSTER_CONDITION_TYPE_READY":       2,
		"CLUSTER_CONDITION_TYPE_FAILED":      3,
	}
)

func (x ClusterConditionType) Enum() *ClusterConditionType {
	p := new(ClusterConditionType)
	*p = x
	return p
}

func (x ClusterConditionType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ClusterConditionType) Descriptor() protoreflect.EnumDescriptor {
	return file_fulfillment_v1_cluster_type_proto_enumTypes[1].Descriptor()
}

func (ClusterConditionType) Type() protoreflect.EnumType {
	return &file_fulfillment_v1_cluster_type_proto_enumTypes[1]
}

func (x ClusterConditionType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Contains the details of the cluster.
//
// The `spec` contains the desired details, and may be modified by the user. The `status` contains the current status of
// the cluster, is provided by the system and can't be modified by the user.
type Cluster struct {
	state protoimpl.MessageState `protogen:"hybrid.v1"`
	// Unique identifier of the cluster.
	Id            string         `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Metadata      *v1.Metadata   `protobuf:"bytes,2,opt,name=metadata,proto3" json:"metadata,omitempty"`
	Spec          *ClusterSpec   `protobuf:"bytes,3,opt,name=spec,proto3" json:"spec,omitempty"`
	Status        *ClusterStatus `protobuf:"bytes,4,opt,name=status,proto3" json:"status,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Cluster) Reset() {
	*x = Cluster{}
	mi := &file_fulfillment_v1_cluster_type_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Cluster) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Cluster) ProtoMessage() {}

func (x *Cluster) ProtoReflect() protoreflect.Message {
	mi := &file_fulfillment_v1_cluster_type_proto_msgTypes[0]
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
		return x.Id
	}
	return ""
}

func (x *Cluster) GetMetadata() *v1.Metadata {
	if x != nil {
		return x.Metadata
	}
	return nil
}

func (x *Cluster) GetSpec() *ClusterSpec {
	if x != nil {
		return x.Spec
	}
	return nil
}

func (x *Cluster) GetStatus() *ClusterStatus {
	if x != nil {
		return x.Status
	}
	return nil
}

func (x *Cluster) SetId(v string) {
	x.Id = v
}

func (x *Cluster) SetMetadata(v *v1.Metadata) {
	x.Metadata = v
}

func (x *Cluster) SetSpec(v *ClusterSpec) {
	x.Spec = v
}

func (x *Cluster) SetStatus(v *ClusterStatus) {
	x.Status = v
}

func (x *Cluster) HasMetadata() bool {
	if x == nil {
		return false
	}
	return x.Metadata != nil
}

func (x *Cluster) HasSpec() bool {
	if x == nil {
		return false
	}
	return x.Spec != nil
}

func (x *Cluster) HasStatus() bool {
	if x == nil {
		return false
	}
	return x.Status != nil
}

func (x *Cluster) ClearMetadata() {
	x.Metadata = nil
}

func (x *Cluster) ClearSpec() {
	x.Spec = nil
}

func (x *Cluster) ClearStatus() {
	x.Status = nil
}

type Cluster_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	// Unique identifier of the cluster.
	Id       string
	Metadata *v1.Metadata
	Spec     *ClusterSpec
	Status   *ClusterStatus
}

func (b0 Cluster_builder) Build() *Cluster {
	m0 := &Cluster{}
	b, x := &b0, m0
	_, _ = b, x
	x.Id = b.Id
	x.Metadata = b.Metadata
	x.Spec = b.Spec
	x.Status = b.Status
	return m0
}

// The spec contains the details of a cluster as desired by the user.
//
// Note that currently this is empty because there are no properties of the cluster that can be modified by the user.
type ClusterSpec struct {
	state         protoimpl.MessageState `protogen:"hybrid.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClusterSpec) Reset() {
	*x = ClusterSpec{}
	mi := &file_fulfillment_v1_cluster_type_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterSpec) ProtoMessage() {}

func (x *ClusterSpec) ProtoReflect() protoreflect.Message {
	mi := &file_fulfillment_v1_cluster_type_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

type ClusterSpec_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

}

func (b0 ClusterSpec_builder) Build() *ClusterSpec {
	m0 := &ClusterSpec{}
	b, x := &b0, m0
	_, _ = b, x
	return m0
}

// The status contains the details of the cluster provided by the system.
type ClusterStatus struct {
	state protoimpl.MessageState `protogen:"hybrid.v1"`
	// Indicates the overall state of the cluster.
	State ClusterState `protobuf:"varint,1,opt,name=state,proto3,enum=fulfillment.v1.ClusterState" json:"state,omitempty"`
	// Contains a list of conditions that describe in detail the status of the cluster.
	//
	// For example, an cluster that is ready could be represented like this (when converted to JSON):
	//
	//	{
	//	  "id": "123",
	//	  "spec": {
	//	  },
	//	  "status": {
	//	    "state": "CLUSTER_STATE_READY",
	//	    "conditions": [
	//	      {
	//	        "type": "CLUSTER_CONDITION_TYPE_READY",
	//	        "status": "CONDITION_STATUS_TRUE",
	//	        "last_transition_time": "2025-03-12 20:15:59+00:00",
	//	        "message": "The cluster is ready to use",
	//	      },
	//	      {
	//	        "type": "CLUSTER_CONDITION_TYPE_FAILED",
	//	        "status": "CONDITION_STATUS_FALSE",
	//	        "last_transition_time": "2025-03-12 20:10:59+00:00"
	//	      }
	//	    ]
	//	  }
	//	}
	//
	// In this example the `READY` condition is true. That tells us that the cluster is ready to use via the API URL
	// provided in the `status.api_url` field.
	//
	// The `FAILED` condition is false. That tells us that the cluster is *not* failed.
	//
	// Note that in this example, to make it shorter, only one condition appears. In general all the conditions (except
	// `UNSPECIFIED`) will appear exactly once.
	//
	// Check the documentation of the values of the `ClusterConditionType` enumerated type to see possible conditions and
	// reasons.
	Conditions []*ClusterCondition `protobuf:"bytes,2,rep,name=conditions,proto3" json:"conditions,omitempty"`
	// URL of te API server of the cluster.
	//
	// This will be empty if the cluster isn't ready.
	ApiUrl string `protobuf:"bytes,3,opt,name=api_url,json=apiUrl,proto3" json:"api_url,omitempty"`
	// URL of the console of the cluster.
	//
	// This will be empty if the cluster isn't ready or the console isn't enabled.
	ConsoleUrl    string `protobuf:"bytes,4,opt,name=console_url,json=consoleUrl,proto3" json:"console_url,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClusterStatus) Reset() {
	*x = ClusterStatus{}
	mi := &file_fulfillment_v1_cluster_type_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterStatus) ProtoMessage() {}

func (x *ClusterStatus) ProtoReflect() protoreflect.Message {
	mi := &file_fulfillment_v1_cluster_type_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterStatus) GetState() ClusterState {
	if x != nil {
		return x.State
	}
	return ClusterState_CLUSTER_STATE_UNSPECIFIED
}

func (x *ClusterStatus) GetConditions() []*ClusterCondition {
	if x != nil {
		return x.Conditions
	}
	return nil
}

func (x *ClusterStatus) GetApiUrl() string {
	if x != nil {
		return x.ApiUrl
	}
	return ""
}

func (x *ClusterStatus) GetConsoleUrl() string {
	if x != nil {
		return x.ConsoleUrl
	}
	return ""
}

func (x *ClusterStatus) SetState(v ClusterState) {
	x.State = v
}

func (x *ClusterStatus) SetConditions(v []*ClusterCondition) {
	x.Conditions = v
}

func (x *ClusterStatus) SetApiUrl(v string) {
	x.ApiUrl = v
}

func (x *ClusterStatus) SetConsoleUrl(v string) {
	x.ConsoleUrl = v
}

type ClusterStatus_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	// Indicates the overall state of the cluster.
	State ClusterState
	// Contains a list of conditions that describe in detail the status of the cluster.
	//
	// For example, an cluster that is ready could be represented like this (when converted to JSON):
	//
	//	{
	//	  "id": "123",
	//	  "spec": {
	//	  },
	//	  "status": {
	//	    "state": "CLUSTER_STATE_READY",
	//	    "conditions": [
	//	      {
	//	        "type": "CLUSTER_CONDITION_TYPE_READY",
	//	        "status": "CONDITION_STATUS_TRUE",
	//	        "last_transition_time": "2025-03-12 20:15:59+00:00",
	//	        "message": "The cluster is ready to use",
	//	      },
	//	      {
	//	        "type": "CLUSTER_CONDITION_TYPE_FAILED",
	//	        "status": "CONDITION_STATUS_FALSE",
	//	        "last_transition_time": "2025-03-12 20:10:59+00:00"
	//	      }
	//	    ]
	//	  }
	//	}
	//
	// In this example the `READY` condition is true. That tells us that the cluster is ready to use via the API URL
	// provided in the `status.api_url` field.
	//
	// The `FAILED` condition is false. That tells us that the cluster is *not* failed.
	//
	// Note that in this example, to make it shorter, only one condition appears. In general all the conditions (except
	// `UNSPECIFIED`) will appear exactly once.
	//
	// Check the documentation of the values of the `ClusterConditionType` enumerated type to see possible conditions and
	// reasons.
	Conditions []*ClusterCondition
	// URL of te API server of the cluster.
	//
	// This will be empty if the cluster isn't ready.
	ApiUrl string
	// URL of the console of the cluster.
	//
	// This will be empty if the cluster isn't ready or the console isn't enabled.
	ConsoleUrl string
}

func (b0 ClusterStatus_builder) Build() *ClusterStatus {
	m0 := &ClusterStatus{}
	b, x := &b0, m0
	_, _ = b, x
	x.State = b.State
	x.Conditions = b.Conditions
	x.ApiUrl = b.ApiUrl
	x.ConsoleUrl = b.ConsoleUrl
	return m0
}

// Contains the details of a condition that describes the status of a cluster.
type ClusterCondition struct {
	state protoimpl.MessageState `protogen:"hybrid.v1"`
	// Indicates the type of condition.
	Type ClusterConditionType `protobuf:"varint,1,opt,name=type,proto3,enum=fulfillment.v1.ClusterConditionType" json:"type,omitempty"`
	// Indicates the status of the condition.
	Status v1.ConditionStatus `protobuf:"varint,2,opt,name=status,proto3,enum=shared.v1.ConditionStatus" json:"status,omitempty"`
	// This time is the last time that the condition was updated.
	LastTransitionTime *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=last_transition_time,json=lastTransitionTime,proto3" json:"last_transition_time,omitempty"`
	// Contains a the reason of the condition in a format suitable for use by programs.
	//
	// The possible values will be documented in the object that contains the condition.
	Reason *string `protobuf:"bytes,4,opt,name=reason,proto3,oneof" json:"reason,omitempty"`
	// Contains a text giving more details of the condition.
	//
	// This will usually be progress reports, or error messages, and are intended for use by humans, to debug problems.
	Message       *string `protobuf:"bytes,5,opt,name=message,proto3,oneof" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClusterCondition) Reset() {
	*x = ClusterCondition{}
	mi := &file_fulfillment_v1_cluster_type_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterCondition) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterCondition) ProtoMessage() {}

func (x *ClusterCondition) ProtoReflect() protoreflect.Message {
	mi := &file_fulfillment_v1_cluster_type_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ClusterCondition) GetType() ClusterConditionType {
	if x != nil {
		return x.Type
	}
	return ClusterConditionType_CLUSTER_CONDITION_TYPE_UNSPECIFIED
}

func (x *ClusterCondition) GetStatus() v1.ConditionStatus {
	if x != nil {
		return x.Status
	}
	return v1.ConditionStatus(0)
}

func (x *ClusterCondition) GetLastTransitionTime() *timestamppb.Timestamp {
	if x != nil {
		return x.LastTransitionTime
	}
	return nil
}

func (x *ClusterCondition) GetReason() string {
	if x != nil && x.Reason != nil {
		return *x.Reason
	}
	return ""
}

func (x *ClusterCondition) GetMessage() string {
	if x != nil && x.Message != nil {
		return *x.Message
	}
	return ""
}

func (x *ClusterCondition) SetType(v ClusterConditionType) {
	x.Type = v
}

func (x *ClusterCondition) SetStatus(v v1.ConditionStatus) {
	x.Status = v
}

func (x *ClusterCondition) SetLastTransitionTime(v *timestamppb.Timestamp) {
	x.LastTransitionTime = v
}

func (x *ClusterCondition) SetReason(v string) {
	x.Reason = &v
}

func (x *ClusterCondition) SetMessage(v string) {
	x.Message = &v
}

func (x *ClusterCondition) HasLastTransitionTime() bool {
	if x == nil {
		return false
	}
	return x.LastTransitionTime != nil
}

func (x *ClusterCondition) HasReason() bool {
	if x == nil {
		return false
	}
	return x.Reason != nil
}

func (x *ClusterCondition) HasMessage() bool {
	if x == nil {
		return false
	}
	return x.Message != nil
}

func (x *ClusterCondition) ClearLastTransitionTime() {
	x.LastTransitionTime = nil
}

func (x *ClusterCondition) ClearReason() {
	x.Reason = nil
}

func (x *ClusterCondition) ClearMessage() {
	x.Message = nil
}

type ClusterCondition_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	// Indicates the type of condition.
	Type ClusterConditionType
	// Indicates the status of the condition.
	Status v1.ConditionStatus
	// This time is the last time that the condition was updated.
	LastTransitionTime *timestamppb.Timestamp
	// Contains a the reason of the condition in a format suitable for use by programs.
	//
	// The possible values will be documented in the object that contains the condition.
	Reason *string
	// Contains a text giving more details of the condition.
	//
	// This will usually be progress reports, or error messages, and are intended for use by humans, to debug problems.
	Message *string
}

func (b0 ClusterCondition_builder) Build() *ClusterCondition {
	m0 := &ClusterCondition{}
	b, x := &b0, m0
	_, _ = b, x
	x.Type = b.Type
	x.Status = b.Status
	x.LastTransitionTime = b.LastTransitionTime
	x.Reason = b.Reason
	x.Message = b.Message
	return m0
}

var File_fulfillment_v1_cluster_type_proto protoreflect.FileDescriptor

var file_fulfillment_v1_cluster_type_proto_rawDesc = string([]byte{
	0x0a, 0x21, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2f, 0x76, 0x31,
	0x2f, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x0e, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74,
	0x2e, 0x76, 0x31, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x25, 0x73, 0x68, 0x61, 0x72, 0x65, 0x64, 0x2f, 0x76, 0x31, 0x2f,
	0x63, 0x6f, 0x6e, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x5f, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1d, 0x73, 0x68, 0x61,
	0x72, 0x65, 0x64, 0x2f, 0x76, 0x31, 0x2f, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x5f,
	0x74, 0x79, 0x70, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xb2, 0x01, 0x0a, 0x07, 0x43,
	0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x2f, 0x0a, 0x08, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61,
	0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x73, 0x68, 0x61, 0x72, 0x65,
	0x64, 0x2e, 0x76, 0x31, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x52, 0x08, 0x6d,
	0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x2f, 0x0a, 0x04, 0x73, 0x70, 0x65, 0x63, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d,
	0x65, 0x6e, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x53, 0x70,
	0x65, 0x63, 0x52, 0x04, 0x73, 0x70, 0x65, 0x63, 0x12, 0x35, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x66, 0x75, 0x6c, 0x66, 0x69,
	0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65,
	0x72, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22,
	0x0d, 0x0a, 0x0b, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x53, 0x70, 0x65, 0x63, 0x22, 0xbf,
	0x01, 0x0a, 0x0d, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x12, 0x32, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x1c, 0x2e, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x76, 0x31,
	0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x05, 0x73,
	0x74, 0x61, 0x74, 0x65, 0x12, 0x40, 0x0a, 0x0a, 0x63, 0x6f, 0x6e, 0x64, 0x69, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x20, 0x2e, 0x66, 0x75, 0x6c, 0x66, 0x69,
	0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65,
	0x72, 0x43, 0x6f, 0x6e, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0a, 0x63, 0x6f, 0x6e, 0x64,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x17, 0x0a, 0x07, 0x61, 0x70, 0x69, 0x5f, 0x75, 0x72,
	0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x61, 0x70, 0x69, 0x55, 0x72, 0x6c, 0x12,
	0x1f, 0x0a, 0x0b, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x55, 0x72, 0x6c,
	0x22, 0xa1, 0x02, 0x0a, 0x10, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x43, 0x6f, 0x6e, 0x64,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x38, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x24, 0x2e, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e,
	0x74, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x43, 0x6f, 0x6e, 0x64,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12,
	0x32, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x1a, 0x2e, 0x73, 0x68, 0x61, 0x72, 0x65, 0x64, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6f, 0x6e, 0x64,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x12, 0x4c, 0x0a, 0x14, 0x6c, 0x61, 0x73, 0x74, 0x5f, 0x74, 0x72, 0x61, 0x6e,
	0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x12, 0x6c,
	0x61, 0x73, 0x74, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x69, 0x6d,
	0x65, 0x12, 0x1b, 0x0a, 0x06, 0x72, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x09, 0x48, 0x00, 0x52, 0x06, 0x72, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x88, 0x01, 0x01, 0x12, 0x1d,
	0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x48,
	0x01, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x88, 0x01, 0x01, 0x42, 0x09, 0x0a,
	0x07, 0x5f, 0x72, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x42, 0x0a, 0x0a, 0x08, 0x5f, 0x6d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x2a, 0x7f, 0x0a, 0x0c, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x53,
	0x74, 0x61, 0x74, 0x65, 0x12, 0x1d, 0x0a, 0x19, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f,
	0x53, 0x54, 0x41, 0x54, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45,
	0x44, 0x10, 0x00, 0x12, 0x1d, 0x0a, 0x19, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x53,
	0x54, 0x41, 0x54, 0x45, 0x5f, 0x50, 0x52, 0x4f, 0x47, 0x52, 0x45, 0x53, 0x53, 0x49, 0x4e, 0x47,
	0x10, 0x01, 0x12, 0x17, 0x0a, 0x13, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x53, 0x54,
	0x41, 0x54, 0x45, 0x5f, 0x52, 0x45, 0x41, 0x44, 0x59, 0x10, 0x02, 0x12, 0x18, 0x0a, 0x14, 0x43,
	0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x45, 0x5f, 0x46, 0x41, 0x49,
	0x4c, 0x45, 0x44, 0x10, 0x03, 0x2a, 0xab, 0x01, 0x0a, 0x14, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65,
	0x72, 0x43, 0x6f, 0x6e, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x26,
	0x0a, 0x22, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x43, 0x4f, 0x4e, 0x44, 0x49, 0x54,
	0x49, 0x4f, 0x4e, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49,
	0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x26, 0x0a, 0x22, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45,
	0x52, 0x5f, 0x43, 0x4f, 0x4e, 0x44, 0x49, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x54, 0x59, 0x50, 0x45,
	0x5f, 0x50, 0x52, 0x4f, 0x47, 0x52, 0x45, 0x53, 0x53, 0x49, 0x4e, 0x47, 0x10, 0x01, 0x12, 0x20,
	0x0a, 0x1c, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x43, 0x4f, 0x4e, 0x44, 0x49, 0x54,
	0x49, 0x4f, 0x4e, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x52, 0x45, 0x41, 0x44, 0x59, 0x10, 0x02,
	0x12, 0x21, 0x0a, 0x1d, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x43, 0x4f, 0x4e, 0x44,
	0x49, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x46, 0x41, 0x49, 0x4c, 0x45,
	0x44, 0x10, 0x03, 0x42, 0xd1, 0x01, 0x0a, 0x12, 0x63, 0x6f, 0x6d, 0x2e, 0x66, 0x75, 0x6c, 0x66,
	0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x76, 0x31, 0x42, 0x10, 0x43, 0x6c, 0x75, 0x73,
	0x74, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x50,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x69, 0x6e, 0x6e, 0x61, 0x62,
	0x6f, 0x78, 0x2f, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2d, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f,
	0x61, 0x70, 0x69, 0x2f, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x2f,
	0x76, 0x31, 0x3b, 0x66, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x76, 0x31,
	0xa2, 0x02, 0x03, 0x46, 0x58, 0x58, 0xaa, 0x02, 0x0e, 0x46, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c,
	0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x0e, 0x46, 0x75, 0x6c, 0x66, 0x69, 0x6c,
	0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x1a, 0x46, 0x75, 0x6c, 0x66, 0x69,
	0x6c, 0x6c, 0x6d, 0x65, 0x6e, 0x74, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74,
	0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x0f, 0x46, 0x75, 0x6c, 0x66, 0x69, 0x6c, 0x6c, 0x6d,
	0x65, 0x6e, 0x74, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var file_fulfillment_v1_cluster_type_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_fulfillment_v1_cluster_type_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_fulfillment_v1_cluster_type_proto_goTypes = []any{
	(ClusterState)(0),             // 0: fulfillment.v1.ClusterState
	(ClusterConditionType)(0),     // 1: fulfillment.v1.ClusterConditionType
	(*Cluster)(nil),               // 2: fulfillment.v1.Cluster
	(*ClusterSpec)(nil),           // 3: fulfillment.v1.ClusterSpec
	(*ClusterStatus)(nil),         // 4: fulfillment.v1.ClusterStatus
	(*ClusterCondition)(nil),      // 5: fulfillment.v1.ClusterCondition
	(*v1.Metadata)(nil),           // 6: shared.v1.Metadata
	(v1.ConditionStatus)(0),       // 7: shared.v1.ConditionStatus
	(*timestamppb.Timestamp)(nil), // 8: google.protobuf.Timestamp
}
var file_fulfillment_v1_cluster_type_proto_depIdxs = []int32{
	6, // 0: fulfillment.v1.Cluster.metadata:type_name -> shared.v1.Metadata
	3, // 1: fulfillment.v1.Cluster.spec:type_name -> fulfillment.v1.ClusterSpec
	4, // 2: fulfillment.v1.Cluster.status:type_name -> fulfillment.v1.ClusterStatus
	0, // 3: fulfillment.v1.ClusterStatus.state:type_name -> fulfillment.v1.ClusterState
	5, // 4: fulfillment.v1.ClusterStatus.conditions:type_name -> fulfillment.v1.ClusterCondition
	1, // 5: fulfillment.v1.ClusterCondition.type:type_name -> fulfillment.v1.ClusterConditionType
	7, // 6: fulfillment.v1.ClusterCondition.status:type_name -> shared.v1.ConditionStatus
	8, // 7: fulfillment.v1.ClusterCondition.last_transition_time:type_name -> google.protobuf.Timestamp
	8, // [8:8] is the sub-list for method output_type
	8, // [8:8] is the sub-list for method input_type
	8, // [8:8] is the sub-list for extension type_name
	8, // [8:8] is the sub-list for extension extendee
	0, // [0:8] is the sub-list for field type_name
}

func init() { file_fulfillment_v1_cluster_type_proto_init() }
func file_fulfillment_v1_cluster_type_proto_init() {
	if File_fulfillment_v1_cluster_type_proto != nil {
		return
	}
	file_fulfillment_v1_cluster_type_proto_msgTypes[3].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_fulfillment_v1_cluster_type_proto_rawDesc), len(file_fulfillment_v1_cluster_type_proto_rawDesc)),
			NumEnums:      2,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_fulfillment_v1_cluster_type_proto_goTypes,
		DependencyIndexes: file_fulfillment_v1_cluster_type_proto_depIdxs,
		EnumInfos:         file_fulfillment_v1_cluster_type_proto_enumTypes,
		MessageInfos:      file_fulfillment_v1_cluster_type_proto_msgTypes,
	}.Build()
	File_fulfillment_v1_cluster_type_proto = out.File
	file_fulfillment_v1_cluster_type_proto_goTypes = nil
	file_fulfillment_v1_cluster_type_proto_depIdxs = nil
}
