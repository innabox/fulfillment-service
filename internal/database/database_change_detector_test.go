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
	"context"
	"database/sql"
	"time"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"

	. "github.com/innabox/fulfillment-service/internal/testing"
)

var _ = Describe("Change detector", func() {
	var (
		ctx      context.Context
		dbObject *Database
		dbURL    string
		dbHandle *sql.DB
		zeroTime time.Time
	)

	BeforeEach(func() {
		// Create a context:
		ctx = context.Background()

		// Create a database:
		dbObject = dbServer.MakeDatabase()
		dbURL = dbObject.MakeURL()
		dbHandle = dbObject.MakeHandle()

		// Create the data table:
		_, err := dbHandle.Exec(`
			create table my_data (
				id integer not null primary key,
				name text not null
			);

			create table my_changes (
				serial serial not null primary key,
				timestamp timestamp with time zone not null default now(),
				source text,
				operation text,
				old jsonb,
				new jsonb
			);

			create function save_my_changes() returns trigger as $$
			begin
				insert into my_changes (
					source,
					operation,
					old,
					new
				) values (
					tg_table_name,
					lower(tg_op),
					row_to_json(old.*),
					row_to_json(new.*)
				);
				notify my_changes;
				return null;
			end;
			$$ language plpgsql;

			create trigger save_my_changes
			after insert or update or delete on my_data
			for each row execute function save_my_changes();
		`)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		// Close the database handle:
		err := dbHandle.Close()
		Expect(err).ToNot(HaveOccurred())

		// Close the database server:
		dbObject.Close()
	})

	// nopCallback is a callback that doesn nothing with the change.
	var nopCallback = func(ctx context.Context, change *Change) {
		// Do nothing, as the name says.
	}

	It("Can't be created without a logger", func() {
		_, err := NewChangeDetector().
			SetName("my").
			SetURL(dbURL).
			SetCallback(nopCallback).
			Build()
		Expect(err).To(HaveOccurred())
		message := err.Error()
		Expect(message).To(ContainSubstring("logger"))
		Expect(message).To(ContainSubstring("mandatory"))
	})

	It("Can't be created without a database URL", func() {
		_, err := NewChangeDetector().
			SetLogger(logger).
			SetName("my").
			SetCallback(nopCallback).
			Build()
		Expect(err).To(HaveOccurred())
		message := err.Error()
		Expect(message).To(ContainSubstring("database"))
		Expect(message).To(ContainSubstring("URL"))
		Expect(message).To(ContainSubstring("mandatory"))
	})

	It("Can't be created with zero interval", func() {
		_, err := NewChangeDetector().
			SetLogger(logger).
			SetName("my").
			SetURL(dbURL).
			SetCallback(nopCallback).
			SetInterval(0).
			Build()
		Expect(err).To(HaveOccurred())
		message := err.Error()
		Expect(message).To(ContainSubstring("interval"))
		Expect(message).To(ContainSubstring("greater or equal than zero"))
	})

	It("Can't be created with zero timeout", func() {
		_, err := NewChangeDetector().
			SetLogger(logger).
			SetName("my").
			SetURL(dbURL).
			SetCallback(nopCallback).
			SetTimeout(0).
			Build()
		Expect(err).To(HaveOccurred())
		message := err.Error()
		Expect(message).To(ContainSubstring("timeout"))
		Expect(message).To(ContainSubstring("greater or equal than zero"))
	})

	It("Processes insert, update and delete", func() {
		// Create a callback function that stores the changes in a list:
		var changes []*Change
		done := make(chan struct{})
		callback := func(ctx context.Context, item *Change) {
			changes = append(changes, item)
			if len(changes) == 3 {
				close(done)
			}
		}

		// Create the detector:
		detector, err := NewChangeDetector().
			SetLogger(logger).
			SetName("my").
			SetURL(dbURL).
			SetCallback(callback).
			SetInterval(100 * time.Millisecond).
			Build()
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := detector.Stop(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()
		go func() {
			defer GinkgoRecover()
			err := detector.Start(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()

		// Insert:
		_, err = dbHandle.Exec(`insert into my_data (id, name) values (123, 'my_name')`)
		Expect(err).ToNot(HaveOccurred())

		// Update:
		_, err = dbHandle.Exec(`update my_data set name = 'your_name' where id = 123`)
		Expect(err).ToNot(HaveOccurred())

		// Delete:
		_, err = dbHandle.Exec(`delete from my_data where id = 123`)
		Expect(err).ToNot(HaveOccurred())

		// Wait for changes to be processed:
		Eventually(done).Should(BeClosed())

		// We need to set the timestamps to zero because otherwise it is complicated to compare the results
		// with the expected values.
		for _, changeItem := range changes {
			changeItem.Timestamp = zeroTime
		}

		// Check the insert:
		insertChange := changes[0]
		Expect(insertChange).To(Equal(&Change{
			Serial:    1,
			Source:    "my_data",
			Operation: "insert",
			Old:       nil,
			New: map[string]any{
				"id":   123.0,
				"name": "my_name",
			},
		}))

		// Check the update:
		updateChange := changes[1]
		Expect(updateChange).To(Equal(&Change{
			Serial:    2,
			Source:    "my_data",
			Operation: "update",
			Old: map[string]any{
				"id":   123.0,
				"name": "my_name",
			},
			New: map[string]any{
				"id":   123.0,
				"name": "your_name",
			},
		}))

		// Check the delete:
		deleteChange := changes[2]
		Expect(deleteChange).To(Equal(&Change{
			Serial:    3,
			Source:    "my_data",
			Operation: "delete",
			Old: map[string]any{
				"id":   123.0,
				"name": "your_name",
			},
			New: nil,
		}))
	})

	It("Processes changes made before creation", func() {
		// Make a change:
		_, err := dbHandle.Exec(`insert into my_data (id, name) values (123, 'my_name')`)
		Expect(err).ToNot(HaveOccurred())

		// Create a callback that stores the change in a variable:
		var change *Change
		done := make(chan struct{})
		callback := func(ctx context.Context, item *Change) {
			change = item
			close(done)
		}

		// Create the detector:
		detector, err := NewChangeDetector().
			SetLogger(logger).
			SetName("my").
			SetURL(dbURL).
			SetCallback(callback).
			SetInterval(100 * time.Millisecond).
			Build()
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := detector.Stop(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()
		go func() {
			defer GinkgoRecover()
			err := detector.Start(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()

		// Wait for the changes to be processed:
		Eventually(done).Should(BeClosed())

		// We need to clear the timestamp because otherwise it is difficult to compare to the expected value:
		change.Timestamp = zeroTime

		// Check the change:
		Expect(change).To(Equal(&Change{
			Serial:    1,
			Source:    "my_data",
			Operation: "insert",
			Old:       nil,
			New: map[string]any{
				"id":   123.0,
				"name": "my_name",
			},
		}))
	})

	It("Process changes made after creation", func() {
		// Create a callback that stores changes in a list:
		var change *Change
		done := make(chan struct{})
		callback := func(ctx context.Context, item *Change) {
			change = item
			close(done)
		}

		// Create the detector:
		detector, err := NewChangeDetector().
			SetLogger(logger).
			SetName("my").
			SetURL(dbURL).
			SetCallback(callback).
			SetInterval(100 * time.Millisecond).
			Build()
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := detector.Stop(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()
		go func() {
			defer GinkgoRecover()
			err := detector.Start(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()

		// Do a change:
		_, err = dbHandle.Exec(`insert into my_data (id, name) values (123, 'my_name')`)
		Expect(err).ToNot(HaveOccurred())

		// Wait for the changes to be processed:
		Eventually(done).Should(BeClosed())

		// We need to clear the timestamp because otherwise it is difficult to compare to the expected value:
		change.Timestamp = zeroTime

		// Check the change:
		Expect(change).To(Equal(&Change{
			Serial:    1,
			Source:    "my_data",
			Operation: "insert",
			Old:       nil,
			New: map[string]any{
				"id":   123.0,
				"name": "my_name",
			},
		}))
	})

	It("Passes context to callback", func() {
		// Add a value to the context:
		type Key int
		key := Key(0)
		ctx = context.WithValue(ctx, key, "my_value")

		// Create a callback that saves the context value:
		done := make(chan struct{})
		var value any
		callback := func(ctx context.Context, item *Change) {
			value = ctx.Value(key)
			close(done)
		}

		// Create the detector:
		detector, err := NewChangeDetector().
			SetLogger(logger).
			SetName("my").
			SetURL(dbURL).
			SetCallback(callback).
			SetInterval(100 * time.Millisecond).
			Build()
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := detector.Stop(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()
		go func() {
			err := detector.Start(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()

		// Do a change:
		_, err = dbHandle.Exec(`insert into my_data (id, name) values (123, 'my_name')`)
		Expect(err).ToNot(HaveOccurred())

		// Wait for the changes to be processed:
		Eventually(done).Should(BeClosed())

		// Check the values:
		Expect(value).To(Equal("my_value"))
	})
})
