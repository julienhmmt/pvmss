//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"
)

type logLevelBody struct {
	Level   string `json:"level"`
	Default string `json:"default"`
}

func logLevelFixture(t *testing.T) (*http.ServeMux, *httpapi.Auth, *slog.LevelVar, *slog.Logger, *bytes.Buffer, func() []string) {
	t.Helper()

	ops, auth, st := newAdminOpsHandler(t)

	var buf bytes.Buffer

	lv := new(slog.LevelVar) // Info
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: lv}))
	st.SetLogger(logger)
	ops.SetLogLevel(lv, slog.LevelInfo)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/admin/ops/log-level", auth.RequireAdmin(http.HandlerFunc(ops.ServeLogLevel)))
	mux.Handle("PUT /api/v1/admin/ops/log-level", auth.RequireAdmin(http.HandlerFunc(ops.ServeLogLevelUpdate)))

	actions := func() []string {
		entries, err := st.QueryAudit(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		var out []string
		for _, e := range entries {
			out = append(out, e.Action)
		}

		return out
	}

	return mux, auth, lv, logger, &buf, actions
}

func doLogLevel(mux http.Handler, method, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/admin/ops/log-level", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

//nolint:paralleltest // serial: shared database fixture
func TestLogLevel_GetReturnsCurrentAndDefault(t *testing.T) {
	mux, auth, _, _, _, _ := logLevelFixture(t)

	rec := doLogLevel(mux, http.MethodGet, "", adminCookie(t, auth))

	var got logLevelBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK || got != (logLevelBody{Level: "info", Default: "info"}) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestLogLevel_PutDebugTakesEffectAuditedAndMirrored(t *testing.T) {
	mux, auth, lv, logger, buf, actions := logLevelFixture(t)

	logger.Debug("before change")

	if strings.Contains(buf.String(), "before change") {
		t.Fatal("debug emitted at info level")
	}

	rec := doLogLevel(mux, http.MethodPut, `{"level":"debug"}`, adminCookie(t, auth))
	if rec.Code != http.StatusOK || lv.Level() != slog.LevelDebug {
		t.Fatalf("code=%d level=%v body=%s", rec.Code, lv.Level(), rec.Body.String())
	}

	var got logLevelBody
	_ = json.Unmarshal(rec.Body.Bytes(), &got)

	if got.Level != "debug" || got.Default != "info" {
		t.Errorf("body = %+v", got)
	}

	logger.Debug("after change")

	if !strings.Contains(buf.String(), "after change") {
		t.Fatal("debug line not emitted after PUT, no restart expected")
	}

	found := false

	for _, a := range actions() {
		found = found || a == "admin.log_level.update"
	}

	if !found || !strings.Contains(buf.String(), `"action":"admin.log_level.update"`) {
		t.Fatalf("audit row or mirror line missing: actions=%v log=%s", actions(), buf.String())
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestLogLevel_InvalidLevelAndBadBodyAre400(t *testing.T) {
	mux, auth, lv, _, _, _ := logLevelFixture(t)
	cookie := adminCookie(t, auth)

	for _, body := range []string{`{"level":"trace"}`, `{"level":""}`, `{bad`, `{}`} {
		if rec := doLogLevel(mux, http.MethodPut, body, cookie); rec.Code != http.StatusBadRequest {
			t.Errorf("%s -> %d, want 400", body, rec.Code)
		}
	}

	if lv.Level() != slog.LevelInfo {
		t.Errorf("level changed by invalid input: %v", lv.Level())
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestLogLevel_NonAdminAndAnonymousRejected(t *testing.T) {
	mux, auth, _, _, _, _ := logLevelFixture(t)

	if rec := doLogLevel(mux, http.MethodPut, `{"level":"debug"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous = %d", rec.Code)
	}

	if rec := doLogLevel(mux, http.MethodPut, `{"level":"debug"}`, aliceCookie(t, auth)); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin = %d", rec.Code)
	}
}

// Through the real router, adminProtect puts CSRF in front of the write: an
// admin session cookie without the CSRF header must not change the level.
//
//nolint:paralleltest // serial: shared router and database fixtures
func TestLogLevel_PutWithoutCSRFRejectedByRouter(t *testing.T) {
	mux, authHandler := newAuthzContractRouter(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/ops/log-level", strings.NewReader(`{"level":"debug"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(adminCookie(t, authHandler))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 without CSRF token: %s", rec.Code, rec.Body.String())
	}
}
