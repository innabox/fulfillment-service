/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package dao

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/wrapperspb"

	api "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/innabox/fulfillment-service/internal/database"
)

var _ = Describe("Generic DAO", func() {
	const (
		defaultLimit = 5
		maxLimit     = 10
		objectCount  = maxLimit + 1
	)

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
	})

	Describe("Creation", func() {
		It("Can be built if all the required parameters are set", func() {
			generic, err := NewGenericDAO[*api.Cluster]().
				SetLogger(logger).
				SetTable("clusters").
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(generic).ToNot(BeNil())
		})

		It("Fails if logger is not set", func() {
			generic, err := NewGenericDAO[*api.Cluster]().
				SetTable("clusters").
				Build()
			Expect(err).To(MatchError("logger is mandatory"))
			Expect(generic).To(BeNil())
		})

		It("Fails if table is not set", func() {
			generic, err := NewGenericDAO[*api.Cluster]().
				SetLogger(logger).
				Build()
			Expect(err).To(MatchError("table is mandatory"))
			Expect(generic).To(BeNil())
		})

		It("Fails if object doesn't have identifier", func() {
			generic, err := NewGenericDAO[*wrapperspb.Int32Value]().
				SetLogger(logger).
				SetTable("integers").
				Build()
			Expect(err).To(MatchError(
				"object of type '*wrapperspb.Int32Value' doesn't have an identifier field",
			))
			Expect(generic).To(BeNil())
		})

		It("Fails if default limit is zero", func() {
			generic, err := NewGenericDAO[*api.Cluster]().
				SetLogger(logger).
				SetTable("clusters").
				SetDefaultLimit(0).
				Build()
			Expect(err).To(MatchError("default limit must be a possitive integer, but it is 0"))
			Expect(generic).To(BeNil())
		})

		It("Fails if default limit is negative", func() {
			generic, err := NewGenericDAO[*api.Cluster]().
				SetLogger(logger).
				SetTable("clusters").
				SetDefaultLimit(-1).
				Build()
			Expect(err).To(MatchError("default limit must be a possitive integer, but it is -1"))
			Expect(generic).To(BeNil())
		})

		It("Fails if max limit is zero", func() {
			generic, err := NewGenericDAO[*api.Cluster]().
				SetLogger(logger).
				SetTable("clusters").
				SetMaxLimit(0).
				Build()
			Expect(err).To(MatchError("max limit must be a possitive integer, but it is 0"))
			Expect(generic).To(BeNil())
		})

		It("Fails if max limit is negative", func() {
			generic, err := NewGenericDAO[*api.Cluster]().
				SetLogger(logger).
				SetTable("clusters").
				SetMaxLimit(-1).
				Build()
			Expect(err).To(MatchError("max limit must be a possitive integer, but it is -1"))
			Expect(generic).To(BeNil())
		})

		It("Fails if max limit is less than default limit", func() {
			generic, err := NewGenericDAO[*api.Cluster]().
				SetLogger(logger).
				SetTable("clusters").
				SetMaxLimit(100).
				SetDefaultLimit(1000).
				Build()
			Expect(err).To(MatchError(
				"max limit must be greater or equal to default limit, but max limit is 100 and " +
					"default limit is 1000",
			))
			Expect(generic).To(BeNil())
		})
	})

	Describe("Behaviour", func() {
		var generic *GenericDAO[*api.Cluster]

		BeforeEach(func() {
			// Create the table:
			_, err := tx.Exec(
				ctx,
				`
				create table clusters (
					id uuid not null primary key,
					data jsonb not null
				)
				`,
			)
			Expect(err).ToNot(HaveOccurred())

			// Create the DAO:
			generic, err = NewGenericDAO[*api.Cluster]().
				SetLogger(logger).
				SetTable("clusters").
				SetDefaultOrder("id").
				SetDefaultLimit(defaultLimit).
				SetMaxLimit(maxLimit).
				Build()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Inserts object", func() {
			// Insert the object:
			object := &api.Cluster{}
			id, err := generic.Insert(ctx, object)
			Expect(err).ToNot(HaveOccurred())

			// Check the database:
			row := tx.QueryRow(ctx, `select data from clusters where id = $1`, id)
			var data []byte
			err = row.Scan(&data)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).ToNot(BeNil())
		})

		It("Generates non empty identifiers", func() {
			object := &api.Cluster{}
			id, err := generic.Insert(ctx, object)
			Expect(err).ToNot(HaveOccurred())
			Expect(id).ToNot(BeEmpty())
		})

		It("Puts the generated identifier inside the object", func() {
			object := &api.Cluster{}
			id, err := generic.Insert(ctx, object)
			Expect(err).ToNot(HaveOccurred())
			Expect(object.Id).To(Equal(id))
		})

		It("Gets object", func() {
			// Insert the row:
			id, err := generic.Insert(ctx, &api.Cluster{})
			Expect(err).ToNot(HaveOccurred())

			// Try to get it:
			object, err := generic.Get(ctx, id)
			Expect(err).ToNot(HaveOccurred())
			Expect(object).ToNot(BeNil())
		})

		It("Lists objects", func() {
			// Insert a couple of rows:
			const count = 2
			for range count {
				_, err := generic.Insert(ctx, &api.Cluster{})
				Expect(err).ToNot(HaveOccurred())
			}

			// Try to list:
			request, err := generic.List(ctx, ListRequest{})
			Expect(err).ToNot(HaveOccurred())
			Expect(request.Items).To(HaveLen(count))
			for _, item := range request.Items {
				Expect(item).ToNot(BeNil())
			}
		})

		Describe("Paging", func() {
			var objects []*api.Cluster

			BeforeEach(func() {
				// Create a list of objects and sort it like they will be sorted by the DAO. Not that
				// this works correctly because the DAO is configured with a default sorting. That is
				// intended for use only in these unit tests.
				objects = make([]*api.Cluster, objectCount)
				for i := range len(objects) {
					objects[i] = &api.Cluster{}
					_, err := generic.Insert(ctx, objects[i])
					Expect(err).ToNot(HaveOccurred())
				}
				sort.Slice(objects, func(i, j int) bool {
					return strings.Compare(objects[i].Id, objects[j].Id) < 0
				})
			})

			It("Uses zero as default offset", func() {
				response, err := generic.List(ctx, ListRequest{
					Limit: 1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.Items[0].Id).To(Equal(objects[0].Id))
			})

			It("Honours valid offset", func() {
				for i := range len(objects) {
					response, err := generic.List(ctx, ListRequest{
						Offset: int32(i),
						Limit:  1,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(response.Items[0].Id).To(Equal(objects[i].Id))
				}
			})

			It("Returns empty list if offset is greater or equal than available items", func() {
				response, err := generic.List(ctx, ListRequest{
					Offset: objectCount,
					Limit:  1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.Items).To(BeEmpty())
			})

			It("Ignores negative offset", func() {
				response, err := generic.List(ctx, ListRequest{
					Offset: -123,
					Limit:  1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.Items[0].Id).To(Equal(objects[0].Id))
			})

			It("Interprets negative limit as requesting zero items", func() {
				response, err := generic.List(ctx, ListRequest{
					Limit: -123,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.Size).To(BeZero())
				Expect(response.Items).To(BeEmpty())
			})

			It("Interprets zero limit as requesting the default number of items", func() {
				response, err := generic.List(ctx, ListRequest{
					Limit: 0,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.Size).To(BeNumerically("==", defaultLimit))
				Expect(response.Items).To(HaveLen(defaultLimit))
			})

			It("Truncates limit to the maximum", func() {
				response, err := generic.List(ctx, ListRequest{
					Limit: maxLimit + 1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.Size).To(BeNumerically("==", maxLimit))
				Expect(response.Items).To(HaveLen(maxLimit))
			})

			It("Honours valid limit", func() {
				for i := 1; i < maxLimit; i++ {
					response, err := generic.List(ctx, ListRequest{
						Limit: int32(i),
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(response.Size).To(BeNumerically("==", i))
					Expect(response.Items).To(HaveLen(i))
				}
			})

			It("Returns less items than requested if there are not enough", func() {
				response, err := generic.List(ctx, ListRequest{
					Offset: objectCount - 2,
					Limit:  10,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.Size).To(BeNumerically("==", 2))
				Expect(response.Items).To(HaveLen(2))
			})

			It("Returns the total number of items", func() {
				response, err := generic.List(ctx, ListRequest{
					Limit: 1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.Total).To(BeNumerically("==", objectCount))
			})
		})

		Describe("Check if object exists", func() {
			It("Returns true if the object exists", func() {
				// Insert the object:
				object := &api.Cluster{}
				id, err := generic.Insert(ctx, object)
				Expect(err).ToNot(HaveOccurred())

				// Check if it exists:
				exists, err := generic.Exists(ctx, id)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
			})

			It("Returns false if the object doesn't exist", func() {
				// The database is empty, check the result:
				exists, err := generic.Exists(ctx, uuid.NewString())
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeFalse())
			})
		})

		It("Updates object", func() {
			// Create the object:
			object := &api.Cluster{
				Status: &api.ClusterStatus{
					ApiUrl: "my_url",
				},
			}
			id, err := generic.Insert(ctx, object)
			Expect(err).ToNot(HaveOccurred())

			// Try to update:
			object.Status.ApiUrl = "your_url"
			err = generic.Update(ctx, id, object)
			Expect(err).ToNot(HaveOccurred())

			// Get it and verify that the changes have been applied:
			object, err = generic.Get(ctx, id)
			Expect(err).ToNot(HaveOccurred())
			Expect(object.Status.ApiUrl).To(Equal("your_url"))
		})
	})
})
