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
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"

	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/auth"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
	"github.com/innabox/fulfillment-service/internal/jq"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
)

type ClustersServerBuilder struct {
	logger           *slog.Logger
	notifier         *database.Notifier
	attributionLogic auth.AttributionLogic
	tenancyLogic     auth.TenancyLogic
}

var _ ffv1.ClustersServer = (*ClustersServer)(nil)

type ClustersServer struct {
	ffv1.UnimplementedClustersServer

	logger          *slog.Logger
	notifier        *database.Notifier
	private         privatev1.ClustersServer
	inMapper        *GenericMapper[*ffv1.Cluster, *privatev1.Cluster]
	outMapper       *GenericMapper[*privatev1.Cluster, *ffv1.Cluster]
	jqTool          *jq.Tool
	hubsDao         *dao.GenericDAO[*privatev1.Hub]
	kubeClients     map[string]clnt.Client
	kubeClientsLock *sync.Mutex
}

func NewClustersServer() *ClustersServerBuilder {
	return &ClustersServerBuilder{}
}

// SetLogger sets the logger to use. This is mandatory.
func (b *ClustersServerBuilder) SetLogger(value *slog.Logger) *ClustersServerBuilder {
	b.logger = value
	return b
}

// SetNotifier sets the notifier to use. This is optional.
func (b *ClustersServerBuilder) SetNotifier(value *database.Notifier) *ClustersServerBuilder {
	b.notifier = value
	return b
}

// SetAttributionLogic sets the attribution logic to use. This is optional.
func (b *ClustersServerBuilder) SetAttributionLogic(value auth.AttributionLogic) *ClustersServerBuilder {
	b.attributionLogic = value
	return b
}

// SetTenancyLogic sets the tenancy logic to use. This is mandatory.
func (b *ClustersServerBuilder) SetTenancyLogic(value auth.TenancyLogic) *ClustersServerBuilder {
	b.tenancyLogic = value
	return b
}

func (b *ClustersServerBuilder) Build() (result *ClustersServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.tenancyLogic == nil {
		err = errors.New("tenancy logic is mandatory")
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
	hubsDao, err := dao.NewGenericDAO[*privatev1.Hub]().
		SetLogger(b.logger).
		SetTable("hubs").
		SetTenancyLogic(b.tenancyLogic).
		Build()
	if err != nil {
		return
	}

	// Find the full name of the 'status' field so that we can configure the generic server to ignore it. This is
	// because users don't have permission to change the status.
	var object *ffv1.Cluster
	objectReflect := object.ProtoReflect()
	objectDesc := objectReflect.Descriptor()
	statusField := objectDesc.Fields().ByName("status")
	if statusField == nil {
		err = fmt.Errorf("failed to find the status field of type '%s'", objectDesc.FullName())
		return
	}

	// Create the mappers:
	inMapper, err := NewGenericMapper[*ffv1.Cluster, *privatev1.Cluster]().
		SetLogger(b.logger).
		SetStrict(true).
		AddIgnoredFields(statusField.FullName()).
		Build()
	if err != nil {
		return
	}
	outMapper, err := NewGenericMapper[*privatev1.Cluster, *ffv1.Cluster]().
		SetLogger(b.logger).
		SetStrict(false).
		Build()
	if err != nil {
		return
	}

	// Create the private server to delegate to:
	delegate, err := NewPrivateClustersServer().
		SetLogger(b.logger).
		SetNotifier(b.notifier).
		SetAttributionLogic(b.attributionLogic).
		SetTenancyLogic(b.tenancyLogic).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &ClustersServer{
		logger:          b.logger,
		notifier:        b.notifier,
		jqTool:          jqTool,
		hubsDao:         hubsDao,
		kubeClients:     map[string]clnt.Client{},
		kubeClientsLock: &sync.Mutex{},
		private:         delegate,
		inMapper:        inMapper,
		outMapper:       outMapper,
	}
	return
}

func (s *ClustersServer) List(ctx context.Context,
	request *ffv1.ClustersListRequest) (response *ffv1.ClustersListResponse, err error) {
	// Create private request with same parameters:
	privateRequest := &privatev1.ClustersListRequest{}
	privateRequest.SetOffset(request.GetOffset())
	privateRequest.SetLimit(request.GetLimit())
	privateRequest.SetFilter(request.GetFilter())

	// Delegate to private server:
	privateResponse, err := s.private.List(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map private response to public format:
	privateItems := privateResponse.GetItems()
	publicItems := make([]*ffv1.Cluster, len(privateItems))
	for i, privateItem := range privateItems {
		publicItem := &ffv1.Cluster{}
		err = s.outMapper.Copy(ctx, privateItem, publicItem)
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to map private cluster to public",
				slog.Any("error", err),
			)
			return nil, grpcstatus.Errorf(grpccodes.Internal, "failed to process clusters")
		}
		publicItems[i] = publicItem
	}

	// Create the public response:
	response = &ffv1.ClustersListResponse{}
	response.SetSize(privateResponse.GetSize())
	response.SetTotal(privateResponse.GetTotal())
	response.SetItems(publicItems)
	return
}

func (s *ClustersServer) Get(ctx context.Context,
	request *ffv1.ClustersGetRequest) (response *ffv1.ClustersGetResponse, err error) {
	// Create private request:
	privateRequest := &privatev1.ClustersGetRequest{}
	privateRequest.SetId(request.GetId())

	// Delegate to private server:
	privateResponse, err := s.private.Get(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map private response to public format:
	privateCluster := privateResponse.GetObject()
	publicCluster := &ffv1.Cluster{}
	err = s.outMapper.Copy(ctx, privateCluster, publicCluster)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private cluster to public",
			slog.Any("error", err),
		)
		return nil, grpcstatus.Errorf(grpccodes.Internal, "failed to process cluster")
	}

	// Create the public response:
	response = &ffv1.ClustersGetResponse{}
	response.SetObject(publicCluster)
	return
}

func (s *ClustersServer) Create(ctx context.Context,
	request *ffv1.ClustersCreateRequest) (response *ffv1.ClustersCreateResponse, err error) {
	// Map the public cluster to private format:
	publicCluster := request.GetObject()
	if publicCluster == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
		return
	}
	privateCluster := &privatev1.Cluster{}
	err = s.inMapper.Copy(ctx, publicCluster, privateCluster)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map public cluster to private",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process cluster")
		return
	}

	// Delegate to the private server:
	privateRequest := &privatev1.ClustersCreateRequest{}
	privateRequest.SetObject(privateCluster)
	privateResponse, err := s.private.Create(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map the private response back to public format:
	createdPrivateCluster := privateResponse.GetObject()
	createdPublicCluster := &ffv1.Cluster{}
	err = s.outMapper.Copy(ctx, createdPrivateCluster, createdPublicCluster)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private cluster to public",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process cluster")
		return
	}

	// Create the public response:
	response = &ffv1.ClustersCreateResponse{}
	response.SetObject(createdPublicCluster)
	return
}

func (s *ClustersServer) Update(ctx context.Context,
	request *ffv1.ClustersUpdateRequest) (response *ffv1.ClustersUpdateResponse, err error) {
	// Validate the request:
	publicCluster := request.GetObject()
	if publicCluster == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
		return
	}
	id := publicCluster.GetId()
	if id == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object identifier is mandatory")
		return
	}

	// Get the existing object from the private server:
	getRequest := &privatev1.ClustersGetRequest{}
	getRequest.SetId(id)
	getResponse, err := s.private.Get(ctx, getRequest)
	if err != nil {
		return nil, err
	}
	existingPrivateCluster := getResponse.GetObject()

	// Map the public changes to the existing private object (preserving private data):
	err = s.inMapper.Copy(ctx, publicCluster, existingPrivateCluster)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map public cluster to private",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process cluster")
		return
	}

	// Apply field mask replacements for map fields. The mapper merges maps by default,
	// but when a field mask explicitly specifies a map field, we want to replace it entirely.
	updateMask := request.GetUpdateMask()
	s.logger.Debug("clusters_server.Update: about to call applyNodeSetsReplacement",
		slog.Any("update_mask", updateMask))
	s.applyNodeSetsReplacement(updateMask, publicCluster, existingPrivateCluster)

	// Delegate to the private server with the merged object:
	privateRequest := &privatev1.ClustersUpdateRequest{}
	privateRequest.SetObject(existingPrivateCluster)
	privateRequest.SetUpdateMask(updateMask)
	privateResponse, err := s.private.Update(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Map the private response back to public format:
	updatedPrivateCluster := privateResponse.GetObject()
	updatedPublicCluster := &ffv1.Cluster{}
	err = s.outMapper.Copy(ctx, updatedPrivateCluster, updatedPublicCluster)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map private cluster to public",
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to process cluster")
		return
	}

	// Create the public response:
	response = &ffv1.ClustersUpdateResponse{}
	response.SetObject(updatedPublicCluster)
	return
}

func (s *ClustersServer) Delete(ctx context.Context,
	request *ffv1.ClustersDeleteRequest) (response *ffv1.ClustersDeleteResponse, err error) {
	// Create private request:
	privateRequest := &privatev1.ClustersDeleteRequest{}
	privateRequest.SetId(request.GetId())

	// Delegate to private server:
	_, err = s.private.Delete(ctx, privateRequest)
	if err != nil {
		return nil, err
	}

	// Create the public response:
	response = &ffv1.ClustersDeleteResponse{}
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
	// Get the data of the cluster from the private server:
	getRequest := &privatev1.ClustersGetRequest{}
	getRequest.SetId(clusterId)
	getResponse, err := s.private.Get(ctx, getRequest)
	if err != nil {
		return
	}
	cluster := getResponse.GetObject()
	if cluster == nil || cluster.GetStatus().GetHub() == "" {
		return
	}

	// Get the data of the hub:
	hub, err := s.hubsDao.Get(ctx, cluster.GetStatus().GetHub())
	if err != nil || hub == nil {
		return
	}

	// Create a client for the hub:
	hubClient, err := s.getKubeClient(ctx, hub)
	if err != nil {
		return
	}

	// Get the cluster order from the hub:
	order, err := s.getKubeClusterOrder(ctx, hubClient, hub.Namespace, cluster.GetId())
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

// applyNodeSetsReplacement handles map field replacement when a field mask is used.
// The mapper.Copy merges maps by default, which is usually desired behavior. However,
// when a field mask explicitly specifies "spec.node_sets", we want to replace the entire
// map rather than merge it. This function removes node sets from the private cluster
// that don't exist in the public cluster when spec.node_sets is in the update mask.
func (s *ClustersServer) applyNodeSetsReplacement(updateMask *fieldmaskpb.FieldMask,
	publicCluster *ffv1.Cluster, privateCluster *privatev1.Cluster) {
	s.logger.Debug("applyNodeSetsReplacement: function called")

	if updateMask == nil {
		s.logger.Debug("applyNodeSetsReplacement: updateMask is nil, returning")
		return
	}

	s.logger.Debug("applyNodeSetsReplacement: checking update mask paths",
		slog.Any("paths", updateMask.GetPaths()))

	// Check if the update mask includes spec.node_sets
	for _, path := range updateMask.GetPaths() {
		s.logger.Debug("applyNodeSetsReplacement: checking path",
			slog.String("path", path),
			slog.Bool("matches", path == "spec.node_sets"))
		if path == "spec.node_sets" {
			s.logger.Debug("applyNodeSetsReplacement: found spec.node_sets in update mask")

			// Remove node sets that exist in private but not in public
			if publicCluster.GetSpec() != nil && privateCluster.GetSpec() != nil {
				publicNodeSets := publicCluster.GetSpec().GetNodeSets()
				privateNodeSets := privateCluster.GetSpec().GetNodeSets()

				// Debug logging
				publicKeys := make([]string, 0, len(publicNodeSets))
				for k := range publicNodeSets {
					publicKeys = append(publicKeys, k)
				}
				privateKeysBefore := make([]string, 0, len(privateNodeSets))
				for k := range privateNodeSets {
					privateKeysBefore = append(privateKeysBefore, k)
				}
				s.logger.Debug(
					"Applying node sets replacement",
					slog.Any("public_keys", publicKeys),
					slog.Any("private_keys_before", privateKeysBefore),
				)

				for nodeSetKey := range privateNodeSets {
					if _, exists := publicNodeSets[nodeSetKey]; !exists {
						s.logger.Debug("Removing node set", slog.String("key", nodeSetKey))
						delete(privateNodeSets, nodeSetKey)
					}
				}

				privateKeysAfter := make([]string, 0, len(privateNodeSets))
				for k := range privateNodeSets {
					privateKeysAfter = append(privateKeysAfter, k)
				}
				s.logger.Debug(
					"After node sets replacement",
					slog.Any("private_keys_after", privateKeysAfter),
				)
			}
			break
		}
	}

	s.logger.Debug("applyNodeSetsReplacement: function completed")
}
