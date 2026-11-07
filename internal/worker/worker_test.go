package worker

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/anrisys/hookline/internal/config"
	"github.com/anrisys/hookline/internal/httpx/httpxtest"
	"github.com/anrisys/hookline/internal/store/storetest"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestRunServesHealthAndShutsDown(t *testing.T) {
	ln := httpxtest.Listen(t)
	cfg := config.Config{DatabaseURL: storetest.NewDatabase(t), ShutdownTimeout: 2 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, discard, ln) }()

	httpxtest.WaitHealthy(t, ln.Addr().String(), 5*time.Second)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}

func TestRunFailsWhenDatabaseUnreachable(t *testing.T) {
	cfg := config.Config{
		DatabaseURL:     "postgres://u:p@127.0.0.1:1/x?connect_timeout=2",
		ShutdownTimeout: time.Second,
	}
	if err := run(context.Background(), cfg, discard, httpxtest.Listen(t)); err == nil {
		t.Fatal("want error when the database is unreachable")
	}
}
