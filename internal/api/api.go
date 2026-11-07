// Package api serves the Hookline REST API.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/anrisys/hookline/internal/config"
	"github.com/anrisys/hookline/internal/httpx"
	"github.com/anrisys/hookline/internal/store"
	"github.com/anrisys/hookline/internal/store/migrations"
)

// NewHandler returns the API's routes. db backs the health check.
func NewHandler(db httpx.Pinger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", httpx.Health(db))
	return mux
}

// Run connects to the database, applies migrations, and serves HTTP on
// cfg.APIAddr until ctx is cancelled, then shuts down within
// cfg.ShutdownTimeout.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.APIAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.APIAddr, err)
	}
	return run(ctx, cfg, log, ln)
}

// run is Run on an existing listener, so tests can bind to port 0.
func run(ctx context.Context, cfg config.Config, log *slog.Logger, ln net.Listener) error {
	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer pool.Close()

	if err := store.MigratePool(ctx, pool, migrations.FS); err != nil {
		_ = ln.Close()
		return err
	}
	log.Info("migrations applied")

	return httpx.Serve(ctx, ln, NewHandler(pool), cfg.ShutdownTimeout, log)
}
