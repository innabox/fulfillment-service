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

	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/innabox/fulfillment-service/internal/database/dao"
	"google.golang.org/genproto/googleapis/api/httpbody"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type ClustersServerBuilder struct {
	logger *slog.Logger
}

var _ ffv1.ClustersServer = (*ClustersServer)(nil)

type ClustersServer struct {
	ffv1.UnimplementedClustersServer

	logger      *slog.Logger
	clustersDao *dao.GenericDAO[*ffv1.Cluster]
	generic     *GenericServer[*ffv1.Cluster]
}

func NewClustersServer() *ClustersServerBuilder {
	return &ClustersServerBuilder{}
}

func (b *ClustersServerBuilder) SetLogger(value *slog.Logger) *ClustersServerBuilder {
	b.logger = value
	return b
}

func (b *ClustersServerBuilder) Build() (result *ClustersServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the DAOs:
	clustersDao, err := dao.NewGenericDAO[*ffv1.Cluster]().
		SetLogger(b.logger).
		SetTable("clusters").
		Build()
	if err != nil {
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*ffv1.Cluster]().
		SetLogger(b.logger).
		SetService(ffv1.Clusters_ServiceDesc.ServiceName).
		SetTable("clusters").
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &ClustersServer{
		logger:      b.logger,
		clustersDao: clustersDao,
		generic:     generic,
	}
	return
}

func (s *ClustersServer) List(ctx context.Context,
	request *ffv1.ClustersListRequest) (response *ffv1.ClustersListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *ClustersServer) Get(ctx context.Context,
	request *ffv1.ClustersGetRequest) (response *ffv1.ClustersGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *ClustersServer) Create(ctx context.Context,
	request *ffv1.ClustersCreateRequest) (response *ffv1.ClustersCreateResponse, err error) {
	err = s.validateId(ctx, request.Object)
	if err != nil {
		return
	}
	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *ClustersServer) Update(ctx context.Context,
	request *ffv1.ClustersUpdateRequest) (response *ffv1.ClustersUpdateResponse, err error) {
	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *ClustersServer) Delete(ctx context.Context,
	request *ffv1.ClustersDeleteRequest) (response *ffv1.ClustersDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}

func (s *ClustersServer) GetKubeconfig(ctx context.Context,
	request *ffv1.ClustersGetKubeconfigRequest) (response *ffv1.ClustersGetKubeconfigResponse, err error) {
	kubeconfig, err := s.getKubeconfig(ctx, request.Id)
	if err != nil {
		return
	}
	response = &ffv1.ClustersGetKubeconfigResponse{
		Kubeconfig: string(kubeconfig),
	}
	return
}

func (s *ClustersServer) GetKubeconfigViaHttp(ctx context.Context,
	request *ffv1.ClustersGetKubeconfigViaHttpRequest) (response *httpbody.HttpBody, err error) {
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
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "field 'cluster_id' is mandatory")
		return
	}

	// Check that the cluster exists:
	exists, err := s.clustersDao.Exists(ctx, id)
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
	if !exists {
		err = grpcstatus.Errorf(grpccodes.NotFound, "cluster with id '%s' not found", id)
		return
	}

	// TODO: Fetch the kubeconfig.
	kubeconfig = []byte{}
	return
}

func (s *ClustersServer) validateId(ctx context.Context, object *ffv1.Cluster) error {
	if object == nil {
		return grpcstatus.Error(
			grpccodes.InvalidArgument,
			"object is mandatory",
		)
	}
	id := object.Id
	if id != "" {
		return grpcstatus.Error(
			grpccodes.InvalidArgument,
			"identifier isn't allowed",
		)
	}
	return nil
}
