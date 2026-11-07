// Package worker claims and delivers pending deliveries.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/anrisys/hookline/internal/config"
	"github.com/anrisys/hookline/internal/httpx"
	"github.com/anrisys/hookline/internal/store"
)

// Run pings the database, serves /healthz on cfg.WorkerAddr, and idles until
// ctx is cancelled. Delivery work arrives in #9.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.WorkerAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.WorkerAddr, err)
	}
	return run(ctx, cfg, log, ln)
}

func run(ctx context.Context, cfg config.Config, log *slog.Logger, ln net.Listener) error {
	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer pool.Close()

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", httpx.Health(pool))
	return httpx.Serve(ctx, ln, mux, cfg.ShutdownTimeout, log)
}
