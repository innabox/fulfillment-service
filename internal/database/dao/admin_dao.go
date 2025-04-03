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

	adminv1 "github.com/innabox/fulfillment-service/internal/api/admin/v1"
)

type AdminSet interface {
	ClusterOrders() *GenericDAO[*adminv1.ClusterOrder]
	Hubs() *GenericDAO[*adminv1.Hub]
}

type adminSet struct {
	logger        *slog.Logger
	clusterOrders *GenericDAO[*adminv1.ClusterOrder]
	hubs          *GenericDAO[*adminv1.Hub]
}

type AdminSetBuilder struct {
	logger *slog.Logger
}

func NewAdminSet() *AdminSetBuilder {
	return &AdminSetBuilder{}
}

func (b *AdminSetBuilder) SetLogger(value *slog.Logger) *AdminSetBuilder {
	b.logger = value
	return b
}

func (b *AdminSetBuilder) Build() (result AdminSet, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the result early, so that we can the reference to the event callback to create the individual DAOs:
	s := &adminSet{
		logger: b.logger,
	}

	// Cluster the individual DAOs:
	s.clusterOrders, err = NewGenericDAO[*adminv1.ClusterOrder]().
		SetLogger(b.logger).
		SetTable("admin.cluster_orders").
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create cluster orders DAO: %w", err)
		return
	}
	s.hubs, err = NewGenericDAO[*adminv1.Hub]().
		SetLogger(b.logger).
		SetTable("admin.hubs").
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create clusters DAO: %w", err)
		return
	}

	result = s
	return
}

func (s *adminSet) ClusterOrders() *GenericDAO[*adminv1.ClusterOrder] {
	return s.clusterOrders
}

func (s *adminSet) Hubs() *GenericDAO[*adminv1.Hub] {
	return s.hubs
}
