package logctx_test

import (
	"bytes"
	"context"
	"log/slog"
	"pvmss/server/internal/logctx"
	"strings"
	"testing"
)

func jsonLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

func TestFrom_WithoutLoggerReturnsDefault(t *testing.T) {
	t.Parallel()

	if logctx.From(context.Background()) == nil {
		t.Fatal("From(background) returned nil")
	}
}

func TestAddAttrs_WithoutHolderIsNoop(t *testing.T) {
	t.Parallel()

	logctx.AddAttrs(context.Background(), slog.String("user", "x")) // must not panic
}

func TestFrom_IncludesBaseAndLateAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	ctx := logctx.With(context.Background(), jsonLogger(&buf).With("requestId", "rid-1"))
	logctx.AddAttrs(ctx, slog.String("user", "alice@pve"))
	logctx.From(ctx).Info("probe")

	out := buf.String()
	if !strings.Contains(out, `"requestId":"rid-1"`) || !strings.Contains(out, `"user":"alice@pve"`) {
		t.Fatalf("missing attrs: %s", out)
	}
}
