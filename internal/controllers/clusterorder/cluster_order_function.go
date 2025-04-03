/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package clusterorder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"

	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"

	fulfillmentv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	sharedv1 "github.com/innabox/fulfillment-service/internal/api/shared/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
)

// objectPrefix is the prefix that will be used in the `generateName` field of the resources created in the hub.
const objectPrefix = "order-"

// FunctionBuilder contains the data and logic needed to build a function that reconciles cluster orders.
type FunctionBuilder struct {
	logger *slog.Logger
	client *grpc.ClientConn
}

type function struct {
	logger          *slog.Logger
	objectClient    fulfillmentv1.ClusterOrdersClient
	hubClient       privatev1.HubsClient
	internalClient  privatev1.ClusterOrdersClient
	kubeClients     map[string]clnt.Client
	kubeClientsLock *sync.Mutex
}

type task struct {
	r        *function
	object   *fulfillmentv1.ClusterOrder
	internal *privatev1.ClusterOrder
	hub      *privatev1.Hub
}

// NewFunction creates a new builder that can then be used to create a new cluster order reconciler function.
func NewFunction() *FunctionBuilder {
	return &FunctionBuilder{}
}

// SetLogger sets the logger. This is mandatory.
func (b *FunctionBuilder) SetLogger(value *slog.Logger) *FunctionBuilder {
	b.logger = value
	return b
}

// SetClient sets the gRPC client connection. This is mandatory.
func (b *FunctionBuilder) SetClient(value *grpc.ClientConn) *FunctionBuilder {
	b.client = value
	return b
}

// Build uses the information stored in the buidler to create a new cluster order reconciler.
func (b *FunctionBuilder) Build() (result controllers.ReconcilerFunction[*fulfillmentv1.ClusterOrder], err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.client == nil {
		err = errors.New("client is mandatory")
		return
	}

	// Create and populate the object:
	object := &function{
		logger:          b.logger,
		objectClient:    fulfillmentv1.NewClusterOrdersClient(b.client),
		internalClient:  privatev1.NewClusterOrdersClient(b.client),
		hubClient:       privatev1.NewHubsClient(b.client),
		kubeClients:     map[string]clnt.Client{},
		kubeClientsLock: &sync.Mutex{},
	}
	result = object.run
	return
}

func (r *function) run(ctx context.Context, object *fulfillmentv1.ClusterOrder) error {
	internal, err := r.getOrCreateInternal(ctx, object.Id)
	if err != nil {
		return err
	}
	t := task{
		r:        r,
		object:   object,
		internal: internal,
	}
	if object.Metadata.DeletionTimestamp != nil {
		err = t.delete(ctx)
	} else {
		err = t.update(ctx)
	}
	if err != nil {
		return err
	}
	_, err = r.internalClient.Update(ctx, &privatev1.ClusterOrdersUpdateRequest{
		Object: internal,
	})
	if err != nil {
		return err
	}
	_, err = r.objectClient.Update(ctx, &fulfillmentv1.ClusterOrdersUpdateRequest{
		Object: object,
	})
	return err
}

func (r *function) getOrCreateInternal(ctx context.Context, id string) (result *privatev1.ClusterOrder, err error) {
	get, err := r.internalClient.Get(ctx, &privatev1.ClusterOrdersGetRequest{
		Id: id,
	})
	if grpcstatus.Code(err) == grpccodes.NotFound {
		object := &privatev1.ClusterOrder{
			Id: id,
		}
		var create *privatev1.ClusterOrdersCreateResponse
		create, err = r.internalClient.Create(ctx, &privatev1.ClusterOrdersCreateRequest{
			Object: object,
		})
		if err != nil {
			return
		}
		result = create.Object
		return
	}
	if err != nil {
		return
	}
	result = get.Object
	return
}

func (r *function) getKubeClient(ctx context.Context, hub *privatev1.Hub) (result clnt.Client, err error) {
	r.kubeClientsLock.Lock()
	defer r.kubeClientsLock.Unlock()
	result, ok := r.kubeClients[hub.Id]
	if ok {
		return
	}
	result, err = r.createKubeClient(ctx, hub)
	if err != nil {
		return
	}
	r.kubeClients[hub.Id] = result
	return
}

func (r *function) createKubeClient(ctx context.Context, hub *privatev1.Hub) (result clnt.Client, err error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(hub.Kubeconfig)
	if err != nil {
		return
	}
	result, err = clnt.New(config, clnt.Options{})
	return
}

func (t *task) update(ctx context.Context) error {
	// Set the default values:
	t.setDefaults()

	// Do nothing if the order isn't progressing:
	if t.object.Status.State != fulfillmentv1.ClusterOrderState_CLUSTER_ORDER_STATE_PROGRESSING {
		return nil
	}

	// Select the hub:
	err := t.selectHub(ctx)
	if err != nil {
		return err
	}
	t.r.logger.DebugContext(
		ctx,
		"Selected hub",
		slog.String("id", t.hub.Id),
	)
	client, err := t.r.getKubeClient(ctx, t.hub)
	if err != nil {
		return err
	}

	// Get the K8S object:
	object, err := t.getKubeObject(ctx)
	if err != nil {
		return err
	}
	spec := map[string]any{
		"templateID": t.object.Spec.TemplateId,
	}
	if object == nil {
		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(ObjectGvk)
		object.SetNamespace(t.hub.Namespace)
		object.SetGenerateName(objectPrefix)
		object.SetLabels(map[string]string{
			idLabel: t.object.Id,
		})
		err = unstructured.SetNestedField(object.Object, spec, "spec")
		if err != nil {
			return err
		}
		err = client.Create(ctx, object)
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
		err = client.Patch(ctx, update, clnt.MergeFrom(object))
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

func (t *task) delete(ctx context.Context) error {
	// Select the hub:
	err := t.selectHub(ctx)
	if err != nil {
		return err
	}
	t.r.logger.DebugContext(
		ctx,
		"Selected hub",
		slog.String("id", t.hub.Id),
	)
	client, err := t.r.getKubeClient(ctx, t.hub)
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
			slog.String("id", t.object.Id),
		)
		return nil
	}
	err = client.Delete(ctx, object)
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

func (t *task) getKubeObject(ctx context.Context) (result *unstructured.Unstructured, err error) {
	client, err := t.r.getKubeClient(ctx, t.hub)
	if err != nil {
		return
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(ListGvk)
	err = client.List(
		ctx, list,
		clnt.InNamespace(t.hub.Namespace),
		clnt.MatchingLabels{
			idLabel: t.object.Id,
		},
	)
	if err != nil {
		return
	}
	items := list.Items
	if len(items) == 0 {
		return
	}
	if len(items) > 1 {
		err = fmt.Errorf(
			"expected at most one cluster order with identifier '%s' but found %d", t.object.Id,
			len(items),
		)
		return
	}
	result = &items[0]
	return
}

func (t *task) setDefaults() {
	if t.object.Status == nil {
		t.object.Status = &fulfillmentv1.ClusterOrderStatus{}
	}
	if t.object.Status.State == fulfillmentv1.ClusterOrderState_CLUSTER_ORDER_STATE_UNSPECIFIED {
		t.object.Status.State = fulfillmentv1.ClusterOrderState_CLUSTER_ORDER_STATE_PROGRESSING
	}
	for value := range fulfillmentv1.ClusterOrderConditionType_name {
		if value != 0 {
			t.setConditionDefaults(fulfillmentv1.ClusterOrderConditionType(value))
		}
	}
}

func (t *task) setConditionDefaults(value fulfillmentv1.ClusterOrderConditionType) {
	exists := false
	for _, current := range t.object.Status.Conditions {
		if current.Type == value {
			exists = true
			break
		}
	}
	if !exists {
		t.object.Status.Conditions = append(t.object.Status.Conditions, &fulfillmentv1.ClusterOrderCondition{
			Type:   value,
			Status: sharedv1.ConditionStatus_CONDITION_STATUS_FALSE,
		})
	}
}

func (t *task) selectHub(ctx context.Context) error {
	if t.internal.HubId == "" {
		response, err := t.r.hubClient.List(ctx, &privatev1.HubsListRequest{})
		if err != nil {
			return err
		}
		if len(response.Items) == 0 {
			return errors.New("there are no hubs")
		}
		t.hub = response.Items[rand.IntN(len(response.Items))]
	} else {
		response, err := t.r.hubClient.Get(ctx, &privatev1.HubsGetRequest{
			Id: t.internal.HubId,
		})
		if err != nil {
			return err
		}
		t.hub = response.Object
	}
	return nil
}
