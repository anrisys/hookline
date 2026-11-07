// Package httpx holds HTTP plumbing shared by every mode: the health handler
// and a server loop with graceful shutdown.
package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// PingTimeout bounds each health check's dependency ping.
const PingTimeout = time.Second

// Pinger reports whether a dependency (usually the database) is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// PingerFunc adapts a function to Pinger.
type PingerFunc func(ctx context.Context) error

// Ping calls f.
func (f PingerFunc) Ping(ctx context.Context) error { return f(ctx) }

// AlwaysOK is a Pinger for modes with no dependencies.
var AlwaysOK = PingerFunc(func(context.Context) error { return nil })

// Health answers 200 {"status":"ok"} when p responds within PingTimeout and
// 503 {"status":"unavailable"} otherwise. Every request pings; nothing is
// cached, so the result reflects the dependency right now.
func Health(p Pinger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), PingTimeout)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		if err := p.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}

// Serve serves h on ln until ctx is cancelled, then stops accepting
// connections and waits up to shutdownTimeout for in-flight requests.
// It returns nil on a clean shutdown.
//
// Requests deliberately do not inherit ctx: a SIGTERM should let in-flight
// requests finish, not cancel them.
func Serve(ctx context.Context, ln net.Listener, h http.Handler, shutdownTimeout time.Duration, log *slog.Logger) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	log.Info("listening", "addr", ln.Addr().String())

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down", "timeout", shutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
