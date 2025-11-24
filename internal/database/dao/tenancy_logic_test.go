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

	It("Filters field based on user visibility", func() {
		// Create a tenancy logic that assigns multiple tenants to objects but only makes some visible
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignedTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant_a",
					"tenant_b",
					"tenant_c",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant_a",
					"tenant_c",
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

		// Create an object an verify that it only shows the visible tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant_a", "tenant_c"))

		// Retrieve the object by identifier and verify again that it only shows the visible tenants:
		object, err = dao.Get(ctx, object.GetId())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant_a", "tenant_c"))

		// Retrieve the object as part of a list and verify again that it only shows the visible tenants:
		response, err := dao.List(ctx, ListRequest{
			Filter: fmt.Sprintf("this.id == %s", strconv.Quote(object.GetId())),
			Limit:  1,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(response.Items).To(HaveLen(1))
		Expect(response.Items[0].GetMetadata().GetTenants()).To(ConsistOf("tenant_a", "tenant_c"))

		// Udate the object and verify again that it only shows the visible tenants:
		object.SetMyString("hello")
		object, err = dao.Update(ctx, object)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.Items[0].GetMetadata().GetTenants()).To(ConsistOf("tenant_a", "tenant_c"))

		// Verify the actual database contains all the tenants:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant_a", "tenant_b", "tenant_c"))
	})

	It("Shows all tenants when user has no tenant restrictions", func() {
		// Create a tenancy logic that returns an empty list, which means no tenant filtering will be applied:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(collections.NewUniversal[string](), nil).
			AnyTimes()
		tenancy.EXPECT().DetermineAssignedTenants(gomock.Any()).
			Return(collections.NewSet("tenant_a", "tenant_b", "tenant_c"), nil).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Create an object and verify that it shows all tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant_a", "tenant_b", "tenant_c"))
	})

	It("Shows no tenants when user has no visible tenants that intersect with object tenants", func() {
		// Create a tenancy logic that assigns tenant X but only makes tenant Y visible:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignedTenants(gomock.Any()).
			Return(collections.NewSet("tenant_y"), nil).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(collections.NewSet("tenant_x"), nil).
			AnyTimes()

		// Create the DAO:
		dao, err := NewGenericDAO[*testsv1.Object]().
			SetLogger(logger).
			SetTable("objects").
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Create an object with tenants that don't overlap with visible tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(BeEmpty())
	})
})
