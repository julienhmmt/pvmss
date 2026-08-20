//nolint:wsl_v5 // authentication handlers keep credential validation and response mapping adjacent
package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
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
	Code    string `json:"code"`
	Message string `json:"message"`
}

type tokenRequest struct {
	Label string `json:"label"`
	Scope string `json:"scope"`
}

type tokenResponse struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Scope     string    `json:"scope"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
}

type tokenListItem struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Scope      string     `json:"scope"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

type tokenListResponse struct {
	Tokens []tokenListItem `json:"tokens"`
}

// Auth exposes browser login, session inspection, and logout endpoints.
type Auth struct {
	cluster      cluster.Client
	clusters     cluster.ClientProvider
	clusterStore *store.Store
	sessions     *auth.SessionManager
	adminHash    string
	tokens       *auth.TokenService
	log          *slog.Logger
}

// NewAuth creates the legacy single-cluster authentication endpoint handlers.
func NewAuth(clusterClient cluster.Client, sessions *auth.SessionManager, adminHash string, tokens *auth.TokenService, log *slog.Logger) *Auth {
	return &Auth{cluster: clusterClient, sessions: sessions, adminHash: adminHash, tokens: tokens, log: log}
}

// NewAuthWithRegistry creates authentication handlers with runtime cluster choice.
func NewAuthWithRegistry(registry cluster.ClientProvider, st *store.Store, sessions *auth.SessionManager, adminHash string, tokens *auth.TokenService, log *slog.Logger) *Auth {
	return &Auth{clusters: registry, clusterStore: st, sessions: sessions, adminHash: adminHash, tokens: tokens, log: log}
}

// Login authenticates a PVE cluster account. The local administrator has its
// own endpoint (AdminLogin) — a wrong password means something different for
// each, so neither is a branch inside the other (contracts/auth-login.md).
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
		h.log.Error("select cluster for login failed", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}
	result, err := client.Authenticate(r.Context(), normalizePVEUsername(request.Username), request.Password)
	if err != nil {
		h.log.Info("pve authentication failed", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", msgInvalidCredentials)
		return
	}

	displayName := clusterName
	if h.clusterStore != nil {
		if row, err := h.clusterStore.GetCluster(r.Context(), clusterName); err == nil && row.DisplayName != "" {
			displayName = row.DisplayName
		}
	}
	h.startSession(w, r, auth.Identity{Username: result.Username, Pool: result.Pool, IsAdmin: result.IsAdmin, Cluster: clusterName, ClusterDisplayName: displayName})
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
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", msgInvalidCredentials)
		return
	}

	h.startSession(w, r, auth.Identity{Username: "admin", IsAdmin: true})
}

func (h *Auth) startSession(w http.ResponseWriter, r *http.Request, identity auth.Identity) {
	if err := h.sessions.SetCookie(r.Context(), w, identity); err != nil {
		h.log.Error("failed to create session", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	writeAuthJSON(w, http.StatusOK, identity)
}

// Me returns the resolved browser or bearer-token identity.
func (h *Auth) Me(w http.ResponseWriter, r *http.Request) {
	identity, err := h.Principal(r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	writeAuthJSON(w, http.StatusOK, identity)
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

// RequireAdmin is the admin-only route guard (FR-008): the resolved identity
// must authenticate (401 if not) and have IsAdmin == true (403 if not). It
// duplicates the Principal resolution from Require rather than composing it,
// so it can issue the role check without an extra handler hop. This is the
// only admin-only route guard in v0.4 — T02 shipped authentication only, not
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

// Principal resolves browser cookies before attempting an Authorization bearer token.
func (h *Auth) Principal(r *http.Request) (auth.Identity, error) {
	identity, err := h.sessions.Resolve(r.Context(), r)
	if err == nil {
		return identity, nil
	}

	const bearerPrefix = "Bearer "

	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, bearerPrefix) {
		return auth.Identity{}, auth.ErrUnauthenticated
	}

	return h.tokens.Resolve(r.Context(), strings.TrimSpace(strings.TrimPrefix(authorization, bearerPrefix)))
}

// Logout revokes the authenticated browser session.
func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
		return
	}

	if err := h.sessions.Logout(r.Context(), w, r); err != nil {
		h.log.Error("failed to revoke session", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateToken creates an API token for the browser session identity. Only a
// browser session may mint a token — a token cannot be used to mint another
// (contracts/auth-tokens.md).
func (h *Auth) CreateToken(w http.ResponseWriter, r *http.Request) {
	identity, err := h.sessions.Resolve(r.Context(), r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	var request tokenRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "invalid token request")
		return
	}

	token, raw, err := h.tokens.Create(r.Context(), identity, request.Label, request.Scope)
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeAuthJSON(w, http.StatusCreated, tokenResponse{ID: token.ID, Label: token.Label, Scope: token.Scope, Value: raw, CreatedAt: token.CreatedAt})
}

// ListTokens returns the browser session identity's own tokens. Values are
// never included — only a creation response ever carries one (SC-004).
func (h *Auth) ListTokens(w http.ResponseWriter, r *http.Request) {
	identity, err := h.sessions.Resolve(r.Context(), r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	tokens, err := h.tokens.List(r.Context(), identity.Username)
	if err != nil {
		h.log.Error("failed to list tokens", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	items := make([]tokenListItem, len(tokens))
	for i, token := range tokens {
		items[i] = tokenListItem{ID: token.ID, Label: token.Label, Scope: token.Scope, CreatedAt: token.CreatedAt, LastUsedAt: token.LastUsedAt}
	}

	writeAuthJSON(w, http.StatusOK, tokenListResponse{Tokens: items})
}

// RevokeToken deletes a token owned by the browser session identity. An
// unknown id and a not-owned id both 404, so a caller cannot probe other
// users' token ids (contracts/auth-tokens.md).
func (h *Auth) RevokeToken(w http.ResponseWriter, r *http.Request) {
	identity, err := h.sessions.Resolve(r.Context(), r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	if err := h.tokens.Revoke(r.Context(), r.PathValue("id"), identity.Username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAuthError(w, http.StatusNotFound, "not_found", "token not found")
			return
		}

		h.log.Error("failed to revoke token", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword rotates the browser session identity's cluster password.
// The local administrator has no password to change through this flow — its
// secret is rotated outside the application (spec Assumption).
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
		h.log.Info("password change failed", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", msgInvalidCredentials)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

var errAuthClusterRequired = errors.New("authentication cluster required")

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
		writeAuthJSON(w, http.StatusOK, []authClusterDTO{{Name: "default"}})
		return
	}
	rows, err := h.clusterStore.ListClusters(r.Context())
	if err != nil {
		h.log.Error("list login clusters failed", "component", "httpapi", "error", err)
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

// OIDC returns the deliberate T15 not-implemented response for enabled realms.
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

func normalizePVEUsername(username string) string {
	if strings.Contains(username, "@") {
		return username
	}

	return username + "@pve"
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
