//nolint:goconst // level and cluster literals reused across mirror cases
package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"pvmss/server/internal/logctx"
	"strings"
	"testing"
)

func mirrorFixture(t *testing.T) (context.Context, func() []map[string]any, func(string, string)) {
	t.Helper()

	st := newAuditStore(t)

	var buf bytes.Buffer

	st.SetLogger(slog.New(slog.NewJSONHandler(&buf, nil)))

	lines := func() []map[string]any {
		var out []map[string]any

		for l := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
			if l == "" {
				continue
			}

			m := map[string]any{}
			if err := json.Unmarshal([]byte(l), &m); err != nil {
				t.Fatalf("bad line %q: %v", l, err)
			}

			out = append(out, m)
		}

		return out
	}

	record := func(actor, action string) {
		if err := st.RecordAdminAction(context.Background(), actor, action, "auth", actor, `{"summary":"s"}`, "192.0.2.1"); err != nil {
			t.Fatal(err)
		}
	}

	return context.Background(), lines, record
}

func TestAuditMirror_VMActionFields(t *testing.T) {
	t.Parallel()

	st := newAuditStore(t)

	var buf bytes.Buffer

	base := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := logctx.With(context.Background(), base.With("requestId", "rid-9", "clientIp", "192.0.2.7"))

	if err := st.RecordAction(ctx, "alice@pve", "default", 101, "vm.power_on"); err != nil {
		t.Fatal(err)
	}

	var got map[string]any

	if n := strings.Count(buf.String(), "\n"); n != 1 {
		t.Fatalf("want exactly 1 line, got %d: %s", n, buf.String())
	}

	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	want := map[string]any{"event": "audit", "actor": "alice@pve", "action": "vm.power_on", "cluster": "default", "vmid": float64(101), "level": "INFO", "requestId": "rid-9", "clientIp": "192.0.2.7"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v (line: %s)", k, got[k], v, buf.String())
		}
	}
}

func TestAuditMirror_LevelsAndAdminFields(t *testing.T) {
	t.Parallel()

	_, lines, record := mirrorFixture(t)

	cases := []struct{ action, level string }{
		{"admin.clusters.create", "INFO"},
		{"admin.tags.delete", "WARN"},
		{"auth.login_failed", "WARN"},
		{"auth.csrf_rejected", "WARN"},
		{"auth.rate_limited", "WARN"},
	}
	for _, c := range cases {
		record("bob", c.action)
	}

	got := lines()
	if len(got) != len(cases) {
		t.Fatalf("want %d lines, got %d", len(cases), len(got))
	}

	for i, c := range cases {
		l := got[i]
		if l["level"] != c.level || l["action"] != c.action || l["event"] != "audit" || l["targetType"] != "auth" || l["targetId"] != "bob" {
			t.Errorf("%s: line = %v", c.action, l)
		}

		if _, hasDetail := l["detail"]; hasDetail {
			t.Errorf("detail must not be logged: %v", l)
		}
	}
}

func TestAuditMirror_ActorCapped(t *testing.T) {
	t.Parallel()

	_, lines, record := mirrorFixture(t)
	record(strings.Repeat("x", 500), "auth.login_failed")

	got, ok := lines()[0]["actor"].(string)
	if !ok || len(got) > 64 {
		t.Fatalf("actor = %q (string=%v), want a string of len <= 64", got, ok)
	}
}
