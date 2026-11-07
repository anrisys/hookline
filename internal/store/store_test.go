package store_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/anrisys/hookline/internal/store"
	"github.com/anrisys/hookline/internal/store/migrations"
	"github.com/anrisys/hookline/internal/store/storetest"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := store.NewPool(context.Background(), storetest.NewDatabase(t))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// appliedVersions returns each applied version and how many rows record it.
func appliedVersions(t *testing.T, pool *pgxpool.Pool) map[int64]int {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT version_id, count(*) FROM goose_db_version
		 WHERE is_applied AND version_id > 0 GROUP BY version_id`)
	if err != nil {
		t.Fatalf("query goose_db_version: %v", err)
	}
	got := map[int64]int{}
	for rows.Next() {
		var v int64
		var n int
		if err := rows.Scan(&v, &n); err != nil {
			t.Fatal(err)
		}
		got[v] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var ok bool
	err := pool.QueryRow(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&ok)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestMigrateFreshDatabase(t *testing.T) {
	pool := newPool(t)
	if err := store.MigratePool(context.Background(), pool, migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if got := appliedVersions(t, pool); got[1] != 1 {
		t.Fatalf("applied versions = %v, want version 1 once", got)
	}
}

func TestMigrateTwiceIsNoOp(t *testing.T) {
	pool := newPool(t)
	for i := 0; i < 2; i++ {
		if err := store.MigratePool(context.Background(), pool, migrations.FS); err != nil {
			t.Fatalf("Migrate #%d: %v", i+1, err)
		}
	}
	got := appliedVersions(t, pool)
	if len(got) != 1 || got[1] != 1 {
		t.Fatalf("applied versions = %v, want only version 1, once", got)
	}
}

func TestMigrateLeavesPoolUsable(t *testing.T) {
	pool := newPool(t)
	if err := store.MigratePool(context.Background(), pool, migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("pool unusable after migrate: %v", err)
	}
}

// Migrate must wait while another session holds goose's advisory lock and
// apply nothing until it's released. This is the test that fails if the
// session locker is removed.
func TestMigrateWaitsForAdvisoryLock(t *testing.T) {
	ctx := context.Background()
	url := storetest.NewDatabase(t)
	pool, err := store.NewPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// Advisory locks belong to a session, so hold one connection for it.
	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_lock($1)", lock.DefaultLockID); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	blocked, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := store.Migrate(blocked, db, migrations.FS); err == nil {
		t.Fatal("Migrate succeeded while another session held the migration lock")
	}
	// goose's unlocked HasPending pre-check may create the empty version table;
	// what must not happen is a migration being applied.
	if got := appliedVersions(t, pool); len(got) != 0 {
		t.Fatalf("migrations applied while the lock was held: %v", got)
	}

	if _, err := holder.Exec(ctx, "SELECT pg_advisory_unlock($1)", lock.DefaultLockID); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, db, migrations.FS); err != nil {
		t.Fatalf("Migrate after unlock: %v", err)
	}
	if got := appliedVersions(t, pool); got[1] != 1 {
		t.Fatalf("applied versions = %v, want version 1 once", got)
	}
}

// Two processes migrating at once both succeed and record each version once.
// On its own this does not prove the lock: goose's version-table handling
// already makes this race wait (verified by removing the locker), which is
// why TestMigrateWaitsForAdvisoryLock exists.
func TestMigrateConcurrentCallersBothSucceed(t *testing.T) {
	url := storetest.NewDatabase(t)
	fsys := fstest.MapFS{
		"00001_slow.sql": {Data: []byte(`-- +goose Up
CREATE TABLE race_probe (id int);
SELECT pg_sleep(0.5);
-- +goose Down
DROP TABLE race_probe;
`)},
	}

	const callers = 2
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Separate pools, like two api processes.
			db, err := sql.Open("pgx", url)
			if err != nil {
				errs[i] = err
				return
			}
			defer func() { _ = db.Close() }()
			errs[i] = store.Migrate(context.Background(), db, fsys)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}

	pool, err := store.NewPool(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if got := appliedVersions(t, pool); got[1] != 1 {
		t.Fatalf("applied versions = %v, want version 1 exactly once", got)
	}
}

func TestMigrateFailedMigrationRollsBack(t *testing.T) {
	pool := newPool(t)
	fsys := fstest.MapFS{
		"00001_ok.sql": {Data: []byte("-- +goose Up\nCREATE TABLE good (id int);\n-- +goose Down\nDROP TABLE good;\n")},
		"00002_bad.sql": {Data: []byte("-- +goose Up\nCREATE TABLE half_done (id int);\nTHIS IS NOT SQL;\n" +
			"-- +goose Down\nDROP TABLE half_done;\n")},
	}
	err := store.MigratePool(context.Background(), pool, fsys)
	if err == nil {
		t.Fatal("want error from broken migration")
	}
	var partial *goose.PartialError
	if !errors.As(err, &partial) || partial.Failed.Source.Version != 2 {
		t.Errorf("want a goose.PartialError for version 2, got %v", err)
	}
	if got := appliedVersions(t, pool); len(got) != 1 || got[1] != 1 {
		t.Fatalf("applied versions = %v, want only version 1", got)
	}
	if !tableExists(t, pool, "good") || tableExists(t, pool, "half_done") {
		t.Fatal("version 1 should be kept and version 2 rolled back")
	}
}

func TestNewPoolUnreachableFailsFast(t *testing.T) {
	start := time.Now()
	// Port 1 on localhost refuses connections; nothing listens there.
	_, err := store.NewPool(context.Background(), "postgres://u:p@127.0.0.1:1/x?connect_timeout=2")
	if err == nil {
		t.Fatal("want error for unreachable database")
	}
	if elapsed := time.Since(start); elapsed > 6*time.Second {
		t.Fatalf("took %s, want under connect timeout", elapsed)
	}
}

func TestNewPoolInvalidURL(t *testing.T) {
	if _, err := store.NewPool(context.Background(), "::not a url::"); err == nil {
		t.Fatal("want error for invalid URL")
	}
}
