//nolint:goconst // test fixture strings
package config_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	cases := []struct {
		name     string
		cfg      config.Configuration
		wantErr  bool
		validate func(t *testing.T, path string)
	}{
		{
			name: "json writes structured entries",
			cfg: config.Configuration{
				Port:      50001,
				DBPath:    ":memory:",
				LogLevel:  "info",
				LogFormat: "json",
				LogOutput: filepath.Join(t.TempDir(), "log.jsonl"),
			},
			validate: func(t *testing.T, path string) {
				t.Helper()

				logger, closer, err := config.NewLogger(config.Configuration{
					Port:      50001,
					DBPath:    ":memory:",
					LogLevel:  "info",
					LogFormat: "json",
					LogOutput: path,
				})
				if err != nil {
					t.Fatalf("NewLogger: %v", err)
				}

				logger.Info("startup complete", "component", "config")
				logger.Warn("approaching limit", "component", "store")
				logger.Error("database unreachable", "component", "httpapi", "error", errors.New("connection refused"))

				if err := closer.Close(); err != nil {
					t.Fatalf("closer.Close: %v", err)
				}

				entries, err := readLogEntries(path)
				if err != nil {
					t.Fatalf("read log: %v", err)
				}

				if len(entries) != 3 {
					t.Fatalf("expected 3 log entries, got %d", len(entries))
				}

				expected := []struct {
					message   string
					level     string
					component string
				}{
					{message: "startup complete", level: "INFO", component: "config"},
					{message: "approaching limit", level: "WARN", component: "store"},
					{message: "database unreachable", level: "ERROR", component: "httpapi"},
				}

				for i, want := range expected {
					got := entries[i]
					if got.Message != want.message {
						t.Fatalf("entry %d: message = %q, want %q", i, got.Message, want.message)
					}

					if got.Level != want.level {
						t.Fatalf("entry %d: level = %q, want %q", i, got.Level, want.level)
					}

					if got.Component != want.component {
						t.Fatalf("entry %d: component = %q, want %q", i, got.Component, want.component)
					}

					if got.Timestamp == "" {
						t.Fatalf("entry %d: timestamp is empty", i)
					}
				}

				if entries[2].Error == "" {
					t.Fatalf("error entry should contain an error attribute")
				}
			},
		},
		{
			name: "console writes text",
			cfg: config.Configuration{
				Port:      50001,
				DBPath:    ":memory:",
				LogLevel:  "info",
				LogFormat: "console",
				LogOutput: filepath.Join(t.TempDir(), "log.txt"),
			},
			validate: func(t *testing.T, path string) {
				t.Helper()

				logger, closer, err := config.NewLogger(config.Configuration{
					Port:      50001,
					DBPath:    ":memory:",
					LogLevel:  "info",
					LogFormat: "console",
					LogOutput: path,
				})
				if err != nil {
					t.Fatalf("NewLogger: %v", err)
				}

				logger.Info("hello console", "component", "test")

				if err := closer.Close(); err != nil {
					t.Fatalf("closer.Close: %v", err)
				}

				content, err := os.ReadFile(path) //nolint:gosec // path is test-controlled via t.TempDir
				if err != nil {
					t.Fatalf("read log: %v", err)
				}

				if !strings.Contains(string(content), "hello console") {
					t.Fatalf("console log should contain message: %q", string(content))
				}

				if !strings.Contains(string(content), "test") {
					t.Fatalf("console log should contain component: %q", string(content))
				}
			},
		},
		{
			name: "unknown level returns error",
			cfg: config.Configuration{
				LogLevel:  "verbose",
				LogFormat: "json",
				LogOutput: "stdout",
			},
			wantErr: true,
		},
		{
			name: "unknown format returns error",
			cfg: config.Configuration{
				LogLevel:  "info",
				LogFormat: "xml",
				LogOutput: "stdout",
			},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, closer, err := config.NewLogger(c.cfg)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("NewLogger: %v", err)
			}

			defer func() { _ = closer.Close() }()

			if c.validate != nil {
				c.validate(t, c.cfg.LogOutput)
			}
		})
	}
}

type logEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Component string `json:"component"`
	Error     string `json:"error"`
}

func readLogEntries(path string) ([]logEntry, error) {
	entries := []logEntry{}

	f, err := os.Open(path) //nolint:gosec // path is test-controlled via t.TempDir
	if err != nil {
		return nil, err
	}

	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e logEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return nil, err
		}

		entries = append(entries, e)
	}

	return entries, scanner.Err()
}
