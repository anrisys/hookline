package config

import (
	"strings"
	"testing"
	"time"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadReadsEveryVariable(t *testing.T) {
	// Non-default values, so a Load that ignored the environment would fail.
	cfg, err := Load("api", envFrom(map[string]string{
		"DATABASE_URL":     "postgres://u:p@db:5432/x",
		"API_ADDR":         ":9001",
		"WORKER_ADDR":      ":9002",
		"CHAOS_ADDR":       ":9003",
		"SHUTDOWN_TIMEOUT": "7s",
		"LOG_LEVEL":        "debug",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		DatabaseURL:     "postgres://u:p@db:5432/x",
		APIAddr:         ":9001",
		WorkerAddr:      ":9002",
		ChaosAddr:       ":9003",
		ShutdownTimeout: 7 * time.Second,
		LogLevel:        "debug",
	}
	if cfg != want {
		t.Fatalf("got %+v\nwant %+v", cfg, want)
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("chaos", envFrom(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		APIAddr:         ":8080",
		WorkerAddr:      ":8081",
		ChaosAddr:       ":8090",
		ShutdownTimeout: 30 * time.Second,
		LogLevel:        "info",
	}
	if cfg != want {
		t.Fatalf("got %+v\nwant %+v", cfg, want)
	}
}

func TestLoadEmptyMeansUnset(t *testing.T) {
	cfg, err := Load("chaos", envFrom(map[string]string{
		"CHAOS_ADDR":       "",
		"SHUTDOWN_TIMEOUT": "  ",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ChaosAddr != ":8090" || cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("empty values should fall back to defaults, got %+v", cfg)
	}
}

func TestLoadDatabaseURLRequiredPerMode(t *testing.T) {
	tests := []struct {
		mode    string
		wantErr bool
	}{
		{"api", true},
		{"worker", true},
		{"chaos", false},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			_, err := Load(tt.mode, envFrom(nil))
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
					t.Fatalf("want DATABASE_URL error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("want no error, got %v", err)
			}
		})
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	tests := []struct {
		name, key, value, wantInErr string
	}{
		{"timeout not a duration", "SHUTDOWN_TIMEOUT", "abc", "SHUTDOWN_TIMEOUT"},
		{"timeout missing unit", "SHUTDOWN_TIMEOUT", "30", "SHUTDOWN_TIMEOUT"},
		{"timeout zero", "SHUTDOWN_TIMEOUT", "0s", "SHUTDOWN_TIMEOUT"},
		{"timeout negative", "SHUTDOWN_TIMEOUT", "-1s", "SHUTDOWN_TIMEOUT"},
		{"unknown log level", "LOG_LEVEL", "loud", "LOG_LEVEL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load("chaos", envFrom(map[string]string{tt.key: tt.value}))
			if err == nil || !strings.Contains(err.Error(), tt.wantInErr) {
				t.Fatalf("want error mentioning %s, got %v", tt.wantInErr, err)
			}
		})
	}
}

func TestLoadAcceptsSmallestPositiveTimeout(t *testing.T) {
	cfg, err := Load("chaos", envFrom(map[string]string{"SHUTDOWN_TIMEOUT": "1ns"}))
	if err != nil || cfg.ShutdownTimeout != time.Nanosecond {
		t.Fatalf("got %v, %v", cfg.ShutdownTimeout, err)
	}
}

func TestLoadLogLevelIsCaseInsensitive(t *testing.T) {
	if _, err := Load("chaos", envFrom(map[string]string{"LOG_LEVEL": "WARN"})); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestLoadReportsAllErrors(t *testing.T) {
	_, err := Load("api", envFrom(map[string]string{
		"SHUTDOWN_TIMEOUT": "abc",
		"LOG_LEVEL":        "loud",
	}))
	if err == nil {
		t.Fatal("want error")
	}
	for _, key := range []string{"DATABASE_URL", "SHUTDOWN_TIMEOUT", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error should mention %s: %v", key, err)
		}
	}
}

func TestLoadUnknownMode(t *testing.T) {
	if _, err := Load("nope", envFrom(nil)); err == nil {
		t.Fatal("want error for unknown mode")
	}
}
