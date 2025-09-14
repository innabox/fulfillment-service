/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package cluster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	sharedv1 "github.com/innabox/fulfillment-service/internal/api/shared/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
	"github.com/innabox/fulfillment-service/internal/utils"
)

// objectPrefix is the prefix that will be used in the `generateName` field of the resources created in the hub.
const objectPrefix = "order-"

// FunctionBuilder contains the data and logic needed to build a function that reconciles clustes.
type FunctionBuilder struct {
	logger     *slog.Logger
	connection *grpc.ClientConn
	hubCache   *controllers.HubCache
}

type function struct {
	logger         *slog.Logger
	hubCache       *controllers.HubCache
	clustersClient privatev1.ClustersClient
	hubsClient     privatev1.HubsClient
}

type task struct {
	r            *function
	cluster      *privatev1.Cluster
	hubId        string
	hubNamespace string
	hubClient    clnt.Client
}

// NewFunction creates a new builder that can then be used to create a new cluster reconciler function.
func NewFunction() *FunctionBuilder {
	return &FunctionBuilder{}
}

// SetLogger sets the logger. This is mandatory.
func (b *FunctionBuilder) SetLogger(value *slog.Logger) *FunctionBuilder {
	b.logger = value
	return b
}

// SetConnection sets the gRPC client connection. This is mandatory.
func (b *FunctionBuilder) SetConnection(value *grpc.ClientConn) *FunctionBuilder {
	b.connection = value
	return b
}

// SetHubCache sets the cache of hubs. This is mandatory.
func (b *FunctionBuilder) SetHubCache(value *controllers.HubCache) *FunctionBuilder {
	b.hubCache = value
	return b
}

// Build uses the information stored in the buidler to create a new cluster reconciler.
func (b *FunctionBuilder) Build() (result controllers.ReconcilerFunction[*privatev1.Cluster], err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.connection == nil {
		err = errors.New("client is mandatory")
		return
	}
	if b.hubCache == nil {
		err = errors.New("hub cache is mandatory")
		return
	}

	// Create and populate the object:
	object := &function{
		logger:         b.logger,
		clustersClient: privatev1.NewClustersClient(b.connection),
		hubsClient:     privatev1.NewHubsClient(b.connection),
		hubCache:       b.hubCache,
	}
	result = object.run
	return
}

func (r *function) run(ctx context.Context, cluster *privatev1.Cluster) error {
	t := task{
		r:       r,
		cluster: cluster,
	}
	var err error
	if cluster.GetMetadata().HasDeletionTimestamp() {
		err = t.delete(ctx)
	} else {
		err = t.update(ctx)
	}
	if err != nil {
		return err
	}
	_, err = r.clustersClient.Update(ctx, privatev1.ClustersUpdateRequest_builder{
		Object: cluster,
	}.Build())
	return err
}

func (t *task) update(ctx context.Context) error {
	// Set the default values:
	t.setDefaults()

	// Do nothing if the order isn't progressing:
	if t.cluster.GetStatus().GetState() != privatev1.ClusterState_CLUSTER_STATE_PROGRESSING {
		return nil
	}

	// Select the hub:
	err := t.selectHub(ctx)
	if err != nil {
		return err
	}

	// Save the selected hub in the private data of the cluster:
	t.cluster.GetStatus().SetHub(t.hubId)

	// Get the K8S object:
	object, err := t.getKubeObject(ctx)
	if err != nil {
		return err
	}

	// Prepare the changes to the spec:
	nodeRequests := t.prepareNodeRequests()
	templateParameters, err := utils.ConvertTemplateParametersToJSON(t.cluster.GetSpec().GetTemplateParameters())
	if err != nil {
		return err
	}
	spec := map[string]any{
		"templateID":         t.cluster.GetSpec().GetTemplate(),
		"templateParameters": templateParameters,
		"nodeRequests":       nodeRequests,
	}

	// Create or update the Kubernetes object:
	if object == nil {
		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(gvks.ClusterOrder)
		object.SetNamespace(t.hubNamespace)
		object.SetGenerateName(objectPrefix)
		object.SetLabels(map[string]string{
			labels.ClusterOrderUuid: t.cluster.GetId(),
		})
		err = unstructured.SetNestedField(object.Object, spec, "spec")
		if err != nil {
			return err
		}
		err = t.hubClient.Create(ctx, object)
		if err != nil {
			return err
		}
		t.r.logger.DebugContext(
			ctx,
			"Created cluster order",
			slog.String("namespace", object.GetNamespace()),
			slog.String("name", object.GetName()),
		)
	} else {
		update := object.DeepCopy()
		err = unstructured.SetNestedField(update.Object, spec, "spec")
		if err != nil {
			return err
		}
		err = t.hubClient.Patch(ctx, update, clnt.MergeFrom(object))
		if err != nil {
			return err
		}
		t.r.logger.DebugContext(
			ctx,
			"Updated cluster order",
			slog.String("namespace", object.GetNamespace()),
			slog.String("name", object.GetName()),
		)
	}

	return err
}

func (t *task) setDefaults() {
	if !t.cluster.HasStatus() {
		t.cluster.SetStatus(&privatev1.ClusterStatus{})
	}
	if t.cluster.GetStatus().GetState() == privatev1.ClusterState_CLUSTER_STATE_UNSPECIFIED {
		t.cluster.GetStatus().SetState(privatev1.ClusterState_CLUSTER_STATE_PROGRESSING)
	}
	for value := range privatev1.ClusterConditionType_name {
		if value != 0 {
			t.setConditionDefaults(privatev1.ClusterConditionType(value))
		}
	}
}

func (t *task) setConditionDefaults(value privatev1.ClusterConditionType) {
	exists := false
	for _, current := range t.cluster.GetStatus().GetConditions() {
		if current.GetType() == value {
			exists = true
			break
		}
	}
	if !exists {
		conditions := t.cluster.GetStatus().GetConditions()
		conditions = append(conditions, privatev1.ClusterCondition_builder{
			Type:   value,
			Status: sharedv1.ConditionStatus_CONDITION_STATUS_FALSE,
		}.Build())
		t.cluster.GetStatus().SetConditions(conditions)
	}
}

func (t *task) prepareNodeRequests() any {
	var nodeRequests []any
	for _, nodeSet := range t.cluster.GetSpec().GetNodeSets() {
		nodeRequest := t.prepareNodeRequest(nodeSet)
		nodeRequests = append(nodeRequests, nodeRequest)
	}
	return nodeRequests
}

func (t *task) prepareNodeRequest(nodeSet *privatev1.ClusterNodeSet) any {
	return map[string]any{
		"resourceClass": nodeSet.GetHostClass(),
		"numberOfNodes": int64(nodeSet.GetSize()),
	}
}

func (t *task) delete(ctx context.Context) error {
	// Do nothing if we don't know the hub yet:
	t.hubId = t.cluster.GetStatus().GetHub()
	if t.hubId == "" {
		return nil
	}
	err := t.getHub(ctx)
	if err != nil {
		return err
	}

	// Delete the K8S object:
	object, err := t.getKubeObject(ctx)
	if err != nil {
		return err
	}
	if object == nil {
		t.r.logger.DebugContext(
			ctx,
			"Cluster order doesn't exist",
			slog.String("id", t.cluster.GetId()),
		)
		return nil
	}
	err = t.hubClient.Delete(ctx, object)
	if err != nil {
		return err
	}
	t.r.logger.DebugContext(
		ctx,
		"Deleted cluster order",
		slog.String("namespace", object.GetNamespace()),
		slog.String("name", object.GetName()),
	)

	return err
}

func (t *task) selectHub(ctx context.Context) error {
	t.hubId = t.cluster.GetStatus().GetHub()
	if t.hubId == "" {
		// Use the new capability-based hub selection
		hubId, hubEntry, err := t.r.hubCache.SelectByCapabilityWithID(ctx, "clusters")
		if err != nil {
			return fmt.Errorf("failed to select cluster-capable hub: %w", err)
		}
		t.hubId = hubId
		t.hubNamespace = hubEntry.Namespace
		t.hubClient = hubEntry.Client
		
		t.r.logger.DebugContext(
			ctx,
			"Selected cluster-capable hub",
			slog.String("id", t.hubId),
			slog.String("namespace", t.hubNamespace),
		)
		return nil
	}
	
	// If hub is already assigned, get it directly
	t.r.logger.DebugContext(
		ctx,
		"Using assigned hub",
		slog.String("id", t.hubId),
	)
	hubEntry, err := t.r.hubCache.Get(ctx, t.hubId)
	if err != nil {
		return err
	}
	t.hubNamespace = hubEntry.Namespace
	t.hubClient = hubEntry.Client
	return nil
}

func (t *task) getHub(ctx context.Context) error {
	t.hubId = t.cluster.GetStatus().GetHub()
	hubEntry, err := t.r.hubCache.Get(ctx, t.hubId)
	if err != nil {
		return err
	}
	t.hubNamespace = hubEntry.Namespace
	t.hubClient = hubEntry.Client
	return nil
}

func (t *task) getKubeObject(ctx context.Context) (result *unstructured.Unstructured, err error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvks.ClusterOrderList)
	err = t.hubClient.List(
		ctx, list,
		clnt.InNamespace(t.hubNamespace),
		clnt.MatchingLabels{
			labels.ClusterOrderUuid: t.cluster.GetId(),
		},
	)
	if err != nil {
		return
	}
	items := list.Items
	count := len(items)
	if count > 1 {
		err = fmt.Errorf(
			"expected at most one cluster order with identifer '%s' but found %d",
			t.cluster.GetId(), count,
		)
		return
	}
	if count > 0 {
		result = &items[0]
	}
	return
}
