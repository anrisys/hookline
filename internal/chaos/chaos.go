// Package chaos is a demo webhook receiver that fails or stalls on request.
package chaos

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/anrisys/hookline/internal/config"
	"github.com/anrisys/hookline/internal/httpx"
)

// NewHandler returns the chaos receiver's routes. The /chaos handler arrives
// in #7.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", httpx.Health(httpx.AlwaysOK))
	return mux
}

// Run serves the chaos receiver on cfg.ChaosAddr until ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.ChaosAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.ChaosAddr, err)
	}
	return httpx.Serve(ctx, ln, NewHandler(), cfg.ShutdownTimeout, log)
}
