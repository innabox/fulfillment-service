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

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
)

var _ = Describe("Priate host classes", func() {
	var (
		ctx    context.Context
		client privatev1.HostClassesClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = privatev1.NewHostClassesClient(adminConn)
	})

	It("Can get the list of host classes", func() {
		// Create the host class:
		id := fmt.Sprintf("my_host_class_%s", uuid.NewString())
		_, err := client.Create(ctx, privatev1.HostClassesCreateRequest_builder{
			Object: privatev1.HostClass_builder{
				Id:          id,
				Title:       "My title",
				Description: "My description.",
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Get the list:
		listResponse, err := client.List(ctx, privatev1.HostClassesListRequest_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(listResponse).ToNot(BeNil())
		items := listResponse.GetItems()
		Expect(items).ToNot(BeEmpty())
	})

	It("Can get a specific host class", func() {
		// Create the host class:
		id := fmt.Sprintf("my_host_class_%s", uuid.NewString())
		_, err := client.Create(ctx, privatev1.HostClassesCreateRequest_builder{
			Object: privatev1.HostClass_builder{
				Id:          id,
				Title:       "My title",
				Description: "My description.",
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Get the host class and verify that the returned object is correct:
		response, err := client.Get(ctx, privatev1.HostClassesGetRequest_builder{
			Id: id,
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		object := response.GetObject()
		Expect(object).ToNot(BeNil())
		Expect(object.GetId()).To(Equal(id))
		metadata := object.GetMetadata()
		Expect(metadata).ToNot(BeNil())
		Expect(metadata.HasCreationTimestamp()).To(BeTrue())
		Expect(metadata.HasDeletionTimestamp()).To(BeFalse())
		Expect(object.GetTitle()).To(Equal("My title"))
		Expect(object.GetDescription()).To(Equal("My description."))
	})

	It("Can create a host class", func() {
		id := fmt.Sprintf("my_template_%s", uuid.NewString())
		response, err := client.Create(ctx, privatev1.HostClassesCreateRequest_builder{
			Object: privatev1.HostClass_builder{
				Id:          id,
				Title:       "My title",
				Description: "My description.",
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		object := response.GetObject()
		Expect(object).ToNot(BeNil())
		Expect(object.GetId()).To(Equal(id))
		metadata := object.GetMetadata()
		Expect(metadata).ToNot(BeNil())
		Expect(metadata.HasCreationTimestamp()).To(BeTrue())
		Expect(metadata.HasDeletionTimestamp()).To(BeFalse())
		Expect(object.GetTitle()).To(Equal("My title"))
		Expect(object.GetDescription()).To(Equal("My description."))
	})

	It("Can update a host class", func() {
		// Create a host class::
		id := fmt.Sprintf("my_host_class_%s", uuid.NewString())
		_, err := client.Create(ctx, privatev1.HostClassesCreateRequest_builder{
			Object: privatev1.HostClass_builder{
				Id:          id,
				Title:       "My title",
				Description: "My description.",
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Update it and verify that the returned object is correct:
		updateResponse, err := client.Update(ctx, privatev1.HostClassesUpdateRequest_builder{
			Object: privatev1.HostClass_builder{
				Id:          id,
				Title:       "My updated title",
				Description: "My updated description.",
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(updateResponse).ToNot(BeNil())
		object := updateResponse.GetObject()
		Expect(object).ToNot(BeNil())
		Expect(object.GetId()).To(Equal(id))
		metadata := object.GetMetadata()
		Expect(metadata).ToNot(BeNil())
		Expect(metadata.HasCreationTimestamp()).To(BeTrue())
		Expect(metadata.HasDeletionTimestamp()).To(BeFalse())
		Expect(object.GetTitle()).To(Equal("My updated title"))
		Expect(object.GetDescription()).To(Equal("My updated description."))

		// Get the host class and verify that the returned object is correct:
		getResponse, err := client.Get(ctx, privatev1.HostClassesGetRequest_builder{
			Id: id,
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(getResponse).ToNot(BeNil())
		object = getResponse.GetObject()
		Expect(object).ToNot(BeNil())
		Expect(object.GetId()).To(Equal(id))
		metadata = object.GetMetadata()
		Expect(metadata).ToNot(BeNil())
		Expect(metadata.HasCreationTimestamp()).To(BeTrue())
		Expect(metadata.HasDeletionTimestamp()).To(BeFalse())
		Expect(object.GetTitle()).To(Equal("My updated title"))
		Expect(object.GetDescription()).To(Equal("My updated description."))
	})

	It("Can delete a host class", func() {
		// Create a host class::
		id := fmt.Sprintf("my_host_class_%s", uuid.NewString())
		_, err := client.Create(ctx, privatev1.HostClassesCreateRequest_builder{
			Object: privatev1.HostClass_builder{
				Metadata: privatev1.Metadata_builder{
					Finalizers: []string{"a"},
				}.Build(),
				Id:          id,
				Title:       "My title",
				Description: "My description.",
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Delete it:
		deleteResponse, err := client.Delete(ctx, privatev1.HostClassesDeleteRequest_builder{
			Id: id,
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(deleteResponse).ToNot(BeNil())

		// Verify that the object no longer exists, or that it has the deletion timestamp:
		getResponse, err := client.Get(ctx, privatev1.HostClassesGetRequest_builder{
			Id: id,
		}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(getResponse).ToNot(BeNil())
		object := getResponse.GetObject()
		Expect(object).ToNot(BeNil())
		metadata := object.GetMetadata()
		Expect(metadata).ToNot(BeNil())
		Expect(metadata.HasDeletionTimestamp()).To(BeTrue())
	})
})
