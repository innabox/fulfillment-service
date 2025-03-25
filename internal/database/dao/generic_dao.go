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
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GenericDAOBuilder is a builder for creating generic data access objects.
type GenericDAOBuilder[M proto.Message] struct {
	logger *slog.Logger
	table  string
	pool   *pgxpool.Pool
}

// GenericDAO provides generic data access operations for protocol buffers messages. It assumes that objects will be
// stored in tables with two columns:
//
//   - `id` - The unique identifier of the object.
//   - `data` - The serialized object, using the protocol buffers JSON serialization.
//
// Objects must have field named `id` of string type.
type GenericDAO[M proto.Message] struct {
	logger     *slog.Logger
	table      string
	pool       *pgxpool.Pool
	reflectMsg protoreflect.Message
	reflectId  protoreflect.FieldDescriptor
}

// NewGenericDAO creates a builder that can then be used to configure and create a generic DAO.
func NewGenericDAO[M proto.Message]() *GenericDAOBuilder[M] {
	return &GenericDAOBuilder[M]{}
}

// SetLogger sets the logger. This is mandatory.
func (b *GenericDAOBuilder[M]) SetLogger(value *slog.Logger) *GenericDAOBuilder[M] {
	b.logger = value
	return b
}

// SetPool sets the database connection pool. This is mandatory.
func (b *GenericDAOBuilder[M]) SetPool(value *pgxpool.Pool) *GenericDAOBuilder[M] {
	b.pool = value
	return b
}

// SetTable sets the table name. This is mandatory.
func (b *GenericDAOBuilder[M]) SetTable(value string) *GenericDAOBuilder[M] {
	b.table = value
	return b
}

// Build creates a new generic DAO using the configuration stored in the builder.
func (b *GenericDAOBuilder[M]) Build() (result *GenericDAO[M], err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.pool == nil {
		err = errors.New("database connection pool is mandatory")
		return
	}
	if b.table == "" {
		err = errors.New("table is mandatory")
		return
	}

	// Find the reflection type of the generic parameter, so that we can allocate instances when needed:
	var object M
	reflectMsg := object.ProtoReflect()

	// Check that the object has an `id` field, and save it for future use:
	reflectId := object.ProtoReflect().Type().Descriptor().Fields().ByName("id")
	if reflectId == nil {
		err = fmt.Errorf("object of type '%T' doesn't have an identifier field", object)
		return
	}

	// Create and populate the object:
	result = &GenericDAO[M]{
		logger:     b.logger,
		table:      b.table,
		pool:       b.pool,
		reflectMsg: reflectMsg,
		reflectId:  reflectId,
	}
	return
}

// List retrieves all rows from the table and deserializes them into a slice of messages.
func (d *GenericDAO[M]) List(ctx context.Context) (results []M, err error) {
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
	query := fmt.Sprintf("select data from %s", d.table)
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return
	}
	var tmp []M
	for rows.Next() {
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			return
		}
		result := d.reflectMsg.New().Interface().(M)
		err = protojson.Unmarshal(data, result)
		if err != nil {
			return
		}
		tmp = append(tmp, result)
	}
	err = rows.Err()
	if err != nil {
		return
	}
	results = tmp
	return
}

// Get retrieves a single row by its identifier and deserializes it into a message. Returns nil and no error if there
// is no row with the given identifier.
func (d *GenericDAO[M]) Get(ctx context.Context, id string) (result M, err error) {
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
	query := fmt.Sprintf("select data from %s where id = $1", d.table)
	row := tx.QueryRow(ctx, query, id)
	var data []byte
	err = row.Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
		return
	}
	tmp := d.reflectMsg.New().Interface().(M)
	err = protojson.Unmarshal(data, tmp)
	if err != nil {
		return
	}
	result = tmp
	return
}

// Exists checks if a row with the given identifiers exists. Returns false and no error if there is no row with the
// given identifier.
func (d *GenericDAO[M]) Exists(ctx context.Context, id string) (ok bool, err error) {
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
	query := fmt.Sprintf("select count(*) from %s where id = $1", d.table)
	row := tx.QueryRow(ctx, query, id)
	var count int
	err = row.Scan(&count)
	if err != nil {
		return
	}
	ok = count > 0
	return
}

// Insert adds a new row to the table with a generated identifier and serialized data.
func (d *GenericDAO[M]) Insert(ctx context.Context, object M) (id string, err error) {
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
	tmp := uuid.NewString()
	object.ProtoReflect().Set(d.reflectId, protoreflect.ValueOfString(tmp))

	// Insert the row:
	data, err := protojson.Marshal(object)
	if err != nil {
		return
	}
	query := fmt.Sprintf("insert into %s (id, data) values ($1, $2)", d.table)
	_, err = tx.Exec(ctx, query, tmp, data)
	if err != nil {
		return
	}
	id = tmp
	return
}

// Update modifies an existing row in the table by its identifier with the result of serializing the provided object.
func (d *GenericDAO[M]) Update(ctx context.Context, id string, object M) error {
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
	data, err := protojson.Marshal(object)
	if err != nil {
		return err
	}
	query := fmt.Sprintf("update %s set data = $1 where id = $2", d.table)
	_, err = tx.Exec(ctx, query, data, id)
	return err
}

// Delete removes a row from the table by its identifier.
func (d *GenericDAO[M]) Delete(ctx context.Context, id string) error {
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
	query := fmt.Sprintf("delete from %s where id = $1", d.table)
	_, err = tx.Exec(ctx, query)
	return err
}
