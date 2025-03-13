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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/innabox/fulfillment-service/internal/database/models"
)

type ClusterOrdersDAO interface {
	List(ctx context.Context) (items []*models.ClusterOrder, err error)
	Get(ctx context.Context, id string) (item *models.ClusterOrder, err error)
	Exists(ctx context.Context, id string) (ok bool, err error)
	Insert(ctx context.Context, order *models.ClusterOrder) (id string, err error)
	Delete(ctx context.Context, id string) error
	UpdateState(ctx context.Context, id string, state models.ClusterOrderState) error
}

type ClusterOrdersDAOBuilder struct {
	logger *slog.Logger
	pool   *pgxpool.Pool
}

type clusterOrdersDAO struct {
	baseDAO
}

func NewClusterOrdersDAO() *ClusterOrdersDAOBuilder {
	return &ClusterOrdersDAOBuilder{}
}

func (b *ClusterOrdersDAOBuilder) SetLogger(value *slog.Logger) *ClusterOrdersDAOBuilder {
	b.logger = value
	return b
}

func (b *ClusterOrdersDAOBuilder) SetPool(value *pgxpool.Pool) *ClusterOrdersDAOBuilder {
	b.pool = value
	return b
}

func (b *ClusterOrdersDAOBuilder) Build() (result ClusterOrdersDAO, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}
	if b.pool == nil {
		err = fmt.Errorf("database connection pool is mandatory")
		return
	}

	// Create and populate the object:
	result = &clusterOrdersDAO{
		baseDAO: baseDAO{
			logger: b.logger,
			pool:   b.pool,
		},
	}
	return
}

func (d *clusterOrdersDAO) List(ctx context.Context) (items []*models.ClusterOrder, err error) {
	// Start a transaction:
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return
	}
	defer func() {
		err := tx.Rollback(ctx)
		if err != nil {
			d.logger.ErrorContext(
				ctx,
				"Failed to rollback transaction",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Fetch the results:
	rows, err := tx.Query(
		ctx,
		`
		select
			id,
			template_id,
			state,
			cluster_id
		from
			cluster_orders
		`,
	)
	if err != nil {
		return
	}
	var tmp []*models.ClusterOrder
	for rows.Next() {
		var (
			id         string
			templateID string
			state      models.ClusterOrderState
			clusterID  pgtype.Text
		)
		err = rows.Scan(&id, &templateID, &state, &clusterID)
		if err != nil {
			return
		}
		tmp = append(tmp, &models.ClusterOrder{
			ID:         id,
			TemplateID: templateID,
			State:      state,
			ClusterID:  clusterID.String,
		})
	}
	items = tmp
	return
}

func (d *clusterOrdersDAO) Exists(ctx context.Context, id string) (ok bool, err error) {
	// Start a transaction:
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return
	}
	defer func() {
		err := tx.Rollback(ctx)
		if err != nil {
			d.logger.ErrorContext(
				ctx,
				"Failed to rollback transaction",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Check if the row exists:
	row := tx.QueryRow(
		ctx,
		`select count(*) from cluster_orders where id = $1`,
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

func (d *clusterOrdersDAO) Get(ctx context.Context, id string) (item *models.ClusterOrder, err error) {
	// Start a transaction:
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return
	}
	defer func() {
		err := tx.Rollback(ctx)
		if err != nil {
			d.logger.ErrorContext(
				ctx,
				"Failed to rollback transaction",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Fetch the results:
	row := tx.QueryRow(
		ctx,
		`
		select
			template_id,
			state,
			cluster_id
		from
			cluster_orders
		where
			id = $1
		`,
		id,
	)
	var (
		templateID string
		state      models.ClusterOrderState
		clusterID  pgtype.Text
	)
	err = row.Scan(&templateID, &state, &clusterID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
		return
	}
	item = &models.ClusterOrder{
		ID:         id,
		TemplateID: templateID,
		State:      state,
		ClusterID:  clusterID.String,
	}
	return
}

func (d *clusterOrdersDAO) Insert(ctx context.Context, order *models.ClusterOrder) (id string, err error) {
	// Start a transaction:
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer func() {
		err := tx.Commit(ctx)
		if err != nil {
			d.logger.ErrorContext(
				ctx,
				"Failed to commit transaction",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Generate a new identifier:
	id = uuid.NewString()

	// Insert the row:
	_, err = tx.Exec(
		ctx,
		`
		insert into cluster_orders (
			id,
			template_id
		) values (
			$1, $2
		)
		`,
		id, order.TemplateID,
	)
	return
}

func (d *clusterOrdersDAO) Delete(ctx context.Context, id string) error {
	// Start a transaction:
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := tx.Commit(ctx)
		if err != nil {
			d.logger.ErrorContext(
				ctx,
				"Failed to commit transaction",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Delete the row:
	_, err = tx.Exec(
		ctx,
		`delete from cluster_orders where id = $1`,
		id,
	)
	return err
}

func (d *clusterOrdersDAO) UpdateState(ctx context.Context, id string, state models.ClusterOrderState) error {
	// Start a transaction:
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := tx.Commit(ctx)
		if err != nil {
			d.logger.ErrorContext(
				ctx,
				"Failed to commit transaction",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Update the row:
	_, err = tx.Exec(
		ctx,
		`update cluster_orders set state = $1 where id = $2`,
		state, id,
	)
	return err
}
