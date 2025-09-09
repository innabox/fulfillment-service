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
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
)

var _ = Describe("Private virtual machines server", func() {
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
			create table virtual_machine_templates (
				id text not null primary key,
				creation_timestamp timestamp with time zone not null default now(),
				deletion_timestamp timestamp with time zone not null default 'epoch',
				finalizers text[] not null default '{}',
				creators text[] not null default '{}',
				tenants text[] not null default '{}',
				data jsonb not null
			);

			create table archived_virtual_machine_templates (
				id text not null,
				creation_timestamp timestamp with time zone not null,
				deletion_timestamp timestamp with time zone not null,
				archival_timestamp timestamp with time zone not null default now(),
				creators text[] not null default '{}',
				tenants text[] not null default '{}',
				data jsonb not null
			);

			create table virtual_machines (
				id text not null primary key,
				creation_timestamp timestamp with time zone not null default now(),
				deletion_timestamp timestamp with time zone not null default 'epoch',
				finalizers text[] not null default '{}',
				creators text[] not null default '{}',
				tenants text[] not null default '{}',
				data jsonb not null
			);

			create table archived_virtual_machines (
				id text not null,
				creation_timestamp timestamp with time zone not null,
				deletion_timestamp timestamp with time zone not null,
				archival_timestamp timestamp with time zone not null default now(),
				creators text[] not null default '{}',
				tenants text[] not null default '{}',
				data jsonb not null
			);
			`,
		)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Builder", func() {
		It("Creates server with logger", func() {
			server, err := NewPrivateVirtualMachinesServer().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(server).ToNot(BeNil())
		})

		It("Doesn't create server without logger", func() {
			server, err := NewPrivateVirtualMachinesServer().
				Build()
			Expect(err).To(HaveOccurred())
			Expect(server).To(BeNil())
		})
	})

	Describe("Behaviour", func() {
		var server *PrivateVirtualMachinesServer

		BeforeEach(func() {
			var err error

			// Create the server:
			server, err = NewPrivateVirtualMachinesServer().
				SetLogger(logger).
				Build()
			Expect(err).ToNot(HaveOccurred())
		})

		// Helper function to create a template
		createTemplate := func(templateID string) {
			// Create a template DAO to insert a template
			templatesDao, err := dao.NewGenericDAO[*privatev1.VirtualMachineTemplate]().
				SetLogger(logger).
				SetTable("virtual_machine_templates").
				Build()
			Expect(err).ToNot(HaveOccurred())

			// Create default values for parameters
			cpuDefault, err := anypb.New(wrapperspb.Int32(1))
			Expect(err).ToNot(HaveOccurred())
			memoryDefault, err := anypb.New(wrapperspb.Int32(2))
			Expect(err).ToNot(HaveOccurred())

			template := privatev1.VirtualMachineTemplate_builder{
				Id:          templateID,
				Title:       "Test Template",
				Description: "Test template for validation",
				Parameters: []*privatev1.VirtualMachineTemplateParameterDefinition{
					{
						Name:        "cpu_count",
						Title:       "CPU Count",
						Description: "Number of CPU cores",
						Required:    false,
						Type:        "type.googleapis.com/google.protobuf.Int32Value",
						Default:     cpuDefault,
					},
					{
						Name:        "memory_gb",
						Title:       "Memory (GB)",
						Description: "Amount of memory in GB",
						Required:    false,
						Type:        "type.googleapis.com/google.protobuf.Int32Value",
						Default:     memoryDefault,
					},
				},
			}.Build()

			_, err = templatesDao.Create(ctx, template)
			Expect(err).ToNot(HaveOccurred())
		}

		It("Creates object", func() {
			// Create a template first
			createTemplate("general.small")

			// Create template parameters
			templateParams := make(map[string]*anypb.Any)
			cpuParam, err := anypb.New(wrapperspb.Int32(2))
			Expect(err).ToNot(HaveOccurred())
			templateParams["cpu_count"] = cpuParam

			memoryParam, err := anypb.New(wrapperspb.Int32(4))
			Expect(err).ToNot(HaveOccurred())
			templateParams["memory_gb"] = memoryParam

			response, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
				Object: privatev1.VirtualMachine_builder{
					Spec: privatev1.VirtualMachineSpec_builder{
						Template:           "general.small",
						TemplateParameters: templateParams,
					}.Build(),
					Status: privatev1.VirtualMachineStatus_builder{
						State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			object := response.GetObject()
			Expect(object).ToNot(BeNil())
			Expect(object.GetId()).ToNot(BeEmpty())
			Expect(object.GetSpec().GetTemplate()).To(Equal("general.small"))
			Expect(object.GetStatus().GetState()).To(Equal(privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING))
		})

		It("List objects", func() {
			// Create templates and objects:
			const count = 10
			for i := range count {
				templateID := fmt.Sprintf("template-%d", i)
				createTemplate(templateID)

				_, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
					Object: privatev1.VirtualMachine_builder{
						Spec: privatev1.VirtualMachineSpec_builder{
							Template: templateID,
						}.Build(),
						Status: privatev1.VirtualMachineStatus_builder{
							State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
						}.Build(),
					}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}

			// List the objects:
			response, err := server.List(ctx, privatev1.VirtualMachinesListRequest_builder{}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			items := response.GetItems()
			Expect(items).To(HaveLen(count))
		})

		It("List objects with limit", func() {
			// Create templates and objects:
			const count = 10
			for i := range count {
				templateID := fmt.Sprintf("template-limit-%d", i)
				createTemplate(templateID)

				_, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
					Object: privatev1.VirtualMachine_builder{
						Spec: privatev1.VirtualMachineSpec_builder{
							Template: templateID,
						}.Build(),
						Status: privatev1.VirtualMachineStatus_builder{
							State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
						}.Build(),
					}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}

			// List the objects with limit:
			response, err := server.List(ctx, privatev1.VirtualMachinesListRequest_builder{
				Limit: proto.Int32(5),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			items := response.GetItems()
			Expect(items).To(HaveLen(5))
		})

		It("List objects with offset", func() {
			// Create templates and objects:
			const count = 10
			for i := range count {
				templateID := fmt.Sprintf("template-offset-%d", i)
				createTemplate(templateID)

				_, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
					Object: privatev1.VirtualMachine_builder{
						Spec: privatev1.VirtualMachineSpec_builder{
							Template: templateID,
						}.Build(),
						Status: privatev1.VirtualMachineStatus_builder{
							State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
						}.Build(),
					}.Build(),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
			}

			// List the objects with offset:
			response, err := server.List(ctx, privatev1.VirtualMachinesListRequest_builder{
				Offset: proto.Int32(5),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			items := response.GetItems()
			Expect(items).To(HaveLen(5))
		})

		It("Gets object", func() {
			// Create a template first
			createTemplate("general.small")

			// Create an object:
			createResponse, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
				Object: privatev1.VirtualMachine_builder{
					Spec: privatev1.VirtualMachineSpec_builder{
						Template: "general.small",
					}.Build(),
					Status: privatev1.VirtualMachineStatus_builder{
						State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(createResponse).ToNot(BeNil())
			createdObject := createResponse.GetObject()
			Expect(createdObject).ToNot(BeNil())
			id := createdObject.GetId()
			Expect(id).ToNot(BeEmpty())

			// Get the object:
			getResponse, err := server.Get(ctx, privatev1.VirtualMachinesGetRequest_builder{
				Id: id,
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(getResponse).ToNot(BeNil())
			object := getResponse.GetObject()
			Expect(object).ToNot(BeNil())
			Expect(object.GetId()).To(Equal(id))
			Expect(object.GetSpec().GetTemplate()).To(Equal("general.small"))
			Expect(object.GetStatus().GetState()).To(Equal(privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING))
		})

		It("Updates object", func() {
			// Create templates first
			createTemplate("general.small")
			createTemplate("general.large")

			// Create an object:
			createResponse, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
				Object: privatev1.VirtualMachine_builder{
					Spec: privatev1.VirtualMachineSpec_builder{
						Template: "general.small",
					}.Build(),
					Status: privatev1.VirtualMachineStatus_builder{
						State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(createResponse).ToNot(BeNil())
			createdObject := createResponse.GetObject()
			Expect(createdObject).ToNot(BeNil())
			id := createdObject.GetId()
			Expect(id).ToNot(BeEmpty())

			// Update the object:
			updateResponse, err := server.Update(ctx, privatev1.VirtualMachinesUpdateRequest_builder{
				Object: privatev1.VirtualMachine_builder{
					Id: id,
					Spec: privatev1.VirtualMachineSpec_builder{
						Template: "general.large",
					}.Build(),
					Status: privatev1.VirtualMachineStatus_builder{
						State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_READY,
					}.Build(),
				}.Build(),
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.template", "status.state"},
				},
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(updateResponse).ToNot(BeNil())
			object := updateResponse.GetObject()
			Expect(object).ToNot(BeNil())
			Expect(object.GetId()).To(Equal(id))
			Expect(object.GetSpec().GetTemplate()).To(Equal("general.large"))
			Expect(object.GetStatus().GetState()).To(Equal(privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_READY))
		})

		It("Deletes object", func() {
			// Create a template first
			createTemplate("general.small")

			// Create an object:
			createResponse, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
				Object: privatev1.VirtualMachine_builder{
					Spec: privatev1.VirtualMachineSpec_builder{
						Template: "general.small",
					}.Build(),
					Status: privatev1.VirtualMachineStatus_builder{
						State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(createResponse).ToNot(BeNil())
			createdObject := createResponse.GetObject()
			Expect(createdObject).ToNot(BeNil())
			id := createdObject.GetId()
			Expect(id).ToNot(BeEmpty())

			// Delete the object:
			deleteResponse, err := server.Delete(ctx, privatev1.VirtualMachinesDeleteRequest_builder{
				Id: id,
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(deleteResponse).ToNot(BeNil())

			// Verify the object is deleted:
			getResponse, err := server.Get(ctx, privatev1.VirtualMachinesGetRequest_builder{
				Id: id,
			}.Build())
			Expect(err).To(HaveOccurred())
			Expect(getResponse).To(BeNil())
		})

		It("Handles non-existent object", func() {
			// Try to get a non-existent object:
			getResponse, err := server.Get(ctx, privatev1.VirtualMachinesGetRequest_builder{
				Id: "non-existent-id",
			}.Build())
			Expect(err).To(HaveOccurred())
			Expect(getResponse).To(BeNil())
		})

		It("Handles empty object in create request", func() {
			// Try to create with nil object:
			response, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{}.Build())
			Expect(err).To(HaveOccurred())
			Expect(response).To(BeNil())
		})

		It("Handles empty object in update request", func() {
			// Try to update with nil object:
			response, err := server.Update(ctx, privatev1.VirtualMachinesUpdateRequest_builder{}.Build())
			Expect(err).To(HaveOccurred())
			Expect(response).To(BeNil())
		})

		It("Handles empty ID in get request", func() {
			// Try to get with empty ID:
			response, err := server.Get(ctx, privatev1.VirtualMachinesGetRequest_builder{}.Build())
			Expect(err).To(HaveOccurred())
			Expect(response).To(BeNil())
		})

		It("Handles empty ID in delete request", func() {
			// Try to delete with empty ID:
			response, err := server.Delete(ctx, privatev1.VirtualMachinesDeleteRequest_builder{}.Build())
			Expect(err).To(HaveOccurred())
			Expect(response).To(BeNil())
		})

		It("Validates template exists on create", func() {
			// Try to create with non-existent template:
			response, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
				Object: privatev1.VirtualMachine_builder{
					Spec: privatev1.VirtualMachineSpec_builder{
						Template: "non-existent-template",
					}.Build(),
					Status: privatev1.VirtualMachineStatus_builder{
						State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).To(HaveOccurred())
			Expect(response).To(BeNil())
		})

		It("Validates template exists on update", func() {
			// Create a template and virtual machine first:
			createTemplate("existing-template")

			createResponse, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
				Object: privatev1.VirtualMachine_builder{
					Spec: privatev1.VirtualMachineSpec_builder{
						Template: "existing-template",
					}.Build(),
					Status: privatev1.VirtualMachineStatus_builder{
						State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(createResponse).ToNot(BeNil())

			id := createResponse.GetObject().GetId()

			// Try to update with non-existent template:
			updateResponse, err := server.Update(ctx, privatev1.VirtualMachinesUpdateRequest_builder{
				Object: privatev1.VirtualMachine_builder{
					Id: id,
					Spec: privatev1.VirtualMachineSpec_builder{
						Template: "non-existent-template",
					}.Build(),
					Status: privatev1.VirtualMachineStatus_builder{
						State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
					}.Build(),
				}.Build(),
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"spec.template"},
				},
			}.Build())
			Expect(err).To(HaveOccurred())
			Expect(updateResponse).To(BeNil())
		})

		It("Validates template ID is not empty", func() {
			// Try to create with empty template ID:
			response, err := server.Create(ctx, privatev1.VirtualMachinesCreateRequest_builder{
				Object: privatev1.VirtualMachine_builder{
					Spec: privatev1.VirtualMachineSpec_builder{
						Template: "",
					}.Build(),
					Status: privatev1.VirtualMachineStatus_builder{
						State: privatev1.VirtualMachineState_VIRTUAL_MACHINE_STATE_PROGRESSING,
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).To(HaveOccurred())
			Expect(response).To(BeNil())
		})
	})
})
