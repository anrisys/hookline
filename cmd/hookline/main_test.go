package main

// These tests build the real binary and drive it as a child process, because
// only a real process proves that SIGTERM is wired to shutdown and that the
// exit codes are right.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/anrisys/hookline/internal/httpx/httpxtest"
	"github.com/anrisys/hookline/internal/store/storetest"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "hookline-bin")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "hookline")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		panic("build hookline: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// childEnv returns the test's environment with Hookline variables removed and
// overrides applied, so a developer's .env can't leak into the child.
func childEnv(overrides ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "DATABASE_URL", "API_ADDR", "WORKER_ADDR", "CHAOS_ADDR", "SHUTDOWN_TIMEOUT", "LOG_LEVEL":
			continue
		}
		env = append(env, kv)
	}
	return append(env, overrides...)
}

func exitCode(t *testing.T, cmd *exec.Cmd, timeout time.Duration) int {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return cmd.ProcessState.ExitCode()
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		t.Fatalf("process did not exit within %s", timeout)
		return -1
	}
}

func runBinary(t *testing.T, env []string, args ...string) (int, string) {
	t.Helper()
	var stderr bytes.Buffer
	cmd := exec.Command(binary, args...)
	cmd.Env = env
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return exitCode(t, cmd, 10*time.Second), stderr.String()
}

func TestUsageErrorsExit2(t *testing.T) {
	for _, args := range [][]string{{}, {"nope"}, {"api", "extra"}} {
		code, stderr := runBinary(t, childEnv(), args...)
		if code != 2 {
			t.Errorf("args %q: exit = %d, want 2", args, code)
		}
		if !strings.Contains(stderr, "usage:") {
			t.Errorf("args %q: stderr should show usage, got %q", args, stderr)
		}
	}
}

func TestMissingConfigExits1(t *testing.T) {
	code, stderr := runBinary(t, childEnv(), "api")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "DATABASE_URL") {
		t.Fatalf("stderr should name the missing variable, got %q", stderr)
	}
}

// sigtermAfterReady starts mode, waits until /healthz answers 200 (so an early
// crash can't masquerade as a clean exit), sends SIGTERM, and returns the exit code.
func sigtermAfterReady(t *testing.T, addr string, env []string, mode string) int {
	t.Helper()
	cmd := exec.Command(binary, mode)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	httpxtest.WaitHealthy(t, addr, 10*time.Second)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	return exitCode(t, cmd, 10*time.Second)
}

func TestChaosExitsCleanlyOnSIGTERM(t *testing.T) {
	addr := httpxtest.FreeAddr(t)
	if code := sigtermAfterReady(t, addr, childEnv("CHAOS_ADDR="+addr), "chaos"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
}

func TestWorkerExitsCleanlyOnSIGTERM(t *testing.T) {
	url := storetest.NewDatabase(t)
	addr := httpxtest.FreeAddr(t)
	if code := sigtermAfterReady(t, addr, childEnv("DATABASE_URL="+url, "WORKER_ADDR="+addr), "worker"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
}

func TestAPIExitsCleanlyOnSIGTERM(t *testing.T) {
	url := storetest.NewDatabase(t)
	addr := httpxtest.FreeAddr(t)
	if code := sigtermAfterReady(t, addr, childEnv("DATABASE_URL="+url, "API_ADDR="+addr), "api"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
}
