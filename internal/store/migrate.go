package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Migrate applies all pending goose migrations from fsys to db. It holds a
// Postgres session-level advisory lock while doing so, so concurrent callers
// (two api replicas starting together) run migrations one at a time; the
// second finds nothing pending. Each migration runs in its own transaction.
func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create migration locker: %w", err)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// MigratePool runs Migrate over a database/sql view of pool. Closing that view
// does not close the pool.
func MigratePool(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()
	return Migrate(ctx, db, fsys)
}
