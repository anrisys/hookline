package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anrisys/hookline/internal/config"
	"github.com/anrisys/hookline/internal/httpx"
	"github.com/anrisys/hookline/internal/httpx/httpxtest"
	"github.com/anrisys/hookline/internal/store"
	"github.com/anrisys/hookline/internal/store/storetest"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestHealthzRoute(t *testing.T) {
	h := NewHandler(httpx.AlwaysOK)
	tests := []struct {
		method string
		header string
		want   int
	}{
		{http.MethodGet, "", http.StatusOK}, // R18: no Authorization needed
		{http.MethodGet, "Bearer wrong", http.StatusOK},
		{http.MethodHead, "", http.StatusOK},
		{http.MethodPost, "", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.header, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/healthz", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestHealthzReflectsRealPool(t *testing.T) {
	pool, err := store.NewPool(context.Background(), storetest.NewDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(pool)
	get := func() int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		return rec.Code
	}
	if got := get(); got != http.StatusOK {
		t.Fatalf("open pool: status = %d, want 200", got)
	}
	pool.Close()
	if got := get(); got != http.StatusServiceUnavailable {
		t.Fatalf("closed pool: status = %d, want 503", got)
	}
}

func TestRunMigratesServesAndShutsDown(t *testing.T) {
	url := storetest.NewDatabase(t)
	ln := httpxtest.Listen(t)
	cfg := config.Config{DatabaseURL: url, ShutdownTimeout: 2 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, discard, ln) }()

	httpxtest.WaitHealthy(t, ln.Addr().String(), 10*time.Second)

	pool, err := store.NewPool(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var version int64
	if err := pool.QueryRow(context.Background(),
		`SELECT max(version_id) FROM goose_db_version WHERE is_applied`).Scan(&version); err != nil {
		t.Fatalf("goose_db_version: %v", err)
	}
	if version != 1 {
		t.Fatalf("schema version = %d, want 1", version)
	}

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
	ln := httpxtest.Listen(t)
	cfg := config.Config{
		DatabaseURL:     "postgres://u:p@127.0.0.1:1/x?connect_timeout=2",
		ShutdownTimeout: time.Second,
	}
	if err := run(context.Background(), cfg, discard, ln); err == nil {
		t.Fatal("want error when the database is unreachable")
	}
	// The listener must be released so the port is free again.
	if _, err := ln.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("listener still open: %v", err)
	}
}

func TestRunFailsWhenPortInUse(t *testing.T) {
	ln := httpxtest.Listen(t)
	defer func() { _ = ln.Close() }()
	cfg := config.Config{APIAddr: ln.Addr().String(), DatabaseURL: "postgres://unused", ShutdownTimeout: time.Second}
	if err := Run(context.Background(), cfg, discard); err == nil {
		t.Fatal("want error when the port is taken")
	}
}
