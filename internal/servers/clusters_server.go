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
	"github.com/innabox/fulfillment-service/internal/database/models"
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
	clusters, err := s.daos.Clusters().List(ctx)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to list clusters",
			slog.String("error", err.Error()),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to list clusters")
		return
	}
	results := make([]*api.Cluster, len(clusters))
	for i, model := range clusters {
		results[i] = &api.Cluster{}
		err = s.mapOutbound(model, results[i])
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to map outbound cluster",
				slog.String("error", err.Error()),
			)
			err = grpcstatus.Errorf(grpccodes.Internal, "failed to map outbound cluster")
			return
		}
	}
	response = &api.ClustersListResponse{
		Size:  proto.Int32(int32(len(results))),
		Total: proto.Int32(int32(len(results))),
		Items: results,
	}
	return
}

func (s *ClustersServer) Get(ctx context.Context,
	request *api.ClustersGetRequest) (response *api.ClustersGetResponse, err error) {
	cluster, err := s.daos.Clusters().Get(ctx, request.ClusterId)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster",
			slog.String("cluster_id", request.ClusterId),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to get cluster with id '%s'", request.ClusterId)
		return
	}
	if cluster == nil {
		err = grpcstatus.Errorf(grpccodes.NotFound, "cluster with id '%s' not found", request.ClusterId)
		return
	}
	result := &api.Cluster{}
	err = s.mapOutbound(cluster, result)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map outbound cluster",
			slog.String("error", err.Error()),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to map outbound cluster")
		return
	}
	response = &api.ClustersGetResponse{
		Cluster: result,
	}
	return
}

func (s *ClustersServer) GetKubeconfig(ctx context.Context,
	request *api.ClustersGetKubeconfigRequest) (response *api.ClustersGetKubeconfigResponse, err error) {
	kubeconfig, err := s.getKubeconfig(ctx, request.ClusterId)
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
	kubeconfig, err := s.getKubeconfig(ctx, request.ClusterId)
	if err != nil {
		return
	}
	response = &httpbody.HttpBody{
		ContentType: "application/yaml",
		Data:        kubeconfig,
	}
	return
}

func (s *ClustersServer) getKubeconfig(ctx context.Context, clusterId string) (kubeconfig []byte, err error) {
	// Validate the request:
	if clusterId == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "field 'cluster_id' is mandatory")
		return
	}

	// Check that the cluster exists:
	ok, err := s.daos.Clusters().Exists(ctx, clusterId)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get cluster",
			slog.String("cluster_id", clusterId),
			slog.String("error", err.Error()),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to get cluster with id '%s'", clusterId)
		return
	}
	if !ok {
		err = grpcstatus.Errorf(grpccodes.NotFound, "cluster with id '%s' not found", clusterId)
		return
	}

	// TODO: Fetch the kubeconfig.
	kubeconfig = []byte{}
	return
}

func (s *ClustersServer) mapOutbound(from *models.Cluster, to *api.Cluster) error {
	to.Id = from.ID
	if to.Spec == nil {
		to.Spec = &api.ClusterSpec{}
	}
	if to.Status == nil {
		to.Status = &api.ClusterStatus{}
	}
	to.Status.ApiUrl = from.APIURL
	to.Status.ConsoleUrl = from.ConsoleURL
	return nil
}
