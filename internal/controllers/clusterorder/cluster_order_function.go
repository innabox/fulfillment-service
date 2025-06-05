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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"

	fulfillmentv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	sharedv1 "github.com/innabox/fulfillment-service/internal/api/shared/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
)

// objectPrefix is the prefix that will be used in the `generateName` field of the resources created in the hub.
const objectPrefix = "order-"

// FunctionBuilder contains the data and logic needed to build a function that reconciles cluster orders.
type FunctionBuilder struct {
	logger     *slog.Logger
	connection *grpc.ClientConn
	hubCache   *controllers.HubCache
}

type function struct {
	logger          *slog.Logger
	ordersClient    privatev1.ClusterOrdersClient
	templatesClient privatev1.ClusterTemplatesClient
	hubsClient      privatev1.HubsClient
	hubCache        *controllers.HubCache
}

type task struct {
	r            *function
	order        *privatev1.ClusterOrder
	template     *privatev1.ClusterTemplate
	hubId        string
	hubNamespace string
	hubClient    clnt.Client
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

// Build uses the information stored in the buidler to create a new cluster order reconciler.
func (b *FunctionBuilder) Build() (result controllers.ReconcilerFunction[*fulfillmentv1.ClusterOrder], err error) {
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
		logger:          b.logger,
		ordersClient:    privatev1.NewClusterOrdersClient(b.connection),
		templatesClient: privatev1.NewClusterTemplatesClient(b.connection),
		hubsClient:      privatev1.NewHubsClient(b.connection),
		hubCache:        b.hubCache,
	}
	result = object.run
	return
}

func (r *function) run(ctx context.Context, object *fulfillmentv1.ClusterOrder) error {
	order, err := r.fetchOrder(ctx, object.GetId())
	if err != nil {
		return err
	}
	template, err := r.fetchTemplate(ctx, object.GetSpec().GetTemplateId())
	if err != nil {
		return err
	}
	t := task{
		r:        r,
		order:    order,
		template: template,
	}
	if object.Metadata.DeletionTimestamp != nil {
		err = t.delete(ctx)
	} else {
		err = t.update(ctx)
	}
	if err != nil {
		return err
	}
	_, err = r.ordersClient.Update(ctx, privatev1.ClusterOrdersUpdateRequest_builder{
		Object: order,
	}.Build())
	if err != nil {
		return err
	}
	return err
}

func (r *function) fetchOrder(ctx context.Context, id string) (result *privatev1.ClusterOrder, err error) {
	response, err := r.ordersClient.Get(ctx, privatev1.ClusterOrdersGetRequest_builder{
		Id: id,
	}.Build())
	if err != nil {
		return
	}
	result = response.Object
	return
}

func (r *function) fetchTemplate(ctx context.Context, id string) (result *privatev1.ClusterTemplate, err error) {
	response, err := r.templatesClient.Get(ctx, privatev1.ClusterTemplatesGetRequest_builder{
		Id: id,
	}.Build())
	if err != nil {
		return
	}
	result = response.Object
	return
}

func (t *task) update(ctx context.Context) error {
	// Set the default values:
	t.setDefaults()

	// Do nothing if the order isn't progressing:
	if t.order.GetStatus().GetState() != privatev1.ClusterOrderState_CLUSTER_ORDER_STATE_PROGRESSING {
		return nil
	}

	// Select the hub:
	err := t.selectHub(ctx)
	if err != nil {
		return err
	}

	// Save the selected hub in the private data of the order:
	t.order.SetHubId(t.hubId)

	// Prepare the node requests:
	nodeRequests, err := t.prepareNodeRequests()
	if err != nil {
		return err
	}

	// Prepare the template parameters:
	templateParameters, err := t.prepareTemplateParameters()
	if err != nil {
		return err
	}

	// Get the K8S object:
	object, err := t.getKubeObject(ctx)
	if err != nil {
		return err
	}
	spec := map[string]any{
		"templateID":         t.order.Spec.TemplateId,
		"templateParameters": templateParameters,
		"nodeRequests":       nodeRequests,
	}
	if object == nil {
		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(gvks.ClusterOrder)
		object.SetNamespace(t.hubNamespace)
		object.SetGenerateName(objectPrefix)
		object.SetLabels(map[string]string{
			labels.ClusterOrderUuid: t.order.Id,
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

func (t *task) delete(ctx context.Context) error {
	// Select the hub:
	err := t.selectHub(ctx)
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
			slog.String("id", t.order.GetId()),
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

func (t *task) getKubeObject(ctx context.Context) (result *unstructured.Unstructured, err error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvks.ClusterOrderList)
	err = t.hubClient.List(
		ctx, list,
		clnt.InNamespace(t.hubNamespace),
		clnt.MatchingLabels{
			labels.ClusterOrderUuid: t.order.Id,
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
			"expected at most one cluster order with identifier '%s' but found %d", t.order.Id,
			len(items),
		)
		return
	}
	result = &items[0]
	return
}

func (t *task) setDefaults() {
	if !t.order.HasStatus() {
		t.order.SetStatus(&privatev1.ClusterOrderStatus{})
	}
	if t.order.GetStatus().GetState() == privatev1.ClusterOrderState_CLUSTER_ORDER_STATE_UNSPECIFIED {
		t.order.GetStatus().SetState(privatev1.ClusterOrderState_CLUSTER_ORDER_STATE_PROGRESSING)
	}
	for value := range fulfillmentv1.ClusterOrderConditionType_name {
		if value != 0 {
			t.setConditionDefaults(privatev1.ClusterOrderConditionType(value))
		}
	}
}

func (t *task) setConditionDefaults(value privatev1.ClusterOrderConditionType) {
	exists := false
	for _, current := range t.order.GetStatus().GetConditions() {
		if current.GetType() == value {
			exists = true
			break
		}
	}
	if !exists {
		conditions := t.order.GetStatus().GetConditions()
		conditions = append(conditions, privatev1.ClusterOrderCondition_builder{
			Type:   value,
			Status: sharedv1.ConditionStatus_CONDITION_STATUS_FALSE,
		}.Build())
		t.order.GetStatus().SetConditions(conditions)
	}
}

func (t *task) selectHub(ctx context.Context) error {
	t.hubId = t.order.GetHubId()
	if t.hubId == "" {
		response, err := t.r.hubsClient.List(ctx, privatev1.HubsListRequest_builder{}.Build())
		if err != nil {
			return err
		}
		if len(response.Items) == 0 {
			return errors.New("there are no hubs")
		}
		t.hubId = response.Items[rand.IntN(len(response.Items))].GetId()
	}
	t.r.logger.DebugContext(
		ctx,
		"Selected hub",
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

func (t *task) prepareTemplateParameters() (result string, err error) {
	// We represent the template parameters as a map where the keys are the names and the values are protocol
	// buffers `Any` objects, but the Kubernetes controller expects a JSON object where the fields are the names
	// of the parameters and the values are the JSON representations of the values, so we need to do the conversion.
	paramsJson := map[string]any{}
	for paramName, paramAny := range t.order.Spec.TemplateParameters {
		var paramJson any
		paramJson, err = t.convertTemplateParam(paramAny)
		if err != nil {
			return
		}
		paramsJson[paramName] = paramJson
	}
	paramsBytes, err := json.Marshal(paramsJson)
	if err != nil {
		return
	}
	result = string(paramsBytes)
	return
}

func (t *task) convertTemplateParam(paramAny *anypb.Any) (result any,
	err error) {
	paramMsg, err := paramAny.UnmarshalNew()
	if err != nil {
		return
	}
	paramBytes, err := protojson.Marshal(paramMsg)
	if err != nil {
		return
	}
	var paramValue any
	err = json.Unmarshal(paramBytes, &paramValue)
	if err != nil {
		return
	}
	result = paramValue
	return
}

func (t *task) prepareNodeRequests() (result any, err error) {
	var nodeRequests []any
	for _, nodeSet := range t.template.GetNodeSets() {
		var nodeRequest any
		nodeRequest, err = t.prepareNodeRequest(nodeSet)
		if err != nil {
			return
		}
		nodeRequests = append(nodeRequests, nodeRequest)
	}
	result = nodeRequests
	return
}

func (t *task) prepareNodeRequest(nodeSet *privatev1.ClusterTemplateNodeSet) (result any, err error) {
	result = map[string]any{
		"resourceClass": nodeSet.GetHostClass(),
		"numberOfNodes": int64(nodeSet.GetSize()),
	}
	return
}
