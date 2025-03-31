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
	"encoding/base64"
	"errors"
	"log/slog"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// NotifierBuilder contains the data and logic needed to build a notifier.
type NotifierBuilder struct {
	logger  *slog.Logger
	channel string
}

// Notifier knows how to send notifications using PostgreSQL's NOTIFY command, using protocol buffers messages as
// payload.
type Notifier struct {
	logger  *slog.Logger
	channel string
}

// NewNotifier uses the information stored in the builder to create a new notifier.
func NewNotifier() *NotifierBuilder {
	return &NotifierBuilder{}
}

// SetLogger sets the logger for the notifier. This is mandatory.
func (b *NotifierBuilder) SetLogger(logger *slog.Logger) *NotifierBuilder {
	b.logger = logger
	return b
}

// SetLogger sets the channel name. This is mandatory.
func (b *NotifierBuilder) SetChannel(value string) *NotifierBuilder {
	b.channel = value
	return b
}

// Build constructs a notifier instance using the configured parameters.
func (b *NotifierBuilder) Build() (result *Notifier, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.channel == "" {
		err = errors.New("channel is mandatory")
		return
	}

	// Create and populate the object:
	logger := b.logger.With(
		slog.String("channel", b.channel),
	)
	result = &Notifier{
		logger:  logger,
		channel: b.channel,
	}
	return
}

// Notify sends a notification with the given payload using the configured channel. The payload is placed into an
// Any object, then marshalled using protocol buffers and encoded with Base64.
//
// Note that this method expects to find a transaction in the context.
func (n *Notifier) Notify(ctx context.Context, payload proto.Message) (err error) {
	// Get the transaction fron the context:
	tx, err := TxFromContext(ctx)
	if err != nil {
		return
	}
	defer tx.ReportError(&err)

	// Encode the payload:
	wrapper, err := anypb.New(payload)
	if err != nil {
		return err
	}
	data, err := proto.Marshal(wrapper)
	if err != nil {
		n.logger.ErrorContext(
			ctx,
			"Failed to serialize payload",
			slog.Any("payload", payload),
			slog.Any("error", err),
		)
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	// Send the notification:
	_, err = tx.Exec(ctx, "select pg_notify($1, $2)", n.channel, encoded)
	if err != nil {
		return err
	}
	if n.logger.Enabled(ctx, slog.LevelDebug) {
		n.logger.DebugContext(
			ctx,
			"Sent notification",
			slog.Any("payload", payload),
		)
	}

	return nil
}
