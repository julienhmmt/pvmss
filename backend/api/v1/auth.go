package apiv1

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"pvmss/constants"
	envpkg "pvmss/env"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
	"pvmss/utils"
)

const (
	accessTokenCookie  = "access_token"
	refreshTokenCookie = "refresh_token"
	accessTokenTTL     = 15 * time.Minute
	refreshTokenTTL    = 7 * 24 * time.Hour
)

// AuthHandler handles JWT authentication endpoints.
type AuthHandler struct {
	state     state.StateManager
	jwtSecret string
}

// MakeAuthHandler creates a new AuthHandler.
// jwtSecret must be the JWT_SECRET environment variable value (minimum 32 bytes).
func MakeAuthHandler(s state.StateManager, jwtSecret string) *AuthHandler {
	return &AuthHandler{state: s, jwtSecret: jwtSecret}
}

// Login handles POST /api/v1/auth/login.
// Body: {"username":"...", "password":"...", "admin": true/false}
// If admin=true → bcrypt check against ADMIN_PASSWORD_HASH env var.
// If admin=false → verify via Proxmox /access/ticket.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	secret := h.jwtSecret
	if secret == "" {
		errNotConfigured(w)
		return
	}

	var req LoginRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		errBadRequest(w, "username and password are required")
		return
	}

	var isAdmin bool

	if req.Admin {
		hash := h.state.GetEnvConfig().AdminPasswordHash
		if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
			logger.SecurityEvent("api_auth_admin_fail").Str("username", req.Username).Msg("Invalid admin credentials")
			errUnauthorized(w)
			return
		}
		isAdmin = true
	} else {
		if h.state.IsOfflineMode() {
			errOffline(w)
			return
		}
		if err := verifyProxmoxCredentials(r.Context(), h.state.GetEnvConfig(), req.Username, req.Password); err != nil {
			logger.SecurityEvent("api_auth_user_fail").Str("username", req.Username).Err(err).Msg("Invalid Proxmox credentials")
			errUnauthorized(w)
			return
		}
		isAdmin = false
	}

	env := h.state.GetEnvConfig().Environment
	if err := issueTokens(w, secret, req.Username, isAdmin, env); err != nil {
		writeAppError(w, err)
		return
	}

	logger.AuthEvent("api_login").Str("username", req.Username).Bool("is_admin", isAdmin).Msg("API login successful")
	writeJSON(w, AuthResponse{Username: req.Username, IsAdmin: isAdmin})
}

// Exchange handles POST /api/v1/auth/exchange.
// Validates the existing JWT access token cookie and returns the current user.
// Used by the SvelteKit SPA on load to verify authentication state.
func (h *AuthHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	secret := h.jwtSecret
	if secret == "" {
		errNotConfigured(w)
		return
	}

	cookie, err := r.Cookie(accessTokenCookie)
	if err != nil {
		errUnauthorized(w)
		return
	}

	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid || claims.Username == "" {
		errUnauthorized(w)
		return
	}

	writeJSON(w, AuthResponse{Username: claims.Username, IsAdmin: claims.IsAdmin})
}

// Refresh handles POST /api/v1/auth/refresh.
// Reads the refresh_token cookie and issues a new access_token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	secret := h.jwtSecret
	if secret == "" {
		errNotConfigured(w)
		return
	}

	cookie, err := r.Cookie(refreshTokenCookie)
	if err != nil {
		errUnauthorized(w)
		return
	}

	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		errUnauthorized(w)
		return
	}

	setTokenCookie(w, secret, accessTokenCookie, claims.Username, claims.IsAdmin, accessTokenTTL, h.state.GetEnvConfig().Environment)
	writeJSON(w, AuthResponse{Username: claims.Username, IsAdmin: claims.IsAdmin})
}

// Me handles GET /api/v1/auth/me. Requires JWTMiddleware upstream.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, MeResponse{
		Username: usernameFromCtx(r),
		IsAdmin:  isAdminFromCtx(r),
	})
}

// Logout handles POST /api/v1/auth/logout. Clears both token cookies.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	secure := utils.IsProduction(h.state.GetEnvConfig().Environment)

	http.SetCookie(w, &http.Cookie{Name: accessTokenCookie, Value: "", MaxAge: -1, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: refreshTokenCookie, Value: "", MaxAge: -1, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
	writeJSON(w, map[string]bool{"ok": true})
}

// ProxmoxAdminLogin handles POST /api/v1/auth/proxmox-admin-login.
// Body: {"username":"user@pve","password":"..."}
// Authenticates via Proxmox @pve realm, requires PVEAdmin or PVMSS_Admin role, issues admin JWT.
func (h *AuthHandler) ProxmoxAdminLogin(w http.ResponseWriter, r *http.Request) {
	secret := h.jwtSecret
	if secret == "" {
		errNotConfigured(w)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		errBadRequest(w, "username and password are required")
		return
	}

	cfg := h.state.GetEnvConfig()
	pxClient, err := proxmox.MakeRestyClientCookieAuthFromEnvConfig(cfg, 10*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ticketResp, err := proxmox.CreateTicketResty(ctx, pxClient, req.Username, req.Password, &proxmox.CreateTicketOptions{Realm: "pve"})
	if err != nil {
		logger.SecurityEvent("api_auth_pve_admin_fail").Str("username", req.Username).Msg("PVE admin login failed")
		errUnauthorized(w)
		return
	}

	if !proxmox.HasRole(ticketResp.Cap, "PVEAdmin") && !proxmox.HasRole(ticketResp.Cap, "PVMSS_Admin") {
		logger.SecurityEvent("api_auth_pve_admin_role_denied").Str("username", req.Username).Msg("PVE admin login denied: no admin role")
		errUnauthorized(w)
		return
	}

	if err := issueTokens(w, secret, ticketResp.Username, true, h.state.GetEnvConfig().Environment); err != nil {
		writeAppError(w, err)
		return
	}

	logger.AuthEvent("api_pve_admin_login").Str("username", ticketResp.Username).Msg("PVE admin login successful")
	writeJSON(w, AuthResponse{Username: ticketResp.Username, IsAdmin: true})
}

// issueTokens creates and sets both access_token and refresh_token cookies.
func issueTokens(w http.ResponseWriter, secret, username string, isAdmin bool, env string) error {
	setTokenCookie(w, secret, accessTokenCookie, username, isAdmin, accessTokenTTL, env)
	setTokenCookie(w, secret, refreshTokenCookie, username, isAdmin, refreshTokenTTL, env)
	return nil
}

// setTokenCookie mints a signed JWT and writes it as an HttpOnly SameSite=Strict cookie.
func setTokenCookie(w http.ResponseWriter, secret, name, username string, isAdmin bool, ttl time.Duration, env string) {
	claims := JWTClaims{
		Username: username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte(secret))

	secure := utils.IsProduction(env)

	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    signed,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ChangePassword handles PUT /api/v1/auth/me/password. Requires JWTMiddleware upstream.
// Body: { "current": "...", "new": "..." }
// Admin users cannot change password via this endpoint (managed via env var).
// For regular users: verifies the current Proxmox password, then updates via API.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}

	isAdmin := isAdminFromCtx(r)
	username := usernameFromCtx(r)

	// Built-in admin password cannot be changed via API.
	// Proxmox admins (who have an @realm in their username) can change their passwords.
	if isAdmin && !strings.Contains(username, "@") {
		writeError(w, http.StatusForbidden, "forbidden", "Admin password is managed via environment variable")
		return
	}

	if strings.HasSuffix(username, "@pam") {
		writeError(w, http.StatusBadRequest, "bad_request", "Passwords for @pam users must be changed at the OS level")
		return
	}

	var req struct {
		Current         string `json:"current"`
		New             string `json:"new"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	currentPassword := req.CurrentPassword
	if currentPassword == "" {
		currentPassword = req.Current
	}
	newPassword := req.NewPassword
	if newPassword == "" {
		newPassword = req.New
	}
	if currentPassword == "" || newPassword == "" {
		errBadRequest(w, "current_password (or current) and new_password (or new) are required")
		return
	}
	if len(newPassword) < 8 {
		errBadRequest(w, "new password must be at least 8 characters")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Verify the current password against Proxmox.
	if err := verifyProxmoxCredentials(ctx, h.state.GetEnvConfig(), username, currentPassword); err != nil {
		logger.SecurityEvent("api_password_change_fail").Str("username", username).Msg("Invalid current password for password change")
		errUnauthorized(w)
		return
	}

	// Update the password using the service account API token.
	// Requires the PVMSS service account to have User.Modify permission.
	client, err := restyClient()
	if err != nil {
		writeAppError(w, err)
		return
	}

	if err := proxmox.UpdateUserPasswordResty(ctx, client, username, newPassword, newPassword, "pve"); err != nil {
		logger.Get().Error().Err(err).Str("username", username).Msg("api/v1: failed to update password")
		writeError(w, http.StatusBadGateway, "proxmox_error", "Failed to update password")
		return
	}

	logger.AuthEvent("api_password_change").Str("username", username).Msg("Password changed successfully")
	w.WriteHeader(http.StatusNoContent)
}

// verifyProxmoxCredentials POSTs to /access/ticket to confirm user credentials.
// Uses a cookie-auth client (no API token headers) because /access/ticket is a
// public endpoint that rejects requests with conflicting Authorization headers.
func verifyProxmoxCredentials(ctx context.Context, cfg *envpkg.EnvConfig, username, password string) error {
	restyClient, err := proxmox.MakeRestyClientCookieAuthFromEnvConfig(cfg, 10*time.Second)
	if err != nil {
		return fmt.Errorf("no proxmox client: %w", err)
	}
	if !strings.Contains(username, "@") {
		username = username + constants.UserSuffix
	}
	values := url.Values{}
	values.Set("username", username)
	values.Set("password", password)

	var resp struct {
		Data struct {
			Ticket string `json:"ticket"`
		} `json:"data"`
	}
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := restyClient.Post(tctx, "/access/ticket", values, &resp); err != nil || resp.Data.Ticket == "" {
		return fmt.Errorf("proxmox authentication failed")
	}
	return nil
}
