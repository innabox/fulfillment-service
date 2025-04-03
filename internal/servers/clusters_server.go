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
	"log/slog"

	api "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/innabox/fulfillment-service/internal/database/dao"
	"github.com/spf13/pflag"
	"google.golang.org/genproto/googleapis/api/httpbody"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type ClustersServerBuilder struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	daos   dao.Set
}

var _ api.ClustersServer = (*ClustersServer)(nil)

type ClustersServer struct {
	api.UnimplementedClustersServer

	logger *slog.Logger
	daos   dao.Set
}

func NewClustersServer() *ClustersServerBuilder {
	return &ClustersServerBuilder{}
}

func (b *ClustersServerBuilder) SetLogger(value *slog.Logger) *ClustersServerBuilder {
	b.logger = value
	return b
}

func (b *ClustersServerBuilder) SetDAOs(value dao.Set) *ClustersServerBuilder {
	b.daos = value
	return b
}

func (b *ClustersServerBuilder) SetFlags(value *pflag.FlagSet) *ClustersServerBuilder {
	b.flags = value
	return b
}

func (b *ClustersServerBuilder) Build() (result *ClustersServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.daos == nil {
		err = errors.New("data access objects are mandatory")
		return
	}

	// Create and populate the object:
	result = &ClustersServer{
		logger: b.logger,
		daos:   b.daos,
	}
	return
}

func (s *ClustersServer) List(ctx context.Context,
	request *api.ClustersListRequest) (response *api.ClustersListResponse, err error) {
	clusters, err := s.daos.Clusters().List(ctx, dao.ListRequest{
		Offset: request.GetOffset(),
		Limit:  request.GetLimit(),
	})
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to list clusters",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to list clusters")
		return
	}
	response = &api.ClustersListResponse{
		Size:  proto.Int32(clusters.Size),
		Total: proto.Int32(clusters.Total),
		Items: clusters.Items,
	}
	return
}

func (s *ClustersServer) Get(ctx context.Context,
	request *api.ClustersGetRequest) (response *api.ClustersGetResponse, err error) {
	cluster, err := s.daos.Clusters().Get(ctx, request.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster",
			slog.String("cluster_id", request.Id),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to get cluster with id '%s'", request.Id)
		return
	}
	if cluster == nil {
		err = grpcstatus.Errorf(grpccodes.NotFound, "cluster with id '%s' not found", request.Id)
		return
	}
	response = &api.ClustersGetResponse{
		Object: cluster,
	}
	return
}

func (s *ClustersServer) GetKubeconfig(ctx context.Context,
	request *api.ClustersGetKubeconfigRequest) (response *api.ClustersGetKubeconfigResponse, err error) {
	kubeconfig, err := s.getKubeconfig(ctx, request.Id)
	if err != nil {
		return
	}
	response = &api.ClustersGetKubeconfigResponse{
		Kubeconfig: string(kubeconfig),
	}
	return
}

func (s *ClustersServer) GetKubeconfigViaHttp(ctx context.Context,
	request *api.ClustersGetKubeconfigViaHttpRequest) (response *httpbody.HttpBody, err error) {
	kubeconfig, err := s.getKubeconfig(ctx, request.Id)
	if err != nil {
		return
	}
	response = &httpbody.HttpBody{
		ContentType: "application/yaml",
		Data:        kubeconfig,
	}
	return
}

func (s *ClustersServer) getKubeconfig(ctx context.Context, id string) (kubeconfig []byte, err error) {
	// Validate the request:
	if id == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "field 'id' is mandatory")
		return
	}

	// Check that the cluster exists:
	ok, err := s.daos.Clusters().Exists(ctx, id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster",
			slog.String("cluster_id", id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to get cluster with id '%s'", id)
		return
	}
	if !ok {
		err = grpcstatus.Errorf(grpccodes.NotFound, "cluster with id '%s' not found", id)
		return
	}

	// TODO: Fetch the kubeconfig.
	kubeconfig = []byte{}
	return
}

func (s *ClustersServer) Create(ctx context.Context,
	request *api.ClustersCreateRequest) (response *api.ClustersCreateResponse, err error) {
	// Validate the request:
	if request.Object == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "cluster is required")
		return
	}
	if request.Object.Id != "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "cluster identifier isn't allowed")
		return
	}
	if request.Object.Spec == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "cluster spec is required")
		return
	}
	if request.Object.Status != nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "status isn't allowed")
		return
	}

	// Save the new cluster:
	cluster := request.Object
	_, err = s.daos.Clusters().Insert(ctx, cluster)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to insert cluster",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to create cluster")
		return
	}

	// Return the result:
	response = &api.ClustersCreateResponse{
		Object: cluster,
	}
	return
}

func (s *ClustersServer) Delete(ctx context.Context,
	request *api.ClustersDeleteRequest) (response *api.ClustersDeleteResponse, err error) {
	// Fetch the cluster:
	cluster, err := s.daos.Clusters().Get(ctx, request.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster",
			slog.String("id", request.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to get cluster '%s'", request.Id)
		return
	}
	if cluster == nil {
		err = grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"cluster with identifier '%s' doesn't exist",
			request.Id,
		)
		return
	}

	// Delete the cluster:
	err = s.daos.Clusters().Delete(ctx, cluster.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to delete cluster",
			slog.String("id", request.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to delete cluster with identifier '%s'",
			request.Id,
		)
		return
	}
	response = &api.ClustersDeleteResponse{}
	return
}

func (s *ClustersServer) UpdateStatus(ctx context.Context,
	request *api.ClustersUpdateStatusRequest) (response *api.ClustersUpdateStatusResponse, err error) {
	// Fetch the cluster:
	cluster, err := s.daos.Clusters().Get(ctx, request.Id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster",
			slog.String("id", request.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to get cluster '%s'", request.Id)
		return
	}
	if cluster == nil {
		err = grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"cluster with identifier '%s' doesn't exist",
			request.Id,
		)
		return
	}

	// Merge the request data into the old data:
	cluster.Status = request.Status

	// Save the result:
	err = s.daos.Clusters().Update(ctx, cluster.Id, cluster)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to update cluster status",
			slog.String("id", request.Id),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to update status for cluster with identifier '%s'",
			request.Id,
		)
		return
	}
	response = &api.ClustersUpdateStatusResponse{
		Status: cluster.Status,
	}
	return
}
