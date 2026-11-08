// Command hookline runs one of the Hookline processes: api, worker or chaos.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/anrisys/hookline/internal/api"
	"github.com/anrisys/hookline/internal/chaos"
	"github.com/anrisys/hookline/internal/config"
	"github.com/anrisys/hookline/internal/telemetry"
	"github.com/anrisys/hookline/internal/worker"
)

const usage = "usage: hookline <api|worker|chaos>\n       hookline healthcheck <url>"

type runFunc func(context.Context, config.Config, *slog.Logger) error

var modes = map[string]runFunc{
	"api":    api.Run,
	"worker": worker.Run,
	"chaos":  chaos.Run,
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// run returns the process exit code: 0 on clean shutdown, 1 on failure, 2 on
// bad usage.
func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "healthcheck" {
		return healthcheck(args[1:], stderr, healthcheckTimeout)
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	mode := args[0]
	runMode, ok := modes[mode]
	if !ok {
		fmt.Fprintf(stderr, "unknown mode %q\n%s\n", mode, usage)
		return 2
	}

	cfg, err := config.Load(mode, getenv)
	if err != nil {
		fmt.Fprintf(stderr, "config: %v\n", err)
		return 1
	}
	log := telemetry.NewLogger(stdout, cfg.LogLevel).With("mode", mode)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	log.Info("starting")
	if err := runMode(ctx, cfg, log); err != nil {
		log.Error("exited with error", "err", err)
		return 1
	}
	log.Info("stopped")
	return 0
}
