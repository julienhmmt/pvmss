package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
)

const minPasswordLength = 8

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
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
	cluster   cluster.Client
	sessions  *auth.SessionManager
	adminHash string
	tokens    *auth.TokenService
	log       *slog.Logger
}

// NewAuth creates the authentication endpoint handlers.
func NewAuth(clusterClient cluster.Client, sessions *auth.SessionManager, adminHash string, tokens *auth.TokenService, log *slog.Logger) *Auth {
	return &Auth{cluster: clusterClient, sessions: sessions, adminHash: adminHash, tokens: tokens, log: log}
}

// Login authenticates a PVE cluster account. The local administrator has its
// own endpoint (AdminLogin) — a wrong password means something different for
// each, so neither is a branch inside the other (contracts/auth-login.md).
func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
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
	result, err := h.cluster.Authenticate(r.Context(), normalizePVEUsername(request.Username), request.Password)
	if err != nil {
		h.log.Info("pve authentication failed", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}
	h.startSession(w, r, auth.Identity{Username: result.Username, IsAdmin: result.IsAdmin})
}

// AdminLogin authenticates the local emergency administrator, independent of any cluster.
func (h *Auth) AdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var request adminLoginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "invalid login request")
		return
	}
	if request.Password == "" || h.adminHash == "" || bcrypt.CompareHashAndPassword([]byte(h.adminHash), []byte(request.Password)) != nil {
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}
	h.startSession(w, r, auth.Identity{Username: "admin", IsAdmin: true})
}

func (h *Auth) startSession(w http.ResponseWriter, r *http.Request, identity auth.Identity) {
	if err := h.sessions.SetCookie(r.Context(), w, identity); err != nil {
		h.log.Error("failed to create session", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	writeAuthJSON(w, http.StatusOK, identity)
}

// Me returns the resolved browser or bearer-token identity.
func (h *Auth) Me(w http.ResponseWriter, r *http.Request) {
	identity, err := h.Principal(r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	writeAuthJSON(w, http.StatusOK, identity)
}

// Require rejects requests that do not resolve to a browser session or API token.
func (h *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := h.Principal(r); err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
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
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if err := h.sessions.Logout(r.Context(), w, r); err != nil {
		h.log.Error("failed to revoke session", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", "internal server error")
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
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
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
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	tokens, err := h.tokens.List(r.Context(), identity.Username)
	if err != nil {
		h.log.Error("failed to list tokens", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", "internal server error")
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
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	if err := h.tokens.Revoke(r.Context(), r.PathValue("id"), identity.Username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAuthError(w, http.StatusNotFound, "not_found", "token not found")
			return
		}
		h.log.Error("failed to revoke token", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "internal_error", "internal server error")
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
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
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
	if err := h.cluster.ChangePassword(r.Context(), identity.Username, request.OldPassword, request.NewPassword); err != nil {
		h.log.Info("password change failed", "component", "httpapi", "error", err)
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) error {
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("decode request: %w", err)
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
		writeAuthError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	_ = writeJSON(w, status, body)
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	writeAuthJSON(w, status, authError{Code: code, Message: message})
}
