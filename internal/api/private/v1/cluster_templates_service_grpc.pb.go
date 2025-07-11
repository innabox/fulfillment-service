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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             (unknown)
// source: private/v1/cluster_templates_service.proto

package privatev1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	ClusterTemplates_List_FullMethodName   = "/private.v1.ClusterTemplates/List"
	ClusterTemplates_Get_FullMethodName    = "/private.v1.ClusterTemplates/Get"
	ClusterTemplates_Create_FullMethodName = "/private.v1.ClusterTemplates/Create"
	ClusterTemplates_Delete_FullMethodName = "/private.v1.ClusterTemplates/Delete"
	ClusterTemplates_Update_FullMethodName = "/private.v1.ClusterTemplates/Update"
)

// ClusterTemplatesClient is the client API for ClusterTemplates service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ClusterTemplatesClient interface {
	List(ctx context.Context, in *ClusterTemplatesListRequest, opts ...grpc.CallOption) (*ClusterTemplatesListResponse, error)
	Get(ctx context.Context, in *ClusterTemplatesGetRequest, opts ...grpc.CallOption) (*ClusterTemplatesGetResponse, error)
	Create(ctx context.Context, in *ClusterTemplatesCreateRequest, opts ...grpc.CallOption) (*ClusterTemplatesCreateResponse, error)
	Delete(ctx context.Context, in *ClusterTemplatesDeleteRequest, opts ...grpc.CallOption) (*ClusterTemplatesDeleteResponse, error)
	Update(ctx context.Context, in *ClusterTemplatesUpdateRequest, opts ...grpc.CallOption) (*ClusterTemplatesUpdateResponse, error)
}

type clusterTemplatesClient struct {
	cc grpc.ClientConnInterface
}

func NewClusterTemplatesClient(cc grpc.ClientConnInterface) ClusterTemplatesClient {
	return &clusterTemplatesClient{cc}
}

func (c *clusterTemplatesClient) List(ctx context.Context, in *ClusterTemplatesListRequest, opts ...grpc.CallOption) (*ClusterTemplatesListResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ClusterTemplatesListResponse)
	err := c.cc.Invoke(ctx, ClusterTemplates_List_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusterTemplatesClient) Get(ctx context.Context, in *ClusterTemplatesGetRequest, opts ...grpc.CallOption) (*ClusterTemplatesGetResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ClusterTemplatesGetResponse)
	err := c.cc.Invoke(ctx, ClusterTemplates_Get_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusterTemplatesClient) Create(ctx context.Context, in *ClusterTemplatesCreateRequest, opts ...grpc.CallOption) (*ClusterTemplatesCreateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ClusterTemplatesCreateResponse)
	err := c.cc.Invoke(ctx, ClusterTemplates_Create_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusterTemplatesClient) Delete(ctx context.Context, in *ClusterTemplatesDeleteRequest, opts ...grpc.CallOption) (*ClusterTemplatesDeleteResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ClusterTemplatesDeleteResponse)
	err := c.cc.Invoke(ctx, ClusterTemplates_Delete_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusterTemplatesClient) Update(ctx context.Context, in *ClusterTemplatesUpdateRequest, opts ...grpc.CallOption) (*ClusterTemplatesUpdateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ClusterTemplatesUpdateResponse)
	err := c.cc.Invoke(ctx, ClusterTemplates_Update_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClusterTemplatesServer is the server API for ClusterTemplates service.
// All implementations must embed UnimplementedClusterTemplatesServer
// for forward compatibility.
type ClusterTemplatesServer interface {
	List(context.Context, *ClusterTemplatesListRequest) (*ClusterTemplatesListResponse, error)
	Get(context.Context, *ClusterTemplatesGetRequest) (*ClusterTemplatesGetResponse, error)
	Create(context.Context, *ClusterTemplatesCreateRequest) (*ClusterTemplatesCreateResponse, error)
	Delete(context.Context, *ClusterTemplatesDeleteRequest) (*ClusterTemplatesDeleteResponse, error)
	Update(context.Context, *ClusterTemplatesUpdateRequest) (*ClusterTemplatesUpdateResponse, error)
	mustEmbedUnimplementedClusterTemplatesServer()
}

// UnimplementedClusterTemplatesServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedClusterTemplatesServer struct{}

func (UnimplementedClusterTemplatesServer) List(context.Context, *ClusterTemplatesListRequest) (*ClusterTemplatesListResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedClusterTemplatesServer) Get(context.Context, *ClusterTemplatesGetRequest) (*ClusterTemplatesGetResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedClusterTemplatesServer) Create(context.Context, *ClusterTemplatesCreateRequest) (*ClusterTemplatesCreateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedClusterTemplatesServer) Delete(context.Context, *ClusterTemplatesDeleteRequest) (*ClusterTemplatesDeleteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedClusterTemplatesServer) Update(context.Context, *ClusterTemplatesUpdateRequest) (*ClusterTemplatesUpdateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedClusterTemplatesServer) mustEmbedUnimplementedClusterTemplatesServer() {}
func (UnimplementedClusterTemplatesServer) testEmbeddedByValue()                          {}

// UnsafeClusterTemplatesServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ClusterTemplatesServer will
// result in compilation errors.
type UnsafeClusterTemplatesServer interface {
	mustEmbedUnimplementedClusterTemplatesServer()
}

func RegisterClusterTemplatesServer(s grpc.ServiceRegistrar, srv ClusterTemplatesServer) {
	// If the following call pancis, it indicates UnimplementedClusterTemplatesServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&ClusterTemplates_ServiceDesc, srv)
}

func _ClusterTemplates_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClusterTemplatesListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterTemplatesServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterTemplates_List_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterTemplatesServer).List(ctx, req.(*ClusterTemplatesListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ClusterTemplates_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClusterTemplatesGetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterTemplatesServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterTemplates_Get_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterTemplatesServer).Get(ctx, req.(*ClusterTemplatesGetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ClusterTemplates_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClusterTemplatesCreateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterTemplatesServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterTemplates_Create_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterTemplatesServer).Create(ctx, req.(*ClusterTemplatesCreateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ClusterTemplates_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClusterTemplatesDeleteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterTemplatesServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterTemplates_Delete_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterTemplatesServer).Delete(ctx, req.(*ClusterTemplatesDeleteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ClusterTemplates_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClusterTemplatesUpdateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusterTemplatesServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ClusterTemplates_Update_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusterTemplatesServer).Update(ctx, req.(*ClusterTemplatesUpdateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ClusterTemplates_ServiceDesc is the grpc.ServiceDesc for ClusterTemplates service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ClusterTemplates_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "private.v1.ClusterTemplates",
	HandlerType: (*ClusterTemplatesServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "List",
			Handler:    _ClusterTemplates_List_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _ClusterTemplates_Get_Handler,
		},
		{
			MethodName: "Create",
			Handler:    _ClusterTemplates_Create_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _ClusterTemplates_Delete_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _ClusterTemplates_Update_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "private/v1/cluster_templates_service.proto",
}
