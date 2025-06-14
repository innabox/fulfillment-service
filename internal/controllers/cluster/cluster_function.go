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

	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
)

// FunctionBuilder contains the data and logic needed to build a function that reconciles cluster orders.
type FunctionBuilder struct {
	logger     *slog.Logger
	connection *grpc.ClientConn
	hubCache   *controllers.HubCache
}

type function struct {
	logger         *slog.Logger
	hubCache       *controllers.HubCache
	clustersClient privatev1.ClustersClient
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
func (b *FunctionBuilder) Build() (result controllers.ReconcilerFunction[*ffv1.Cluster], err error) {
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
		hubCache:       b.hubCache,
	}
	result = object.run
	return
}

func (r *function) run(ctx context.Context, object *ffv1.Cluster) error {
	cluster, err := r.fetchCluster(ctx, object.Id)
	if err != nil {
		return err
	}
	t := task{
		r:       r,
		cluster: cluster,
	}
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

func (r *function) fetchCluster(ctx context.Context, id string) (result *privatev1.Cluster, err error) {
	request, err := r.clustersClient.Get(ctx, &privatev1.ClustersGetRequest{
		Id: id,
	})
	if err != nil {
		return
	}
	result = request.Object
	return
}

func (t *task) update(ctx context.Context) error {
	// Do nothing if we don't know the hub yet:
	t.hubId = t.cluster.GetHubId()
	if t.hubId == "" {
		return nil
	}
	err := t.getHub(ctx)
	if err != nil {
		return err
	}

	// Get the K8S object:
	object, err := t.getKubeObject(ctx)
	if err != nil {
		return err
	}

	// Prepare the changes to the spec:
	update := object.DeepCopy()
	nodeRequests := t.prepareNodeRequests()
	err = unstructured.SetNestedField(update.Object, nodeRequests, "spec", "nodeRequests")
	if err != nil {
		return err
	}

	// Send the patch:
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

	return err
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
	t.hubId = t.cluster.GetHubId()
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

func (t *task) getHub(ctx context.Context) error {
	t.hubId = t.cluster.GetHubId()
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
			labels.ClusterOrderUuid: t.cluster.GetOrderId(),
		},
	)
	if err != nil {
		return
	}
	items := list.Items
	if len(items) != 1 {
		err = fmt.Errorf(
			"expected exactly one cluster order with identifer '%s' but found %d",
			t.cluster.GetOrderId(), len(items),
		)
		return
	}
	result = &items[0]
	return
}
