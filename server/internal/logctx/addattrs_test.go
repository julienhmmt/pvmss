package logctx_test

import (
	"bytes"
	"context"
	"log/slog"
	"pvmss/server/internal/logctx"
	"strings"
	"testing"
)

// Auth resolves the principal several times per request; the request
// logger must still carry one user attr.
func TestAddAttrs_ReplacesSameKey(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	ctx := logctx.With(context.Background(), slog.New(slog.NewTextHandler(&buf, nil)))
	logctx.AddAttrs(ctx, slog.String("user", "alice"))
	logctx.AddAttrs(ctx, slog.String("user", "alice"))
	logctx.From(ctx).InfoContext(ctx, "x")

	if n := strings.Count(buf.String(), "user="); n != 1 {
		t.Fatalf("user attr appears %d times: %s", n, buf.String())
	}
}
