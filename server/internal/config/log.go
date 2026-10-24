package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// NewLogger creates a structured logger from the configuration.
// It returns the logger, the LevelVar that controls it at runtime, and a
// WriteCloser that should be closed before shutdown.
func NewLogger(cfg Configuration) (*slog.Logger, *slog.LevelVar, io.WriteCloser, error) {
	level, err := parseLogLevel(cfg.LogLevel)
	if err != nil {
		return nil, nil, nil, err
	}

	levelVar := new(slog.LevelVar)
	levelVar.Set(level)

	var w io.WriteCloser

	switch strings.ToLower(cfg.LogOutput) {
	case "stdout":
		w = nopWriteCloser{Writer: os.Stdout}
	case "stderr":
		w = nopWriteCloser{Writer: os.Stderr}
	default:
		f, err := os.OpenFile(cfg.LogOutput, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("open log output %q: %w", cfg.LogOutput, err)
		}

		w = f
	}

	opts := &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: true, // gated by sourceGate: only emitted while level is debug
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return renameKeys(groups, redactSecrets(groups, a))
		},
	}

	var handler slog.Handler

	switch strings.ToLower(cfg.LogFormat) {
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	case "console":
		handler = slog.NewTextHandler(w, opts)
	default:
		return nil, nil, nil, fmt.Errorf("unknown log format %q", cfg.LogFormat)
	}

	return slog.New(sourceGate{Handler: handler, level: levelVar}), levelVar, w, nil
}

func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}

	return slog.LevelInfo, fmt.Errorf("unknown log level %q", level)
}

func renameKeys(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey:
		a.Key = "timestamp"
	case slog.LevelKey:
		a.Key = "level"
	case slog.MessageKey:
		a.Key = "message"
	}

	return a
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
