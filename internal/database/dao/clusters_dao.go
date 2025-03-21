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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/models"
)

type ClustersDAO interface {
	List(ctx context.Context) (items []*models.Cluster, err error)
	Get(ctx context.Context, id string) (item *models.Cluster, err error)
	Exists(ctx context.Context, id string) (ok bool, err error)
	Insert(ctx context.Context, order *models.Cluster) (id string, err error)
	Delete(ctx context.Context, id string) error
}

type ClustersDAOBuilder struct {
	logger *slog.Logger
}

type clustersDAO struct {
	baseDAO
}

func NewClustersDAO() *ClustersDAOBuilder {
	return &ClustersDAOBuilder{}
}

func (b *ClustersDAOBuilder) SetLogger(value *slog.Logger) *ClustersDAOBuilder {
	b.logger = value
	return b
}

func (b *ClustersDAOBuilder) Build() (result ClustersDAO, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}

	// Create and populate the object:
	result = &clustersDAO{
		baseDAO: baseDAO{
			logger: b.logger,
		},
	}
	return
}

func (d *clustersDAO) List(ctx context.Context) (items []*models.Cluster, err error) {
	// Get the transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Fetch the results:
	rows, err := tx.Query(
		ctx,
		`
		select
			id,
			api_url,
			console_url
		from
			clusters
		`,
	)
	if err != nil {
		return
	}
	var tmp []*models.Cluster
	for rows.Next() {
		var (
			id         string
			apiURL     string
			consoleURL string
		)
		err = rows.Scan(&id, &apiURL, &consoleURL)
		if err != nil {
			return
		}
		tmp = append(tmp, &models.Cluster{
			ID:         id,
			APIURL:     apiURL,
			ConsoleURL: consoleURL,
		})
	}
	items = tmp
	return
}

func (d *clustersDAO) Exists(ctx context.Context, id string) (ok bool, err error) {
	// Get the transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Check if the row exists:
	row := tx.QueryRow(
		ctx,
		`select count(*) from clusters where id = $1`,
		id,
	)
	var count int
	err = row.Scan(&count)
	if err != nil {
		return
	}
	ok = count > 0
	return
}

func (d *clustersDAO) Get(ctx context.Context, id string) (item *models.Cluster, err error) {
	// Get the transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Fetch the results:
	row := tx.QueryRow(
		ctx,
		`
		select
			api_url,
			console_url
		from
			clusters
		where
			id = $1
		`,
		id,
	)
	var (
		apiURL     string
		consoleURL string
	)
	err = row.Scan(&apiURL, &consoleURL)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
		return
	}
	item = &models.Cluster{
		ID:         id,
		APIURL:     apiURL,
		ConsoleURL: consoleURL,
	}
	return
}

func (d *clustersDAO) Insert(ctx context.Context, cluster *models.Cluster) (id string, err error) {
	// Get the transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Generate a new identifier:
	id = uuid.NewString()

	// Insert the row:
	_, err = tx.Exec(
		ctx,
		`
		insert into clusters (
			id,
			api_url,
			console_url
		) values (
			$1, $2, $3
		)
		`,
		id,
		cluster.APIURL,
		cluster.ConsoleURL,
	)
	return
}

func (d *clustersDAO) Delete(ctx context.Context, id string) (err error) {
	// Get the transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Delete the row:
	_, err = tx.Exec(
		ctx,
		`delete from clusters where id = $1`,
		id,
	)
	return
}
