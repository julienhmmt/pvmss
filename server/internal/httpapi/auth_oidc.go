package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/telemetry"
	"strconv"
	"strings"
	"time"
)

const (
	// OIDCCookieName carries the signed SSO handshake (cluster, Proxmox state,
	// next path, expiry) from start to callback.
	OIDCCookieName   = "pvmss_oidc"
	oidcCookiePath   = "/api/v1/auth/oidc"
	oidcCallbackPath = oidcCookiePath + "/callback"
	oidcHandshakeTTL = 10 * time.Minute
	maxNextPathBytes = 512
)

// OIDCStart handles GET /api/v1/auth/oidc/start?cluster=&next=: it asks the
// cluster's Proxmox OpenID realm for the IdP URL, pins the returned state in
// a signed cookie and redirects the browser to the IdP. Failures redirect to
// /login with a reason code: this is a browser navigation, not a fetch.
func (h *Auth) OIDCStart(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	client, clusterName, err := h.loginClient(query.Get("cluster"))
	if err != nil {
		h.oidcFail(w, r, oidcClusterReason(err), err)
		return
	}

	realm := h.oidcRealm(r, clusterName)
	openid, ok := client.(cluster.OpenIDAuthenticator)

	if realm == "" || !ok {
		h.oidcFail(w, r, "oidc_not_configured", nil)
		return
	}

	if !h.isClusterAvailable(clusterName) {
		h.oidcFail(w, r, "cluster_unavailable", nil)
		return
	}

	authURL, state, err := openid.OpenIDAuthURL(r.Context(), realm, h.oidcRedirectURL(r))
	if err != nil {
		h.oidcFail(w, r, "oidc_unavailable", err)
		return
	}

	expires := strconv.FormatInt(time.Now().Add(oidcHandshakeTTL).Unix(), 10)
	payload := strings.Join([]string{clusterName, state, safeNextPath(query.Get("next")), expires}, "\n")
	http.SetCookie(w, h.sessions.HandshakeCookie(OIDCCookieName, h.sessions.Sign(payload), oidcCookiePath, int(oidcHandshakeTTL.Seconds())))
	http.Redirect(w, r, authURL, http.StatusFound) //nolint:gosec // G710: authURL is the IdP URL returned by Proxmox for the admin-configured realm
}

// OIDCCallback handles GET /api/v1/auth/oidc/callback?code=&state=: it checks
// the state against the signed handshake, lets Proxmox exchange the code, and
// opens a PVMSS session for the returned identity.
func (h *Auth) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	// One-shot: the handshake is cleared whatever happens next.
	http.SetCookie(w, h.sessions.HandshakeCookie(OIDCCookieName, "", oidcCookiePath, -1))

	clusterName, state, next, ok := h.readHandshake(r)
	if !ok {
		h.oidcFail(w, r, "oidc_state_invalid", nil)
		return
	}

	// The user backed out at the IdP: not a failed credential, never throttled.
	if r.URL.Query().Get("error") != "" {
		h.oidcFail(w, r, "oidc_cancelled", nil)
		return
	}

	failKey := clusterName + "|oidc|" + clientIP(r, h.trustedProxyHops)
	if h.pveFailures.exhausted(failKey, time.Now()) {
		h.oidcFail(w, r, "rate_limited", nil)
		return
	}

	client, _, err := h.loginClient(clusterName)
	openid, isOpenID := client.(cluster.OpenIDAuthenticator)

	if err != nil || !isOpenID {
		h.oidcFail(w, r, "oidc_not_configured", err)
		return
	}

	result, err := openid.OpenIDLogin(r.Context(), r.URL.Query().Get("code"), state, h.oidcRedirectURL(r))
	if err != nil {
		h.pveFailures.allow(failKey, time.Now())
		h.recordLoginFailed(r.Context(), "oidc", clientIP(r, h.trustedProxyHops))
		h.oidcFail(w, r, oidcLoginReason(err), err)

		return
	}

	h.pveFailures.reset(failKey)

	identity := auth.Identity{Username: result.Username, DisplayName: userDisplayName(result.Username), Pool: result.Pool, IsAdmin: result.IsAdmin, Cluster: clusterName, ClusterDisplayName: h.clusterDisplayName(r, clusterName), Provider: auth.ProviderOIDC}
	if err := h.sessions.SetCookie(r.Context(), w, identity); err != nil {
		h.oidcFail(w, r, "internal_error", err)
		return
	}

	telemetry.RecordLogin(r.Context(), "success")
	logctx.FromOr(r.Context(), h.log).InfoContext(r.Context(), "login succeeded", "component", "httpapi", "event", "auth", "result", "success", "user", identity.Username, "provider", string(auth.ProviderOIDC))
	h.recordOIDCLogin(r, identity)
	http.Redirect(w, r, next, http.StatusFound) //nolint:gosec // G710: next passed safeNextPath and was signed at start
}

// readHandshake verifies the signed cookie, its expiry and the echoed state.
func (h *Auth) readHandshake(r *http.Request) (clusterName, state, next string, ok bool) {
	cookie, err := r.Cookie(OIDCCookieName)
	if err != nil {
		return "", "", "", false
	}

	payload, ok := h.sessions.Verify(cookie.Value)
	if !ok {
		return "", "", "", false
	}

	parts := strings.Split(payload, "\n")
	if len(parts) != 4 {
		return "", "", "", false
	}

	expires, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return "", "", "", false
	}

	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(parts[1])) != 1 {
		return "", "", "", false
	}

	return parts[0], parts[1], parts[2], true
}

// oidcFail redirects to /login with a reason code. A redirect is not an error
// status, so the access log does not carry err: log it here, once.
func (h *Auth) oidcFail(w http.ResponseWriter, r *http.Request, reason string, err error) {
	attrs := []any{"component", "httpapi", "event", "auth", "result", "failure", "reason", reason}
	if err != nil {
		attrs = append(attrs, "error", err)
	}

	logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "oidc login failed", attrs...)
	http.Redirect(w, r, "/login?oidc=error&reason="+reason, http.StatusFound)
}

func (h *Auth) oidcRealm(r *http.Request, clusterName string) string {
	if h.clusterStore == nil {
		return ""
	}

	row, err := h.clusterStore.GetCluster(r.Context(), clusterName)
	if err != nil {
		return ""
	}

	return row.OIDCRealm
}

func (h *Auth) clusterDisplayName(r *http.Request, clusterName string) string {
	if h.clusterStore != nil {
		if row, err := h.clusterStore.GetCluster(r.Context(), clusterName); err == nil && row.DisplayName != "" {
			return row.DisplayName
		}
	}

	return clusterName
}

// oidcRedirectURL is PVMSS's own callback, from the request origin. Forwarded
// headers count only behind trusted proxies. The IdP's allow-list of redirect
// URIs is what stops a forged Host from steering the code elsewhere.
func (h *Auth) oidcRedirectURL(r *http.Request) string {
	scheme, host := "http", r.Host
	if r.TLS != nil {
		scheme = "https"
	}

	if h.trustedProxyHops > 0 {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
			scheme = proto
		}

		if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
			host = fwd
		}
	}

	return (&url.URL{Scheme: scheme, Host: host, Path: oidcCallbackPath}).String()
}

func (h *Auth) recordOIDCLogin(r *http.Request, identity auth.Identity) {
	if h.clusterStore == nil {
		return
	}

	detail, _ := json.Marshal(map[string]any{auditKeySummary: "signed in with SSO", auditKeyChanges: []any{map[string]any{"provider": string(auth.ProviderOIDC), "cluster": identity.Cluster}}})
	if err := h.clusterStore.RecordAdminAction(r.Context(), identity.Username, "auth.login_oidc", "auth", identity.Username, string(detail), clientIP(r, h.trustedProxyHops)); err != nil {
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "oidc login audit failed", "component", "httpapi", "error", err)
	}
}

// safeNextPath keeps only an in-app SPA path; anything that could leave the
// origin (scheme, //host, backslash tricks) or hit the API falls back to "/".
func safeNextPath(next string) string {
	if len(next) > maxNextPathBytes || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") ||
		strings.ContainsAny(next, "\\\r\n\t") || strings.HasPrefix(next, "/api/") {
		return "/"
	}

	return next
}

func oidcClusterReason(err error) string {
	switch {
	case errors.Is(err, errAuthClusterRequired):
		return "cluster_required"
	case errors.Is(err, cluster.ErrClusterNotFound):
		return "invalid_cluster"
	default:
		return "internal_error"
	}
}

func oidcLoginReason(err error) string {
	if errors.Is(err, cluster.ErrNotFound) {
		return "oidc_rejected"
	}

	return "oidc_unavailable"
}
