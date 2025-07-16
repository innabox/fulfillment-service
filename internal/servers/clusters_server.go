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
	"sort"
	"sync"

	"golang.org/x/exp/maps"
	"google.golang.org/genproto/googleapis/api/httpbody"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dustin/go-humanize/english"
	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
	"github.com/innabox/fulfillment-service/internal/jq"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
)

type ClustersServerBuilder struct {
	logger   *slog.Logger
	notifier *database.Notifier
}

var _ ffv1.ClustersServer = (*ClustersServer)(nil)

type ClustersServer struct {
	ffv1.UnimplementedClustersServer

	logger          *slog.Logger
	jqTool          *jq.Tool
	clustersDao     *dao.GenericDAO[*privatev1.Cluster]
	templatesDao    *dao.GenericDAO[*privatev1.ClusterTemplate]
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

func (b *ClustersServerBuilder) SetNotifier(value *database.Notifier) *ClustersServerBuilder {
	b.notifier = value
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
	templatesDao, err := dao.NewGenericDAO[*privatev1.ClusterTemplate]().
		SetLogger(b.logger).
		SetTable("cluster_templates").
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

	// Create the generic server:
	generic, err := NewGenericServer[*ffv1.Cluster, *privatev1.Cluster]().
		SetLogger(b.logger).
		SetService(ffv1.Clusters_ServiceDesc.ServiceName).
		SetTable("clusters").
		AddIgnoredFields(statusField.FullName()).
		SetNotifier(b.notifier).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &ClustersServer{
		logger:          b.logger,
		jqTool:          jqTool,
		clustersDao:     clustersDao,
		templatesDao:    templatesDao,
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
	// Check that the template is specified and that refers to a existing template:
	cluster := request.GetObject()
	if cluster == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
		return
	}
	templateId := cluster.GetSpec().GetTemplate()
	if templateId == "" {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "template is mandatory")
		return
	}
	template, err := s.templatesDao.Get(ctx, templateId)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get template",
			slog.String("template", templateId),
			slog.Any("error", err),
		)
		err = grpcstatus.Errorf(grpccodes.Internal, "failed to get template '%s'", templateId)
		return
	}
	if template == nil {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "template '%s' doesn't exist", templateId)
		return
	}
	if template.GetMetadata().HasDeletionTimestamp() {
		err = grpcstatus.Errorf(grpccodes.InvalidArgument, "template '%s' has been deleted", templateId)
		return
	}

	// Check that all the node sets given in the cluster correspond to node sets that exist in the template:
	templateNodeSets := template.GetNodeSets()
	clusterNodeSets := cluster.GetSpec().GetNodeSets()
	for clusterNodeSetKey := range clusterNodeSets {
		templateNodeSet := templateNodeSets[clusterNodeSetKey]
		if templateNodeSet == nil {
			templateNodeSetKeys := maps.Keys(templateNodeSets)
			sort.Strings(templateNodeSetKeys)
			for i, templateNodeSetKey := range templateNodeSetKeys {
				templateNodeSetKeys[i] = fmt.Sprintf("'%s'", templateNodeSetKey)
			}
			err = grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"node set '%s' doesn't exist, valid values for template '%s' are %s",
				clusterNodeSetKey, templateId, english.WordSeries(templateNodeSetKeys, "and"),
			)
			return
		}
	}

	// Check that all the node sets given in the cluster specify the same host class that is specified in the
	// template:
	for clusterNodeSetKey, clusterNodeSet := range clusterNodeSets {
		templateNodeSet := templateNodeSets[clusterNodeSetKey]
		clusterHostClass := clusterNodeSet.GetHostClass()
		if clusterHostClass == "" {
			continue
		}
		templateHostClass := templateNodeSet.GetHostClass()
		if clusterHostClass != templateHostClass {
			err = grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"host class for node set '%s' should be empty or '%s', like in template '%s', "+
					"but it is '%s'",
				clusterNodeSetKey, templateHostClass, templateId, clusterHostClass,
			)
			return
		}
	}

	// Check that all the node sets given in the cluster have a positive size:
	for clusterNodeSetKey, clusterNodeSet := range clusterNodeSets {
		clusterNodeSetSize := clusterNodeSet.GetSize()
		if clusterNodeSetSize <= 0 {
			err = grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"size for node set '%s' should be greater than zero, but it is %d",
				clusterNodeSetKey, clusterNodeSetSize,
			)
			return
		}
	}

	// Replace the node sets given in the cluster with those from the template, taking only the size from cluster:
	actualNodeSets := map[string]*ffv1.ClusterNodeSet{}
	for templateNodeSetKey, templateNodeSet := range templateNodeSets {
		var actualNodeSetSize int32
		clusterNodeSet := clusterNodeSets[templateNodeSetKey]
		if clusterNodeSet != nil {
			actualNodeSetSize = clusterNodeSet.GetSize()
		} else {
			actualNodeSetSize = templateNodeSet.GetSize()
		}
		actualNodeSets[templateNodeSetKey] = ffv1.ClusterNodeSet_builder{
			HostClass: templateNodeSet.GetHostClass(),
			Size:      actualNodeSetSize,
		}.Build()
	}
	cluster.GetSpec().SetNodeSets(actualNodeSets)

	// Check that all the specified template parameters are in the template:
	templateParameters := template.GetParameters()
	clusterParameters := cluster.GetSpec().GetTemplateParameters()
	var invalidParameterNames []string
	for clusterParameterName := range clusterParameters {
		clusterParameterValid := false
		for _, templateParameter := range templateParameters {
			if templateParameter.GetName() == clusterParameterName {
				clusterParameterValid = true
				break
			}
		}
		if !clusterParameterValid {
			invalidParameterNames = append(invalidParameterNames, clusterParameterName)
		}
	}
	if len(invalidParameterNames) > 0 {
		templateParameterNames := make([]string, len(templateParameters))
		for i, templateParameter := range templateParameters {
			templateParameterNames[i] = templateParameter.GetName()
		}
		sort.Strings(templateParameterNames)
		for i, templateParameterName := range templateParameterNames {
			templateParameterNames[i] = fmt.Sprintf("'%s'", templateParameterName)
		}
		sort.Strings(invalidParameterNames)
		for i, invalidParameterName := range invalidParameterNames {
			invalidParameterNames[i] = fmt.Sprintf("'%s'", invalidParameterName)
		}
		if len(invalidParameterNames) == 1 {
			err = grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"template parameter %s doesn't exist, valid values for template '%s' are %s",
				invalidParameterNames[0],
				templateId,
				english.WordSeries(templateParameterNames, "and"),
			)
		} else {
			err = grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"template parameters %s don't exist, valid values for template '%s' are %s",
				english.WordSeries(invalidParameterNames, "and"),
				templateId,
				english.WordSeries(templateParameterNames, "and"),
			)
		}
		return
	}

	// Check that all the mandatory parameters have a value:
	for _, templateParameter := range templateParameters {
		if !templateParameter.GetRequired() {
			continue
		}
		templateParameterName := templateParameter.GetName()
		clusterParameter := clusterParameters[templateParameterName]
		if clusterParameter == nil {
			err = grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"parameter '%s' of template '%s' is mandatory",
				templateParameterName, templateId,
			)
			return
		}
	}

	// Check that the parameter values are compatible with the template:
	for clusterParameterName, clusterParameter := range clusterParameters {
		for _, templateParameter := range templateParameters {
			templateParameterName := templateParameter.GetName()
			if clusterParameterName != templateParameterName {
				continue
			}
			clusterParameterType := clusterParameter.GetTypeUrl()
			templateParameterType := templateParameter.GetType()
			if clusterParameterType != templateParameterType {
				err = grpcstatus.Errorf(
					grpccodes.InvalidArgument,
					"type of parameter '%s' of template '%s' should be '%s', "+
						"but it is '%s'",
					clusterParameterName,
					templateId,
					templateParameterType,
					clusterParameterType,
				)
				return
			}
		}
	}

	// Set default values for template parameters:
	actualClusterParameters := make(map[string]*anypb.Any)
	for _, templateParameter := range templateParameters {
		templateParameterName := templateParameter.GetName()
		clusterParameter := clusterParameters[templateParameterName]
		actualClusterParameter := &anypb.Any{
			TypeUrl: templateParameter.GetType(),
		}
		if clusterParameter != nil {
			actualClusterParameter.Value = clusterParameter.Value
		} else {
			actualClusterParameter.Value = templateParameter.GetDefault().GetValue()
		}
		actualClusterParameters[templateParameterName] = actualClusterParameter
	}
	cluster.GetSpec().SetTemplateParameters(actualClusterParameters)

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
	if err != nil || cluster == nil || cluster.GetStatus().GetHub() == "" {
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
