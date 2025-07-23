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
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
)

var _ = Describe("Private clusters server", func() {
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

		// Create the tables:
		_, err = tx.Exec(
			ctx,
			`
			create table clusters (
				id text not null primary key,
				creation_timestamp timestamp with time zone not null default now(),
				deletion_timestamp timestamp with time zone not null default 'epoch',
				finalizers text[] not null default '{}',
				data jsonb not null
			);

			create table archived_clusters (
				id text not null,
				creation_timestamp timestamp with time zone not null,
				deletion_timestamp timestamp with time zone not null,
				archival_timestamp timestamp with time zone not null default now(),
				data jsonb not null
			);

			create table cluster_templates (
				id text not null primary key,
				creation_timestamp timestamp with time zone not null default now(),
				deletion_timestamp timestamp with time zone not null default 'epoch',
				finalizers text[] not null default '{}',
				data jsonb not null
			);

			create table archived_cluster_templates (
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
			server, err := NewPrivateClustersServer().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(server).ToNot(BeNil())
		})

		It("Fails if logger is not set", func() {
			server, err := NewPrivateClustersServer().
				Build()
			Expect(err).To(MatchError("logger is mandatory"))
			Expect(server).To(BeNil())
		})
	})

	Describe("Behaviour", func() {
		var server *PrivateClustersServer

		BeforeEach(func() {
			var err error

			// Create the server:
			server, err = NewPrivateClustersServer().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())

			// Create the templates DAO:
			templatesDao, err := dao.NewGenericDAO[*privatev1.ClusterTemplate]().
				SetLogger(logger).
				SetTable("cluster_templates").
				Build()
			Expect(err).ToNot(HaveOccurred())

			// Create a usable template:
			_, err = templatesDao.Create(ctx, privatev1.ClusterTemplate_builder{
				Id:          "my_template",
				Title:       "My template",
				Description: "My template",
				NodeSets: map[string]*privatev1.ClusterTemplateNodeSet{
					"compute": privatev1.ClusterTemplateNodeSet_builder{
						HostClass: "acme_1tib",
						Size:      3,
					}.Build(),
					"gpu": privatev1.ClusterTemplateNodeSet_builder{
						HostClass: "acme_gpu",
						Size:      1,
					}.Build(),
				},
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Create numbered templates for list tests:
			for i := 0; i < 10; i++ {
				_, err = templatesDao.Create(ctx, privatev1.ClusterTemplate_builder{
					Id:          fmt.Sprintf("my_template_%d", i),
					Title:       fmt.Sprintf("My template %d", i),
					Description: fmt.Sprintf("My template %d", i),
					NodeSets: map[string]*privatev1.ClusterTemplateNodeSet{
						"compute": privatev1.ClusterTemplateNodeSet_builder{
							HostClass: "acme_1tib",
							Size:      3,
						}.Build(),
					},
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("Creates object", func() {
			response, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
				Object: privatev1.Cluster_builder{
					Spec: privatev1.ClusterSpec_builder{
						Template: "my_template",
					}.Build(),
					Status: privatev1.ClusterStatus_builder{
						Hub: "my_hub",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			object := response.GetObject()
			Expect(object).ToNot(BeNil())
			Expect(object.GetId()).ToNot(BeEmpty())
		})

		It("List objects", func() {
			// Create a few objects:
			const count = 10
			for i := range count {
				_, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
					Object: privatev1.Cluster_builder{
						Spec: privatev1.ClusterSpec_builder{
							Template: fmt.Sprintf("my_template_%d", i),
						}.Build(),
						Status: privatev1.ClusterStatus_builder{
							Hub: fmt.Sprintf("my_hub_%d", i),
						}.Build(),
					}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}

			// List the objects:
			response, err := server.List(ctx, privatev1.ClustersListRequest_builder{}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			items := response.GetItems()
			Expect(items).To(HaveLen(count))
		})

		It("List objects with limit", func() {
			// Create a few objects:
			const count = 10
			for i := range count {
				_, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
					Object: privatev1.Cluster_builder{
						Spec: privatev1.ClusterSpec_builder{
							Template: fmt.Sprintf("my_template_%d", i),
						}.Build(),
						Status: privatev1.ClusterStatus_builder{
							Hub: fmt.Sprintf("my_hub_%d", i),
						}.Build(),
					}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}

			// List the objects:
			response, err := server.List(ctx, privatev1.ClustersListRequest_builder{
				Limit: proto.Int32(1),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response.GetSize()).To(BeNumerically("==", 1))
		})

		It("List objects with offset", func() {
			// Create a few objects:
			const count = 10
			for i := range count {
				_, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
					Object: privatev1.Cluster_builder{
						Spec: privatev1.ClusterSpec_builder{
							Template: fmt.Sprintf("my_template_%d", i),
						}.Build(),
						Status: privatev1.ClusterStatus_builder{
							Hub: fmt.Sprintf("my_hub_%d", i),
						}.Build(),
					}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}

			// List the objects:
			response, err := server.List(ctx, privatev1.ClustersListRequest_builder{
				Offset: proto.Int32(1),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response.GetSize()).To(BeNumerically("==", count-1))
		})

		It("List objects with filter", func() {
			// Create a few objects:
			const count = 10
			var objects []*privatev1.Cluster
			for i := range count {
				response, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
					Object: privatev1.Cluster_builder{
						Spec: privatev1.ClusterSpec_builder{
							Template: fmt.Sprintf("my_template_%d", i),
						}.Build(),
						Status: privatev1.ClusterStatus_builder{
							Hub: fmt.Sprintf("my_hub_%d", i),
						}.Build(),
					}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
				objects = append(objects, response.GetObject())
			}

			// List the objects:
			for _, object := range objects {
				response, err := server.List(ctx, privatev1.ClustersListRequest_builder{
					Filter: proto.String(fmt.Sprintf("this.id == '%s'", object.GetId())),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
				Expect(response.GetSize()).To(BeNumerically("==", 1))
				Expect(response.GetItems()[0].GetId()).To(Equal(object.GetId()))
			}
		})

		It("Get object", func() {
			// Create the object:
			createResponse, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
				Object: privatev1.Cluster_builder{
					Spec: privatev1.ClusterSpec_builder{
						Template: "my_template",
					}.Build(),
					Status: privatev1.ClusterStatus_builder{
						Hub: "my_hub",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Get it:
			getResponse, err := server.Get(ctx, privatev1.ClustersGetRequest_builder{
				Id: createResponse.GetObject().GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(proto.Equal(createResponse.GetObject(), getResponse.GetObject())).To(BeTrue())
		})

		It("Update object", func() {
			// Create the object:
			createResponse, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
				Object: privatev1.Cluster_builder{
					Spec: privatev1.ClusterSpec_builder{
						Template: "my_template",
					}.Build(),
					Status: privatev1.ClusterStatus_builder{
						Hub: "my_hub",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object := createResponse.GetObject()

			// Update the object:
			updateResponse, err := server.Update(ctx, privatev1.ClustersUpdateRequest_builder{
				Object: privatev1.Cluster_builder{
					Id: object.GetId(),
					Spec: privatev1.ClusterSpec_builder{
						Template: "your_template",
					}.Build(),
					Status: privatev1.ClusterStatus_builder{
						Hub: "your_hub",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(updateResponse.GetObject().GetSpec().GetTemplate()).To(Equal("your_template"))
			Expect(updateResponse.GetObject().GetStatus().GetHub()).To(Equal("your_hub"))

			// Get and verify:
			getResponse, err := server.Get(ctx, privatev1.ClustersGetRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(getResponse.GetObject().GetSpec().GetTemplate()).To(Equal("your_template"))
			Expect(getResponse.GetObject().GetStatus().GetHub()).To(Equal("your_hub"))
		})

		It("Delete object", func() {
			// Create the object:
			createResponse, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
				Object: privatev1.Cluster_builder{
					Metadata: privatev1.Metadata_builder{
						Finalizers: []string{"a"},
					}.Build(),
					Spec: privatev1.ClusterSpec_builder{
						Template: "my_template",
					}.Build(),
					Status: privatev1.ClusterStatus_builder{
						Hub: "my_hub",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object := createResponse.GetObject()

			// Delete the object:
			_, err = server.Delete(ctx, privatev1.ClustersDeleteRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Get and verify:
			getResponse, err := server.Get(ctx, privatev1.ClustersGetRequest_builder{
				Id: object.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object = getResponse.GetObject()
			Expect(object.GetMetadata().GetDeletionTimestamp()).ToNot(BeNil())
		})

		It("Rejects creation with duplicate condition", func() {
			_, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
				Object: privatev1.Cluster_builder{
					Spec: privatev1.ClusterSpec_builder{
						Template: "my_template",
					}.Build(),
					Status: privatev1.ClusterStatus_builder{
						Conditions: []*privatev1.ClusterCondition{
							privatev1.ClusterCondition_builder{
								Type: privatev1.ClusterConditionType_CLUSTER_CONDITION_TYPE_READY,
							}.Build(),
							privatev1.ClusterCondition_builder{
								Type: privatev1.ClusterConditionType_CLUSTER_CONDITION_TYPE_READY,
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.InvalidArgument))
			Expect(status.Message()).To(Equal("condition 'CLUSTER_CONDITION_TYPE_READY' is duplicated"))
		})

		It("Rejects update with duplicate condition", func() {
			_, err := server.Create(ctx, privatev1.ClustersCreateRequest_builder{
				Object: privatev1.Cluster_builder{
					Spec: privatev1.ClusterSpec_builder{
						Template: "my_template",
					}.Build(),
					Status: privatev1.ClusterStatus_builder{
						Conditions: []*privatev1.ClusterCondition{
							privatev1.ClusterCondition_builder{
								Type: privatev1.ClusterConditionType_CLUSTER_CONDITION_TYPE_READY,
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			_, err = server.Update(ctx, privatev1.ClustersUpdateRequest_builder{
				Object: privatev1.Cluster_builder{
					Status: privatev1.ClusterStatus_builder{
						Conditions: []*privatev1.ClusterCondition{
							privatev1.ClusterCondition_builder{
								Type: privatev1.ClusterConditionType_CLUSTER_CONDITION_TYPE_READY,
							}.Build(),
							privatev1.ClusterCondition_builder{
								Type: privatev1.ClusterConditionType_CLUSTER_CONDITION_TYPE_READY,
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.InvalidArgument))
			Expect(status.Message()).To(Equal("condition 'CLUSTER_CONDITION_TYPE_READY' is duplicated"))
		})
	})
})
