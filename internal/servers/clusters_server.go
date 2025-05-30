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
	"sync"

	"google.golang.org/genproto/googleapis/api/httpbody"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"

	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/database/dao"
	"github.com/innabox/fulfillment-service/internal/jq"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
)

type ClustersServerBuilder struct {
	logger *slog.Logger
}

var _ ffv1.ClustersServer = (*ClustersServer)(nil)

type ClustersServer struct {
	ffv1.UnimplementedClustersServer

	logger          *slog.Logger
	jqTool          *jq.Tool
	clustersDao     *dao.GenericDAO[*privatev1.Cluster]
	hubsDao         *dao.GenericDAO[*privatev1.Hub]
	generic         *GenericServer[*ffv1.Cluster, *privatev1.Cluster]
	kubeClients     map[string]clnt.Client
	kubeClientsLock *sync.Mutex
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

	// Create the JQ tool:
	jqTool, err := jq.NewTool().
		SetLogger(b.logger).
		Build()
	if err != nil {
		return
	}

	// Create the DAOs:
	clustersDao, err := dao.NewGenericDAO[*privatev1.Cluster]().
		SetLogger(b.logger).
		SetTable("clusters").
		Build()
	if err != nil {
		return
	}
	hubsDao, err := dao.NewGenericDAO[*privatev1.Hub]().
		SetLogger(b.logger).
		SetTable("hubs").
		Build()
	if err != nil {
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*ffv1.Cluster, *privatev1.Cluster]().
		SetLogger(b.logger).
		SetService(ffv1.Clusters_ServiceDesc.ServiceName).
		SetTable("clusters").
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &ClustersServer{
		logger:          b.logger,
		jqTool:          jqTool,
		clustersDao:     clustersDao,
		hubsDao:         hubsDao,
		generic:         generic,
		kubeClients:     map[string]clnt.Client{},
		kubeClientsLock: &sync.Mutex{},
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

func (s *ClustersServer) getKubeconfig(ctx context.Context, clusterId string) (result []byte, err error) {
	// Validate the request:
	if clusterId == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "cluster identifier is mandatory")
		return
	}

	// Prepare a logger with additional information about the cluster:
	logger := s.logger.With(
		slog.String("cluster_id", clusterId),
	)

	// Prepare the error to return to the client if some internal error happens:
	internalErr := grpcstatus.Errorf(
		grpccodes.Internal,
		"failed to get kubeconfig for cluster with identifier '%s'",
		clusterId,
	)

	// Try to get the secret that contains the kubeconfig:
	secret, err := s.getHostedClusterSecret(ctx, clusterId, ".status.kubeconfig.name")
	if err != nil {
		logger.ErrorContext(
			ctx,
			"Failed to get hosted cluster secret",
			slog.Any("error", err),
		)
		err = internalErr
		return
	}
	if secret == nil {
		err = grpcstatus.Errorf(
			grpccodes.NotFound,
			"kubeconfig for cluster with identifier '%s' isn't yet available",
			clusterId,
		)
		return
	}

	// Check that the secret has the expected entry, and that it isn't empty:
	kcBytes, ok := secret.Data["kubeconfig"]
	if !ok {
		logger.ErrorContext(ctx, "Kubeconfig secret entry doesn't exist")
		err = internalErr
		return
	}
	if len(kcBytes) == 0 {
		logger.ErrorContext(ctx, "Kubeconfig secret entry is empty")
		err = internalErr
		return
	}

	// Done:
	logger.DebugContext(
		ctx,
		"Returning kubeconfig",
		slog.Int("kc_bytes", len(kcBytes)),
	)
	result = kcBytes
	return
}

func (s *ClustersServer) GetPassword(ctx context.Context,
	request *ffv1.ClustersGetPasswordRequest) (response *ffv1.ClustersGetPasswordResponse, err error) {
	password, err := s.getPassword(ctx, request.Id)
	if err != nil {
		return
	}
	response = &ffv1.ClustersGetPasswordResponse{
		Password: password,
	}
	return
}

func (s *ClustersServer) GetPasswordViaHttp(ctx context.Context,
	request *ffv1.ClustersGetPasswordViaHttpRequest) (response *httpbody.HttpBody, err error) {
	password, err := s.getPassword(ctx, request.Id)
	if err != nil {
		return
	}
	response = &httpbody.HttpBody{
		ContentType: "text/plain",
		Data:        []byte(password),
	}
	return
}

func (s *ClustersServer) getPassword(ctx context.Context, clusterId string) (result string, err error) {
	// Validate the request:
	if clusterId == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "cluster identifier is mandatory")
		return
	}

	// Prepare a logger with additional information about the cluster:
	logger := s.logger.With(
		slog.String("cluster_id", clusterId),
	)

	// Prepare the error to return to the client if some internal error happens:
	internalErr := grpcstatus.Errorf(
		grpccodes.Internal,
		"failed to get password for cluster with identifier '%s'",
		clusterId,
	)

	// Try to get the secret that contains the password:
	secret, err := s.getHostedClusterSecret(ctx, clusterId, ".status.kubeadminPassword.name")
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get hosted cluster",
			slog.Any("error", err),
		)
		err = internalErr
		return
	}
	if secret == nil {
		err = grpcstatus.Errorf(
			grpccodes.NotFound,
			"password for cluster with identifier '%s' isn't available yet",
			clusterId,
		)
		return
	}

	// Check that the secret has the expected entry, and that it isn't empty:
	passwordBytes, ok := secret.Data["password"]
	if !ok {
		logger.ErrorContext(ctx, "Password secret entry doesn't exist")
		err = internalErr
		return
	}
	if len(passwordBytes) == 0 {
		logger.ErrorContext(ctx, "Password secret entry is empty")
		err = internalErr
		return
	}

	// Done:
	logger.DebugContext(
		ctx,
		"Returning password",
		slog.Int("password_bytes", len(passwordBytes)),
	)
	result = string(passwordBytes)
	return
}

func (s *ClustersServer) getHostedClusterSecret(ctx context.Context, clusterId string,
	secretField string) (result *corev1.Secret, err error) {
	// Get the data of the cluster:
	cluster, err := s.clustersDao.Get(ctx, clusterId)
	if err != nil || cluster == nil || cluster.HubId == "" {
		return
	}

	// Get the data of the hub:
	hub, err := s.hubsDao.Get(ctx, cluster.HubId)
	if err != nil || hub == nil {
		return
	}

	// Create a client for the hub:
	hubClient, err := s.getKubeClient(ctx, hub)
	if err != nil {
		return
	}

	// Get the cluster order from the hub:
	order, err := s.getKubeClusterOrder(ctx, hubClient, hub.Namespace, cluster.OrderId)
	if err != nil {
		return
	}

	// Extract the location of the hosted cluster:
	hcKey := clnt.ObjectKey{}
	err = s.jqTool.Evaluate(
		`.status.clusterReference | {
			Namespace: .namespace,
			Name: .hostedClusterName
		}`,
		order.Object, &hcKey,
	)
	if err != nil {
		return
	}

	// Get the hosted cluster from the hub:
	hc, err := s.getKubeHostedCluster(ctx, hubClient, hcKey)
	if err != nil || hc == nil {
		return
	}

	// Extract the name of the secret from the hosted cluster:
	secretKey := clnt.ObjectKey{
		Namespace: hc.GetNamespace(),
	}
	err = s.jqTool.Evaluate(secretField, hc.Object, &secretKey.Name)
	if err != nil {
		return
	}

	// Get the secret from the hub:
	result, err = s.getKubeSecret(ctx, hubClient, secretKey)
	return
}

func (s *ClustersServer) getKubeClient(ctx context.Context, hub *privatev1.Hub) (result clnt.Client, err error) {
	s.kubeClientsLock.Lock()
	defer s.kubeClientsLock.Unlock()
	result, ok := s.kubeClients[hub.Id]
	if ok {
		return
	}
	result, err = s.createKubeClient(ctx, hub)
	if err != nil {
		return
	}
	s.kubeClients[hub.Id] = result
	return
}

func (s *ClustersServer) createKubeClient(ctx context.Context, hub *privatev1.Hub) (result clnt.Client, err error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(hub.Kubeconfig)
	if err != nil {
		return
	}
	result, err = clnt.New(config, clnt.Options{})
	return
}

func (s *ClustersServer) getKubeClusterOrder(ctx context.Context, client clnt.Client,
	namespace string, id string) (result *unstructured.Unstructured, err error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvks.ClusterOrderList)
	err = client.List(
		ctx, list,
		clnt.InNamespace(namespace),
		clnt.MatchingLabels{
			labels.ClusterOrderUuid: id,
		},
	)
	if err != nil {
		return
	}
	items := list.Items
	if len(items) != 1 {
		err = fmt.Errorf(
			"expected exactly one cluster order with identifier '%s' but found %d",
			id, len(items),
		)
		return
	}
	result = &items[0]
	return
}

func (s *ClustersServer) getKubeHostedCluster(ctx context.Context, client clnt.Client,
	key clnt.ObjectKey) (result *unstructured.Unstructured, err error) {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvks.HostedCluster)
	err = client.Get(ctx, key, object)
	if apierrors.IsNotFound(err) {
		err = nil
		return
	}
	if err != nil {
		return
	}
	result = object
	return
}

func (s *ClustersServer) getKubeSecret(ctx context.Context, client clnt.Client,
	key clnt.ObjectKey) (result *corev1.Secret, err error) {
	object := &corev1.Secret{}
	err = client.Get(ctx, key, object)
	if apierrors.IsNotFound(err) {
		err = nil
		return
	}
	if err != nil {
		return
	}
	result = object
	return
}
