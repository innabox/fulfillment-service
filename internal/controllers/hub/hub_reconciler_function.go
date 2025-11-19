/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package hub

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
)

// FunctionBuilder contains the data and logic needed to build a function that reconciles hubs.
type FunctionBuilder struct {
	logger     *slog.Logger
	connection *grpc.ClientConn
	hubCache   *controllers.HubCache
}

type function struct {
	logger     *slog.Logger
	hubCache   *controllers.HubCache
	hubsClient privatev1.HubsClient
}

type task struct {
	parent           *function
	hub              *privatev1.Hub
	hubClient        clnt.Client
	namespace        *corev1.Namespace
	pullSecret       *corev1.Secret
	infraEnv         *unstructured.Unstructured
	capiProviderRole *rbacv1.Role
}

// NewFunction creates a new builder that can then be used to create a new hub reconciler function.
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

// Build uses the information stored in the builder to create a new hub reconciler.
func (b *FunctionBuilder) Build() (result controllers.ReconcilerFunction[*privatev1.Hub], err error) {
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
		hubsClient: privatev1.NewHubsClient(b.connection),
		hubCache:   b.hubCache,
	}
	result = object.run
	return
}

func (f *function) run(ctx context.Context, hub *privatev1.Hub) error {
	oldHub := proto.Clone(hub).(*privatev1.Hub)
	t := task{
		parent: f,
		hub:    hub,
	}
	var err error
	if hub.GetMetadata().HasDeletionTimestamp() {
		err = t.delete(ctx)
	} else {
		err = t.update(ctx)
	}
	if err != nil {
		return err
	}
	if !proto.Equal(hub, oldHub) {
		_, err = f.hubsClient.Update(ctx, privatev1.HubsUpdateRequest_builder{
			Object: hub,
		}.Build())
	}
	return err
}

func (t *task) update(ctx context.Context) error {
	// Shortcuts for the hub data:
	hubMeta := t.hub.GetMetadata()

	// Add a finalizer to ensure that the hub isn't completely deleted before we have time to clean up the
	// resources that depend on it.
	finalizers := hubMeta.GetFinalizers()
	if !slices.Contains(finalizers, hubFinalizer) {
		finalizers = append(finalizers, hubFinalizer)
	}
	hubMeta.SetFinalizers(finalizers)

	// Get the hub client from the cache:
	entry, err := t.parent.hubCache.Get(ctx, t.hub.GetId())
	if err != nil {
		return err
	}
	t.hubClient = entry.Client

	// Ensure that the namespace exists:
	t.namespace = &corev1.Namespace{}
	t.namespace.SetName(t.hub.GetNamespace())
	_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, t.namespace, t.mutateNamespace)
	if err != nil {
		return err
	}

	// Ensure that the pull secret exists:
	t.pullSecret = &corev1.Secret{}
	t.pullSecret.SetNamespace(t.hub.GetNamespace())
	t.pullSecret.SetName("pull-secret")
	_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, t.pullSecret, t.mutatePullSecret)
	if err != nil {
		return err
	}

	// Ensure that the infrastructure environment exists:
	t.infraEnv = &unstructured.Unstructured{}
	t.infraEnv.SetGroupVersionKind(gvks.InfraEnv)
	t.infraEnv.SetNamespace(t.hub.GetNamespace())
	t.infraEnv.SetName(t.hub.GetNamespace())
	_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, t.infraEnv, t.mutateInfraEnv)
	if err != nil {
		return err
	}

	// Ensure that the CAPI provider role exists:
	t.capiProviderRole = &rbacv1.Role{}
	t.capiProviderRole.SetNamespace(t.hub.GetNamespace())
	t.capiProviderRole.SetName("capi-provider-role")
	_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, t.capiProviderRole, t.mutateCapiProviderRole)
	if err != nil {
		return err
	}

	return nil
}

func (t *task) mutateNamespace() error {
	t.mutateLabels(t.namespace)
	return nil
}

func (t *task) mutatePullSecret() error {
	t.mutateLabels(t.pullSecret)
	t.pullSecret.Type = corev1.SecretTypeDockerConfigJson
	if t.pullSecret.Data == nil {
		t.pullSecret.Data = make(map[string][]byte)
	}
	t.pullSecret.Data[corev1.DockerConfigJsonKey] = []byte(t.hub.GetPullSecret())
	return nil
}

func (t *task) mutateInfraEnv() error {
	// Set the labels:
	t.mutateLabels(t.infraEnv)
	labels := t.infraEnv.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["networkType"] = "dhcp"

	// Set the spec:
	spec := map[string]any{
		"cpuArchitecture": "x86_64",
		"pullSecretRef": map[string]any{
			"name": t.pullSecret.GetName(),
		},
		"sshAuthorizedKey": strings.TrimSpace(t.hub.GetSshPublicKey()),
	}
	err := unstructured.SetNestedField(t.infraEnv.Object, spec, "spec")
	if err != nil {
		return err
	}

	return nil
}

func (t *task) mutateLabels(object clnt.Object) {
	values := object.GetLabels()
	if values == nil {
		values = map[string]string{}
	}
	values[labels.HubId] = t.hub.GetId()
	values[labels.HubName] = t.hub.GetMetadata().GetName()
	object.SetLabels(values)
}

func (t *task) mutateCapiProviderRole() error {
	t.mutateLabels(t.capiProviderRole)
	t.capiProviderRole.Rules = []rbacv1.PolicyRule{{
		APIGroups: []string{
			"agent-install.openshift.io",
		},
		Resources: []string{
			"agents",
		},
		Verbs: []string{
			"*",
		},
	}}
	return nil
}

func (t *task) delete(ctx context.Context) error {
	// Remove the hub from the cache:
	t.parent.logger.DebugContext(
		ctx,
		"Removing hub from cache",
		slog.String("hub_id", t.hub.GetId()),
	)
	t.parent.hubCache.Remove(t.hub.GetId())

	return nil
}

// hubFinalizer is the finalizer that will be used to ensure that the hub isn't completely deleted before we have
// time to clean up the resources that depend on it.
const hubFinalizer = "fulfillment-controller"
