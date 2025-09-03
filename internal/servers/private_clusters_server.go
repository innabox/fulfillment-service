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
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/bits-and-blooms/bitset"
	"github.com/dustin/go-humanize/english"
	"golang.org/x/exp/maps"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
	"github.com/innabox/fulfillment-service/internal/utils"
)

type PrivateClustersServerBuilder struct {
	logger   *slog.Logger
	notifier *database.Notifier
}

var _ privatev1.ClustersServer = (*PrivateClustersServer)(nil)

type PrivateClustersServer struct {
	privatev1.UnimplementedClustersServer
	logger       *slog.Logger
	templatesDao *dao.GenericDAO[*privatev1.ClusterTemplate]
	generic      *GenericServer[*privatev1.Cluster]
}

func NewPrivateClustersServer() *PrivateClustersServerBuilder {
	return &PrivateClustersServerBuilder{}
}

func (b *PrivateClustersServerBuilder) SetLogger(value *slog.Logger) *PrivateClustersServerBuilder {
	b.logger = value
	return b
}

func (b *PrivateClustersServerBuilder) SetNotifier(value *database.Notifier) *PrivateClustersServerBuilder {
	b.notifier = value
	return b
}

func (b *PrivateClustersServerBuilder) Build() (result *PrivateClustersServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the templates DAO:
	templatesDao, err := dao.NewGenericDAO[*privatev1.ClusterTemplate]().
		SetLogger(b.logger).
		SetTable("cluster_templates").
		Build()
	if err != nil {
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*privatev1.Cluster]().
		SetLogger(b.logger).
		SetService(privatev1.Clusters_ServiceDesc.ServiceName).
		SetTable("clusters").
		SetNotifier(b.notifier).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &PrivateClustersServer{
		logger:       b.logger,
		templatesDao: templatesDao,
		generic:      generic,
	}
	return
}

func (s *PrivateClustersServer) List(ctx context.Context,
	request *privatev1.ClustersListRequest) (response *privatev1.ClustersListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *PrivateClustersServer) Get(ctx context.Context,
	request *privatev1.ClustersGetRequest) (response *privatev1.ClustersGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *PrivateClustersServer) Create(ctx context.Context,
	request *privatev1.ClustersCreateRequest) (response *privatev1.ClustersCreateResponse, err error) {
	// Validate duplicate conditions first:
	err = s.validateNoDuplicateConditions(request.GetObject())
	if err != nil {
		return
	}

	// Validate template and perform transformations:
	err = s.validateAndTransformCluster(ctx, request.GetObject())
	if err != nil {
		return
	}

	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *PrivateClustersServer) Update(ctx context.Context,
	request *privatev1.ClustersUpdateRequest) (response *privatev1.ClustersUpdateResponse, err error) {
	err = s.validateNoDuplicateConditions(request.GetObject())
	if err != nil {
		return
	}
	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *PrivateClustersServer) Delete(ctx context.Context,
	request *privatev1.ClustersDeleteRequest) (response *privatev1.ClustersDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}

func (s *PrivateClustersServer) validateNoDuplicateConditions(object *privatev1.Cluster) error {
	conditions := object.GetStatus().GetConditions()
	if conditions == nil {
		return nil
	}
	conditionTypes := &bitset.BitSet{}
	for _, condition := range conditions {
		conditionType := condition.GetType()
		if conditionTypes.Test(uint(conditionType)) {
			return grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"condition '%s' is duplicated",
				conditionType.String(),
			)
		}
		conditionTypes.Set(uint(conditionType))
	}
	return nil
}

func (s *PrivateClustersServer) validateAndTransformCluster(ctx context.Context, cluster *privatev1.Cluster) error {
	// Check that the template is specified and that refers to a existing template:
	if cluster == nil {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
	}
	templateId := cluster.GetSpec().GetTemplate()
	if templateId == "" {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "template is mandatory")
	}
	template, err := s.templatesDao.Get(ctx, templateId)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get template",
			slog.String("template", templateId),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to get template '%s'", templateId)
	}
	if template == nil {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "template '%s' doesn't exist", templateId)
	}
	if template.GetMetadata().HasDeletionTimestamp() {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "template '%s' has been deleted", templateId)
	}

	// Check that all the node sets given in the cluster correspond to node sets that exist in the template:
	templateNodeSets := template.GetNodeSets()
	clusterNodeSets := cluster.GetSpec().GetNodeSets()
	for clusterNodeSetKey := range clusterNodeSets {
		templateNodeSet := templateNodeSets[clusterNodeSetKey]
		if templateNodeSet == nil {
			templateNodeSetKeys := maps.Keys(templateNodeSets)
			sort.Strings(templateNodeSetKeys)
			for i, templateNodeSetKey := range templateNodeSetKeys {
				templateNodeSetKeys[i] = fmt.Sprintf("'%s'", templateNodeSetKey)
			}
			return grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"node set '%s' doesn't exist, valid values for template '%s' are %s",
				clusterNodeSetKey, templateId, english.WordSeries(templateNodeSetKeys, "and"),
			)
		}
	}

	// Check that all the node sets given in the cluster specify the same host class that is specified in the
	// template:
	for clusterNodeSetKey, clusterNodeSet := range clusterNodeSets {
		templateNodeSet := templateNodeSets[clusterNodeSetKey]
		clusterHostClass := clusterNodeSet.GetHostClass()
		if clusterHostClass == "" {
			continue
		}
		templateHostClass := templateNodeSet.GetHostClass()
		if clusterHostClass != templateHostClass {
			return grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"host class for node set '%s' should be empty or '%s', like in template '%s', "+
					"but it is '%s'",
				clusterNodeSetKey, templateHostClass, templateId, clusterHostClass,
			)
		}
	}

	// Check that all the node sets given in the cluster have a positive size:
	for clusterNodeSetKey, clusterNodeSet := range clusterNodeSets {
		clusterNodeSetSize := clusterNodeSet.GetSize()
		if clusterNodeSetSize <= 0 {
			return grpcstatus.Errorf(
				grpccodes.InvalidArgument,
				"size for node set '%s' should be greater than zero, but it is %d",
				clusterNodeSetKey, clusterNodeSetSize,
			)
		}
	}

	// Replace the node sets given in the cluster with those from the template, taking only the size from cluster:
	actualNodeSets := map[string]*privatev1.ClusterNodeSet{}
	for templateNodeSetKey, templateNodeSet := range templateNodeSets {
		var actualNodeSetSize int32
		clusterNodeSet := clusterNodeSets[templateNodeSetKey]
		if clusterNodeSet != nil {
			actualNodeSetSize = clusterNodeSet.GetSize()
		} else {
			actualNodeSetSize = templateNodeSet.GetSize()
		}
		actualNodeSets[templateNodeSetKey] = privatev1.ClusterNodeSet_builder{
			HostClass: templateNodeSet.GetHostClass(),
			Size:      actualNodeSetSize,
		}.Build()
	}
	cluster.GetSpec().SetNodeSets(actualNodeSets)

	// Validate template parameters:
	clusterParameters := cluster.GetSpec().GetTemplateParameters()
	err = utils.ValidateClusterTemplateParameters(template, clusterParameters)
	if err != nil {
		return err
	}

	// Set default values for template parameters:
	actualClusterParameters := utils.ProcessTemplateParametersWithDefaults(
		utils.ClusterTemplateAdapter{ClusterTemplate: template},
		clusterParameters,
	)
	cluster.GetSpec().SetTemplateParameters(actualClusterParameters)

	return nil
}
