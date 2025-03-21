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
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/models"
)

type ClusterTemplatesDAO interface {
	List(ctx context.Context) (items []*models.ClusterTemplate, err error)
	Get(ctx context.Context, id string) (item *models.ClusterTemplate, err error)
}

type ClusterTemplatesDAOBuilder struct {
	logger *slog.Logger
}

type clusterTemplatesDAO struct {
	baseDAO
}

func NewClusterTemplatesDAO() *ClusterTemplatesDAOBuilder {
	return &ClusterTemplatesDAOBuilder{}
}

func (b *ClusterTemplatesDAOBuilder) SetLogger(value *slog.Logger) *ClusterTemplatesDAOBuilder {
	b.logger = value
	return b
}

func (b *ClusterTemplatesDAOBuilder) Build() (result ClusterTemplatesDAO, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}

	// Create and populate the object:
	result = &clusterTemplatesDAO{
		baseDAO: baseDAO{
			logger: b.logger,
		},
	}
	return
}

func (d *clusterTemplatesDAO) List(ctx context.Context) (items []*models.ClusterTemplate, err error) {
	// Get the transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Fetch the results:
	rows, err := tx.Query(
		ctx,
		"select id, title, description from cluster_templates",
	)
	if err != nil {
		return
	}
	var tmp []*models.ClusterTemplate
	for rows.Next() {
		var (
			id          string
			title       string
			description string
		)
		err = rows.Scan(&id, &title, &description)
		if err != nil {
			return
		}
		tmp = append(tmp, &models.ClusterTemplate{
			ID:          id,
			Title:       title,
			Description: description,
		})
	}
	items = tmp
	return
}

func (d *clusterTemplatesDAO) Get(ctx context.Context, id string) (item *models.ClusterTemplate, err error) {
	// Get the transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Fetch the results:
	row := tx.QueryRow(
		ctx,
		"select title, description from cluster_templates where id = $1",
		id,
	)
	var (
		title       string
		description string
	)
	err = row.Scan(&title, &description)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
		return
	}
	item = &models.ClusterTemplate{
		ID:          id,
		Title:       title,
		Description: description,
	}
	return
}
