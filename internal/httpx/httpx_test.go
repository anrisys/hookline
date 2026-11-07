package httpx

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anrisys/hookline/internal/httpx/httpxtest"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func get(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	return rec
}

func TestHealthOK(t *testing.T) {
	rec := get(t, Health(AlwaysOK))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if body := rec.Body.String(); body != `{"status":"ok"}` {
		t.Errorf("body = %s", body)
	}
}

func TestHealthUnavailableWhenPingFails(t *testing.T) {
	rec := get(t, Health(PingerFunc(func(context.Context) error { return errors.New("down") })))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if body := rec.Body.String(); body != `{"status":"unavailable"}` {
		t.Errorf("body = %s", body)
	}
}

func TestHealthTimesOutHangingPing(t *testing.T) {
	hang := PingerFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	start := time.Now()
	rec := get(t, Health(hang))
	elapsed := time.Since(start)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if elapsed < PingTimeout || elapsed > PingTimeout+500*time.Millisecond {
		t.Fatalf("took %s, want about %s", elapsed, PingTimeout)
	}
}

func TestHealthIsNotCached(t *testing.T) {
	var healthy atomic.Bool
	h := Health(PingerFunc(func(context.Context) error {
		if healthy.Load() {
			return nil
		}
		return errors.New("down")
	}))
	for i, want := range []int{503, 200, 503} {
		healthy.Store(want == 200)
		if got := get(t, h).Code; got != want {
			t.Fatalf("call %d: status = %d, want %d", i, got, want)
		}
	}
}

func TestServeShutsDownCleanly(t *testing.T) {
	ln := httpxtest.Listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, Health(AlwaysOK), time.Second, discard) }()

	httpxtest.WaitHealthy(t, ln.Addr().String(), 2*time.Second)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
	if resp, err := http.Get("http://" + ln.Addr().String() + "/healthz"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("server still accepting after shutdown")
	}
}

func TestServeLetsInFlightRequestFinish(t *testing.T) {
	ln := httpxtest.Listen(t)
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		if r.Context().Err() != nil {
			http.Error(w, "request context cancelled", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, h, 2*time.Second, discard) }()

	status := make(chan int, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			status <- -1
			return
		}
		_ = resp.Body.Close()
		status <- resp.StatusCode
	}()

	<-started
	cancel() // the "SIGTERM" arrives mid-request
	if got := <-status; got != http.StatusOK {
		t.Fatalf("in-flight request status = %d, want 200", got)
	}
	if err := <-done; err != nil {
		t.Fatalf("Serve = %v, want nil", err)
	}
}

func TestServeReturnsErrorWhenShutdownTimesOut(t *testing.T) {
	ln := httpxtest.Listen(t)
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, h, 100*time.Millisecond, discard) }()
	go func() {
		if resp, err := http.Get("http://" + ln.Addr().String() + "/"); err == nil {
			_ = resp.Body.Close()
		}
	}()

	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Serve = %v, want deadline exceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve ignored the shutdown timeout")
	}
}

func TestServeReturnsListenerError(t *testing.T) {
	ln := httpxtest.Listen(t)
	_ = ln.Close()
	err := Serve(context.Background(), ln, Health(AlwaysOK), time.Second, discard)
	if err == nil {
		t.Fatal("want error from a closed listener")
	}
}
