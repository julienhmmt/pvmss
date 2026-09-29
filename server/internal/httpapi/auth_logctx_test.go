//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/logctx"
	"strings"
	"testing"
)

// Principal tags the request logger with the resolved user, so any later
// log line (and the access log) carries it.
//
//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_Principal_AddsUserToRequestLogger(t *testing.T) {
	handler := newAuthHandler(t)

	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d", login.Code)
	}

	var buf bytes.Buffer

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, c := range login.Result().Cookies() {
		req.AddCookie(c)
	}

	ctx := logctx.With(req.Context(), slog.New(slog.NewJSONHandler(&buf, nil)))
	req = req.WithContext(ctx)

	if _, err := handler.Principal(req); err != nil {
		t.Fatalf("Principal: %v", err)
	}

	logctx.From(req.Context()).Info("probe")

	if want := `"user":"` + cluster.FakeUserAlice + `"`; !strings.Contains(buf.String(), want) {
		t.Fatalf("log missing %s: %s", want, buf.String())
	}
}
