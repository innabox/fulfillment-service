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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	sharedv1 "github.com/innabox/fulfillment-service/internal/api/shared/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ = Describe("Generic mapper", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	parseDate := func(text string) *timestamppb.Timestamp {
		value, err := time.Parse(time.RFC3339, text)
		Expect(err).ToNot(HaveOccurred())
		return timestamppb.New(value)
	}

	DescribeTable(
		"Map cluster private to public",
		func(input *privatev1.Cluster, expected *ffv1.Cluster) {
			mapper, err := NewGenericMapper[*privatev1.Cluster, *ffv1.Cluster]().
				SetLogger(logger).
				SetStrict(false).
				Build()
			Expect(err).ToNot(HaveOccurred())
			actual, err := mapper.Map(ctx, input)
			Expect(err).ToNot(HaveOccurred())
			marshalOptions := protojson.MarshalOptions{
				UseProtoNames: true,
			}
			actualJson, err := marshalOptions.Marshal(actual)
			Expect(err).ToNot(HaveOccurred())
			expectedJson, err := marshalOptions.Marshal(expected)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualJson).To(MatchJSON(expectedJson))
		},
		Entry(
			"Nil",
			nil,
			nil,
		),
		Entry(
			"Empty",
			&privatev1.Cluster{},
			&ffv1.Cluster{},
		),
		Entry(
			"Identifier",
			privatev1.Cluster_builder{
				Id: "123",
			}.Build(),
			ffv1.Cluster_builder{
				Id: "123",
			}.Build(),
		),
		Entry(
			"Creation timestamp",
			privatev1.Cluster_builder{
				Metadata: privatev1.Metadata_builder{
					CreationTimestamp: parseDate("2025-06-02T14:53:00Z"),
				}.Build(),
			}.Build(),
			ffv1.Cluster_builder{
				Metadata: sharedv1.Metadata_builder{
					CreationTimestamp: parseDate("2025-06-02T14:53:00Z"),
				}.Build(),
			}.Build(),
		),
		Entry(
			"Deletion timestamp",
			privatev1.Cluster_builder{
				Metadata: privatev1.Metadata_builder{
					DeletionTimestamp: parseDate("2025-06-02T14:53:00Z"),
				}.Build(),
			}.Build(),
			ffv1.Cluster_builder{
				Metadata: sharedv1.Metadata_builder{
					DeletionTimestamp: parseDate("2025-06-02T14:53:00Z"),
				}.Build(),
			}.Build(),
		),
		Entry(
			"Empty spec",
			privatev1.Cluster_builder{
				Spec: privatev1.ClusterSpec_builder{}.Build(),
			}.Build(),
			ffv1.Cluster_builder{
				Spec: ffv1.ClusterSpec_builder{}.Build(),
			}.Build(),
		),
		Entry(
			"Spec with node sets",
			privatev1.Cluster_builder{
				Spec: privatev1.ClusterSpec_builder{
					NodeSets: map[string]*privatev1.ClusterNodeSet{
						"my_node_set": privatev1.ClusterNodeSet_builder{
							HostClass: "my_host_class",
							Size:      123,
						}.Build(),
						"your_node_set": privatev1.ClusterNodeSet_builder{
							HostClass: "your_host_class",
							Size:      456,
						}.Build(),
					},
				}.Build(),
			}.Build(),
			ffv1.Cluster_builder{
				Spec: ffv1.ClusterSpec_builder{
					NodeSets: map[string]*ffv1.ClusterNodeSet{
						"my_node_set": ffv1.ClusterNodeSet_builder{
							HostClass: "my_host_class",
							Size:      123,
						}.Build(),
						"your_node_set": ffv1.ClusterNodeSet_builder{
							HostClass: "your_host_class",
							Size:      456,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		),
		Entry(
			"Empty status",
			privatev1.Cluster_builder{
				Status: privatev1.ClusterStatus_builder{}.Build(),
			}.Build(),
			ffv1.Cluster_builder{
				Status: ffv1.ClusterStatus_builder{}.Build(),
			}.Build(),
		),
		Entry(
			"Status with state",
			privatev1.Cluster_builder{
				Status: privatev1.ClusterStatus_builder{
					State: privatev1.ClusterState_CLUSTER_STATE_READY,
				}.Build(),
			}.Build(),
			ffv1.Cluster_builder{
				Status: ffv1.ClusterStatus_builder{
					State: ffv1.ClusterState_CLUSTER_STATE_READY,
				}.Build(),
			}.Build(),
		),
	)
})
