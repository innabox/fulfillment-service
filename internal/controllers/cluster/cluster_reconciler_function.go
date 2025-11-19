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
	"math/rand/v2"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	sharedv1 "github.com/innabox/fulfillment-service/internal/api/shared/v1"
	"github.com/innabox/fulfillment-service/internal/controllers"
	"github.com/innabox/fulfillment-service/internal/json"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
)

// FunctionBuilder contains the data and logic needed to build a function that reconciles clustes.
type FunctionBuilder struct {
	logger       *slog.Logger
	connection   *grpc.ClientConn
	hubCache     *controllers.HubCache
	dnsServer    string
	dnsZone      string
	dnsKeyName   string
	dnsKeySecret string
}

type function struct {
	logger            *slog.Logger
	hubCache          *controllers.HubCache
	clustersClient    privatev1.ClustersClient
	hubsClient        privatev1.HubsClient
	hostClassesClient privatev1.HostClassesClient
	hostsClient       privatev1.HostsClient
	dnsServer         string
	dnsZone           string
	dnsZoneFq         string
	dnsKeyName        string
	dnsKeyNameFq      string
	dnsKeySecret      string
	dnsClient         *dns.Client
}

type task struct {
	parent        *function
	cluster       *privatev1.Cluster
	hosts         map[string]*privatev1.Host
	hub           *privatev1.Hub
	alias         string
	baseDomain    string
	apiDomain     string
	apiIntDomain  string
	ingressDomain string
	hubClient     clnt.Client
	pullSecret    *corev1.Secret
	sshSecret     *corev1.Secret
	hostedCluster *unstructured.Unstructured
	nodePools     map[string]*unstructured.Unstructured
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

// SetDnsServer sets the DNS server address for dynamic updates (e.g., "my-dns-server.com:53"). This is optional.
func (b *FunctionBuilder) SetDnsServer(value string) *FunctionBuilder {
	b.dnsServer = value
	return b
}

// SetDnsZone sets the DNS zone for dynamic updates (e.g., "my.demo"). This is optional.
func (b *FunctionBuilder) SetDnsZone(value string) *FunctionBuilder {
	b.dnsZone = value
	return b
}

// SetDnsKeyName sets the name of the TSIG key for authentication. This is optional.
//
// For example, to generate a TSIG key for use wit the 'bind' DNS server you can use the follwing command:
//
//	tsig-keygen -a hmac-sha256 fulfillment-service
//
// It will generate something like this:
//
//	key "my-key" {
//		algorithm hmac-sha256;
//		secret "...";
//	};
//
// That needs to be included in the 'named.confg' configuration file.
//
// You will also need a zone with update permissions for this key, something like this:
//
//	zone "my.demo" in {
//		type master;
//		file "my.demo.zone";
//		update-policy {
//			grant my-key zonesub ANY;
//		};
//	};
//
// The you will need to reload the DNS server to apply the changes.
//
// The key name, 'my-key' in this example, is what should be passed to the SetDnsKeyName function. The secret is what
// should be passed to the SetDnsKeySecret function.
func (b *FunctionBuilder) SetDnsKeyName(value string) *FunctionBuilder {
	b.dnsKeyName = value
	return b
}

// SetDnsKeySecret sets the secret value of the TSIG key for authentication. This is optional.
//
// See the SetDnsKeyName function for more information.
func (b *FunctionBuilder) SetDnsKeySecret(value string) *FunctionBuilder {
	b.dnsKeySecret = value
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

	// The DNS library that we use requires fully qualified domain names, but HyperShift chokes on the training
	// dot, so we need to have both flavours.
	dnsZoneFq := dns.Fqdn(b.dnsZone)
	dnsKeyNameFq := dns.Fqdn(b.dnsKeyName)

	// Create DNS client if DNS server is provided:
	var dnsClient *dns.Client
	if b.dnsServer != "" {
		dnsClient = &dns.Client{
			Net:     "tcp",
			Timeout: 5 * time.Second,
		}
		if b.dnsKeyName != "" && b.dnsKeySecret != "" {
			dnsClient.TsigSecret = map[string]string{
				dnsKeyNameFq: b.dnsKeySecret,
			}
		}
	}

	// Create and populate the object:
	object := &function{
		logger:            b.logger,
		clustersClient:    privatev1.NewClustersClient(b.connection),
		hubsClient:        privatev1.NewHubsClient(b.connection),
		hostsClient:       privatev1.NewHostsClient(b.connection),
		hostClassesClient: privatev1.NewHostClassesClient(b.connection),
		hubCache:          b.hubCache,
		dnsServer:         b.dnsServer,
		dnsZone:           b.dnsZone,
		dnsZoneFq:         dnsZoneFq,
		dnsKeyName:        b.dnsKeyName,
		dnsKeyNameFq:      dnsKeyNameFq,
		dnsKeySecret:      b.dnsKeySecret,
		dnsClient:         dnsClient,
	}
	result = object.run
	return
}

func (f *function) run(ctx context.Context, cluster *privatev1.Cluster) error {
	oldCluster := proto.Clone(cluster).(*privatev1.Cluster)
	t := task{
		parent:  f,
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
	if !proto.Equal(cluster, oldCluster) {
		_, err = f.clustersClient.Update(ctx, privatev1.ClustersUpdateRequest_builder{
			Object: cluster,
		}.Build())
	}
	return err
}

func (t *task) update(ctx context.Context) error {
	// Set the default values:
	t.setDefaults()

	// Shortcuts for the cluster metadata and spec:
	clusterMeta := t.cluster.GetMetadata()
	clusterStatus := t.cluster.GetStatus()

	// Add a finalizer to ensure that the cluster isn't completely deleted before we have time to delete the
	// resources in the hub.
	finalizers := clusterMeta.GetFinalizers()
	if !slices.Contains(finalizers, clusterFinalizer) {
		finalizers = append(finalizers, clusterFinalizer)
		clusterMeta.SetFinalizers(finalizers)
	}

	// Do nothing if the order isn't progressing:
	if clusterStatus.GetState() != privatev1.ClusterState_CLUSTER_STATE_PROGRESSING {
		return nil
	}

	// Load the hosts that are assigned to the cluster:
	err := t.loadHosts(ctx)
	if err != nil {
		return err
	}

	// Check if hosts need to be assigned or unassigned:
	err = t.checkHostAssignment(ctx)
	if err != nil {
		return err
	}

	// Select the hub, and if no hubs are available, update the condition to inform the user that creation is
	// pending. Note that we don't want to disclose the existence of hubs to the user, as that is a internal
	// implementation detail, so keep the message generic enough to not reveal that information.
	err = t.selectHub(ctx)
	if err != nil {
		t.parent.logger.ErrorContext(
			ctx,
			"Failed to select hub",
			slog.String("error", err.Error()),
		)
		t.updateCondition(
			privatev1.ClusterConditionType_CLUSTER_CONDITION_TYPE_PROGRESSING,
			sharedv1.ConditionStatus_CONDITION_STATUS_FALSE,
			"ResourcesUnavailable",
			"The cluster cannot be created because there are no resources available to fulfill the "+
				"request.",
		)
		return nil
	}

	// Save the selected hub in the private data of the cluster:
	clusterStatus.SetHub(t.hub.GetId())

	// Select an alias for the cluster:
	err = t.selectAlias(ctx)
	if err != nil {
		return err
	}

	// Calculate and create the DNS records for the API:
	t.baseDomain = fmt.Sprintf("%s.%s", clusterMeta.GetName(), t.parent.dnsZone)
	t.apiDomain = fmt.Sprintf("api.%s", t.baseDomain)
	t.apiIntDomain = fmt.Sprintf("api-int.%s", t.baseDomain)
	err = t.parent.createDnsARecord(ctx, t.apiDomain, "192.168.101.3")
	if err != nil {
		return err
	}
	err = t.parent.createDnsARecord(ctx, t.apiIntDomain, "192.168.101.3")
	if err != nil {
		return err
	}

	// Calculate and create the DNS record for the ingress:
	t.ingressDomain = fmt.Sprintf("*.apps.%s", t.baseDomain)
	err = t.parent.createDnsARecord(ctx, t.ingressDomain, "192.168.101.3")
	if err != nil {
		return err
	}

	// Ensure that the pull secret exists:
	t.pullSecret = &corev1.Secret{}
	t.pullSecret.SetNamespace(t.hub.GetNamespace())
	t.pullSecret.SetName(fmt.Sprintf("%s-pull", clusterMeta.GetName()))
	_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, t.pullSecret, t.mutatePullSecret)
	if err != nil {
		return err
	}

	// Ensure that the SSH key secret exists:
	t.sshSecret = &corev1.Secret{}
	t.sshSecret.SetNamespace(t.hub.GetNamespace())
	t.sshSecret.SetName(fmt.Sprintf("%s-ssh", clusterMeta.GetName()))
	_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, t.sshSecret, t.mutateSshSecret)
	if err != nil {
		return err
	}

	// Ensure that the hosted cluster exists:
	t.hostedCluster = &unstructured.Unstructured{}
	t.hostedCluster.SetGroupVersionKind(gvks.HostedCluster)
	t.hostedCluster.SetNamespace(t.hub.GetNamespace())
	t.hostedCluster.SetName(clusterMeta.GetName())
	_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, t.hostedCluster, t.mutateHostedCluster)
	if err != nil {
		return err
	}

	// Ensure that there is a node pool for each node set:
	t.nodePools = map[string]*unstructured.Unstructured{}
	for nodeSetName := range t.cluster.GetSpec().GetNodeSets() {
		nodePool := &unstructured.Unstructured{}
		nodePool.SetGroupVersionKind(gvks.NodePool)
		nodePool.SetNamespace(t.hostedCluster.GetNamespace())
		nodePool.SetName(fmt.Sprintf("%s-%s", t.hostedCluster.GetName(), nodeSetName))
		t.nodePools[nodeSetName] = nodePool
		_, err = controllerutil.CreateOrPatch(ctx, t.hubClient, nodePool, func() error {
			return t.mutateNodePool(nodeSetName)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *task) loadHosts(ctx context.Context) error {
	var hostIds []string
	for _, nodeSet := range t.cluster.GetSpec().GetNodeSets() {
		hostIds = append(hostIds, nodeSet.GetHosts()...)
	}
	for i, hostId := range hostIds {
		hostIds[i] = strconv.Quote(hostId)
	}
	hostIdList := strings.Join(hostIds, ", ")
	hostFilter := fmt.Sprintf("this.id in [%s]", hostIdList)
	hostListResponse, err := t.parent.hostsClient.List(ctx, privatev1.HostsListRequest_builder{
		Filter: &hostFilter,
		Limit:  proto.Int32(int32(len(hostIds))),
	}.Build())
	if err != nil {
		return err
	}
	t.hosts = map[string]*privatev1.Host{}
	for _, host := range hostListResponse.GetItems() {
		hostId := host.GetId()
		t.hosts[hostId] = host
	}
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

func (t *task) mutateSshSecret() error {
	t.mutateLabels(t.sshSecret)
	t.sshSecret.Type = corev1.SecretTypeOpaque
	if t.sshSecret.StringData == nil {
		t.sshSecret.StringData = make(map[string]string)
	}
	t.sshSecret.StringData["id_rsa.pub"] = t.hub.GetSshPublicKey()
	return nil
}

func (t *task) mutateHostedCluster() error {
	t.mutateLabels(t.hostedCluster)
	spec, err := t.renderHostedClusterSpec()
	if err != nil {
		return err
	}
	err = unstructured.SetNestedMap(t.hostedCluster.Object, spec, "spec")
	if err != nil {
		return err
	}

	return nil
}

func (t *task) renderHostedClusterSpec() (result json.Object, err error) {
	dnsSpec, err := t.renderHostedClusterDns()
	if err != nil {
		return
	}
	etcdSpec, err := t.renderHostedClusterEtcd()
	if err != nil {
		return
	}
	networkingSpec, err := t.renderHostedClusterNetworking()
	if err != nil {
		return
	}
	platformSpec, err := t.renderHostedClusterPlatform()
	if err != nil {
		return
	}
	releaseSpec, err := t.renderRelease()
	if err != nil {
		return
	}
	servicesSpec, err := t.renderHostedClusterServices()
	if err != nil {
		return
	}
	pullSecretRef := json.Object{
		"name": t.pullSecret.GetName(),
	}
	sshKeyRef := json.Object{
		"name": t.sshSecret.GetName(),
	}
	result = json.Object{
		"clusterID":                        t.cluster.GetId(),
		"controllerAvailabilityPolicy":     "SingleReplica",
		"dns":                              dnsSpec,
		"etcd":                             etcdSpec,
		"infraID":                          t.cluster.GetId(),
		"infrastructureAvailabilityPolicy": "SingleReplica",
		"issuerURL":                        "https://kubernetes.default.svc",
		"networking":                       networkingSpec,
		"olmCatalogPlacement":              "guest",
		"platform":                         platformSpec,
		"pullSecret":                       pullSecretRef,
		"release":                          releaseSpec,
		"services":                         servicesSpec,
		"sshKey":                           sshKeyRef,
	}
	return
}

func (t *task) renderHostedClusterDns() (result json.Object, err error) {
	result = json.Object{
		"baseDomain": t.parent.dnsZone,
	}
	return
}

func (t *task) renderHostedClusterEtcd() (result json.Object, err error) {
	result = json.Object{
		"managed": json.Object{
			"storage": json.Object{
				"type": "PersistentVolume",
				"persistentVolume": json.Object{
					"size": "8Gi",
				},
			},
		},
		"managementType": "Managed",
	}
	return
}

func (t *task) renderHostedClusterNetworking() (result json.Object, err error) {
	result = json.Object{
		"clusterNetwork": json.List{
			json.Object{
				"cidr": "10.132.0.0/14",
			},
		},
		"networkType": "OVNKubernetes",
		"serviceNetwork": json.List{
			json.Object{
				"cidr": "172.31.0.0/16",
			},
		},
	}
	return
}

func (t *task) renderHostedClusterPlatform() (result json.Object, err error) {
	result = json.Object{
		"agent": json.Object{
			"agentNamespace": t.hub.GetNamespace(),
		},
		"type": "Agent",
	}
	return
}

func (t *task) renderHostedClusterServices() (result json.List, err error) {
	result = json.List{
		json.Object{
			"service": "APIServer",
			"servicePublishingStrategy": json.Object{
				"type": "NodePort",
				"nodePort": json.Object{
					"address": t.apiDomain,
					"port":    int64(30000),
				},
			},
		},
		json.Object{
			"service": "OAuthServer",
			"servicePublishingStrategy": json.Object{
				"type": "Route",
			},
		},
		json.Object{
			"service": "OIDC",
			"servicePublishingStrategy": json.Object{
				"type": "Route",
			},
		},
		json.Object{
			"service": "Konnectivity",
			"servicePublishingStrategy": json.Object{
				"type": "Route",
			},
		},
		json.Object{
			"service": "Ignition",
			"servicePublishingStrategy": json.Object{
				"type": "Route",
			},
		},
	}
	return
}

func (t *task) mutateNodePool(nodeSetName string) error {
	t.mutateLabels(t.nodePools[nodeSetName])
	nodeSet := t.cluster.GetSpec().GetNodeSets()[nodeSetName]
	if nodeSet == nil {
		return fmt.Errorf("node set '%s' not found", nodeSetName)
	}
	spec, err := t.renderNodePoolSpec(nodeSet)
	if err != nil {
		return err
	}
	nodePool := t.nodePools[nodeSetName]
	err = unstructured.SetNestedMap(nodePool.Object, spec, "spec")
	if err != nil {
		return err
	}

	return nil
}

func (t *task) renderNodePoolSpec(nodeSet *privatev1.ClusterNodeSet) (result json.Object, err error) {
	platformSpec, err := t.renderNodePoolPlatform()
	if err != nil {
		return
	}
	managementSpec, err := t.renderNodePoolManagement()
	if err != nil {
		return
	}
	releaseSpec, err := t.renderRelease()
	if err != nil {
		return
	}
	result = json.Object{
		"arch":        "amd64",
		"clusterName": t.hostedCluster.GetName(),
		"management":  managementSpec,
		"platform":    platformSpec,
		"release":     releaseSpec,
		"replicas":    int64(nodeSet.GetSize()),
	}
	return
}

func (t *task) renderNodePoolPlatform() (result json.Object, err error) {
	result = json.Object{
		"type": "Agent",
		"agent": json.Object{
			"agentLabelSelector": json.Object{},
		},
	}
	return
}

func (t *task) renderNodePoolManagement() (result json.Object, err error) {
	result = json.Object{
		"upgradeType": "Replace",
	}
	return
}

func (t *task) renderRelease() (result json.Object, err error) {
	result = json.Object{
		"image": "quay.io/openshift-release-dev/ocp-release:4.19.18-x86_64",
	}
	return
}

func (t *task) mutateLabels(object clnt.Object) {
	values := object.GetLabels()
	if values == nil {
		values = map[string]string{}
	}
	values[labels.ClusterId] = t.cluster.GetId()
	values[labels.ClusterName] = t.cluster.GetMetadata().GetName()
	object.SetLabels(values)
}

func (t *task) setDefaults() {
	status := t.cluster.GetStatus()
	if status == nil {
		status = privatev1.ClusterStatus_builder{}.Build()
		t.cluster.SetStatus(status)
	}
	if status.GetState() == privatev1.ClusterState_CLUSTER_STATE_UNSPECIFIED {
		status.SetState(privatev1.ClusterState_CLUSTER_STATE_PROGRESSING)
	}
	for value := range privatev1.ClusterConditionType_name {
		if value != 0 {
			t.setConditionDefaults(privatev1.ClusterConditionType(value))
		}
	}
	nodeSets := status.GetNodeSets()
	if nodeSets == nil {
		nodeSets = map[string]*privatev1.ClusterNodeSet{}
		status.SetNodeSets(nodeSets)
	}
	for nodeSetName, nodeSet := range t.cluster.GetSpec().GetNodeSets() {
		statusNodeSet := nodeSets[nodeSetName]
		if statusNodeSet == nil {
			statusNodeSet = privatev1.ClusterNodeSet_builder{
				HostClass: nodeSet.GetHostClass(),
			}.Build()
			nodeSets[nodeSetName] = statusNodeSet
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

func (t *task) delete(ctx context.Context) error {
	// Unassign all the hosts from the cluster:
	for _, nodeSet := range t.cluster.GetSpec().GetNodeSets() {
		err := t.unassignHosts(ctx, nodeSet.GetHostClass(), int(nodeSet.GetSize()))
		if err != nil {
			return err
		}
	}

	// TODO: Skip the rest of the logic for now, as we just want to test the hosts assignment.
	if true {
		return nil
	}

	// Do nothing if we don't know the hub yet:
	hub := t.cluster.GetStatus().GetHub()
	if hub == "" {
		return nil
	}
	err := t.getHub(ctx)
	if err != nil {
		return err
	}

	return err
}

func (t *task) checkHostAssignment(ctx context.Context) error {
	for nodeSetName, nodeSet := range t.cluster.GetSpec().GetNodeSets() {
		err := t.checkNodeSetHostAssignment(ctx, nodeSetName, nodeSet)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *task) checkNodeSetHostAssignment(ctx context.Context, nodeSetName string, nodeSet *privatev1.ClusterNodeSet) error {
	// Get the desired number of hosts for this node set:
	desiredSize := int(nodeSet.GetSize())

	// The complete list of hosts assigned to this cluster has been already loaded, so we only need to iterate
	// over it and count the ones that are assigned to this node set.
	actualSize := 0
	for _, hostId := range nodeSet.GetHosts() {
		host := t.hosts[hostId]
		if host != nil {
			actualSize++
		}
	}

	// Calculate the delta, which can be zero, positive or negative:
	deltaSize := desiredSize - actualSize
	t.parent.logger.DebugContext(
		ctx,
		"Host assignment delta",
		slog.String("node_set", nodeSetName),
		slog.Int("desired", desiredSize),
		slog.Int("actual", actualSize),
		slog.Int("delta", deltaSize),
	)

	// If the delta is positive, we need to assign new hosts, if it is negative, we need to unassign hosts.
	switch {
	case deltaSize > 0:
		err := t.assignHosts(ctx, nodeSetName, deltaSize)
		if err != nil {
			return err
		}
	case deltaSize < 0:
		err := t.unassignHosts(ctx, nodeSetName, -deltaSize)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *task) assignHosts(ctx context.Context, nodeSetName string, hostCount int) error {
	// Get the nodeSetSpec and status of the node set:
	nodeSetSpec := t.cluster.GetSpec().GetNodeSets()[nodeSetName]
	nodeSetStatus := t.cluster.GetStatus().GetNodeSets()[nodeSetName]

	// Find available hosts with matching hostClassId:
	hostClassId := nodeSetSpec.GetHostClass()
	hostFilter := fmt.Sprintf(
		"!has(this.metadata.deletion_timestamp) && "+
			"this.spec.class == '%s' && "+
			"!has(this.status.cluster)",
		hostClassId,
	)
	hostListResponse, err := t.parent.hostsClient.List(ctx, privatev1.HostsListRequest_builder{
		Filter: &hostFilter,
		Limit:  proto.Int32(int32(hostCount)),
	}.Build())
	if err != nil {
		return err
	}
	hosts := hostListResponse.GetItems()

	// Add the hosts to the node set:
	for _, host := range hosts {
		host.GetStatus().SetCluster(t.cluster.GetId())
		_, err = t.parent.hostsClient.Update(ctx, privatev1.HostsUpdateRequest_builder{
			Object: host,
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{
					"status.cluster",
				},
			},
		}.Build())
		if err != nil {
			return err
		}
		hostId := host.GetId()
		hostIds := nodeSetStatus.GetHosts()
		if !slices.Contains(hostIds, hostId) {
			hostIds = append(hostIds, hostId)
			nodeSetStatus.SetHosts(hostIds)
		}
	}

	// Add the new hosts to the list of loaded hosts:
	for _, host := range hosts {
		hostId := host.GetId()
		t.hosts[hostId] = host
	}

	return nil
}

func (t *task) unassignHosts(ctx context.Context, nodeSetName string, hostCount int) error {
	// Get the node set spec and status:
	nodeSetStatus := t.cluster.GetStatus().GetNodeSets()[nodeSetName]

	// Randomly select the hosts to unassign:
	allHostIds := nodeSetStatus.GetHosts()
	selectedHostIds := make([]string, 0, hostCount)
	for len(selectedHostIds) < hostCount {
		randomIndex := rand.IntN(len(allHostIds))
		randomHostId := allHostIds[randomIndex]
		if !slices.Contains(selectedHostIds, randomHostId) {
			selectedHostIds = append(selectedHostIds, randomHostId)
		}
	}

	// Remove the reference to the cluster from the selected hosts:
	for _, hostId := range selectedHostIds {
		_, err := t.parent.hostsClient.Update(ctx, privatev1.HostsUpdateRequest_builder{
			Object: privatev1.Host_builder{
				Id: hostId,
				Status: privatev1.HostStatus_builder{
					Cluster: "",
				}.Build(),
			}.Build(),
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{
					"status.cluster",
				},
			},
		}.Build())
		if err != nil {
			return err
		}
	}

	// Update the status of the node set:
	nodeSetStatus.SetHosts(allHostIds)

	// Remove the unassigned hosts from the list of loaded hosts:
	for _, hostId := range selectedHostIds {
		delete(t.hosts, hostId)
	}

	return nil
}

func (t *task) lookupHostClass(ctx context.Context, key string) (result string, err error) {
	filter := fmt.Sprintf("this.id == %[1]s || this.metadata.name == %[1]s", strconv.Quote(key))
	response, err := t.parent.hostClassesClient.List(ctx, privatev1.HostClassesListRequest_builder{
		Filter: &filter,
		Limit:  proto.Int32(1),
	}.Build())
	if err != nil {
		return
	}
	total := response.GetTotal()
	switch {
	case total == 0:
		err = fmt.Errorf("there is no host class with identifier or name '%s'", key)
	case total == 1:
		result = response.GetItems()[0].GetId()
	default:
		err = fmt.Errorf("there are multiple host classes with identifier or name '%s'", key)
	}
	return
}

func (t *task) selectHub(ctx context.Context) error {
	hub := t.cluster.GetStatus().GetHub()
	if hub == "" {
		response, err := t.parent.hubsClient.List(ctx, privatev1.HubsListRequest_builder{}.Build())
		if err != nil {
			return err
		}
		if len(response.Items) == 0 {
			return errors.New("there are no hubs")
		}
		hub = response.Items[rand.IntN(len(response.Items))].GetId()
		t.parent.logger.DebugContext(
			ctx,
			"Selected hub",
			slog.String("name", t.hub.GetMetadata().GetName()),
			slog.String("id", t.hub.GetId()),
		)
	}
	entry, err := t.parent.hubCache.Get(ctx, hub)
	if err != nil {
		return err
	}
	t.hub = entry.Hub
	t.hubClient = entry.Client
	return nil
}

func (t *task) selectAlias(ctx context.Context) error {
	// If the alias is already set, do nothing:
	t.alias = t.cluster.GetStatus().GetAlias()
	if t.alias != "" {
		return nil
	}

	// Try to use the name of the cluster as the alias:
	t.alias = t.cluster.GetMetadata().GetName()
	if t.alias != "" {
		t.cluster.GetStatus().SetAlias(t.alias)
		return nil
	}

	// Try to use the identifier of the cluster as the alias:
	t.alias = t.cluster.GetId()
	t.cluster.GetStatus().SetAlias(t.alias)
	return nil
}

func (t *task) getHub(ctx context.Context) error {
	hub := t.cluster.GetStatus().GetHub()
	if hub == "" {
		return errors.New("hub is not set")
	}
	entry, err := t.parent.hubCache.Get(ctx, hub)
	if err != nil {
		return err
	}
	t.hub = entry.Hub
	t.hubClient = entry.Client
	return nil
}

// updateCondition updates or creates a condition with the specified type, status, reason, and message.
func (t *task) updateCondition(conditionType privatev1.ClusterConditionType, status sharedv1.ConditionStatus,
	reason string, message string) {
	conditions := t.cluster.GetStatus().GetConditions()
	updated := false
	for i, condition := range conditions {
		if condition.GetType() == conditionType {
			conditions[i] = privatev1.ClusterCondition_builder{
				Type:    conditionType,
				Status:  status,
				Reason:  &reason,
				Message: &message,
			}.Build()
			updated = true
			break
		}
	}
	if !updated {
		conditions = append(conditions, privatev1.ClusterCondition_builder{
			Type:    conditionType,
			Status:  status,
			Reason:  &reason,
			Message: &message,
		}.Build())
	}
	t.cluster.GetStatus().SetConditions(conditions)
}

// createDnsARecord creates a new DNS A record using RFC 2136 dynamic update.
func (f *function) createDnsARecord(ctx context.Context, domain string, address string) error {
	// Ensure the domain name is fully qualified:
	domain = dns.Fqdn(domain)

	// Log the operation:
	f.logger.DebugContext(
		ctx,
		"Creating DNS record",
		slog.String("domain", domain),
		slog.String("zone", f.dnsZone),
		slog.String("ip", address),
		slog.String("server", f.dnsServer),
	)

	// Create the DNS update message:
	msg := &dns.Msg{}
	msg.SetUpdate(f.dnsZoneFq)

	// Create the A record:
	record := &dns.A{
		Hdr: dns.RR_Header{
			Name:   domain,
			Ttl:    300,
			Class:  dns.ClassINET,
			Rrtype: dns.TypeA,
		},
		A: net.ParseIP(address),
	}
	msg.Insert([]dns.RR{record})

	// Sign the message with TSIG if configured:
	if f.dnsKeyName != "" && f.dnsKeySecret != "" {
		msg.SetTsig(f.dnsKeyNameFq, dns.HmacSHA256, 300, time.Now().Unix())
	}

	// Send the update to the DNS server:
	response, _, err := f.dnsClient.Exchange(msg, f.dnsServer)
	if err != nil {
		f.logger.ErrorContext(
			ctx,
			"Failed to send DNS update",
			slog.Any("error", err),
			slog.String("server", f.dnsServer),
		)
		return fmt.Errorf("failed to send DNS update to %s: %w", f.dnsServer, err)
	}

	// Check the response:
	if response.Rcode != dns.RcodeSuccess {
		f.logger.ErrorContext(
			ctx,
			"DNS update failed",
			slog.String("rcode", dns.RcodeToString[response.Rcode]),
			slog.Int("rcode_value", response.Rcode),
		)
		return fmt.Errorf("DNS update failed with rcode: %s", dns.RcodeToString[response.Rcode])
	}

	f.logger.InfoContext(
		ctx,
		"DNS record created successfully",
		slog.String("domain", domain),
		slog.String("ip", address),
	)

	return nil
}

// clusterFinalizer is the finalizer that will be used to ensure that the cluster isn't completely deleted before we have
// time clean up the other resources that depend on it.
const clusterFinalizer = "fulfillment-controller"
