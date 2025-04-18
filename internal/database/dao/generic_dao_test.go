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
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	testsv1 "github.com/innabox/fulfillment-service/internal/api/tests/v1"
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

	sort := func(objects []*testsv1.Object) {
		sort.Slice(objects, func(i, j int) bool {
			return strings.Compare(objects[i].GetId(), objects[j].GetId()) < 0
		})
	}

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
			generic, err := NewGenericDAO[*testsv1.Object]().
				SetLogger(logger).
				SetTable("objects").
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(generic).ToNot(BeNil())
		})

		It("Fails if logger is not set", func() {
			generic, err := NewGenericDAO[*testsv1.Object]().
				SetTable("objects").
				Build()
			Expect(err).To(MatchError("logger is mandatory"))
			Expect(generic).To(BeNil())
		})

		It("Fails if table is not set", func() {
			generic, err := NewGenericDAO[*testsv1.Object]().
				SetLogger(logger).
				Build()
			Expect(err).To(MatchError("table is mandatory"))
			Expect(generic).To(BeNil())
		})

		It("Fails if default limit is zero", func() {
			generic, err := NewGenericDAO[*testsv1.Object]().
				SetLogger(logger).
				SetTable("objects").
				SetDefaultLimit(0).
				Build()
			Expect(err).To(MatchError("default limit must be a possitive integer, but it is 0"))
			Expect(generic).To(BeNil())
		})

		It("Fails if default limit is negative", func() {
			generic, err := NewGenericDAO[*testsv1.Object]().
				SetLogger(logger).
				SetTable("objects").
				SetDefaultLimit(-1).
				Build()
			Expect(err).To(MatchError("default limit must be a possitive integer, but it is -1"))
			Expect(generic).To(BeNil())
		})

		It("Fails if max limit is zero", func() {
			generic, err := NewGenericDAO[*testsv1.Object]().
				SetLogger(logger).
				SetTable("objects").
				SetMaxLimit(0).
				Build()
			Expect(err).To(MatchError("max limit must be a possitive integer, but it is 0"))
			Expect(generic).To(BeNil())
		})

		It("Fails if max limit is negative", func() {
			generic, err := NewGenericDAO[*testsv1.Object]().
				SetLogger(logger).
				SetTable("objects").
				SetMaxLimit(-1).
				Build()
			Expect(err).To(MatchError("max limit must be a possitive integer, but it is -1"))
			Expect(generic).To(BeNil())
		})

		It("Fails if max limit is less than default limit", func() {
			generic, err := NewGenericDAO[*testsv1.Object]().
				SetLogger(logger).
				SetTable("objects").
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
		var generic *GenericDAO[*testsv1.Object]

		BeforeEach(func() {
			// Create the table:
			_, err := tx.Exec(
				ctx,
				`
				create table objects (
					id text not null primary key,
					creation_timestamp timestamp with time zone not null default now(),
					deletion_timestamp timestamp with time zone not null default 'epoch',
					data jsonb not null
				)
				`,
			)
			Expect(err).ToNot(HaveOccurred())

			// Create the DAO:
			generic, err = NewGenericDAO[*testsv1.Object]().
				SetLogger(logger).
				SetTable("objects").
				SetDefaultOrder("id").
				SetDefaultLimit(defaultLimit).
				SetMaxLimit(maxLimit).
				Build()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Creates object", func() {
			object := &testsv1.Object{}
			created, err := generic.Create(ctx, object)
			Expect(err).ToNot(HaveOccurred())
			result, err := generic.Get(ctx, created.GetId())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
		})

		It("Sets metadata when creating", func() {
			object := &testsv1.Object{}
			result, err := generic.Create(ctx, object)
			Expect(err).ToNot(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Metadata).ToNot(BeNil())
		})

		It("Sets creation timestamp when creating", func() {
			object := &testsv1.Object{}
			result, err := generic.Create(ctx, object)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Metadata).ToNot(BeNil())
			Expect(result.Metadata.CreationTimestamp).ToNot(BeNil())
			Expect(result.Metadata.CreationTimestamp.AsTime()).ToNot(BeZero())
		})

		It("Doesn't set deletion timestamp when creating", func() {
			object, err := generic.Create(ctx, &testsv1.Object{})
			Expect(err).ToNot(HaveOccurred())
			Expect(object).ToNot(BeNil())
			Expect(object.Metadata).ToNot(BeNil())
			Expect(object.Metadata.DeletionTimestamp).To(BeNil())
		})

		It("Generates non empty identifiers", func() {
			object, err := generic.Create(ctx, &testsv1.Object{})
			Expect(err).ToNot(HaveOccurred())
			Expect(object).ToNot(BeNil())
			Expect(object.GetId()).ToNot(BeEmpty())
		})

		It("Doesn't put the generated identifier inside the input object", func() {
			object := &testsv1.Object{}
			_, err := generic.Create(ctx, object)
			Expect(err).ToNot(HaveOccurred())
			Expect(object.GetId()).To(BeEmpty())
		})

		It("Doesn't put the generated metadata inside the input object", func() {
			object := &testsv1.Object{}
			_, err := generic.Create(ctx, object)
			Expect(err).ToNot(HaveOccurred())
			Expect(object.Metadata).To(BeNil())
		})

		It("Gets object", func() {
			object, err := generic.Create(ctx, &testsv1.Object{})
			Expect(err).ToNot(HaveOccurred())
			result, err := generic.Get(ctx, object.GetId())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
		})

		It("Lists objects", func() {
			// Insert a couple of rows:
			const count = 2
			for range count {
				_, err := generic.Create(ctx, &testsv1.Object{})
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
			var objects []*testsv1.Object

			BeforeEach(func() {
				// Create a list of objects and sort it like they will be sorted by the DAO. Not that
				// this works correctly because the DAO is configured with a default sorting. That is
				// intended for use only in these unit tests.
				objects = make([]*testsv1.Object, objectCount)
				for i := range len(objects) {
					objects[i] = &testsv1.Object{
						Id: uuid.NewString(),
					}
					_, err := generic.Create(ctx, objects[i])
					Expect(err).ToNot(HaveOccurred())
				}
				sort(objects)
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
				object, err := generic.Create(ctx, &testsv1.Object{})
				Expect(err).ToNot(HaveOccurred())
				exists, err := generic.Exists(ctx, object.GetId())
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
			})

			It("Returns false if the object doesn't exist", func() {
				exists, err := generic.Exists(ctx, uuid.NewString())
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeFalse())
			})
		})

		It("Updates object", func() {
			object, err := generic.Create(ctx, &testsv1.Object{
				MyString: "my_value",
			})
			Expect(err).ToNot(HaveOccurred())
			object.MyString = "your_value"
			object, err = generic.Update(ctx, object)
			Expect(err).ToNot(HaveOccurred())
			Expect(object).ToNot(BeNil())
			Expect(object.GetMyString()).To(Equal("your_value"))
		})

		Describe("Filtering", func() {
			It("Filters by identifier", func() {
				for i := range 10 {
					_, err := generic.Create(
						ctx,
						testsv1.Object_builder{
							Id: fmt.Sprintf("%d", i),
						}.Build(),
					)
					Expect(err).ToNot(HaveOccurred())
				}
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.id == '5'",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("5"))
			})

			It("Filters by identifier set", func() {
				for i := range 10 {
					_, err := generic.Create(
						ctx,
						testsv1.Object_builder{
							Id: fmt.Sprintf("%d", i),
						}.Build(),
					)
					Expect(err).ToNot(HaveOccurred())
				}
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.id in ['1', '3', '5', '7', '9']",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				sort(items)
				Expect(items).To(HaveLen(5))
				Expect(items[0].GetId()).To(Equal("1"))
				Expect(items[1].GetId()).To(Equal("3"))
				Expect(items[2].GetId()).To(Equal("5"))
				Expect(items[3].GetId()).To(Equal("7"))
				Expect(items[4].GetId()).To(Equal("9"))
			})

			It("Filters by string JSON field", func() {
				for i := range 10 {
					_, err := generic.Create(
						ctx,
						testsv1.Object_builder{
							MyString: fmt.Sprintf("my_value_%d", i),
						}.Build(),
					)
					Expect(err).ToNot(HaveOccurred())
				}
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.my_string == 'my_value_5'",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetMyString()).To(Equal("my_value_5"))
			})

			It("Filters by identifier or JSON field", func() {
				for i := range 10 {
					_, err := generic.Create(
						ctx,
						testsv1.Object_builder{
							Id:       fmt.Sprintf("%d", i),
							MyString: fmt.Sprintf("my_value_%d", i),
						}.Build(),
					)
					Expect(err).ToNot(HaveOccurred())
				}
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.id == '1' || this.my_string == 'my_value_3'",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				sort(items)
				Expect(items).To(HaveLen(2))
				Expect(items[0].GetId()).To(Equal("1"))
				Expect(items[0].GetMyString()).To(Equal("my_value_1"))
				Expect(items[1].GetId()).To(Equal("3"))
				Expect(items[1].GetMyString()).To(Equal("my_value_3"))
			})

			It("Filters by identifier and JSON field", func() {
				for i := range 10 {
					_, err := generic.Create(
						ctx,
						testsv1.Object_builder{
							Id:       fmt.Sprintf("%d", i),
							MyString: fmt.Sprintf("my_value_%d", i),
						}.Build(),
					)
					Expect(err).ToNot(HaveOccurred())
				}
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.id == '1' && this.my_string == 'my_value_1'",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("1"))
				Expect(items[0].GetMyString()).To(Equal("my_value_1"))
			})

			It("Filters by calculated value", func() {
				for i := range 10 {
					_, err := generic.Create(
						ctx,
						testsv1.Object_builder{
							MyInt32: int32(i),
						}.Build(),
					)
					Expect(err).ToNot(HaveOccurred())
				}
				response, err := generic.List(ctx, ListRequest{
					Filter: "(this.my_int32 + 1) == 2",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetMyInt32()).To(BeNumerically("==", 1))
			})

			It("Filters by nested JSON string field", func() {
				for i := range 10 {
					_, err := generic.Create(
						ctx,
						testsv1.Object_builder{
							Spec: testsv1.Spec_builder{
								SpecString: fmt.Sprintf("my_value_%d", i),
							}.Build(),
						}.Build(),
					)
					Expect(err).ToNot(HaveOccurred())
				}
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.spec.spec_string == 'my_value_5'",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetSpec().GetSpecString()).ToNot(BeNil())
				Expect(items[0].GetSpec().GetSpecString()).To(Equal("my_value_5"))
			})

			It("Filters deleted", func() {
				object, err := generic.Create(
					ctx,
					testsv1.Object_builder{
						Id: "0",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				err = generic.Delete(ctx, object.GetId())
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.metadata.deletion_timestamp != null",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("0"))
			})

			It("Filters not deleted", func() {
				object, err := generic.Create(
					ctx,
					testsv1.Object_builder{
						Id: "0",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				err = generic.Delete(ctx, object.GetId())
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.metadata.deletion_timestamp == null",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(0))
			})

			It("Filters by timestamp in the future", func() {
				var err error
				now := time.Now()
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:          "old",
						MyTimestamp: timestamppb.New(now.Add(-time.Minute)),
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:          "new",
						MyTimestamp: timestamppb.New(now.Add(+time.Minute)),
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.my_timestamp > now",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("new"))
			})

			It("Filters by timestamp in the past", func() {
				var err error
				now := time.Now()
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:          "old",
						MyTimestamp: timestamppb.New(now.Add(-time.Minute)),
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:          "new",
						MyTimestamp: timestamppb.New(now.Add(+time.Minute)),
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.my_timestamp < now",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("old"))
			})

			It("Filters by presence of message field", func() {
				var err error
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:   "good",
						Spec: testsv1.Spec_builder{}.Build(),
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:   "bad",
						Spec: nil,
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "has(this.spec)",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("good"))
			})

			It("Filters by presence of string field", func() {
				var err error
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "good",
						MyString: "my value",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "bad",
						MyString: "",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "has(this.my_string)",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("good"))
			})

			It("Filters by presence of deletion timestamp", func() {
				var err error
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id: "good",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id: "bad",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				err = generic.Delete(ctx, "good")
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "has(this.metadata.deletion_timestamp)",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("good"))
			})

			It("Filters by absence of deletion timestamp", func() {
				var err error
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id: "good",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id: "bad",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				err = generic.Delete(ctx, "bad")
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "!has(this.metadata.deletion_timestamp)",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("good"))
			})

			It("Filters by presence of nested string field", func() {
				var err error
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id: "good",
						Spec: testsv1.Spec_builder{
							SpecString: "my value",
						}.Build(),
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id: "bad",
						Spec: testsv1.Spec_builder{
							SpecString: "",
						}.Build(),
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "has(this.spec.spec_string)",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("good"))
			})

			It("Filters by string prefix", func() {
				var err error
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "good",
						MyString: "my value",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "bad",
						MyString: "your value",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.my_string.startsWith('my')",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("good"))
			})

			It("Filters by string suffix", func() {
				var err error
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "good",
						MyString: "value my",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "bad",
						MyString: "value your",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.my_string.endsWith('my')",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("good"))
			})

			It("Escapes percent in prefix", func() {
				var err error
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "good",
						MyString: "my% value",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "bad",
						MyString: "my value",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.my_string.startsWith('my%')",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("good"))
			})

			It("Escapes underscore in prefix", func() {
				var err error
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "good",
						MyString: "my_ value",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = generic.Create(
					ctx,
					testsv1.Object_builder{
						Id:       "bad",
						MyString: "my value",
					}.Build(),
				)
				Expect(err).ToNot(HaveOccurred())
				response, err := generic.List(ctx, ListRequest{
					Filter: "this.my_string.startsWith('my_')",
				})
				Expect(err).ToNot(HaveOccurred())
				items := response.Items
				Expect(items).To(HaveLen(1))
				Expect(items[0].GetId()).To(Equal("good"))
			})
		})
	})
})
