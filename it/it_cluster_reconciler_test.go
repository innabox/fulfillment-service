/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package it

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/kubernetes/gvks"
	"github.com/innabox/fulfillment-service/internal/kubernetes/labels"
)

var _ = Describe("Cluster reconciler", func() {
	var (
		ctx             context.Context
		clustersClient  ffv1.ClustersClient
		templatesClient privatev1.ClusterTemplatesClient
		templateId      string
	)

	BeforeEach(func() {
		// Create a context:
		ctx = context.Background()

		// Create the clients:
		clustersClient = ffv1.NewClustersClient(clientConn)
		templatesClient = privatev1.NewClusterTemplatesClient(adminConn)

		// Create a template for testing:
		templateId = fmt.Sprintf("my_template_%s", uuid.NewString())
		_, err := templatesClient.Create(ctx, privatev1.ClusterTemplatesCreateRequest_builder{
			Object: privatev1.ClusterTemplate_builder{
				Id:          templateId,
				Title:       "My template %s",
				Description: "My template.",
				NodeSets: map[string]*privatev1.ClusterTemplateNodeSet{
					"my_node_set": privatev1.ClusterTemplateNodeSet_builder{
						HostClass: "my_host_class",
						Size:      3,
					}.Build(),
				},
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			_, err := templatesClient.Delete(ctx, privatev1.ClusterTemplatesDeleteRequest_builder{
				Id: templateId,
			}.Build())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	It("Creates the Kubernetes object when a cluster is created", func() {
		// Create the cluster
		response, err := clustersClient.Create(ctx, ffv1.ClustersCreateRequest_builder{
			Object: ffv1.Cluster_builder{
				Spec: ffv1.ClusterSpec_builder{
					Template: templateId,
				}.Build(),
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		object := response.GetObject()
		DeferCleanup(func() {
			_, err := clustersClient.Delete(ctx, ffv1.ClustersDeleteRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
		})

		// Check that the Kubernetes object is eventually created:
		kubeClient := kind.Client()
		kubeList := &unstructured.UnstructuredList{}
		kubeList.SetGroupVersionKind(gvks.ClusterOrderList)
		var kubeObject *unstructured.Unstructured
		Eventually(
			func(g Gomega) {
				err := kubeClient.List(ctx, kubeList, crclient.MatchingLabels{
					labels.ClusterOrderUuid: object.GetId(),
				})
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(kubeList.Items).To(HaveLen(1))
				kubeObject = &kubeList.Items[0]
			},
			time.Minute,
			time.Second,
		).Should(Succeed())

		// Check that the Kubernetes object has the correct data:
		Expect(kubeObject).ToNot(BeNil())
		Expect(kubeObject.GetNamespace()).To(Equal(hubNamespace))
		Expect(kubeObject.GetLabels()).To(HaveKeyWithValue(labels.ClusterOrderUuid, object.GetId()))
	})

	It("Deletes the Kubernetes object when a cluster is deleted", func() {
		// Create the cluster
		createResponse, err := clustersClient.Create(ctx, ffv1.ClustersCreateRequest_builder{
			Object: ffv1.Cluster_builder{
				Spec: ffv1.ClusterSpec_builder{
					Template: templateId,
				}.Build(),
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		object := createResponse.GetObject()

		// Wait for the corresponding Kubernetes object to be created:
		kubeClient := kind.Client()
		clusterOrderList := &unstructured.UnstructuredList{}
		clusterOrderList.SetGroupVersionKind(gvks.ClusterOrderList)
		var clusterOrderObj *unstructured.Unstructured
		Eventually(
			func(g Gomega) {
				err := kubeClient.List(ctx, clusterOrderList, crclient.MatchingLabels{
					labels.ClusterOrderUuid: object.GetId(),
				})
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(clusterOrderList.Items).To(HaveLen(1))
				clusterOrderObj = &clusterOrderList.Items[0]
			},
			time.Minute,
			time.Second,
		).Should(Succeed())

		// Delete the cluster:
		_, err = clustersClient.Delete(ctx, ffv1.ClustersDeleteRequest_builder{
			Id: object.GetId(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Verify that the corresponding Kubernetes object is eventually deleted:
		clusterOrderKey := crclient.ObjectKey{
			Namespace: clusterOrderObj.GetNamespace(),
			Name:      clusterOrderObj.GetName(),
		}
		Eventually(
			func(g Gomega) {
				err := kubeClient.Get(ctx, clusterOrderKey, clusterOrderObj)
				if err != nil {
					g.Expect(kubeerrors.IsNotFound(err)).To(BeTrue())
				} else {
					g.Expect(clusterOrderObj.GetDeletionTimestamp()).ToNot(BeNil())
				}
			},
			time.Minute,
			time.Second,
		).Should(Succeed())
	})

	It("Updates the Kubernetes object when a cluster node set size is changed", func() {
		// Create the cluster with initial node set size:
		createResponse, err := clustersClient.Create(ctx, ffv1.ClustersCreateRequest_builder{
			Object: ffv1.Cluster_builder{
				Spec: ffv1.ClusterSpec_builder{
					Template: templateId,
					NodeSets: map[string]*ffv1.ClusterNodeSet{
						"my_node_set": ffv1.ClusterNodeSet_builder{
							HostClass: "my_host_class",
							Size:      3,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		object := createResponse.GetObject()
		DeferCleanup(func() {
			_, err := clustersClient.Delete(ctx, ffv1.ClustersDeleteRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
		})

		// Wait for the corresponding Kubernetes object to be created:
		kubeClient := kind.Client()
		clusterOrderList := &unstructured.UnstructuredList{}
		clusterOrderList.SetGroupVersionKind(gvks.ClusterOrderList)
		var clusterOrderObj *unstructured.Unstructured
		Eventually(
			func(g Gomega) {
				err := kubeClient.List(ctx, clusterOrderList, crclient.MatchingLabels{
					labels.ClusterOrderUuid: object.GetId(),
				})
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(clusterOrderList.Items).To(HaveLen(1))
				clusterOrderObj = &clusterOrderList.Items[0]
			},
			time.Minute,
			time.Second,
		).Should(Succeed())

		// Verify the initial node set size in the Kubernetes object:
		nodeRequests, found, err := unstructured.NestedSlice(clusterOrderObj.Object, "spec", "nodeRequests")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(nodeRequests).To(HaveLen(1))
		nodeRequest := nodeRequests[0].(map[string]any)
		Expect(nodeRequest["resourceClass"]).To(Equal("my_host_class"))
		Expect(nodeRequest["numberOfNodes"]).To(BeNumerically("==", 3))

		// Update the cluster to change the node set size
		_, err = clustersClient.Update(ctx, ffv1.ClustersUpdateRequest_builder{
			Object: ffv1.Cluster_builder{
				Id: object.GetId(),
				Spec: ffv1.ClusterSpec_builder{
					Template: templateId,
					NodeSets: map[string]*ffv1.ClusterNodeSet{
						"my_node_set": ffv1.ClusterNodeSet_builder{
							HostClass: "my_host_class",
							Size:      5,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Verify that the ClusterOrder is updated to reflect the new size
		clusterOrderKey := crclient.ObjectKey{
			Namespace: clusterOrderObj.GetNamespace(),
			Name:      clusterOrderObj.GetName(),
		}
		Eventually(
			func(g Gomega) {
				err := kubeClient.Get(ctx, clusterOrderKey, clusterOrderObj)
				g.Expect(err).ToNot(HaveOccurred())
				nodeRequests, found, err := unstructured.NestedSlice(clusterOrderObj.Object, "spec", "nodeRequests")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(nodeRequests).To(HaveLen(1))
				nodeRequest := nodeRequests[0].(map[string]any)
				g.Expect(nodeRequest["resourceClass"]).To(Equal("my_host_class"))
				g.Expect(nodeRequest["numberOfNodes"]).To(BeNumerically("==", 5))
			},
			time.Minute,
			time.Second,
		).Should(Succeed())
	})
})
