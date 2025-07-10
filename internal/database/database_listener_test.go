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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ = Describe("Listener", func() {
	const channel = "my_channel"

	Describe("Creation", func() {
		// This doesn't need to be a working URL, just enough to be able to create the object.
		const url = "postgresql://myserver/mydb"

		// A payload callback that does nothing.
		nothing := func(ctx context.Context, payload proto.Message) error {
			return nil
		}

		It("Can be created when all the required parameters are set", func() {
			listener, err := NewListener().
				SetLogger(logger).
				SetUrl(url).
				SetChannel(channel).
				AddPayloadCallback(nothing).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(listener).ToNot(BeNil())
		})

		It("Can't be created without a logger", func() {
			listener, err := NewListener().
				SetChannel(channel).
				SetUrl(url).
				AddPayloadCallback(nothing).
				Build()
			Expect(err).To(MatchError("logger is mandatory"))
			Expect(listener).To(BeNil())
		})

		It("Can't be created without a channel", func() {
			listener, err := NewListener().
				SetLogger(logger).
				SetUrl(url).
				AddPayloadCallback(nothing).
				Build()
			Expect(err).To(MatchError("channel is mandatory"))
			Expect(listener).To(BeNil())
		})

		It("Can't be created without database connection URL", func() {
			listener, err := NewListener().
				SetLogger(logger).
				SetChannel(channel).
				AddPayloadCallback(nothing).
				Build()
			Expect(err).To(MatchError("database connection URL is mandatory"))
			Expect(listener).To(BeNil())
		})

		It("Can't be created without at least one payload callback", func() {
			listener, err := NewListener().
				SetLogger(logger).
				SetChannel(channel).
				SetUrl(url).
				Build()
			Expect(err).To(MatchError("at least one payload callback is mandatory"))
			Expect(listener).To(BeNil())
		})

		It("Checks that wait timeout is positive", func() {
			listener, err := NewListener().
				SetLogger(logger).
				SetChannel(channel).
				SetUrl(url).
				AddPayloadCallback(nothing).
				SetWaitTimeout(-1 * time.Second).
				Build()
			Expect(err).To(MatchError("wait timeout should be positive, but it is -1s"))
			Expect(listener).To(BeNil())
		})

		It("Checks that retry interval is positive", func() {
			listener, err := NewListener().
				SetLogger(logger).
				SetChannel(channel).
				SetUrl(url).
				AddPayloadCallback(nothing).
				SetRetryInterval(-1 * time.Second).
				Build()
			Expect(err).To(MatchError("retry interval should be positive, but it is -1s"))
			Expect(listener).To(BeNil())
		})
	})

	Describe("Behaviour", func() {
		var (
			ctx      context.Context
			tm       TxManager
			payloads chan proto.Message
			listener *Listener
			notifier *Notifier
		)

		BeforeEach(func() {
			var err error

			// Create a cancelable context that will be used to stop the listener:
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(context.Background())
			DeferCleanup(cancel)

			// Prepare the database pool:
			db := dbServer.MakeDatabase()
			DeferCleanup(db.Close)
			pool, err := pgxpool.New(ctx, db.MakeURL())
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

			// Create the payloads channel:
			payloads = make(chan proto.Message)

			// Create the listener and wait till it is ready:
			ready := make(chan struct{})
			listener, err = NewListener().
				SetLogger(logger).
				SetUrl(db.MakeURL()).
				SetChannel(channel).
				SetWaitTimeout(100 * time.Millisecond).
				SetRetryInterval(10 * time.Millisecond).
				AddReadyCallback(func(ctx context.Context) error {
					close(ready)
					return nil
				}).
				AddPayloadCallback(func(ctx context.Context, payload proto.Message) error {
					payloads <- payload
					return nil
				}).
				Build()
			Expect(err).ToNot(HaveOccurred())
			go func() {
				defer GinkgoRecover()
				err := listener.Listen(ctx)
				Expect(err).To(MatchError(context.Canceled))
			}()
			Eventually(ready).Should(BeClosed())

			// Create the notifier:
			notifier, err = NewNotifier().
				SetLogger(logger).
				SetChannel(channel).
				Build()
			Expect(err).ToNot(HaveOccurred())

			// Prepare the transaction manager:
			tm, err = NewTxManager().
				SetLogger(logger).
				SetPool(pool).
				Build()
			Expect(err).ToNot(HaveOccurred())
		})

		// runWithTx starts a transaction, runs the given function using it, and ends the transaction when it
		// finishes.
		runWithTx := func(task func(ctx context.Context)) {
			tx, err := tm.Begin(ctx)
			Expect(err).ToNot(HaveOccurred())
			taskCtx := TxIntoContext(ctx, tx)
			task(taskCtx)
			err = tm.End(ctx, tx)
			Expect(err).ToNot(HaveOccurred())
		}

		It("Receives one notification", func() {
			var err error
			sent := wrapperspb.String("my payload")
			runWithTx(func(ctx context.Context) {
				err = notifier.Notify(ctx, sent)
			})
			Expect(err).ToNot(HaveOccurred())
			var received *wrapperspb.StringValue
			Eventually(payloads).Should(Receive(&received))
			Expect(proto.Equal(received, sent)).To(BeTrue())
		})

		It("Receives multiple notifications", func() {
			var err error
			sent := []string{
				"cero",
				"uno",
				"dos",
				"tres",
				"cuatro",
				"cinco",
				"seis",
				"siete",
				"ocho",
				"nueve",
			}
			for _, value := range sent {
				payload := wrapperspb.String(value)
				runWithTx(func(ctx context.Context) {
					err = notifier.Notify(ctx, payload)
				})
				Expect(err).ToNot(HaveOccurred())
			}
			var received []string
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()
		loop:
			for {
				select {
				case <-ctx.Done():
				case payload := <-payloads:
					wrapper := payload.(*wrapperspb.StringValue)
					received = append(received, wrapper.Value)
					if len(received) == len(sent) {
						break loop
					}
				}
			}
			Expect(received).To(Equal(sent))
		})
	})
})
