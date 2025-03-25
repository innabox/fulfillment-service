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
	"errors"
	"fmt"
	"log/slog"

	api "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
)

type Set interface {
	ClusterTemplates() *GenericDAO[*api.ClusterTemplate]
	ClusterOrders() *GenericDAO[*api.ClusterOrder]
	Clusters() *GenericDAO[*api.Cluster]
}

type set struct {
	clusterTemplates *GenericDAO[*api.ClusterTemplate]
	clusterOrders    *GenericDAO[*api.ClusterOrder]
	clusters         *GenericDAO[*api.Cluster]
}

type SetBuilder struct {
	logger *slog.Logger
}

func NewSet() *SetBuilder {
	return &SetBuilder{}
}

func (b *SetBuilder) SetLogger(value *slog.Logger) *SetBuilder {
	b.logger = value
	return b
}

func (b *SetBuilder) Build() (result Set, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the individual DAOs:
	clusterTemplatesDAO, err := NewGenericDAO[*api.ClusterTemplate]().
		SetLogger(b.logger).
		SetTable("cluster_templates").
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create cluster templates DAO: %w", err)
		return
	}
	clusterOrdersDAO, err := NewGenericDAO[*api.ClusterOrder]().
		SetLogger(b.logger).
		SetTable("cluster_orders").
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create cluster orders DAO: %w", err)
		return
	}
	clustersDAO, err := NewGenericDAO[*api.Cluster]().
		SetLogger(b.logger).
		SetTable("clusters").
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create cluster DAO: %w", err)
		return
	}

	// Create and populate the result:
	result = &set{
		clusterTemplates: clusterTemplatesDAO,
		clusterOrders:    clusterOrdersDAO,
		clusters:         clustersDAO,
	}
	return
}

func (s *set) ClusterTemplates() *GenericDAO[*api.ClusterTemplate] {
	return s.clusterTemplates
}

func (s *set) ClusterOrders() *GenericDAO[*api.ClusterOrder] {
	return s.clusterOrders
}

func (s *set) Clusters() *GenericDAO[*api.Cluster] {
	return s.clusters
}
