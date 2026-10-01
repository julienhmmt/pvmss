//nolint:wsl_v5 // authentication handlers keep credential validation and response mapping adjacent
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"pvmss/server/internal/telemetry"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const minPasswordLength = 8

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Cluster  string `json:"cluster"`
}

type adminLoginRequest struct {
	Password string `json:"password"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type authError struct {
	Code              string `json:"code"`
	Message           string `json:"message"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
}

// Auth exposes browser login, session inspection, and logout endpoints.
type Auth struct {
	cluster          cluster.Client
	clusters         cluster.ClientProvider
	clusterStore     *store.Store
	sessions         *auth.SessionManager
	adminHash        string
	log              *slog.Logger
	trustedProxyHops int
	freshness        ClusterFreshnessChecker
	staleThreshold   time.Duration
}

// NewAuth creates the legacy single-cluster authentication endpoint handlers.
func NewAuth(clusterClient cluster.Client, sessions *auth.SessionManager, adminHash string, log *slog.Logger) *Auth {
	return &Auth{cluster: clusterClient, sessions: sessions, adminHash: adminHash, log: log}
}

// NewAuthWithRegistry creates authentication handlers with runtime cluster choice.
func NewAuthWithRegistry(registry cluster.ClientProvider, st *store.Store, sessions *auth.SessionManager, adminHash string, log *slog.Logger) *Auth {
	return &Auth{clusters: registry, clusterStore: st, sessions: sessions, adminHash: adminHash, log: log}
}

// SetTrustedProxyHops configures how many X-Forwarded-For hops are trusted
// when extracting the client IP for audit entries.
func (h *Auth) SetTrustedProxyHops(n int) {
	h.trustedProxyHops = n
}

// SetClusterFreshnessChecker wires the cluster health source used to reject
// user logins when the selected cluster is unreachable. Admin login is never
// gated by this check.
func (h *Auth) SetClusterFreshnessChecker(freshness ClusterFreshnessChecker, staleThreshold time.Duration) {
	h.freshness = freshness
	h.staleThreshold = staleThreshold
}

// Login authenticates a PVE cluster account. The local administrator has its
// own endpoint (AdminLogin) - a wrong password means something different for
// each, so neither is a branch inside the other.
func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
		return
	}

	var request loginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "invalid login request")
		return
	}

	request.Username = strings.TrimSpace(request.Username)
	if request.Username == "" || request.Password == "" {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "username and password are required")
		return
	}

	client, clusterName, err := h.loginClient(request.Cluster)
	if errors.Is(err, errAuthClusterRequired) {
		writeAuthError(w, http.StatusBadRequest, "cluster_required", "cluster is required when multiple clusters are configured")
		return
	}
	if errors.Is(err, cluster.ErrClusterNotFound) {
		writeAuthError(w, http.StatusBadRequest, "invalid_cluster", "unknown cluster")
		return
	}
	if err != nil {
		SetErrorMsg(w, "select cluster for login failed", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	if !h.isClusterAvailable(clusterName) {
		logctx.FromOr(r.Context(), h.log).InfoContext(r.Context(), "cluster unavailable, rejecting user login", "component", "httpapi", "cluster", clusterName)
		writeAuthError(w, http.StatusServiceUnavailable, "cluster_unavailable", msgClusterUnavailable)
		return
	}

	result, err := authenticatePVE(r.Context(), client, request.Username, request.Password)
	if err != nil {
		SetErrorMsg(w, "pve authentication failed", err)
		h.recordLoginFailed(r.Context(), request.Username, clientIP(r, h.trustedProxyHops))
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", msgInvalidCredentials)
		return
	}

	displayName := clusterName
	if h.clusterStore != nil {
		if row, err := h.clusterStore.GetCluster(r.Context(), clusterName); err == nil && row.DisplayName != "" {
			displayName = row.DisplayName
		}
	}
	h.startSession(w, r, auth.Identity{Username: result.Username, DisplayName: userDisplayName(request.Username), Pool: result.Pool, IsAdmin: result.IsAdmin, Cluster: clusterName, ClusterDisplayName: displayName})
}

// AdminLogin authenticates the local emergency administrator, independent of any cluster.
func (h *Auth) AdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
		return
	}

	var request adminLoginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "invalid login request")
		return
	}

	if request.Password == "" || h.adminHash == "" || bcrypt.CompareHashAndPassword([]byte(h.adminHash), []byte(request.Password)) != nil {
		// Admin login failures write no audit row (keeps /activity unchanged),
		// so the log stream is the only record. Never log the password.
		telemetry.RecordLogin(r.Context(), "failure")
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "login failed", "component", "httpapi", "event", "auth", "result", "failure", "user", "admin")
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", msgInvalidCredentials)
		return
	}

	h.startSession(w, r, auth.Identity{Username: "admin", DisplayName: "admin", IsAdmin: true})
}

func (h *Auth) startSession(w http.ResponseWriter, r *http.Request, identity auth.Identity) {
	if err := h.sessions.SetCookie(r.Context(), w, identity); err != nil {
		SetErrorMsg(w, "failed to create session", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	telemetry.RecordLogin(r.Context(), "success")
	logctx.FromOr(r.Context(), h.log).InfoContext(r.Context(), "login succeeded", "component", "httpapi", "event", "auth", "result", "success", "user", identity.Username)
	writeAuthJSON(w, http.StatusOK, identity)
}

// Me returns the resolved browser session identity. The cluster
// display name is refreshed from the current database row so the sidebar
// reflects an updated display name without forcing a re-login.
func (h *Auth) Me(w http.ResponseWriter, r *http.Request) {
	identity, err := h.Principal(r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	identity = h.refreshClusterDisplayName(r.Context(), identity)
	if identity.DisplayName == "" {
		identity.DisplayName = userDisplayName(identity.Username)
	}

	writeAuthJSON(w, http.StatusOK, identity)
}

// refreshClusterDisplayName re-reads the cluster row's DisplayName so the
// frontend sidebar and login choices stay up to date when an admin test or
// seeding populates it after the session was created.
func (h *Auth) refreshClusterDisplayName(ctx context.Context, identity auth.Identity) auth.Identity {
	if h.clusterStore == nil || identity.Cluster == "" {
		return identity
	}

	row, err := h.clusterStore.GetCluster(ctx, identity.Cluster)
	if err != nil {
		return identity
	}

	if row.DisplayName != "" {
		identity.ClusterDisplayName = row.DisplayName
	}

	return identity
}

// Require rejects requests that do not resolve to a browser session or API token.
func (h *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := h.Principal(r); err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireAdmin is the admin-only route guard: the resolved identity
// must authenticate (401 if not) and have IsAdmin == true (403 if not). It
// duplicates the Principal resolution from Require rather than composing it,
// so it can issue the role check without an extra handler hop. This is the
// only admin-only route guard in v0.4 - earlier work shipped authentication only, not
// role enforcement. Every /api/v1/admin/* route is wrapped by this, so a
// non-admin identity can never reach an admin handler regardless of the HTTP
// method or path.
func (h *Auth) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := h.Principal(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
			return
		}

		if !identity.IsAdmin {
			writeAuthError(w, http.StatusForbidden, "forbidden", msgAdminOnly)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CSRFToken returns the persisted CSRF token for the request's session, or an
// error if the session cookie is missing, unknown, or expired.
func (h *Auth) CSRFToken(r *http.Request) (string, error) {
	return h.sessions.CSRFToken(r.Context(), r)
}

// Principal resolves the browser session cookie. Bearer credentials are not
// accepted: PVMSS has no API tokens.
func (h *Auth) Principal(r *http.Request) (auth.Identity, error) {
	identity, err := h.sessions.Resolve(r.Context(), r)
	if err == nil {
		// Principal is the single resolution point (Require, RequireAdmin and
		// handlers that call it directly), so tagging here covers them all.
		logctx.AddAttrs(r.Context(), slog.String("user", identity.Username))

		return identity, nil
	}

	return auth.Identity{}, auth.ErrUnauthenticated
}

// Logout revokes the authenticated browser session.
func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
		return
	}

	// Resolve first so the request logger carries the user being logged out.
	_, _ = h.Principal(r)

	if err := h.sessions.Logout(r.Context(), w, r); err != nil {
		SetErrorMsg(w, "failed to revoke session", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	logctx.FromOr(r.Context(), h.log).InfoContext(r.Context(), "logout", "component", "httpapi", "event", "auth", "result", "success")

	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword rotates the browser session identity's cluster password.
// The local administrator has no password to change through this flow - its
// secret is rotated outside the application.
func (h *Auth) ChangePassword(w http.ResponseWriter, r *http.Request) {
	identity, err := h.sessions.Resolve(r.Context(), r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	var request changePasswordRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "invalid password change request")
		return
	}

	if len(request.NewPassword) < minPasswordLength {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("new password must be at least %d characters", minPasswordLength))
		return
	}

	client := h.cluster
	if h.clusters != nil && identity.Cluster != "" {
		selected, selectErr := h.clusters.Client(identity.Cluster)
		if selectErr != nil {
			writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", msgInvalidCredentials)
			return
		}
		client = selected
	}
	if err := client.ChangePassword(r.Context(), identity.Username, request.OldPassword, request.NewPassword); err != nil {
		SetErrorMsg(w, "password change failed", err)
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", msgInvalidCredentials)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

var errAuthClusterRequired = errors.New("authentication cluster required")

// isClusterAvailable reports whether the named cluster has completed a recent
// successful inventory refresh. A missing checker or missing cluster means the
// availability is unknown and the login proceeds (legacy/no-freshness paths).
func (h *Auth) isClusterAvailable(name string) bool {
	if h.freshness == nil {
		return true
	}

	for _, c := range h.freshness.Clusters() {
		if c.Name != name {
			continue
		}

		if c.RefreshedAt.IsZero() {
			return false
		}

		return time.Since(c.RefreshedAt) <= h.staleThreshold
	}

	return false
}

func (h *Auth) loginClient(name string) (cluster.Client, string, error) {
	if h.clusters == nil {
		return h.cluster, "", nil
	}
	names := h.clusters.List()
	if name == "" {
		if len(names) != 1 {
			return nil, "", errAuthClusterRequired
		}
		name = names[0]
	}
	client, err := h.clusters.Client(name)
	if err != nil {
		return nil, "", err
	}
	return client, name, nil
}

type authClusterDTO struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	OIDCEnabled bool   `json:"oidcEnabled"`
}

// ServeClusters exposes the non-secret cluster choices needed before login.
func (h *Auth) ServeClusters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
		return
	}
	if h.clusterStore == nil {
		writeAuthJSON(w, http.StatusOK, []authClusterDTO{})
		return
	}
	rows, err := h.clusterStore.ListClusters(r.Context())
	if err != nil {
		SetErrorMsg(w, "list login clusters failed", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}
	result := make([]authClusterDTO, 0, len(rows))
	for _, row := range rows {
		result = append(result, authClusterDTO{Name: row.Name, DisplayName: row.DisplayName, OIDCEnabled: row.OIDCEnabled})
	}
	writeAuthJSON(w, http.StatusOK, result)
}

type oidcRequest struct {
	Cluster string `json:"cluster"`
}

// OIDC returns the deliberate not-implemented response for enabled realms.
func (h *Auth) OIDC(w http.ResponseWriter, r *http.Request) {
	var request oidcRequest
	if err := decodeJSON(w, r, &request); err != nil || request.Cluster == "" {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "cluster is required")
		return
	}
	if h.clusterStore != nil {
		row, err := h.clusterStore.GetCluster(r.Context(), request.Cluster)
		if err != nil || !row.OIDCEnabled {
			writeAuthError(w, http.StatusNotFound, "not_found", "OIDC is not enabled for this cluster")
			return
		}
	}
	writeAuthError(w, http.StatusNotImplemented, "not_implemented", "OIDC sign-in is not implemented")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) error {
	return decodeJSONLimit(w, r, dest, 4096)
}

func decodeJSONLimit(w http.ResponseWriter, r *http.Request, dest any, maxBytes int64) error {
	defer func() { _ = r.Body.Close() }()

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("decode request: multiple json values")
	}

	return nil
}

// authenticatePVE tries the typed username, then - on a not-found rejection -
// retries once under the "pvmss-" pool prefix. Self-service pool users are
// provisioned as "pvmss-{pool}@{realm}" (pools.CreateManaged) but only ever
// told their pool name, so a bare "jho" must resolve to "pvmss-jho@pve"
// without the user needing to know the internal naming convention.
func authenticatePVE(ctx context.Context, client cluster.Client, rawUsername, password string) (cluster.Identity, error) {
	username := normalizePVEUsername(rawUsername)

	result, err := client.Authenticate(ctx, username, password)
	if err != nil && errors.Is(err, cluster.ErrNotFound) {
		if poolUsername, ok := withPoolPrefix(username); ok {
			result, err = client.Authenticate(ctx, poolUsername, password)
		}
	}

	return result, err
}

func normalizePVEUsername(username string) string {
	if strings.Contains(username, "@") {
		return username
	}

	return username + "@pve"
}

// withPoolPrefix inserts pools.PoolPrefix before the realm, e.g.
// "jho@pve" -> "pvmss-jho@pve". ok is false when the local part already
// carries the prefix, since there is then nothing new to retry with.
func withPoolPrefix(username string) (prefixed string, ok bool) {
	local, realm, _ := strings.Cut(username, "@")
	if strings.HasPrefix(local, pools.PoolPrefix) {
		return "", false
	}

	return pools.PoolPrefix + local + "@" + realm, true
}

func writeAuthJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	_ = writeJSON(w, status, body)
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	writeAuthJSON(w, status, authError{Code: code, Message: message})
}

// userDisplayName extracts the local part of a Proxmox username, stripping the
// optional pvmss- pool prefix so the UI can show "jho" instead of "pvmss-jho@pve".
func userDisplayName(username string) string {
	local, _, _ := strings.Cut(username, "@")
	return strings.TrimPrefix(local, pools.PoolPrefix)
}

func (h *Auth) recordLoginFailed(ctx context.Context, username, ip string) {
	telemetry.RecordLogin(ctx, "failure")

	if h.clusterStore == nil {
		return
	}

	detail := loginFailedDetail(username)
	_ = h.clusterStore.RecordAdminAction(ctx, username, "auth.login_failed", "auth", username, detail, ip)
}

func loginFailedDetail(username string) string {
	body, err := json.Marshal(map[string]any{auditKeySummary: fmt.Sprintf("login failed for user %q", username), auditKeyChanges: []any{}})
	if err != nil {
		return `{"summary":"login failed","changes":[]}`
	}

	return string(body)
}
