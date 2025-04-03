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
	"log/slog"
	"math/rand/v2"
	"sync"

	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/imdario/mergo"
	adminv1 "github.com/innabox/fulfillment-service/internal/api/admin/v1"
	fulfillmentv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	sharedv1 "github.com/innabox/fulfillment-service/internal/api/shared/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// FunctionBuilder contains the data and logic needed to build a function that reconciles cluster orders.
type FunctionBuilder struct {
	logger *slog.Logger
	client *grpc.ClientConn
}

type function struct {
	logger          *slog.Logger
	objectClient    fulfillmentv1.ClusterOrdersClient
	hubClient       adminv1.HubsClient
	internalClient  adminv1.ClusterOrdersClient
	kubeClients     map[string]clnt.Client
	kubeClientsLock *sync.Mutex
}

type task struct {
	r        *function
	object   *fulfillmentv1.ClusterOrder
	internal *adminv1.ClusterOrder
	hub      *adminv1.Hub
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
		internalClient:  adminv1.NewClusterOrdersClient(b.client),
		hubClient:       adminv1.NewHubsClient(b.client),
		kubeClients:     map[string]clnt.Client{},
		kubeClientsLock: &sync.Mutex{},
	}
	result = object.run
	return
}

func (r *function) run(ctx context.Context, object *fulfillmentv1.ClusterOrder) error {
	internal, err := r.fetch(ctx, object.Id)
	if err != nil {
		return err
	}
	t := task{
		r:        r,
		object:   object,
		internal: internal,
	}
	err = t.run(ctx)
	if err != nil {
		return err
	}
	_, err = r.internalClient.Update(ctx, &adminv1.ClusterOrdersUpdateRequest{
		Object: internal,
	})
	if err != nil {
		return err
	}
	_, err = r.objectClient.UpdateStatus(ctx, &fulfillmentv1.ClusterOrdersUpdateStatusRequest{
		Id:     object.Id,
		Status: object.Status,
	})
	return err
}

func (r *function) fetch(ctx context.Context, id string) (result *adminv1.ClusterOrder, err error) {
	get, err := r.internalClient.Get(ctx, &adminv1.ClusterOrdersGetRequest{
		Id: id,
	})
	if grpcstatus.Code(err) == grpccodes.NotFound {
		object := &adminv1.ClusterOrder{
			Id: id,
		}
		var create *adminv1.ClusterOrdersCreateResponse
		create, err = r.internalClient.Create(ctx, &adminv1.ClusterOrdersCreateRequest{
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

func (r *function) getKubeClient(ctx context.Context, hub *adminv1.Hub) (result clnt.Client, err error) {
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

func (r *function) createKubeClient(ctx context.Context, hub *adminv1.Hub) (result clnt.Client, err error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(hub.Kubeconfig)
	if err != nil {
		return
	}
	result, err = clnt.New(config, clnt.Options{})
	return
}

func (t *task) run(ctx context.Context) error {
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

	// Create the K8S object:
	client, err := t.r.getKubeClient(ctx, t.hub)
	if err != nil {
		return err
	}
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cloudkit.openshift.io",
		Version: "v1alpha1",
		Kind:    "ClusterOrder",
	})
	object.SetNamespace(t.hub.Namespace)
	object.SetName(t.object.Id)
	result, err := controllerutil.CreateOrPatch(ctx, client, object, func() error {
		return mergo.Merge(
			&object.Object,
			map[string]any{
				"spec": map[string]any{
					"templateID": t.object.Spec.TemplateId,
				},
			},
			mergo.WithOverride,
		)
	})
	if err != nil {
		return err
	}
	t.r.logger.DebugContext(
		ctx,
		"Created cluster order",
		slog.String("namespace", object.GetNamespace()),
		slog.String("name", object.GetName()),
		slog.String("result", string(result)),
	)

	return err
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
		response, err := t.r.hubClient.List(ctx, &adminv1.HubsListRequest{})
		if err != nil {
			return err
		}
		if len(response.Items) == 0 {
			return errors.New("there are no hubs")
		}
		t.hub = response.Items[rand.IntN(len(response.Items))]
	} else {
		response, err := t.r.hubClient.Get(ctx, &adminv1.HubsGetRequest{
			Id: t.internal.HubId,
		})
		if err != nil {
			return err
		}
		t.hub = response.Object
	}
	return nil
}
