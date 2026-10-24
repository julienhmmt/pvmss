package store_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// A failing audit insert must log through the injected logger (format and
// level honoured), not the package-level slog default.
func TestInsertAuditRow_FailureUsesInjectedLogger(t *testing.T) {
	t.Parallel()

	st := newAuditStore(t)

	var buf bytes.Buffer

	st.SetLogger(slog.New(slog.NewJSONHandler(&buf, nil)))

	if _, err := st.DB().ExecContext(context.Background(), `DROP TABLE audit_log`); err != nil {
		t.Fatalf("drop: %v", err)
	}

	if err := st.RecordAction(context.Background(), "alice@pve", "default", 101, "start"); err != nil {
		t.Fatalf("RecordAction must not propagate: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"msg":"audit log insert failed"`) || !strings.Contains(out, `"level":"ERROR"`) {
		t.Fatalf("expected JSON error line from injected logger, got %q", out)
	}
}
