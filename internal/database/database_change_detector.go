/*
Copyright (c) 2025 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package database

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ChangeDetectorBuilder contains the data and logic needed to build a database change detector.
type ChangeDetectorBuilder struct {
	logger   *slog.Logger
	url      string
	name     string
	callback func(context.Context, *Change)
	interval time.Duration
	timeout  time.Duration
}

// ChangeDetector is a mechanism to detect changes made to database tables. It uses triggers that write to a changes
// table the details of the changes made to other tables. It then waits for data written to that changes table and
// processes it using the callback given in the configuration.
//
// To use it you will need to create the following database objects in advance:
//
// 1. The table where the changes are stored. For example, if the name of the detector is `my`:
//
//	create table my_changes (
//		serial serial not null primary key,
//		timestamp timestamp with time zone not null default now(),
//		source text,
//		operation text,
//		old jsonb,
//		new jsonb
//	);
//
// 2. The trigger function that will be called by the triggers to write changes to the table:
//
//	create function save_my_changes() returns trigger as $$
//	begin
//		insert into my_changes (
//			source,
//			operation,
//			old,
//			new
//		) values (
//			tg_table_name,
//			lower(tg_op),
//			row_to_json(old.*),
//			row_to_json(new.*)
//		);
//		notify my_changes;
//		return null;
//	end;
//	$$ language plpgsql;
//
// 3. For each data table the trigger that calls the save function. For example, if the name of the data table is
// `my_data`:
//
//	create trigger my_data_save_changes
//	after insert or update or delete on my_data
//	for each row execute function save_my_changes();
type ChangeDetector struct {
	// Basic fields:
	logger        *slog.Logger
	url           string
	callback      func(context.Context, *Change)
	waitInterval  time.Duration
	retryInterval time.Duration
	timeout       time.Duration

	// We use the flag to ask the loop to stop as soon as possible, and then the loop uses the
	// channel to tell the close method that it actually finished.
	closeFlag int32
	closeChan chan struct{}

	// Database connection used to receive notifications.
	waitConn *pgx.Conn

	// Function to cancel the context used to run the current wait.
	waitCancel func()

	// Database connection used to fetch changes.
	fetchConn *pgx.Conn

	// Precalculated SQL for frequently used statements:
	fetchSQL  string
	listenSQL string
}

// Change contains the data of one single change.
type Change struct {
	Serial    int            `json:"serial"`
	Timestamp time.Time      `json:"timestamp"`
	Source    string         `json:"source"`
	Operation string         `json:"operation"`
	Old       map[string]any `json:"old"`
	New       map[string]any `json:"new"`
}

// changeRow is used to read rows from the database.
type changeRow struct {
	Serial    int
	Timestamp time.Time
	Source    string
	Operation string
	Old       []byte
	New       []byte
}

// NewChangeDetector creates a builder that can then be used to configure and create a change detector. Note that the
// required database objects (table, functions and triggers) need to be created in advance.
func NewChangeDetector() *ChangeDetectorBuilder {
	return &ChangeDetectorBuilder{
		interval: defaultChangeDetectorInterval,
		timeout:  defaultChangeDetectorTimeout,
	}
}

// SetLogger sets the logger that the detector will use to write to the log. This is mandatory.
func (b *ChangeDetectorBuilder) SetLogger(value *slog.Logger) *ChangeDetectorBuilder {
	b.logger = value
	return b
}

// SetURL sets the database URL that the detector will use connect to the database. This is mandatory.
func (b *ChangeDetectorBuilder) SetURL(value string) *ChangeDetectorBuilder {
	b.url = value
	return b
}

// SetName sets the name of the detector. This can be used to have different detectors for different uses, or for
// different sets of tables, or for different environments that happen to share the database. The name will be used as
// prefix for the database objects, for example, if the name is `my` then the names of the database table and channel
// will be `my_changes`. This is optional and the default is to use no prefix, so the default names for the database and
// the channel are just `changes`.
func (b *ChangeDetectorBuilder) SetName(value string) *ChangeDetectorBuilder {
	b.name = value
	return b
}

// SetCallback sets the function that will be called to process changes. This function will be called in the same
// goroutine that reads the item from the database, so processing of later changes will not proceed till this returns.
func (b *ChangeDetectorBuilder) SetCallback(value func(context.Context, *Change)) *ChangeDetectorBuilder {
	b.callback = value
	return b
}

// SetInterval sets the iterval for periodic checks. Usually changes from will be processed inmediately because the
// database sends notifications when new changes are available. If that fails, for whatever the raeson, changes will be
// processed after this interval. Default value is 30 seconds.
func (b *ChangeDetectorBuilder) SetInterval(value time.Duration) *ChangeDetectorBuilder {
	b.interval = value
	return b
}

// SetTimeout sets the timeout for database operations. The default is one second.
func (b *ChangeDetectorBuilder) SetTimeout(value time.Duration) *ChangeDetectorBuilder {
	b.timeout = value
	return b
}

// Build uses the data stored in the builder to configure and create a new change detector.
func (b *ChangeDetectorBuilder) Build() (result *ChangeDetector, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.url == "" {
		err = errors.New("database URL is mandatory")
		return
	}
	if b.callback == nil {
		err = errors.New("callback function is mandatory")
		return
	}
	if b.interval <= 0 {
		err = fmt.Errorf(
			"check interval %s isn't valid, should be greater or equal than zero",
			b.interval,
		)
		return
	}
	if b.timeout <= 0 {
		err = fmt.Errorf(
			"timeout %s isn't valid, should be greater or equal than zero",
			b.timeout,
		)
		return
	}

	// Calculate specific intervals from the general interval given in the configuration:
	waitInterval := b.interval
	retryInterval := b.interval / 10

	// Calculate the SQL for frequently used statements:
	prefix := b.name
	if prefix != "" && !strings.HasSuffix(prefix, "_") {
		prefix += "_"
	}
	type TemplateData struct {
		Prefix string
	}
	templateData := &TemplateData{
		Prefix: prefix,
	}
	fetchSQL, err := evaluateTemplate(changeDetectorFetchTemplate, templateData)
	if err != nil {
		return
	}
	listenSQL, err := evaluateTemplate(changeDetectorListenTemplate, templateData)
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &ChangeDetector{
		logger:        b.logger,
		url:           b.url,
		callback:      b.callback,
		waitInterval:  waitInterval,
		retryInterval: retryInterval,
		timeout:       b.timeout,
		fetchSQL:      fetchSQL,
		listenSQL:     listenSQL,
		closeFlag:     0,
		closeChan:     make(chan struct{}),
	}
	return
}

// Start starts processing the queue and blocks till it is explicitly stopped.
func (q *ChangeDetector) Start(ctx context.Context) error {
	for !q.closing() {
		// Check for pending changes:
		q.check(ctx)

		// Wait for notifications. It is normal if this finishes with a timeout. Any other error isn't normal
		// and we should wait a bit before trying again to avoid too many attempts when the error isn't resolved
		// quickly.
		err := q.wait(ctx)
		if err != nil {
			q.logger.InfoContext(
				ctx,
				"Wait failed, will wait a bit before trying again",
				slog.String("error", err.Error()),
				slog.Duration("delay", q.retryInterval),
			)
			time.Sleep(q.retryInterval)
		}
	}

	// Let the close method know that we finished:
	close(q.closeChan)

	return nil
}

// wait waits for the next notification from the database is received or the operation times out.
func (q *ChangeDetector) wait(ctx context.Context) error {
	var err error

	// Start listening if needed:
	if q.waitConn == nil {
		q.waitConn, err = pgx.Connect(ctx, q.url)
		if err != nil {
			return err
		}
		_, err = q.waitConn.Exec(ctx, q.listenSQL)
		if err != nil {
			return err
		}
	}

	// Wait for a new notification. We set a timeout so that we will process pending changes periodically even if
	// the notification mechanism fails. If the wait results in an error other than a timeout then we will close and
	// discard the connection, so that we recover from database restarts and other errors that may make the
	// connection unusable.
	var waitCtx context.Context
	waitCtx, q.waitCancel = context.WithTimeout(ctx, q.waitInterval)
	defer q.waitCancel()
	_, err = q.waitConn.WaitForNotification(waitCtx)
	if pgconn.Timeout(err) {
		err = nil
	}
	if err != nil {
		q.logger.DebugContext(
			ctx,
			"Wait failed, will close the connection",
			slog.String("error", err.Error()),
		)
		closeErr := q.waitConn.Close(ctx)
		if closeErr != nil {
			q.logger.ErrorContext(
				ctx,
				"Failed to close connection",
				slog.String("error", err.Error()),
			)
		}
		q.waitConn = nil
		return err
	}

	return nil
}

// check checks the contents of the changes table and process them.
func (q *ChangeDetector) check(ctx context.Context) {
	// Fetch and process all the available changes.
	for !q.closing() {
		// Fetch the next available row:
		found, row, err := q.fetch(ctx)
		if err != nil {
			q.logger.ErrorContext(
				ctx,
				"Failed to fetch change",
				slog.String("error", err.Error()),
			)
			return
		}
		if !found {
			break
		}

		// Process the change:
		q.logger.DebugContext(
			ctx,
			"Processing change",
			slog.Int("serial", row.Serial),
		)
		var old map[string]any
		if row.Old != nil {
			err = json.Unmarshal(row.Old, &old)
			if err != nil {
				q.logger.ErrorContext(
					ctx,
					"Failed to unmarshal old data",
					slog.String("error", err.Error()),
				)
				return
			}
		}
		var new map[string]any
		if row.New != nil {
			err = json.Unmarshal(row.New, &new)
			if err != nil {
				q.logger.ErrorContext(
					ctx,
					"Failed to unmarshal old data",
					slog.String("error", err.Error()),
				)
				return
			}
		}
		change := &Change{
			Serial:    row.Serial,
			Timestamp: row.Timestamp,
			Source:    row.Source,
			Operation: row.Operation,
			Old:       old,
			New:       new,
		}
		q.callback(ctx, change)
	}
}

// fetch tries to read the next row from the changes table. It returns a boolean flag indicating if there was such a row
// and the row itself.
func (q *ChangeDetector) fetch(ctx context.Context) (found bool, result *changeRow,
	err error) {
	// Create the connection if needed:
	if q.fetchConn == nil {
		q.fetchConn, err = pgx.Connect(ctx, q.url)
		if err != nil {
			return
		}
	}

	// Run the query:
	queryCtx, queryCancel := context.WithTimeout(ctx, q.timeout)
	defer queryCancel()
	row := q.fetchConn.QueryRow(queryCtx, q.fetchSQL)
	var tmp changeRow
	err = row.Scan(
		&tmp.Serial,
		&tmp.Timestamp,
		&tmp.Source,
		&tmp.Operation,
		&tmp.Old,
		&tmp.New,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
		return
	}
	if pgconn.Timeout(err) {
		err = nil
		return
	}
	if err != nil {
		q.logger.DebugContext(
			ctx,
			"Fetch failed, will close the connection",
			slog.String("error", err.Error()),
		)
		closeErr := q.fetchConn.Close(ctx)
		if closeErr != nil {
			q.logger.InfoContext(
				ctx,
				"Failed to close connection",
				slog.String("error", err.Error()),
			)
		}
		q.fetchConn = nil
		err = nil
	}
	found = true
	result = &tmp
	return
}

func evaluateTemplate(source string, data any) (result string,
	err error) {
	// Parse the template:
	tmpl, err := template.New("").Parse(source)
	if err != nil {
		err = fmt.Errorf("failed to parse template '%s': %v", source, err)
		return
	}

	// Execute the template:
	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, data)
	if err != nil {
		err = fmt.Errorf("failed to execute template '%s': %v", source, err)
		return
	}

	// Return the result:
	result = buffer.String()
	return
}

// Stop releases all the resources used by the detector. Note that the changes will continue to be written to the
// changes table even when the detector is closed, and that may continue to be processed by other detector objects
// running in a this or other processes.
func (q *ChangeDetector) Stop(ctx context.Context) error {
	// Raise the closing flag so that the loop will finish as soon as it checks it:
	q.close()

	// Cancel waiting and close the connection:
	if q.waitCancel != nil {
		q.waitCancel()
	}

	// Wait for the loop to finish:
	<-q.closeChan

	// Close the connection used for fetching changes:
	if q.fetchConn != nil {
		err := q.fetchConn.Close(ctx)
		if err != nil {
			q.logger.ErrorContext(
				ctx,
				"Failed to close connection",
				slog.String("error", err.Error()),
			)
		}
	}

	// Close the connection used for listening for notifications:
	if q.waitConn != nil {
		err := q.waitConn.Close(ctx)
		if err != nil {
			q.logger.ErrorContext(
				ctx,
				"Failed to close connection",
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// close asks the loop to stop as soon as possible.
func (q *ChangeDetector) close() {
	atomic.StoreInt32(&q.closeFlag, 1)
}

// closing returns true if we are in the process of closing.
func (q *ChangeDetector) closing() bool {
	return atomic.LoadInt32(&q.closeFlag) == 1
}

// template used to fetch the next change.
const changeDetectorFetchTemplate = `
delete from {{ .Prefix }}changes where serial = (
	select serial
	from {{ .Prefix }}changes
	order by serial
	for update
	skip locked
	limit 1
)
returning
	serial,
	timestamp,
	source,
	operation,
	old,
	new
`

// template used to listen for notifications.
const changeDetectorListenTemplate = `
listen {{ .Prefix }}changes
`

// Defaults for configuration settings:
const (
	defaultChangeDetectorInterval = 30 * time.Second
	defaultChangeDetectorTimeout  = 1 * time.Second
)
