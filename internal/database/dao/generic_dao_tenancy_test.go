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
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	testsv1 "github.com/innabox/fulfillment-service/internal/api/tests/v1"
	"github.com/innabox/fulfillment-service/internal/auth"
	"github.com/innabox/fulfillment-service/internal/collections"
	"github.com/innabox/fulfillment-service/internal/database"
)

var _ = Describe("Tenancy logic", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
		tx   database.Tx
	)

	BeforeEach(func() {
		var err error

		// Create the mock controller:
		ctrl = gomock.NewController(GinkgoT())
		DeferCleanup(ctrl.Finish)

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

		// Create the objects table:
		err = CreateTables(ctx, "objects")
		Expect(err).ToNot(HaveOccurred())
	})

	It("When an object is created, it only shows the visible tenants", func() {
		// Create a tenancy logic that assigns a visible tenant and an invisible one:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Create an object and verify that it only shows the visible tenant:
		response, err := dao.Create().SetObject(testsv1.Object_builder{}.Build()).Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		object := response.GetObject()
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a"))

		// Verify that both tenants have been saved in the database:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a", "tenant-b"))
	})

	It("When an object is retrieved by identifier, it only shows the visible tenants", func() {
		// Create a tenancy logic that assigns a visible tenant and an invisible one:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Create an object:
		createResponse, err := dao.Create().SetObject(testsv1.Object_builder{}.Build()).Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		object := createResponse.GetObject()

		// Retrieve the object and verify that it only shows the visible tenant:
		getResponse, err := dao.Get().SetId(object.GetId()).Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		object = getResponse.GetObject()
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a"))
	})

	It("When an object is retrieved by list, it only shows the visible tenants", func() {
		// Create a tenancy logic that assigns a visible tenant and an invisible one:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Create an object:
		createResponse, err := dao.Create().SetObject(testsv1.Object_builder{}.Build()).Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		object := createResponse.GetObject()

		// Retrieve the object and verify that it only shows the visible tenant:
		response, err := dao.List().
			SetFilter(fmt.Sprintf("this.id == %s", strconv.Quote(object.GetId()))).
			SetLimit(1).
			Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		items := response.GetItems()
		Expect(items).To(HaveLen(1))
		Expect(items[0].GetMetadata().GetTenants()).To(ConsistOf("tenant-a"))
	})

	It("When an object is updated, it only shows the visible tenants", func() {
		// Create a tenancy logic that assigns a visible tenant and an invisible one:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Create an object:
		createResponse, err := dao.Create().SetObject(testsv1.Object_builder{}.Build()).Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		object := createResponse.GetObject()

		// Update the object and verify that it only shows the visible tenant:
		object.SetMyString("my-string")
		updateResponse, err := dao.Update().SetObject(object).Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		object = updateResponse.GetObject()
		Expect(object.GetMetadata().GetTenants()).To(ContainElement("tenant-a"))
	})

	It("Allows user to override the assigned tenants", func() {
		// Create a tenancy logic that returns two assigned and visible tenants:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Create an object overridng the assigned tenants:
		response, err := dao.Create().SetObject(testsv1.Object_builder{
			Metadata: testsv1.Metadata_builder{
				Tenants: []string{
					"tenant-a",
				},
			}.Build(),
		}.Build()).Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		object := response.GetObject()
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a"))

		// Verify that only the overridden tenant is saved in the database:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a"))
	})

	It("Can't create an object explicitly setting an invisible tenant", func() {
		// Create a tenancy logic that assigns a visible tenant and an invisible one:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Try to create an object with an invisible tenant and verify that it fails:
		_, err = dao.Create().SetObject(testsv1.Object_builder{
			Metadata: testsv1.Metadata_builder{
				Tenants: []string{
					"tenant-b",
				},
			}.Build(),
		}.Build()).Do(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("tenant 'tenant-b' doesn't exist"))
	})

	It("Can't create an object explicitly setting two invisible tenants", func() {
		// Create a tenancy logic that assigns one visibile tenant and two invisible ones:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
					"tenant-c",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
					"tenant-c",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Try to create an object with an invisible tenant and verify that it fails:
		_, err = dao.Create().SetObject(testsv1.Object_builder{
			Metadata: testsv1.Metadata_builder{
				Tenants: []string{
					"tenant-b",
					"tenant-c",
				},
			}.Build(),
		}.Build()).Do(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("tenants 'tenant-b' and 'tenant-c' don't exist"))
	})

	It("Can't create an object explicitly setting one tenant that can't be assigned", func() {
		// Create a tenancy logic that has two visibile tenants, but only assigns the first one:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Try to create an object using the unnassignable tenant and verify that it fails:
		_, err = dao.Create().SetObject(testsv1.Object_builder{
			Metadata: testsv1.Metadata_builder{
				Tenants: []string{
					"tenant-b",
				},
			}.Build(),
		}.Build()).Do(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("tenant 'tenant-b' can't be assigned"))
	})

	It("Can't create an object explicitly setting two tenants that can't be assigned", func() {
		// Create a tenancy logic that has three visible tenants, but only assigns the first one:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
					"tenant-c",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Try to create an object using the unnassignable tenant and verify that it fails:
		_, err = dao.Create().SetObject(testsv1.Object_builder{
			Metadata: testsv1.Metadata_builder{
				Tenants: []string{
					"tenant-b",
					"tenant-c",
				},
			}.Build(),
		}.Build()).Do(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("tenants 'tenant-b' and 'tenant-c' can't be assigned"))
	})

	It("Can't update an object to add an invisible tenant", func() {
		// Create a tenancy logic that a visible tenant and an invisible one:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Create the object with the default assigned tenants::
		createResponse, err := dao.Create().SetObject(testsv1.Object_builder{}.Build()).Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		object := createResponse.GetObject()

		// Verify that that both tenants have been saved in the database:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a", "tenant-b"))

		// Try to update the object to add the invisible tenant and verify that it fails:
		object.GetMetadata().SetTenants([]string{
			"tenant-a",
			"tenant-b",
		})
		_, err = dao.Update().SetObject(object).Do(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("tenant 'tenant-b' doesn't exist"))
	})

	It("Can't update an object to add an unassignable tenant", func() {
		// Create a tenancy logic that has two visible tenants, but only assigns the first one:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
				),
				nil,
			).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Create the object with the default assigned tenants:
		createResponse, err := dao.Create().SetObject(testsv1.Object_builder{}.Build()).Do(ctx)
		Expect(err).ToNot(HaveOccurred())
		object := createResponse.GetObject()

		// Verify that only the assigned tenant has been saved in the database:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a"))

		// Try to update the object to add the unassignable tenant and verify that it fails:
		object.GetMetadata().SetTenants([]string{
			"tenant-a",
			"tenant-b",
		})
		_, err = dao.Update().SetObject(object).Do(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("tenant 'tenant-b' can't be assigned"))
	})
})
