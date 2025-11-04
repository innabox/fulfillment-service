/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package vm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
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
const objectPrefix = "vm-"

// FunctionBuilder contains the data and logic needed to build a function that reconciles virtual machines.
type FunctionBuilder struct {
	logger     *slog.Logger
	connection *grpc.ClientConn
	hubCache   *controllers.HubCache
}

type function struct {
	logger     *slog.Logger
	hubCache   *controllers.HubCache
	vmsClient  privatev1.VirtualMachinesClient
	hubsClient privatev1.HubsClient
}

type task struct {
	r            *function
	vm           *privatev1.VirtualMachine
	hubId        string
	hubNamespace string
	hubClient    clnt.Client
}

// NewFunction creates a new builder that can then be used to create a new virtual machine reconciler function.
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

// Build uses the information stored in the builder to create a new virtual machine reconciler.
func (b *FunctionBuilder) Build() (result controllers.ReconcilerFunction[*privatev1.VirtualMachine], err error) {
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
		logger:     b.logger,
		vmsClient:  privatev1.NewVirtualMachinesClient(b.connection),
		hubsClient: privatev1.NewHubsClient(b.connection),
		hubCache:   b.hubCache,
	}
	result = object.run
	return
}

func (r *function) run(ctx context.Context, vm *privatev1.VirtualMachine) error {
	oldVM := proto.Clone(vm).(*privatev1.VirtualMachine)
	t := task{
		r:  r,
		vm: vm,
	}
	var err error
	if vm.GetMetadata().HasDeletionTimestamp() {
		err = t.delete(ctx)
	} else {
		err = t.update(ctx)
	}
	if err != nil {
		return err
	}

	if !proto.Equal(vm, oldVM) {
		_, err = r.vmsClient.Update(ctx, privatev1.VirtualMachinesUpdateRequest_builder{
			Object: vm,
		}.Build())
	}

	return err
}

func (t *task) update(ctx context.Context) error {
	// Set the default values:
	t.setDefaults()

	// Do nothing if the VM isn't progressing:
	if t.vm.GetStatus().GetState() != privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING {
		return nil
	}

	// Select the hub:
	err := t.selectHub(ctx)
	if err != nil {
		return err
	}

	// Save the selected hub in the private data of the VM:
	t.vm.GetStatus().SetHub(t.hubId)

	// Get the K8S object:
	object, err := t.getKubeObject(ctx)
	if err != nil {
		return err
	}

	// Prepare the changes to the spec:
	templateParameters, err := utils.ConvertTemplateParametersToJSON(t.vm.GetSpec().GetTemplateParameters())
	if err != nil {
		return err
	}
	spec := map[string]any{
		"templateID":         t.vm.GetSpec().GetTemplate(),
		"templateParameters": templateParameters,
	}

	// Create or update the Kubernetes object:
	if object == nil {
		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(gvks.VirtualMachine)
		object.SetNamespace(t.hubNamespace)
		object.SetGenerateName(objectPrefix)
		object.SetLabels(map[string]string{
			labels.VirtualMachineUuid: t.vm.GetId(),
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
			"Created virtual machine",
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
			"Updated virtual machine",
			slog.String("namespace", object.GetNamespace()),
			slog.String("name", object.GetName()),
		)
	}

	return err
}

func (t *task) setDefaults() {
	if !t.vm.HasStatus() {
		t.vm.SetStatus(&privatev1.VirtualMachineStatus{})
	}
	if t.vm.GetStatus().GetState() == privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_UNSPECIFIED {
		t.vm.GetStatus().SetState(privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING)
	}
	for value := range privatev1.VirtualMachineConditionType_name {
		if value != 0 {
			t.setConditionDefaults(privatev1.VirtualMachineConditionType(value))
		}
	}
}

func (t *task) setConditionDefaults(value privatev1.VirtualMachineConditionType) {
	exists := false
	for _, current := range t.vm.GetStatus().GetConditions() {
		if current.GetType() == value {
			exists = true
			break
		}
	}
	if !exists {
		conditions := t.vm.GetStatus().GetConditions()
		conditions = append(conditions, privatev1.VirtualMachineCondition_builder{
			Type:   value,
			Status: sharedv1.ConditionStatus_CONDITION_STATUS_FALSE,
		}.Build())
		t.vm.GetStatus().SetConditions(conditions)
	}
}

func (t *task) delete(ctx context.Context) error {
	// Do nothing if we don't know the hub yet:
	t.hubId = t.vm.GetStatus().GetHub()
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
			"Virtual machine doesn't exist",
			slog.String("id", t.vm.GetId()),
		)
		return nil
	}
	err = t.hubClient.Delete(ctx, object)
	if err != nil {
		return err
	}
	t.r.logger.DebugContext(
		ctx,
		"Deleted virtual machine",
		slog.String("namespace", object.GetNamespace()),
		slog.String("name", object.GetName()),
	)

	return err
}

func (t *task) selectHub(ctx context.Context) error {
	t.hubId = t.vm.GetStatus().GetHub()
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

func (t *task) getHub(ctx context.Context) error {
	t.hubId = t.vm.GetStatus().GetHub()
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
	list.SetGroupVersionKind(gvks.VirtualMachineList)
	err = t.hubClient.List(
		ctx, list,
		clnt.InNamespace(t.hubNamespace),
		clnt.MatchingLabels{
			labels.VirtualMachineUuid: t.vm.GetId(),
		},
	)
	if err != nil {
		return
	}
	items := list.Items
	count := len(items)
	if count > 1 {
		err = fmt.Errorf(
			"expected at most one virtual machine with identifier '%s' but found %d",
			t.vm.GetId(), count,
		)
		return
	}
	if count > 0 {
		result = &items[0]
	}
	return
}
