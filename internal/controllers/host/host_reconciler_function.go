/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package host

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
)

// FunctionBuilder contains the data and logic needed to build a function that reconciles hosts.
type FunctionBuilder struct {
	logger     *slog.Logger
	connection *grpc.ClientConn
	hubCache   *controllers.HubCache
}

type function struct {
	logger          *slog.Logger
	hubCache        *controllers.HubCache
	hostsClient     privatev1.HostsClient
	hostPoolsClient privatev1.HostPoolsClient
	hubsClient      privatev1.HubsClient
}

type task struct {
	parent        *function
	host          *privatev1.Host
	hub           *privatev1.Hub
	hubClient     clnt.Client
	bmcSecret     *corev1.Secret
	bareMetalHost *unstructured.Unstructured
}

// NewFunction creates a new builder that can then be used to create a new host reconciler function.
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

// Build uses the information stored in the builder to create a new host reconciler.
func (b *FunctionBuilder) Build() (result controllers.ReconcilerFunction[*privatev1.Host], err error) {
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
		hostsClient:     privatev1.NewHostsClient(b.connection),
		hostPoolsClient: privatev1.NewHostPoolsClient(b.connection),
		hubsClient:      privatev1.NewHubsClient(b.connection),
		hubCache:        b.hubCache,
	}
	result = object.run
	return
}

func (f *function) run(ctx context.Context, host *privatev1.Host) error {
	oldHost := proto.Clone(host).(*privatev1.Host)
	t := task{
		parent: f,
		host:   host,
	}
	var err error
	if host.GetMetadata().HasDeletionTimestamp() {
		err = t.delete(ctx)
	} else {
		err = t.update(ctx)
	}
	if err != nil {
		return err
	}
	if !proto.Equal(host, oldHost) {
		_, err = f.hostsClient.Update(ctx, privatev1.HostsUpdateRequest_builder{
			Object: host,
		}.Build())
	}
	return err
}

func (t *task) update(ctx context.Context) error {
	// Set the default values:
	t.setDefaults()

	// Add a finalizer to ensure that the host isn't completely deleted before we have time to delete the
	// resources in the hub.
	finalizers := t.host.GetMetadata().GetFinalizers()
	if !slices.Contains(finalizers, hostFinalizer) {
		finalizers = append(finalizers, hostFinalizer)
	}
	t.host.GetMetadata().SetFinalizers(finalizers)

	// Select the hub:
	err := t.selectHub(ctx)
	if err != nil {
		return err
	}

	// Ensure tha the BMC credentials secret exists:
	t.bmcSecret = &corev1.Secret{}
	t.bmcSecret.SetNamespace(t.hub.GetNamespace())
	t.bmcSecret.SetName(fmt.Sprintf("%s-bmc", t.host.GetMetadata().GetName()))
	_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, t.bmcSecret, t.mutateBmcSecret)
	if err != nil {
		return err
	}

	// Ensure that the bare metal host exists:
	t.bareMetalHost = &unstructured.Unstructured{}
	t.bareMetalHost.SetGroupVersionKind(gvks.BareMetalHost)
	t.bareMetalHost.SetNamespace(t.hub.GetNamespace())
	t.bareMetalHost.SetName(t.host.GetMetadata().GetName())
	_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, t.bareMetalHost, t.mutateBareMetalHost)
	if err != nil {
		return err
	}

	return nil
}

func (t *task) mutateBmcSecret() error {
	t.mutateLabels(t.bmcSecret)
	t.bmcSecret.Type = corev1.SecretTypeBasicAuth
	if t.bmcSecret.Data == nil {
		t.bmcSecret.Data = map[string][]byte{}
	}
	t.bmcSecret.Data[corev1.BasicAuthUsernameKey] = []byte(t.host.GetSpec().GetBmc().GetUser())
	t.bmcSecret.Data[corev1.BasicAuthPasswordKey] = []byte(t.host.GetSpec().GetBmc().GetPassword())
	return nil
}

func (t *task) mutateBareMetalHost() error {
	// Set labels and annotations
	t.mutateLabels(t.bareMetalHost)
	t.mutateAnnotations(t.bareMetalHost)

	// Set the spec:
	spec := map[string]any{
		"automatedCleaningMode": "disabled",
		"bmc": map[string]any{
			"address":                        t.host.GetSpec().GetBmc().GetUrl(),
			"credentialsName":                t.bmcSecret.GetName(),
			"disableCertificateVerification": t.host.GetSpec().GetBmc().GetInsecure(),
		},
		"bootMACAddress": t.host.GetSpec().GetBootMac(),
		"customDeploy": map[string]any{
			"method": "start_assisted_install",
		},
		"online": true,
	}
	err := unstructured.SetNestedField(t.bareMetalHost.Object, spec, "spec")
	if err != nil {
		return err
	}

	return nil
}

func (t *task) mutateLabels(object clnt.Object) {
	// Make sure the labels map is initialized:
	values := object.GetLabels()
	if values == nil {
		values = map[string]string{}
	}

	// Add labels to simplify identification of the host:
	values[labels.HostId] = t.host.GetId()
	values[labels.HostName] = t.host.GetMetadata().GetName()

	// Add the label that indicates the infrastructure envirionment that this host belongs to, which should be
	// the one inside the namespace of the hub, and with the same name:
	values["infraenvs.agent-install.openshift.io"] = t.hub.GetNamespace()

	// If the host is assiged to a cluster, then add the label with the cluster identifier, otherwise remove it, in
	// case it was added in the past.
	cluster := t.host.GetStatus().GetCluster()
	if cluster != "" {
		values[labels.ClusterId] = cluster
	} else {
		delete(values, labels.ClusterId)
	}

	// Save the labels:
	object.SetLabels(values)
}

func (t *task) mutateAnnotations(object clnt.Object) {
	// Make sure the annotations map is initialized:
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	// Add the annotation that indicates the hostname of the host:
	annotations["bmac.agent-install.openshift.io/hostname"] = t.host.GetMetadata().GetName()
	annotations["inspect.metal3.io"] = "disabled"

	// Save the annotations:
	object.SetAnnotations(annotations)
}

func (t *task) setDefaults() {
	if !t.host.HasStatus() {
		t.host.SetStatus(&privatev1.HostStatus{})
	}
	if t.host.GetStatus().GetPowerState() == privatev1.HostPowerState_HOST_POWER_STATE_UNSPECIFIED {
		t.host.GetStatus().SetPowerState(privatev1.HostPowerState_HOST_POWER_STATE_OFF)
	}
}

func (t *task) delete(ctx context.Context) error {
	err := t.selectHub(ctx)
	if err != nil {
		return err
	}
	return err
}

func (t *task) selectHub(ctx context.Context) error {
	hub := t.host.GetStatus().GetHub()
	if hub == "" {
		response, err := t.parent.hubsClient.List(ctx, privatev1.HubsListRequest_builder{}.Build())
		if err != nil {
			return err
		}
		if len(response.Items) == 0 {
			return errors.New("there are no hubs")
		}
		t.hub = response.Items[rand.IntN(len(response.Items))]
	}
	t.parent.logger.DebugContext(
		ctx,
		"Selected hub",
		slog.String("name", t.hub.GetMetadata().GetName()),
		slog.String("id", t.hub.GetId()),
	)
	entry, err := t.parent.hubCache.Get(ctx, t.hub.GetId())
	if err != nil {
		return err
	}
	t.hubClient = entry.Client
	return nil
}

// hostFinalizer is the finalizer that will be used to ensure that the host isn't completely deleted before we have
// time clean up the other resources that depend on it.
const hostFinalizer = "fulfillment-controller"
