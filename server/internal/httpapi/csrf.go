package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/store"
	"strings"
)

//nolint:gosec // G101: this is a public HTTP header name, not a credential.
const csrfTokenHeader = "X-CSRF-Token"

// csrfMiddleware holds the CSRF enforcer and the audit dependency.
type csrfMiddleware struct {
	auth             *Auth
	store            *store.Store
	trustedProxyHops int
}

// newCSRFMiddleware returns a middleware that enforces the CSRF token for
// state-changing requests under /api/v1/. Browser sessions must present the
// pvmss_csrf cookie and the X-CSRF-Token header; both must match the token
// stored in the server-side session row.
//
// Public unauthenticated routes (login, admin-login, clusters, OIDC) and
// requests authenticated by an Authorization bearer token are not subject to
// the cookie-based check.
func newCSRFMiddleware(authHandler *Auth, st *store.Store, trustedProxyHops int) func(http.Handler) http.Handler {
	m := &csrfMiddleware{auth: authHandler, store: st, trustedProxyHops: trustedProxyHops}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !csrfRequired(r) {
				next.ServeHTTP(w, r)

				return
			}

			// Bearer-token clients (automation scripts) are not browser
			// sessions, so the cookie-based CSRF check does not apply.
			if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				next.ServeHTTP(w, r)

				return
			}

			// No session cookie means the request cannot be a forged browser
			// session; the downstream handler is responsible for 401.
			if _, err := r.Cookie(auth.SessionCookieName); err != nil {
				next.ServeHTTP(w, r)

				return
			}

			serverToken, err := authHandler.CSRFToken(r)
			if err != nil {
				m.recordCSRFRejected(r.Context(), r, "missing or invalid CSRF token")
				writeAuthError(w, http.StatusForbidden, "invalid_csrf_token", "invalid or missing csrf token")

				return
			}

			header := r.Header.Get(csrfTokenHeader)
			if header == "" {
				header = r.PostFormValue("csrf_token")
			}

			cookie, err := r.Cookie(auth.CSRFCookieName)
			if err != nil || !constantTimeEqual(serverToken, header) || !constantTimeEqual(serverToken, cookie.Value) {
				m.recordCSRFRejected(r.Context(), r, "CSRF token mismatch")
				writeAuthError(w, http.StatusForbidden, "invalid_csrf_token", "invalid or missing csrf token")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *csrfMiddleware) recordCSRFRejected(ctx context.Context, r *http.Request, summary string) {
	if m.store == nil {
		return
	}

	actor := ""
	if identity, err := m.auth.Principal(r); err == nil {
		actor = identity.Username
	}

	body, _ := json.Marshal(map[string]any{auditKeySummary: "CSRF rejected: " + summary, auditKeyChanges: []any{}})
	_ = m.store.RecordAdminAction(ctx, actor, "auth.csrf_rejected", "auth", actor, string(body), clientIP(r, m.trustedProxyHops))
}

// csrfRequired reports whether the request is a state-changing API call that
// should carry a CSRF token.
func csrfRequired(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}

	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		return false
	}

	switch r.URL.Path {
	case "/api/v1/auth/login", "/api/v1/auth/admin-login", "/api/v1/auth/clusters", "/api/v1/auth/oidc":
		return false
	}

	return true
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
