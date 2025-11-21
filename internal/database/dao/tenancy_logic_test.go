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
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant_a",
					"tenant_b",
					"tenant_c",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
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
		// Create a tenancy logic without restrictions:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewUniversal[string](),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewUniversal[string](),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant_a",
					"tenant_b",
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

		// Create an object and verify that it shows all tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant_a", "tenant_b", "tenant_c"))
	})

	It("Shows no tenants when user has no visible tenants that intersect with object tenants", func() {
		// Create a tenancy logic that assigns tenant X but only makes tenant Y visible:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet("tenant_y"),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet("tenant_y"),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewSet("tenant_x"),
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

		// Create an object with tenants that don't overlap with visible tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(BeEmpty())
	})

	It("Assigns user default tenants when creating object without explicit tenants", func() {
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

		// Create an object without specifying tenants in the metadata:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Verify the returned object has the user's default tenants:
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a", "tenant-b"))

		// Verify the actual database contains the default tenants:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a", "tenant-b"))
	})

	It("Assigns only the requested tenants when user explicitly specifies a subset", func() {
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

		// Create an object explicitly requesting only a subset of the available tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{
			Metadata: testsv1.Metadata_builder{
				Tenants: []string{"tenant-a"},
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Verify the returned object has only the requested tenant:
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a"))

		// Verify the actual database contains only the requested tenant:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a"))
	})

	It("Preserves invisible tenants when creating object with explicit tenants", func() {
		// Configure tenancy logic so that with an invisible tenant:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
					"invisible",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
					"invisible",
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

		// Create an object explicitly requesting only 'tenant-a':
		object, err := dao.Create(ctx, testsv1.Object_builder{
			Metadata: testsv1.Metadata_builder{
				Tenants: []string{"tenant-a"},
			}.Build(),
		}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Verify the returned object shows only the visible tenant:
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a"))

		// Verify the database contains both the requested tenant and the invisible tenant:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a", "invisible"))
	})

	It("Rejects request when user explicitly tries to add an invisible tenant", func() {
		// Configure tenancy logic with an invisible tenant:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
					"invisible",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
					"invisible",
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

		// Try to create an object explicitly requesting the invisible tenant:
		_, err = dao.Create(ctx, testsv1.Object_builder{
			Metadata: testsv1.Metadata_builder{
				Tenants: []string{"tenant-a", "invisible"},
			}.Build(),
		}.Build())

		// Verify the request is rejected with an error indicating the tenant doesn't exist:
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("tenant 'invisible' doesn't exist"))
	})

	It("Rejects request when user tries to add a tenant that isn't assignable", func() {
		// Configure tenancy logic where the user can only assign specific tenants:
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

		// Try to create an object requesting a tenant that is visible but not assignable:
		_, err = dao.Create(ctx, testsv1.Object_builder{
			Metadata: testsv1.Metadata_builder{
				Tenants: []string{"tenant-a", "tenant-c"},
			}.Build(),
		}.Build())

		// Verify the request is rejected with an error indicating the tenant can't be assigned:
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("tenant 'tenant-c' can't be assigned"))
	})

	It("Preserves existing tenants when updating without specifying tenants", func() {
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

		// Create an object with default tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a", "tenant-b"))

		// Update the object without specifying tenants:
		object.SetMyString("updated")
		object.GetMetadata().SetTenants(nil)
		updated, err := dao.Update(ctx, object)
		Expect(err).ToNot(HaveOccurred())

		// Verify the tenants are preserved:
		Expect(updated.GetMetadata().GetTenants()).To(ConsistOf("tenant-a", "tenant-b"))

		// Verify the database still has the original tenants:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a", "tenant-b"))
	})

	It("Keeps tenants unchanged when updating with same tenants", func() {
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

		// Create an object with default tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a", "tenant-b"))

		// Update the object explicitly specifying the same tenants:
		object.SetMyString("updated")
		object.GetMetadata().SetTenants([]string{"tenant-a", "tenant-b"})
		updated, err := dao.Update(ctx, object)
		Expect(err).ToNot(HaveOccurred())

		// Verify the tenants remain the same:
		Expect(updated.GetMetadata().GetTenants()).To(ConsistOf("tenant-a", "tenant-b"))

		// Verify the database has the same tenants:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a", "tenant-b"))
	})

	It("Removes visible and assignable tenant when user explicitly removes it", func() {
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

		// Create an object with default tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a", "tenant-b"))

		// Update the object removing one tenant:
		object.GetMetadata().SetTenants([]string{"tenant-a"})
		updated, err := dao.Update(ctx, object)
		Expect(err).ToNot(HaveOccurred())

		// Verify the tenant was removed:
		Expect(updated.GetMetadata().GetTenants()).To(ConsistOf("tenant-a"))

		// Verify the database has only the remaining tenant:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a"))
	})

	It("Preserves invisible tenants when updating object", func() {
		// Configure tenancy logic with an invisible tenant:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
					"invisible",
				),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"invisible",
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

		// Create an object (will get default tenants including invisible one):
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("tenant-a"))

		// Verify the database has the invisible tenant:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a", "invisible"))

		// Update the object changing the visible tenant:
		object.GetMetadata().SetTenants([]string{"tenant-b"})
		updated, err := dao.Update(ctx, object)
		Expect(err).ToNot(HaveOccurred())

		// Verify only the visible tenant is shown:
		Expect(updated.GetMetadata().GetTenants()).To(ConsistOf("tenant-b"))

		// Verify the database still has the invisible tenant:
		row = tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-b", "invisible"))
	})

	It("Rejects update when user tries to add an invisible tenant", func() {
		// Configure tenancy logic with an invisible tenant:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewSet(
					"tenant-a",
					"tenant-b",
					"invisible",
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

		// Create an object with visible tenants only:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Try to update the object adding the invisible tenant:
		object.GetMetadata().SetTenants([]string{"tenant-a", "invisible"})
		_, err = dao.Update(ctx, object)

		// Verify the request is rejected:
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("tenant 'invisible' doesn't exist"))
	})

	It("Rejects update when user tries to add a tenant that isn't assignable", func() {
		// Configure tenancy logic where tenant-c is visible but not assignable:
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

		// Create an object:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())

		// Try to update the object adding a non-assignable tenant:
		object.GetMetadata().SetTenants([]string{"tenant-a", "tenant-c"})
		_, err = dao.Update(ctx, object)

		// Verify the request is rejected:
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("tenant 'tenant-c' can't be assigned"))
	})

	It("Allows user with universal assignability and visibility to create objects", func() {
		// Configure tenancy logic with universal assignability and visibility:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewUniversal[string](),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet("system"),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewUniversal[string](),
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

		// Create an object without specifying tenants (should get default):
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("system"))

		// Verify the database contains the default tenants:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("system"))
	})

	It("Allows user with universal assignability and visibility to update objects", func() {
		// Configure tenancy logic with universal assignability and visibility:
		tenancy := auth.NewMockTenancyLogic(ctrl)
		tenancy.EXPECT().DetermineAssignableTenants(gomock.Any()).
			Return(
				collections.NewUniversal[string](),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineDefaultTenants(gomock.Any()).
			Return(
				collections.NewSet("system"),
				nil,
			).
			AnyTimes()
		tenancy.EXPECT().DetermineVisibleTenants(gomock.Any()).
			Return(
				collections.NewUniversal[string](),
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

		// Create an object with default tenants:
		object, err := dao.Create(ctx, testsv1.Object_builder{}.Build())
		Expect(err).ToNot(HaveOccurred())
		Expect(object.GetMetadata().GetTenants()).To(ConsistOf("system"))

		// Update the object changing the tenants to arbitrary values:
		object.GetMetadata().SetTenants([]string{"tenant-a", "tenant-b"})
		updated, err := dao.Update(ctx, object)
		Expect(err).ToNot(HaveOccurred())
		Expect(updated.GetMetadata().GetTenants()).To(ConsistOf("tenant-a", "tenant-b"))

		// Verify the database contains the new tenants:
		var tenants []string
		row := tx.QueryRow(ctx, "select tenants from objects where id = $1", object.GetId())
		err = row.Scan(&tenants)
		Expect(err).ToNot(HaveOccurred())
		Expect(tenants).To(ConsistOf("tenant-a", "tenant-b"))
	})
})
