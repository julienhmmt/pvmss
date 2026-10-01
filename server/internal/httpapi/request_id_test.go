package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/logctx"
	"strings"
	"testing"
)

func serveWithRequestID(t *testing.T, inbound string) (*httptest.ResponseRecorder, string) {
	t.Helper()

	var buf bytes.Buffer

	log := slog.New(slog.NewJSONHandler(&buf, nil))
	h := withRequestID(log, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		logctx.AddAttrs(r.Context(), slog.String("user", "alice@pve"))
		logctx.From(r.Context()).InfoContext(r.Context(), "in handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if inbound != "" {
		req.Header["X-Request-Id"] = []string{inbound} // bypass canonicalisation checks
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec, buf.String()
}

func TestRequestID_ValidInboundEchoed(t *testing.T) {
	t.Parallel()

	rec, logs := serveWithRequestID(t, "abc-123_X.y")
	if got := rec.Header().Get("X-Request-Id"); got != "abc-123_X.y" {
		t.Fatalf("header = %q", got)
	}

	if !strings.Contains(logs, `"requestId":"abc-123_X.y"`) || !strings.Contains(logs, `"user":"alice@pve"`) {
		t.Fatalf("handler log missing attrs: %s", logs)
	}
}

func TestRequestID_InvalidInboundReplaced(t *testing.T) {
	t.Parallel()

	for name, bad := range map[string]string{
		"too long": strings.Repeat("a", 65),
		"crlf":     "abc\r\nSet-Cookie: x=1",
		"space":    "has space",
		"slash":    "a/b",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec, _ := serveWithRequestID(t, bad)

			got := rec.Header().Get("X-Request-Id")
			if got == bad || len(got) != 32 {
				t.Fatalf("header = %q, want fresh 32-hex id", got)
			}
		})
	}
}

func TestRequestID_GeneratedWhenAbsent(t *testing.T) {
	t.Parallel()

	rec, _ := serveWithRequestID(t, "")
	if len(rec.Header().Get("X-Request-Id")) != 32 {
		t.Fatalf("header = %q", rec.Header().Get("X-Request-Id"))
	}
}
