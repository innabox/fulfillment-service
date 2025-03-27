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
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GenericDAOBuilder is a builder for creating generic data access objects.
type GenericDAOBuilder[M proto.Message] struct {
	logger       *slog.Logger
	table        string
	defaultOrder string
	defaultLimit int32
	maxLimit     int32
}

// GenericDAO provides generic data access operations for protocol buffers messages. It assumes that objects will be
// stored in tables with two columns:
//
//   - `id` - The unique identifier of the object.
//   - `data` - The serialized object, using the protocol buffers JSON serialization.
//
// Objects must have field named `id` of string type.
type GenericDAO[M proto.Message] struct {
	logger       *slog.Logger
	table        string
	defaultOrder string
	defaultLimit int32
	maxLimit     int32
	reflectMsg   protoreflect.Message
	reflectId    protoreflect.FieldDescriptor
}

// NewGenericDAO creates a builder that can then be used to configure and create a generic DAO.
func NewGenericDAO[M proto.Message]() *GenericDAOBuilder[M] {
	return &GenericDAOBuilder[M]{
		defaultLimit: 100,
		maxLimit:     1000,
	}
}

// SetLogger sets the logger. This is mandatory.
func (b *GenericDAOBuilder[M]) SetLogger(value *slog.Logger) *GenericDAOBuilder[M] {
	b.logger = value
	return b
}

// SetTable sets the table name. This is mandatory.
func (b *GenericDAOBuilder[M]) SetTable(value string) *GenericDAOBuilder[M] {
	b.table = value
	return b
}

// SetDefaultOrder sets the default order criteria to use when nothing has been requested by the user. This is optional
// and the default is no order. This is intended only for use in unit tests, where it is convenient to have some
// predictable ordering.
func (b *GenericDAOBuilder[M]) SetDefaultOrder(value string) *GenericDAOBuilder[M] {
	b.defaultOrder = value
	return b
}

// SetDefaultLimit sets the default number of items returned. It will be used when the value of the limit parameter
// of the list request is zero. This is optional, and the default is 100.
func (b *GenericDAOBuilder[M]) SetDefaultLimit(value int) *GenericDAOBuilder[M] {
	b.defaultLimit = int32(value)
	return b
}

// SetMaxLimit sets the maximum number of items returned. This is optional and the default value is 1000.
func (b *GenericDAOBuilder[M]) SetMaxLimit(value int) *GenericDAOBuilder[M] {
	b.maxLimit = int32(value)
	return b
}

// Build creates a new generic DAO using the configuration stored in the builder.
func (b *GenericDAOBuilder[M]) Build() (result *GenericDAO[M], err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.table == "" {
		err = errors.New("table is mandatory")
		return
	}
	if b.defaultLimit <= 0 {
		err = fmt.Errorf("default limit must be a possitive integer, but it is %d", b.defaultLimit)
		return
	}
	if b.maxLimit <= 0 {
		err = fmt.Errorf("max limit must be a possitive integer, but it is %d", b.maxLimit)
		return
	}
	if b.maxLimit < b.defaultLimit {
		err = fmt.Errorf(
			"max limit must be greater or equal to default limit, but max limit is %d and default limit "+
				"is %d",
			b.maxLimit, b.defaultLimit,
		)
		return
	}

	// Find the reflection type of the generic parameter, so that we can allocate instances when needed:
	var object M
	reflectMsg := object.ProtoReflect()

	// Check that the object has an `id` field, and save it for future use:
	reflectId := reflectMsg.Type().Descriptor().Fields().ByName("id")
	if reflectId == nil {
		err = fmt.Errorf("object of type '%T' doesn't have an identifier field", object)
		return
	}

	// Create and populate the object:
	result = &GenericDAO[M]{
		logger:       b.logger,
		table:        b.table,
		defaultOrder: b.defaultOrder,
		defaultLimit: b.defaultLimit,
		maxLimit:     b.maxLimit,
		reflectMsg:   reflectMsg,
		reflectId:    reflectId,
	}
	return
}

// ListRequest represents the parameters for paginated queries.
type ListRequest struct {
	// Offset specifies the starting point.
	Offset int32

	// Limit specifies the maximum number of items.
	Limit int32
}

// ListResponse represents the result of a paginated query.
type ListResponse[I any] struct {
	// Size is the actual number of items returned.
	Size int32

	// Total is the total number of items available.
	Total int32

	// Items is the list of items.
	Items []I
}

// List retrieves all rows from the table and deserializes them into a slice of messages.
func (d *GenericDAO[M]) List(ctx context.Context, request ListRequest) (response ListResponse[M], err error) {
	// Start a transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Calculate the order cluase:
	var order string
	if d.defaultOrder != "" {
		order = fmt.Sprintf("order by %s", d.defaultOrder)
	}

	// Calculate the offset:
	offset := request.Offset
	if offset < 0 {
		offset = 0
	}

	// Calculate the limit:
	limit := request.Limit
	if limit < 0 {
		limit = 0
	} else if limit == 0 {
		limit = d.defaultLimit
	} else if limit > d.maxLimit {
		limit = d.maxLimit
	}

	// Count the total number of results, disregarding the offset and the limit:
	totalQuery := fmt.Sprintf("select count(*) from %s", d.table)
	row := tx.QueryRow(ctx, totalQuery)
	var total int
	err = row.Scan(&total)
	if err != nil {
		return
	}

	// Fetch the results:
	itemsQuery := fmt.Sprintf("select data from %s %s offset $1 limit $2", d.table, order)
	rows, err := tx.Query(ctx, itemsQuery, offset, limit)
	if err != nil {
		return
	}
	var items []M
	for rows.Next() {
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			return
		}
		item := d.reflectMsg.New().Interface().(M)
		err = protojson.Unmarshal(data, item)
		if err != nil {
			return
		}
		items = append(items, item)
	}
	err = rows.Err()
	if err != nil {
		return
	}
	response.Size = int32(len(items))
	response.Total = int32(total)
	response.Items = items
	return
}

// Get retrieves a single row by its identifier and deserializes it into a message. Returns nil and no error if there
// is no row with the given identifier.
func (d *GenericDAO[M]) Get(ctx context.Context, id string) (result M, err error) {
	// Start a transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

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
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

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
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

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
func (d *GenericDAO[M]) Update(ctx context.Context, id string, object M) (err error) {
	// Start a transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Update the row:
	data, err := protojson.Marshal(object)
	if err != nil {
		return
	}
	query := fmt.Sprintf("update %s set data = $1 where id = $2", d.table)
	_, err = tx.Exec(ctx, query, data, id)
	return
}

// Delete removes a row from the table by its identifier.
func (d *GenericDAO[M]) Delete(ctx context.Context, id string) (err error) {
	// Start a transaction:
	tx, err := database.TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Delete the row:
	query := fmt.Sprintf("delete from %s where id = $1", d.table)
	_, err = tx.Exec(ctx, query)
	return
}
