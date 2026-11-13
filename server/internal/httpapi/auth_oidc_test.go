package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"strconv"
	"strings"
	"testing"
	"time"
)

const oidcStartPath = "/api/v1/auth/oidc/start"

func newOIDCHandler(t *testing.T, realm string) *httpapi.Auth {
	t.Helper()

	handler, st := newAuthHandlerWithStore(t)
	if err := st.SetClusterOIDCRealm(context.Background(), auditTestCluster, realm); err != nil {
		t.Fatalf("SetClusterOIDCRealm: %v", err)
	}

	return handler
}

func oidcStart(handler *httpapi.Auth, query string) *httptest.ResponseRecorder {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://pvmss.example"+oidcStartPath+"?"+query, nil)
	response := httptest.NewRecorder()
	handler.OIDCStart(response, request)

	return response
}

func oidcCallback(handler *httpapi.Auth, location string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, location, nil)
	for _, c := range cookies {
		request.AddCookie(c)
	}

	response := httptest.NewRecorder()
	handler.OIDCCallback(response, request)

	return response
}

func findCookie(response *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range response.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}

	return nil
}

func TestOIDC_StartAndCallbackSignsIn(t *testing.T) {
	t.Parallel()

	handler := newOIDCHandler(t, "zitadel")

	start := oidcStart(handler, "cluster="+auditTestCluster+"&next=%2Fvms%2F42")
	if start.Code != http.StatusFound {
		t.Fatalf("start status = %d: %s", start.Code, start.Body.String())
	}

	handshake := findCookie(start, httpapi.OIDCCookieName)
	if handshake == nil || handshake.SameSite != http.SameSiteLaxMode || !handshake.HttpOnly || handshake.MaxAge <= 0 || handshake.MaxAge > 600 {
		t.Fatalf("handshake cookie = %+v, want Lax, HttpOnly, short-lived", handshake)
	}

	// The fake IdP redirects straight back to the callback with its code.
	idp := start.Header().Get("Location")
	if !strings.HasPrefix(idp, "https://pvmss.example/api/v1/auth/oidc/callback?") {
		t.Fatalf("auth url = %q, want the request origin's callback", idp)
	}

	callback := oidcCallback(handler, idp, []*http.Cookie{handshake})
	if callback.Code != http.StatusFound || callback.Header().Get("Location") != "/vms/42" {
		t.Fatalf("callback = %d -> %q", callback.Code, callback.Header().Get("Location"))
	}

	assertOIDCSession(t, handler, callback)
}

func assertOIDCSession(t *testing.T, handler *httpapi.Auth, callback *httptest.ResponseRecorder) {
	t.Helper()

	session := findCookie(callback, auth.SessionCookieName)
	if session == nil || session.SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie = %+v, want Strict", session)
	}

	if cleared := findCookie(callback, httpapi.OIDCCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("handshake cookie not cleared: %+v", cleared)
	}

	me := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/me", nil)
	me.AddCookie(session)

	identity, err := handler.Principal(me)
	if err != nil || identity.Provider != auth.ProviderOIDC || identity.Username != cluster.FakeUserAlice {
		t.Fatalf("principal = %+v, %v", identity, err)
	}
}

func TestOIDC_StartRejections(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, realm, query, reason string
	}{
		{name: "realm unset", realm: "", query: "cluster=" + auditTestCluster, reason: "oidc_not_configured"},
		{name: "unknown cluster", realm: "zitadel", query: "cluster=nope", reason: "invalid_cluster"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			response := oidcStart(newOIDCHandler(t, tc.realm), tc.query)
			if response.Code != http.StatusFound || response.Header().Get("Location") != "/login?oidc=error&reason="+tc.reason {
				t.Fatalf("got %d -> %q", response.Code, response.Header().Get("Location"))
			}

			if findCookie(response, httpapi.OIDCCookieName) != nil {
				t.Fatal("handshake cookie set on a rejected start")
			}
		})
	}
}

func TestOIDC_NextOpenRedirectFallsBackToRoot(t *testing.T) {
	t.Parallel()

	for _, next := range []string{"//evil.example", "https://evil.example", "/\\evil.example", "evil", "/api/v1/auth/logout"} {
		handler := newOIDCHandler(t, "zitadel")
		start := oidcStart(handler, "cluster="+auditTestCluster+"&next="+url.QueryEscape(next))
		callback := oidcCallback(handler, start.Header().Get("Location"), []*http.Cookie{findCookie(start, httpapi.OIDCCookieName)})

		if got := callback.Header().Get("Location"); got != "/" {
			t.Fatalf("next %q redirected to %q, want /", next, got)
		}
	}
}

func TestOIDC_CallbackRejectsForgedOrMissingState(t *testing.T) {
	t.Parallel()

	handler := newOIDCHandler(t, "zitadel")
	start := oidcStart(handler, "cluster="+auditTestCluster)
	handshake := findCookie(start, httpapi.OIDCCookieName)
	callbackURL := start.Header().Get("Location")

	forged := &http.Cookie{Name: handshake.Name, Value: "x" + handshake.Value, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}

	cases := map[string]struct {
		location string
		cookies  []*http.Cookie
	}{
		"no cookie":     {location: callbackURL},
		"forged cookie": {location: callbackURL, cookies: []*http.Cookie{forged}},
		"wrong state":   {location: strings.Replace(callbackURL, "state=", "state=x", 1), cookies: []*http.Cookie{handshake}},
		"bad code":      {location: strings.Replace(callbackURL, "code=", "code=x", 1), cookies: []*http.Cookie{handshake}},
	}

	for name, tc := range cases {
		response := oidcCallback(handler, tc.location, tc.cookies)
		if !strings.HasPrefix(response.Header().Get("Location"), "/login?oidc=error") {
			t.Fatalf("%s: redirected to %q, want an error", name, response.Header().Get("Location"))
		}

		if findCookie(response, auth.SessionCookieName) != nil {
			t.Fatalf("%s: session cookie issued", name)
		}
	}
}

func TestOIDC_PasswordChangeRefused(t *testing.T) {
	t.Parallel()

	handler := newOIDCHandler(t, "zitadel")
	start := oidcStart(handler, "cluster="+auditTestCluster)
	session := findCookie(oidcCallback(handler, start.Header().Get("Location"), []*http.Cookie{findCookie(start, httpapi.OIDCCookieName)}), auth.SessionCookieName)

	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"whatever1","newPassword":"whatever2"}`))
	request.AddCookie(session)

	response := httptest.NewRecorder()
	handler.ChangePassword(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "provider_unsupported") {
		t.Fatalf("got %d %s, want 400 provider_unsupported", response.Code, response.Body.String())
	}
}

// TestOIDC_CallbackThroughRouter: the IdP's redirect back is a cross-site
// GET with no session and no CSRF header; no global guard may block it.
func TestOIDC_CallbackThroughRouter(t *testing.T) {
	t.Parallel()

	handler := newOIDCHandler(t, "zitadel")
	mux := newCSRFRouter(t, handler)

	startReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://pvmss.example"+oidcStartPath+"?cluster="+auditTestCluster, nil)
	start := httptest.NewRecorder()
	mux.ServeHTTP(start, startReq)

	if start.Code != http.StatusFound {
		t.Fatalf("start through router = %d: %s", start.Code, start.Body.String())
	}

	callbackReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, start.Header().Get("Location"), nil)
	callbackReq.Header.Set("Sec-Fetch-Site", "cross-site")
	callbackReq.AddCookie(findCookie(start, httpapi.OIDCCookieName))

	callback := httptest.NewRecorder()
	mux.ServeHTTP(callback, callbackReq)

	if callback.Code != http.StatusFound || callback.Header().Get("Location") != "/" {
		t.Fatalf("callback through router = %d -> %q: %s", callback.Code, callback.Header().Get("Location"), callback.Body.String())
	}
}

func TestOIDC_ExpiredHandshakeRejected(t *testing.T) {
	t.Parallel()

	handler := newOIDCHandler(t, "zitadel")

	signer, err := auth.NewSessionManager(newSessionRepository(), "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	expired := strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10)
	cookie := &http.Cookie{Name: httpapi.OIDCCookieName, Value: signer.Sign(auditTestCluster + "\nfake-oidc-state\n/\n" + expired), HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}

	response := oidcCallback(handler, "https://pvmss.example/api/v1/auth/oidc/callback?code="+cluster.FakeOpenIDCode+"&state=fake-oidc-state", []*http.Cookie{cookie})
	if response.Header().Get("Location") != "/login?oidc=error&reason=oidc_state_invalid" {
		t.Fatalf("expired handshake redirected to %q", response.Header().Get("Location"))
	}
}

func TestOIDC_IdPCancelIsNotAFailedLogin(t *testing.T) {
	t.Parallel()

	handler := newOIDCHandler(t, "zitadel")

	for range 10 {
		start := oidcStart(handler, "cluster="+auditTestCluster)
		response := oidcCallback(handler, "https://pvmss.example/api/v1/auth/oidc/callback?error=access_denied&state=fake-oidc-state", []*http.Cookie{findCookie(start, httpapi.OIDCCookieName)})

		if got := response.Header().Get("Location"); got != "/login?oidc=error&reason=oidc_cancelled" {
			t.Fatalf("cancel redirected to %q", got)
		}
	}

	// Ten cancels later, a real sign-in is still allowed.
	start := oidcStart(handler, "cluster="+auditTestCluster)
	if callback := oidcCallback(handler, start.Header().Get("Location"), []*http.Cookie{findCookie(start, httpapi.OIDCCookieName)}); callback.Header().Get("Location") != "/" {
		t.Fatalf("after cancels: %q", callback.Header().Get("Location"))
	}
}

func TestOIDC_StartClusterUnavailable(t *testing.T) {
	t.Parallel()

	handler := newOIDCHandler(t, "zitadel")
	handler.SetClusterFreshnessChecker(fakeFreshnessChecker{clusters: []httpapi.ClusterFreshness{{Name: auditTestCluster, RefreshedAt: time.Now().Add(-time.Hour)}}}, time.Minute)

	response := oidcStart(handler, "cluster="+auditTestCluster)
	if response.Code != http.StatusFound || response.Header().Get("Location") != "/login?oidc=error&reason=cluster_unavailable" {
		t.Fatalf("got %d -> %q, want cluster_unavailable redirect", response.Code, response.Header().Get("Location"))
	}
}

func TestOIDC_StartBehindTrustedProxy(t *testing.T) {
	t.Parallel()

	handler := newOIDCHandler(t, "zitadel")
	handler.SetTrustedProxyHops(1)

	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://pvmss.example"+oidcStartPath+"?cluster="+auditTestCluster, nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "portal.example.com")

	response := httptest.NewRecorder()
	handler.OIDCStart(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want redirect to IdP", response.Code)
	}

	if loc := response.Header().Get("Location"); !strings.Contains(loc, "portal.example.com") {
		t.Fatalf("redirect = %q, want the forwarded host in the callback URL", loc)
	}
}
