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
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/pflag"
)

//go:embed migrations
var migrationsFS embed.FS

// Tool tries to simplify and centralize database operations that are needed frequently during the startup of a process
// that uses the database, like creating the database connection string from the command line flags, waiting till the
// database is up and running, applying the migrations and creationg the database connection pool.
type Tool interface {
	// Wait waits till the database is available.
	Wait(ctx context.Context) error

	// Migrate runs the database migrations.
	Migrate(ctx context.Context) error

	// Pool returns the pool of database connections.
	Pool(ctx context.Context) (result *pgxpool.Pool, err error)

	// URL returns the database connection URL.
	URL() string
}

type ToolBuilder struct {
	logger *slog.Logger
	url    string
}

type tool struct {
	logger *slog.Logger
	url    string
}

func NewTool() *ToolBuilder {
	return &ToolBuilder{}
}

func (b *ToolBuilder) SetLogger(value *slog.Logger) *ToolBuilder {
	b.logger = value
	return b
}

func (b *ToolBuilder) SetURL(value string) *ToolBuilder {
	b.url = value
	return b
}

// SetFlags sets the command line flags that should be used to configure the tool. This is optional.
func (b *ToolBuilder) SetFlags(flags *pflag.FlagSet) *ToolBuilder {
	if flags == nil {
		return b
	}

	var (
		flag  string
		value string
		err   error
	)
	failure := func() {
		b.logger.Error(
			"Failed to get flag value",
			slog.String("flag", flag),
			slog.String("error", err.Error()),
		)
	}

	// URL:
	flag = urlFlagName
	value, err = flags.GetString(flag)
	if err != nil {
		failure()
	} else {
		b.SetURL(value)
	}

	return b
}

func (b *ToolBuilder) Build() (result Tool, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.url == "" {
		err = errors.New("connection URL is mandatory")
		return
	}

	// Create and populate the object:
	result = &tool{
		logger: b.logger,
		url:    b.url,
	}
	return
}

// Wait waits for the database to be available.
func (t *tool) Wait(ctx context.Context) error {
	// If the database IP address or host name have not been created yet then the connection will take a long time
	// to fail, approximately five minutes. To avoid that we need to explicitly set a shorter timeout.
	parsed, err := url.Parse(t.url)
	if err != nil {
		return err
	}
	query := parsed.Query()
	query.Set("connect_timeout", "1")
	parsed.RawQuery = query.Encode()
	url := parsed.String()

	// Try to connect to the database until we succeed, without limit of attempts:
	for {
		conn, err := pgx.Connect(ctx, url)
		if err != nil {
			t.logger.InfoContext(
				ctx,
				"Database isn't responding yet",
				slog.String("error", err.Error()),
			)
			time.Sleep(1 * time.Second)
			continue
		}
		err = conn.Close(ctx)
		if err != nil {
			t.logger.ErrorContext(
				ctx,
				"Failed to close database connection",
				slog.String("error", err.Error()),
			)
		}
		return nil
	}
}

// Migrate runs the database migrations.
func (t *tool) Migrate(ctx context.Context) error {
	// The database connection URL given by the user will probably start with 'postgres', and that works fine for
	// regular connections, but for the migration library it needs to be 'pgx5'.
	parsed, err := url.Parse(t.url)
	if err != nil {
		return err
	}
	parsed.Scheme = "pgx5"
	url := parsed.String()

	// Load the migration files:
	driver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	migrations, err := migrate.NewWithSourceInstance("iofs", driver, url)
	if err != nil {
		return err
	}
	migrations.Log = &migrationsLogger{
		ctx:    ctx,
		logger: t.logger.WithGroup("migrations"),
	}
	defer func() {
		sourceErr, databaseErr := migrations.Close()
		if sourceErr != nil || databaseErr != nil {
			t.logger.ErrorContext(
				ctx,
				"Failed to close migrations",
				slog.String("source", sourceErr.Error()),
				slog.String("database", databaseErr.Error()),
			)
		}
	}()

	// Show the schema version before running the migrations:
	version, dirty, err := migrations.Version()
	switch {
	case err == nil:
		t.logger.InfoContext(
			ctx,
			"Version before running migrations",
			slog.Uint64("version", uint64(version)),
			slog.Bool("dirty", dirty),
		)
	case err == migrate.ErrNilVersion:
		t.logger.InfoContext(
			ctx,
			"Schema hasn't been created yet, will create it now",
		)
	default:
		return err
	}

	// Run the migrations:
	err = migrations.Up()
	switch {
	case err == nil:
		t.logger.InfoContext(
			ctx,
			"Migrations executed successfully",
		)
	case err == migrate.ErrNoChange:
		t.logger.InfoContext(
			ctx,
			"Migrationd don't need to be executed",
		)
	default:
		return err
	}

	// Show the schema version after running the migrations:
	version, dirty, err = migrations.Version()
	if err != nil {
		return err
	}
	t.logger.InfoContext(
		ctx,
		"Schema version after running migrations",
		slog.Uint64("version", uint64(version)),
		slog.Bool("dirty", dirty),
	)

	return nil
}

// URL returns the database connection URL.
func (t *tool) URL() string {
	return t.url
}

// Pool returns the pool of database connections.
func (t *tool) Pool(ctx context.Context) (result *pgxpool.Pool, err error) {
	result, err = pgxpool.New(ctx, t.url)
	return
}

// migrationsLogger is an adapter to implement the logging interface of the underlying migrations library using our
// logging library.
type migrationsLogger struct {
	ctx    context.Context
	logger *slog.Logger
}

// Verbose is part of the implementation of the migrate.Logger interface.
func (l *migrationsLogger) Verbose() bool {
	return true
}

// Printf is part of the implementation of the migrate.Logger interface.
func (l *migrationsLogger) Printf(format string, v ...any) {
	message := strings.TrimSpace(fmt.Sprintf(format, v...))
	l.logger.InfoContext(l.ctx, message)
}
