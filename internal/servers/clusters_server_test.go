/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package servers

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
)

var _ = Describe("Clusters server", func() {
	var (
		ctx context.Context
		tx  database.Tx
	)

	BeforeEach(func() {
		var err error

		// Create a context:
		ctx = context.Background()

		// Prepare the database pool:
		db := server.MakeDatabase()
		DeferCleanup(db.Close)
		pool, err := pgxpool.New(ctx, db.MakeURL())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(pool.Close)

		// Create the transaction manager:
		tm, err := database.NewTxManager().
			SetLogger(logger).
			SetPool(pool).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Start a transaction and add it to the context:
		tx, err = tm.Begin(ctx)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			err := tm.End(ctx, tx)
			Expect(err).ToNot(HaveOccurred())
		})
		ctx = database.TxIntoContext(ctx, tx)

		// Create the table:
		_, err = tx.Exec(
			ctx,
			`
			create table clusters (
				id text not null primary key,
				creation_timestamp timestamp with time zone not null default now(),
				deletion_timestamp timestamp with time zone not null default 'epoch',
				finalizers text[] not null default array ['default'],
				data jsonb not null
			);

			create table archived_clusters (
				id text not null,
				creation_timestamp timestamp with time zone not null,
				deletion_timestamp timestamp with time zone not null,
				archival_timestamp timestamp with time zone not null default now(),
				data jsonb not null
			);
			`,
		)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Creation", func() {
		It("Can be built if all the required parameters are set", func() {
			server, err := NewClustersServer().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(server).ToNot(BeNil())
		})

		It("Fails if logger is not set", func() {
			server, err := NewClustersServer().
				Build()
			Expect(err).To(MatchError("logger is mandatory"))
			Expect(server).To(BeNil())
		})
	})

	Describe("Behaviour", func() {
		var server *ClustersServer

		BeforeEach(func() {
			var err error

			// Create the server:
			server, err = NewClustersServer().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Creates object", func() {
			response, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
				Object: ffv1.Cluster_builder{}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			object := response.GetObject()
			Expect(object).ToNot(BeNil())
			Expect(object.GetId()).ToNot(BeEmpty())
		})

		It("Ignores status when object is created", func() {
			// Create the object:
			createResponse, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
				Object: ffv1.Cluster_builder{
					Status: ffv1.ClusterStatus_builder{
						ApiUrl:     "https://my.api.com",
						ConsoleUrl: "https://my.console.com",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object := createResponse.GetObject()
			Expect(object.GetStatus().GetApiUrl()).To(BeEmpty())
			Expect(object.GetStatus().GetConsoleUrl()).To(BeEmpty())

			// Get the object and verify that the change to the status hasn't been applied:
			getResponse, err := server.Get(ctx, ffv1.ClustersGetRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object = getResponse.GetObject()
			Expect(object.GetStatus().GetApiUrl()).To(BeEmpty())
			Expect(object.GetStatus().GetConsoleUrl()).To(BeEmpty())
		})

		It("List objects", func() {
			// Create a few objects:
			const count = 10
			for range count {
				_, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
					Object: ffv1.Cluster_builder{}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}

			// List the objects:
			response, err := server.List(ctx, ffv1.ClustersListRequest_builder{}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			items := response.GetItems()
			Expect(items).To(HaveLen(count))
		})

		It("List objects with limit", func() {
			// Create a few objects:
			const count = 10
			for range count {
				_, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
					Object: ffv1.Cluster_builder{}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}

			// List the objects:
			response, err := server.List(ctx, ffv1.ClustersListRequest_builder{
				Limit: proto.Int32(1),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response.GetSize()).To(BeNumerically("==", 1))
		})

		It("List objects with offset", func() {
			// Create a few objects:
			const count = 10
			for range count {
				_, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
					Object: ffv1.Cluster_builder{}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}

			// List the objects:
			response, err := server.List(ctx, ffv1.ClustersListRequest_builder{
				Offset: proto.Int32(1),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response.GetSize()).To(BeNumerically("==", count-1))
		})

		It("List objects with filter", func() {
			// Create a few objects:
			const count = 10
			var objects []*ffv1.Cluster
			for range count {
				response, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
					Object: ffv1.Cluster_builder{}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
				objects = append(objects, response.GetObject())
			}

			// List the objects:
			for _, object := range objects {
				response, err := server.List(ctx, ffv1.ClustersListRequest_builder{
					Filter: proto.String(fmt.Sprintf("this.id == '%s'", object.GetId())),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
				Expect(response.GetSize()).To(BeNumerically("==", 1))
				Expect(response.GetItems()[0].GetId()).To(Equal(object.GetId()))
			}
		})

		It("Get object", func() {
			// Create the object:
			createResponse, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
				Object: ffv1.Cluster_builder{}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Get it:
			getResponse, err := server.Get(ctx, ffv1.ClustersGetRequest_builder{
				Id: createResponse.GetObject().GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(proto.Equal(createResponse.GetObject(), getResponse.GetObject())).To(BeTrue())
		})

		It("Update object", func() {
			// Create the object:
			createResponse, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
				Object: ffv1.Cluster_builder{
					Spec: ffv1.ClusterSpec_builder{
						NodeSets: map[string]*ffv1.ClusterNodeSet{
							"compute": ffv1.ClusterNodeSet_builder{
								HostClass: "acme_1tib",
								Size:      3,
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object := createResponse.GetObject()

			// Update the object:
			updateResponse, err := server.Update(ctx, ffv1.ClustersUpdateRequest_builder{
				Object: ffv1.Cluster_builder{
					Id: object.GetId(),
					Spec: ffv1.ClusterSpec_builder{
						NodeSets: map[string]*ffv1.ClusterNodeSet{
							"compute": ffv1.ClusterNodeSet_builder{
								HostClass: "acme_1tib",
								Size:      4,
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object = updateResponse.GetObject()
			nodeSet := object.GetSpec().GetNodeSets()["compute"]
			Expect(nodeSet.GetHostClass()).To(Equal("acme_1tib"))
			Expect(nodeSet.GetSize()).To(BeNumerically("==", 4))

			// Get and verify:
			getResponse, err := server.Get(ctx, ffv1.ClustersGetRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object = getResponse.GetObject()
			nodeSet = object.GetSpec().GetNodeSets()["compute"]
			Expect(nodeSet.GetHostClass()).To(Equal("acme_1tib"))
			Expect(nodeSet.GetSize()).To(BeNumerically("==", 4))
		})

		It("Ignores changes to the status when an object is updated", func() {
			// Create the object:
			createResponse, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
				Object: ffv1.Cluster_builder{}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object := createResponse.GetObject()

			// Try to update the status:
			updateResponse, err := server.Update(ctx, ffv1.ClustersUpdateRequest_builder{
				Object: ffv1.Cluster_builder{
					Id: object.GetId(),
					Status: ffv1.ClusterStatus_builder{
						ApiUrl:     "https://my.api.com",
						ConsoleUrl: "https://my.console.com",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object = updateResponse.GetObject()
			Expect(object.GetStatus().GetApiUrl()).To(BeEmpty())
			Expect(object.GetStatus().GetConsoleUrl()).To(BeEmpty())

			// Get the response and verify that the status hasn't been updated:
			getResponse, err := server.Get(ctx, ffv1.ClustersGetRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object = getResponse.GetObject()
			Expect(object.GetStatus().GetApiUrl()).To(BeEmpty())
			Expect(object.GetStatus().GetConsoleUrl()).To(BeEmpty())
		})

		It("Delete object", func() {
			// Create the object:
			createResponse, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
				Object: ffv1.Cluster_builder{}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object := createResponse.GetObject()

			// Add a finalizer, as otherwise the object will be immediatelly deleted and archived and it
			// won't be possible to verify the deletion timestamp. This can't be done using the server
			// because this is a public object, and public objects don't have the finalizers field.
			_, err = tx.Exec(
				ctx,
				`update clusters set finalizers = '{"a"}' where id = $1`,
				object.GetId(),
			)
			Expect(err).ToNot(HaveOccurred())

			// Delete the object:
			_, err = server.Delete(ctx, ffv1.ClustersDeleteRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Get and verify:
			getResponse, err := server.Get(ctx, ffv1.ClustersGetRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object = getResponse.GetObject()
			Expect(object.GetMetadata().GetDeletionTimestamp()).ToNot(BeNil())
		})

		It("Preserves private data during update", func() {
			// Use the DAO directly to create an object with private data:
			dao, err := dao.NewGenericDAO[*privatev1.Cluster]().
				SetLogger(logger).
				SetTable("clusters").
				Build()
			Expect(err).ToNot(HaveOccurred())
			object, err := dao.Create(ctx, privatev1.Cluster_builder{
				Spec: privatev1.ClusterSpec_builder{
					Template: "my_template",
					NodeSets: map[string]*privatev1.ClusterNodeSet{
						"compute": privatev1.ClusterNodeSet_builder{
							HostClass: "my_host_class",
							Size:      3,
						}.Build(),
					},
				}.Build(),
				Status: privatev1.ClusterStatus_builder{
					Hub: "123",
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Update the object using the public server:
			_, err = server.Update(ctx, ffv1.ClustersUpdateRequest_builder{
				Object: ffv1.Cluster_builder{
					Id: object.GetId(),
					Spec: ffv1.ClusterSpec_builder{
						Template: "my_template",
						NodeSets: map[string]*ffv1.ClusterNodeSet{
							"compute": ffv1.ClusterNodeSet_builder{
								HostClass: "my_host_class",
								Size:      4,
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Get the object again and verify that the private data hasn't changed:
			object, err = dao.Get(ctx, object.GetId())
			Expect(err).ToNot(HaveOccurred())
			Expect(object.GetStatus().GetHub()).To(Equal("123"))
		})

		It("Ignores status during creation", func() {
			// Try to create an object with status:
			createResponse, err := server.Create(ctx, ffv1.ClustersCreateRequest_builder{
				Object: ffv1.Cluster_builder{
					Spec: ffv1.ClusterSpec_builder{
						Template: "my_template",
					}.Build(),
					Status: ffv1.ClusterStatus_builder{
						ApiUrl: "https://your.api",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object := createResponse.GetObject()

			// Get the object and verify that the status was ignored:
			getResponse, err := server.Get(ctx, ffv1.ClustersGetRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object = getResponse.GetObject()
			Expect(object.GetStatus().GetApiUrl()).To(BeEmpty())
		})

		It("Ignores changes to status during update", func() {
			// Use the DAO directly to create an object with status:
			dao, err := dao.NewGenericDAO[*privatev1.Cluster]().
				SetLogger(logger).
				SetTable("clusters").
				Build()
			Expect(err).ToNot(HaveOccurred())
			object, err := dao.Create(ctx, privatev1.Cluster_builder{
				Spec: privatev1.ClusterSpec_builder{
					Template: "my_template",
				}.Build(),
				Status: privatev1.ClusterStatus_builder{
					ApiUrl: "https://my.api",
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Try to update the status:
			_, err = server.Update(ctx, ffv1.ClustersUpdateRequest_builder{
				Object: ffv1.Cluster_builder{
					Id: object.GetId(),
					Spec: ffv1.ClusterSpec_builder{
						Template: "my_template",
					}.Build(),
					Status: ffv1.ClusterStatus_builder{
						ApiUrl: "https://your.api",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Get the object again and verify that the status hasn't changed:
			object, err = dao.Get(ctx, object.GetId())
			Expect(err).ToNot(HaveOccurred())
			Expect(object.GetStatus().GetApiUrl()).To(Equal("https://my.api"))
		})
	})
})
