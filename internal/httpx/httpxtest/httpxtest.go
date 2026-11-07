// Package httpxtest has helpers for tests that start real servers.
package httpxtest

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// Listen binds a TCP listener on a random localhost port.
func Listen(t testing.TB) net.Listener {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

// FreeAddr returns a localhost address that was free a moment ago. Use it only
// when the server must bind the address itself (e.g. a child process).
func FreeAddr(t testing.TB) string {
	t.Helper()
	ln := Listen(t)
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// WaitHealthy polls http://addr/healthz until it returns 200 or timeout passes.
func WaitHealthy(t testing.TB, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	url := "http://" + addr + "/healthz"
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s not healthy after %s", url, timeout)
}
