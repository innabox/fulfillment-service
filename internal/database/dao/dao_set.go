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

	"github.com/jackc/pgx/v5/pgxpool"
)

type Set interface {
	ClusterTemplates() ClusterTemplatesDAO
	ClusterOrders() ClusterOrdersDAO
	Clusters() ClustersDAO
}

type set struct {
	clusterTemplates ClusterTemplatesDAO
	clusterOrders    ClusterOrdersDAO
	clusters         ClustersDAO
}

type SetBuilder struct {
	logger *slog.Logger
	pool   *pgxpool.Pool
}

func NewSet() *SetBuilder {
	return &SetBuilder{}
}

func (b *SetBuilder) SetLogger(value *slog.Logger) *SetBuilder {
	b.logger = value
	return b
}

func (b *SetBuilder) SetPool(value *pgxpool.Pool) *SetBuilder {
	b.pool = value
	return b
}

func (b *SetBuilder) Build() (result Set, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.pool == nil {
		err = errors.New("database connection pool is mandatory")
		return
	}

	// Create the individual DAOs:
	clusterTemplatesDAO, err := NewClusterTemplatesDAO().
		SetLogger(b.logger).
		SetPool(b.pool).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create cluster templates DAO: %w", err)
		return
	}
	clusterOrdersDAO, err := NewClusterOrdersDAO().
		SetLogger(b.logger).
		SetPool(b.pool).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create cluster orders DAO: %w", err)
		return
	}
	clustersDAO, err := NewClustersDAO().
		SetLogger(b.logger).
		SetPool(b.pool).
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

func (s *set) ClusterTemplates() ClusterTemplatesDAO {
	return s.clusterTemplates
}

func (s *set) ClusterOrders() ClusterOrdersDAO {
	return s.clusterOrders
}

func (s *set) Clusters() ClustersDAO {
	return s.clusters
}
