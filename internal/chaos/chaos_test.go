package chaos

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/anrisys/hookline/internal/config"
	"github.com/anrisys/hookline/internal/httpx/httpxtest"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestRunServesHealthAndShutsDown(t *testing.T) {
	addr := httpxtest.FreeAddr(t)
	cfg := config.Config{ChaosAddr: addr, ShutdownTimeout: 2 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg, discard) }()

	httpxtest.WaitHealthy(t, addr, 5*time.Second)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
