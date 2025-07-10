/*
Copyright (c) 2025 Red Hat Inc.

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

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ = Describe("Notifier", func() {
	const channel = "my_channel"

	var (
		ctx  context.Context
		pool *pgxpool.Pool
		tm   TxManager
	)

	BeforeEach(func() {
		var err error

		// Create a context:
		ctx = context.Background()

		// Prepare the database pool:
		db := dbServer.MakeDatabase()
		DeferCleanup(db.Close)
		pool, err = pgxpool.New(ctx, db.MakeURL())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(pool.Close)

		// Create the notifications table:
		_, err = pool.Exec(
			ctx,
			`
				create table notifications (
					id text not null primary key,
					creation_timestamp timestamp with time zone default now(),
					payload bytea
				);
				`,
		)
		Expect(err).ToNot(HaveOccurred())

		// Prepare the transaction manager:
		tm, err = NewTxManager().
			SetLogger(logger).
			SetPool(pool).
			Build()
		Expect(err).ToNot(HaveOccurred())
	})

	// runWithTx starts a transaction, runs the given function using it, and ends the transaction when it finishes.
	runWithTx := func(task func(ctx context.Context)) {
		tx, err := tm.Begin(ctx)
		Expect(err).ToNot(HaveOccurred())
		taskCtx := TxIntoContext(ctx, tx)
		task(taskCtx)
		err = tm.End(ctx, tx)
		Expect(err).ToNot(HaveOccurred())
	}

	Describe("Creation", func() {
		It("Can be created when all the required parameters are set", func() {
			notifier, err := NewNotifier().
				SetLogger(logger).
				SetChannel(channel).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(notifier).ToNot(BeNil())
		})

		It("Can't be created without a logger", func() {
			notifier, err := NewNotifier().
				SetChannel(channel).
				Build()
			Expect(err).To(MatchError("logger is mandatory"))
			Expect(notifier).To(BeNil())
		})

		It("Can't be created without a channel", func() {
			notifier, err := NewNotifier().
				SetLogger(logger).
				Build()
			Expect(err).To(MatchError("channel is mandatory"))
			Expect(notifier).To(BeNil())
		})
	})

	Describe("Behaviour", func() {
		It("Sends notification", func() {
			// Create the notifier:
			notifier, err := NewNotifier().
				SetLogger(logger).
				SetChannel(channel).
				Build()
			Expect(err).ToNot(HaveOccurred())

			// Send the notification:
			payload := wrapperspb.Int32(42)
			runWithTx(func(ctx context.Context) {
				err = notifier.Notify(ctx, payload)
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Deletes old notifications", func() {
			// Create the notifier:
			notifier, err := NewNotifier().
				SetLogger(logger).
				SetChannel(channel).
				Build()
			Expect(err).ToNot(HaveOccurred())

			// Manually create notifications older than one minute:
			_, err = pool.Exec(
				ctx,
				`
				insert into notifications (id, creation_timestamp, payload) values
				('123', now() - interval '2 minutes', 'junk'),
				('456', now() - interval '10 minutes', 'stuff')
				`,
			)
			Expect(err).ToNot(HaveOccurred())

			// Send the notification:
			payload := wrapperspb.Int32(42)
			runWithTx(func(ctx context.Context) {
				err = notifier.Notify(ctx, payload)
			})
			Expect(err).ToNot(HaveOccurred())

			// Check that the old notifications have been deleted:
			row := pool.QueryRow(ctx, `select count(*) from notifications where id in ('123', '456')`)
			var count int
			err = row.Scan(&count)
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(BeZero())
		})
	})
})
