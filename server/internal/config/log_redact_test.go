package config_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"
	"strings"
	"testing"
)

func newTestLogger(t *testing.T, format, level string) (*slog.Logger, *slog.LevelVar, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "log.out")

	logger, lv, closer, err := config.NewLogger(config.Configuration{
		LogLevel: level, LogFormat: format, LogOutput: path,
	})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	t.Cleanup(func() { _ = closer.Close() })

	return logger, lv, path
}

func readAll(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	return string(b)
}

func TestNewLogger_RedactsSecretKeys(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			logger, _, path := newTestLogger(t, format, "info")
			logger.Info("probe", "token", "tok-abc", "password", "pw-x", "cookie", "ck-y",
				slog.Group("req", slog.String("authorization", "az-z"), slog.String("csrfToken", "cs-w")),
				"ticket", "tk-v", "secret", "sc-u", "keep", "visible")

			out := readAll(t, path)
			for _, leaked := range []string{"tok-abc", "pw-x", "ck-y", "az-z", "cs-w", "tk-v", "sc-u"} {
				if strings.Contains(out, leaked) {
					t.Errorf("%s output leaks %q: %s", format, leaked, out)
				}
			}

			if !strings.Contains(out, "[redacted]") || !strings.Contains(out, "visible") {
				t.Errorf("%s output missing redaction marker or plain attr: %s", format, out)
			}
		})
	}
}

func TestNewLogger_LevelVarControlsFilteringAndSource(t *testing.T) {
	t.Parallel()

	logger, lv, path := newTestLogger(t, "json", "info")

	logger.Debug("hidden debug")
	logger.Info("info no source")

	lv.Set(slog.LevelDebug)
	logger.Debug("shown debug")

	lv.Set(slog.LevelInfo)
	logger.Info("info no source again")

	lines := strings.Split(strings.TrimSpace(readAll(t, path)), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %v", len(lines), lines)
	}

	for i, wantSource := range []bool{false, true, false} {
		if got := strings.Contains(lines[i], `"source"`); got != wantSource {
			t.Errorf("line %d source present = %v, want %v: %s", i, got, wantSource, lines[i])
		}
	}

	if strings.Contains(strings.Join(lines, ""), "hidden debug") {
		t.Error("debug line emitted at info level")
	}
}

func TestConfiguration_LogValueHidesSecrets(t *testing.T) {
	t.Parallel()

	logger, _, path := newTestLogger(t, "json", "info")
	logger.Info("cfg", "config", config.Configuration{ //nolint:gosec // fake credentials for the redaction test
		SessionSecret: "sess-secret-value", ProxmoxAPITokenValue: "px-token-value", AdminPasswordHash: "$2a$hash",
	})

	out := readAll(t, path)
	for _, leaked := range []string{"sess-secret-value", "px-token-value", "$2a$hash"} {
		if strings.Contains(out, leaked) {
			t.Errorf("config leaks %q: %s", leaked, out)
		}
	}
}
