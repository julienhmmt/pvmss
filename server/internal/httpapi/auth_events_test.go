//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/logctx"
	"strings"
	"testing"
)

func postWithLog(h func(http.ResponseWriter, *http.Request), path, body string, cookies []*http.Cookie) (*httptest.ResponseRecorder, string) {
	var buf bytes.Buffer

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	req = req.WithContext(logctx.With(req.Context(), slog.New(slog.NewJSONHandler(&buf, nil))))
	rec := httptest.NewRecorder()
	h(rec, req)

	return rec, buf.String()
}

func findLine(t *testing.T, out, msg string) map[string]any {
	t.Helper()

	for _, l := range strings.Split(out, "\n") {
		m := map[string]any{}
		if json.Unmarshal([]byte(l), &m) == nil && m["msg"] == msg {
			return m
		}
	}

	t.Fatalf("no %q line in: %s", msg, out)

	return nil
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthEvents_LoginSuccessLogout(t *testing.T) {
	handler := newAuthHandler(t)

	rec, out := postWithLog(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d", rec.Code)
	}

	l := findLine(t, out, "login succeeded")
	if l["level"] != "INFO" || l["event"] != "auth" || l["result"] != "success" || l["user"] == nil {
		t.Errorf("login line = %v", l)
	}

	if strings.Contains(out, "pvmss-alice\"") && strings.Contains(out, `"password"`) {
		t.Errorf("password leaked: %s", out)
	}

	_, out = postWithLog(handler.Logout, "/api/v1/auth/logout", "", rec.Result().Cookies())

	l = findLine(t, out, "logout")
	if l["level"] != "INFO" || l["event"] != "auth" || l["user"] == nil {
		t.Errorf("logout line = %v", l)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthEvents_AdminLoginFailureWarnsWithoutPassword(t *testing.T) {
	handler := newAuthHandler(t)

	const secret = "wrong-Adm1n-pw"

	rec, out := postWithLog(handler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"`+secret+`"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}

	l := findLine(t, out, "login failed")
	if l["level"] != "WARN" || l["event"] != "auth" || l["result"] != "failure" {
		t.Errorf("line = %v", l)
	}

	if strings.Contains(out, secret) {
		t.Errorf("password leaked: %s", out)
	}
}

// A failed cluster login is already an audit row, so the mirror is its one
// Warn line (no second direct line) and must not carry the password.
//
//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuthEvents_PVELoginFailureMirroredWithoutPassword(t *testing.T) {
	handler, _ := newAuthHandlerWithStore(t)

	const secret = "definitely-wrong-pw"

	rec, out := postWithLog(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"`+secret+`"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	l := findLine(t, out, "audit event")
	if l["level"] != "WARN" || l["action"] != "auth.login_failed" || l["event"] != "audit" {
		t.Errorf("line = %v", l)
	}

	if strings.Contains(out, secret) {
		t.Errorf("password leaked: %s", out)
	}
}
