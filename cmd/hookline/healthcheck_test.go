package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anrisys/hookline/internal/httpx/httpxtest"
)

func statusServer(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestHealthcheckStatus(t *testing.T) {
	cases := []struct {
		status int
		want   int
	}{
		{http.StatusOK, 0},
		{http.StatusServiceUnavailable, 1},
		{http.StatusInternalServerError, 1},
		{http.StatusNotFound, 1},
		{http.StatusNoContent, 1}, // only 200 counts; /healthz always sends 200 when ok
	}
	for _, c := range cases {
		var stderr bytes.Buffer
		if got := healthcheck([]string{statusServer(t, c.status)}, &stderr, time.Second); got != c.want {
			t.Errorf("status %d: exit = %d, want %d (stderr %q)", c.status, got, c.want, stderr.String())
		}
	}
}

func TestHealthcheckDoesNotFollowRedirects(t *testing.T) {
	ok := statusServer(t, http.StatusOK)
	srv := httptest.NewServer(http.RedirectHandler(ok, http.StatusFound))
	t.Cleanup(srv.Close)
	if got := healthcheck([]string{srv.URL}, &bytes.Buffer{}, time.Second); got != 1 {
		t.Fatalf("exit = %d, want 1: a redirect is not healthy", got)
	}
}

func TestHealthcheckConnectionRefused(t *testing.T) {
	url := "http://" + httpxtest.FreeAddr(t) + "/healthz"
	var stderr bytes.Buffer
	if got := healthcheck([]string{url}, &stderr, time.Second); got != 1 {
		t.Fatalf("exit = %d, want 1", got)
	}
	if stderr.Len() == 0 {
		t.Fatal("want the error on stderr for `docker inspect` health logs")
	}
}

func TestHealthcheckTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })

	start := time.Now()
	got := healthcheck([]string{srv.URL}, &bytes.Buffer{}, 100*time.Millisecond)
	if got != 1 {
		t.Fatalf("exit = %d, want 1", got)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("took %s; the timeout was not applied", el)
	}
}

func TestHealthcheckBadInput(t *testing.T) {
	for _, args := range [][]string{{}, {"a", "b"}} {
		var stderr bytes.Buffer
		if got := healthcheck(args, &stderr, time.Second); got != 2 {
			t.Errorf("args %q: exit = %d, want 2", args, got)
		}
		if !strings.Contains(stderr.String(), "usage:") {
			t.Errorf("args %q: want usage on stderr, got %q", args, stderr.String())
		}
	}
	for _, url := range []string{"://nope", "not a url", ""} {
		if got := healthcheck([]string{url}, &bytes.Buffer{}, time.Second); got == 0 {
			t.Errorf("url %q: exit 0, want non-zero", url)
		}
	}
}

// The real binary must run the probe without any config: compose healthchecks
// for chaos have no DATABASE_URL, and the probe must not need one either way.
func TestHealthcheckBinaryNeedsNoConfig(t *testing.T) {
	code, stderr := runBinary(t, childEnv(), "healthcheck", statusServer(t, http.StatusOK))
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, stderr)
	}
	code, _ = runBinary(t, childEnv(), "healthcheck", statusServer(t, http.StatusServiceUnavailable))
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
}
