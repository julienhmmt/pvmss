package middleware

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"pvmss/proxmox"
	"pvmss/state"
)

type mockStateManager struct {
	connected bool
	message   string
}

func (m *mockStateManager) GetTemplates() *template.Template               { return nil }
func (m *mockStateManager) SetTemplates(t *template.Template) error        { return nil }
func (m *mockStateManager) GetSessionManager() *scs.SessionManager         { return nil }
func (m *mockStateManager) SetSessionManager(sm *scs.SessionManager) error { return nil }
func (m *mockStateManager) StartOnlineMode() error                         { return nil }
func (m *mockStateManager) SetOfflineMode()                                {}
func (m *mockStateManager) IsOfflineMode() bool                            { return false }
func (m *mockStateManager) GetProxmoxStatus() (bool, string)               { return m.connected, m.message }
func (m *mockStateManager) CheckProxmoxConnection() bool                   { return m.connected }
func (m *mockStateManager) GetNodeCache() ([]*proxmox.NodeDetails, time.Time) {
	return nil, time.Time{}
}
func (m *mockStateManager) GetProxmoxSnapshot() *state.ProxmoxClusterSnapshot  { return nil }
func (m *mockStateManager) RequestSnapshotRefresh()                            {}
func (m *mockStateManager) GetSettings() *state.AppSettings                    { return nil }
func (m *mockStateManager) SetSettings(settings *state.AppSettings) error      { return nil }
func (m *mockStateManager) SetSettingsWithoutSave(settings *state.AppSettings) {}
func (m *mockStateManager) GetTags() []string                                  { return nil }
func (m *mockStateManager) GetISOs() []string                                  { return nil }
func (m *mockStateManager) GetVMBRs() []string                                 { return nil }
func (m *mockStateManager) GetLimits() map[string]interface{}                  { return nil }
func (m *mockStateManager) GetStorages() []string                              { return nil }
func (m *mockStateManager) AddCSRFToken(token string, expiry time.Time) error  { return nil }
func (m *mockStateManager) ValidateAndRemoveCSRFToken(token string) bool       { return false }
func (m *mockStateManager) CleanExpiredCSRFTokens()                            {}
func (m *mockStateManager) GetFrontendPath() string                            { return "" }
func (m *mockStateManager) SetFrontendPath(path string)                        {}
func (m *mockStateManager) SetGuestAgentCleanupFunc(cleanupFunc func())        {}

func TestClientIP(t *testing.T) {
	testCases := []struct {
		name   string
		xff    string
		xReal  string
		remote string
		want   string
	}{
		{name: "XForwardedFor", xff: "203.0.113.10, 10.0.0.1", remote: "192.0.2.1:1234", want: "203.0.113.10"},
		{name: "XRealIP", xReal: "198.51.100.5", remote: "192.0.2.1:1234", want: "198.51.100.5"},
		{name: "RemoteAddr", remote: "192.0.2.1:1234", want: "192.0.2.1"},
		{name: "FallbackRemoteAddr", remote: "invalid-addr", want: "invalid-addr"},
	}
	for _, tc := range testCases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.xff != "" {
			req.Header.Set("X-Forwarded-For", tc.xff)
		}
		if tc.xReal != "" {
			req.Header.Set("X-Real-IP", tc.xReal)
		}
		req.RemoteAddr = tc.remote
		got := clientIP(req)
		if got != tc.want {
			t.Fatalf("%s: clientIP()=%s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestRateLimitMiddlewareRejectsAfterQuota(t *testing.T) {
	limiter := MakeRateLimiter(10*time.Second, 10*time.Second)
	limiter.AddRule(http.MethodGet, "/test", Rule{Capacity: 1, Refill: time.Second})
	calls := 0
	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first request status=%d, want 200", rec.Code)
	}
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status=%d, want 429", rec2.Code)
	}
	if calls != 1 {
		t.Fatalf("handler calls=%d, want 1", calls)
	}
	retry := rec2.Header().Get("Retry-After")
	if retry == "" {
		t.Fatalf("Retry-After header missing")
	}
	limit := rec2.Header().Get("X-RateLimit-Limit")
	if limit != "1" {
		t.Fatalf("X-RateLimit-Limit=%s, want 1", limit)
	}
}

func TestRateLimitRefillAllowsAfterWait(t *testing.T) {
	limiter := MakeRateLimiter(10*time.Second, 10*time.Second)
	limiter.AddRule(http.MethodGet, "/refill", Rule{Capacity: 1, Refill: 50 * time.Millisecond})
	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/refill", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first request status=%d, want 200", rec.Code)
	}
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status=%d, want 429", rec2.Code)
	}
	time.Sleep(60 * time.Millisecond)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req)
	if rec3.Code != http.StatusOK {
		t.Fatalf("third request status=%d, want 200 after refill", rec3.Code)
	}
}

func TestProxmoxStatusMiddlewareInjectsContext(t *testing.T) {
	sm := &mockStateManager{connected: false, message: "offline mode"}
	nextCalled := false
	handler := ProxmoxStatusMiddlewareWithState(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		data, ok := r.Context().Value(TemplateDataKey).(map[string]interface{})
		if !ok {
			t.Fatalf("template data missing from context")
		}
		if data["existing"] != "keep" {
			t.Fatalf("existing context value lost")
		}
		if data["ProxmoxConnected"] != false {
			t.Fatalf("ProxmoxConnected not injected correctly")
		}
		if data["ProxmoxError"] != "offline mode" {
			t.Fatalf("ProxmoxError not injected")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), TemplateDataKey, map[string]interface{}{"existing": "keep"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	if !nextCalled {
		t.Fatalf("next handler was not called")
	}
}
