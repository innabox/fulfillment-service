/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package computeinstance

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
)

var _ = Describe("buildSpec", func() {
	Describe("RestartRequestedAt field", func() {
		It("Includes restartRequestedAt in spec map when present", func() {
			requestedAt := time.Date(2026, 1, 28, 13, 27, 0, 0, time.UTC)
			cpuCores, err := anypb.New(wrapperspb.String("2"))
			Expect(err).ToNot(HaveOccurred())
			memory, err := anypb.New(wrapperspb.String("4Gi"))
			Expect(err).ToNot(HaveOccurred())
			template := "osac.templates.ocp_virt_vm"
			task := &task{
				computeInstance: privatev1.ComputeInstance_builder{
					Id: "test-instance-123",
					Spec: privatev1.ComputeInstanceSpec_builder{
						Template: template,
						TemplateParameters: map[string]*anypb.Any{
							"cpu_cores": cpuCores,
							"memory":    memory,
						},
						RestartRequestedAt: timestamppb.New(requestedAt),
					}.Build(),
				}.Build(),
			}

			// Call the actual buildSpec function
			spec, err := task.buildSpec()
			Expect(err).ToNot(HaveOccurred())

			// Verify restartRequestedAt was added with correct format
			Expect(spec["restartRequestedAt"]).To(Equal(requestedAt.Format(time.RFC3339)))

			// Verify other required fields are present
			Expect(spec["templateID"]).To(Equal(template))
			Expect(spec["templateParameters"]).ToNot(BeNil())
		})

		It("Excludes restartRequestedAt from spec map when not set", func() {
			cpuCores, err := anypb.New(wrapperspb.String("1"))
			Expect(err).ToNot(HaveOccurred())
			memory, err := anypb.New(wrapperspb.String("2Gi"))
			Expect(err).ToNot(HaveOccurred())
			template := "osac.templates.ocp_virt_vm"
			task := &task{
				computeInstance: privatev1.ComputeInstance_builder{
					Id: "test-instance-456",
					Spec: privatev1.ComputeInstanceSpec_builder{
						Template: template,
						TemplateParameters: map[string]*anypb.Any{
							"cpu_cores": cpuCores,
							"memory":    memory,
						},
						// No RestartRequestedAt set
					}.Build(),
				}.Build(),
			}

			// Call the actual buildSpec function
			spec, err := task.buildSpec()
			Expect(err).ToNot(HaveOccurred())

			// Verify restartRequestedAt was NOT added
			Expect(spec).ToNot(HaveKey("restartRequestedAt"))

			// Verify other required fields are present
			Expect(spec["templateID"]).To(Equal(template))
			Expect(spec["templateParameters"]).ToNot(BeNil())
		})
	})
})

var _ = Describe("syncStatusFromKubernetes", func() {
	var (
		ctx      context.Context
		testTask *task
	)

	BeforeEach(func() {
		ctx = context.Background()
		testTask = &task{
			r: &function{
				logger: logger,
			},
			computeInstance: privatev1.ComputeInstance_builder{
				Id: "test-sync-instance",
			}.Build(),
		}
	})

	It("Syncs lastRestartedAt from Kubernetes CR to database object", func() {
		lastRestartedAt := time.Date(2026, 1, 28, 14, 30, 0, 0, time.UTC)

		// Create a mock Kubernetes CR with status.lastRestartedAt set
		object := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"lastRestartedAt": lastRestartedAt.Format(time.RFC3339),
				},
			},
		}

		// Call syncStatusFromKubernetes
		err := testTask.syncStatusFromKubernetes(ctx, object)
		Expect(err).ToNot(HaveOccurred())

		// Verify lastRestartedAt was synced to database object
		Expect(testTask.computeInstance.GetStatus().HasLastRestartedAt()).To(BeTrue())
		Expect(testTask.computeInstance.GetStatus().GetLastRestartedAt().AsTime()).To(Equal(lastRestartedAt))
	})

	It("Does not error when lastRestartedAt is not present in Kubernetes CR", func() {
		// Create a mock Kubernetes CR without status.lastRestartedAt
		object := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{},
			},
		}

		// Call syncStatusFromKubernetes
		err := testTask.syncStatusFromKubernetes(ctx, object)
		Expect(err).ToNot(HaveOccurred())

		// Verify lastRestartedAt was NOT set
		Expect(testTask.computeInstance.GetStatus().HasLastRestartedAt()).To(BeFalse())
	})

	It("Handles invalid timestamp format gracefully without failing", func() {
		// Create a mock Kubernetes CR with invalid timestamp
		object := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": map[string]interface{}{
					"lastRestartedAt": "invalid-timestamp-format",
				},
			},
		}

		// Call syncStatusFromKubernetes - should not error
		err := testTask.syncStatusFromKubernetes(ctx, object)
		Expect(err).ToNot(HaveOccurred())

		// Verify lastRestartedAt was NOT set due to parse error
		Expect(testTask.computeInstance.GetStatus().HasLastRestartedAt()).To(BeFalse())
	})

	It("Does not error when status field is missing entirely", func() {
		// Create a mock Kubernetes CR without status field at all
		object := &unstructured.Unstructured{
			Object: map[string]interface{}{},
		}

		// Call syncStatusFromKubernetes
		err := testTask.syncStatusFromKubernetes(ctx, object)
		Expect(err).ToNot(HaveOccurred())

		// Verify lastRestartedAt was NOT set
		Expect(testTask.computeInstance.GetStatus().HasLastRestartedAt()).To(BeFalse())
	})
})
