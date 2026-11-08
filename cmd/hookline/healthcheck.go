package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const healthcheckUsage = "usage: hookline healthcheck <url>"

// healthcheckTimeout bounds one probe. It is longer than httpx.PingTimeout so
// a dependency outage shows up as a 503 rather than a client timeout.
const healthcheckTimeout = 2 * time.Second

// healthcheck GETs args[0] and returns 0 only on a 200. It exists because the
// distroless image has no shell, curl or wget for a container healthcheck. It
// needs no config, so it works in every mode's container.
func healthcheck(args []string, stderr io.Writer, timeout time.Duration) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, healthcheckUsage)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args[0], nil)
	if err != nil {
		fmt.Fprintf(stderr, "healthcheck: %v\n", err)
		return 1
	}

	client := &http.Client{
		// A redirect means we probed the wrong URL; report it, don't chase it.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stderr, "healthcheck: %s returned %d\n", args[0], resp.StatusCode)
		return 1
	}
	return 0
}
