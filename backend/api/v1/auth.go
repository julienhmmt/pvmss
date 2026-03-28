package apiv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

const (
	accessTokenCookie  = "access_token"
	refreshTokenCookie = "refresh_token"
	accessTokenTTL     = 15 * time.Minute
	refreshTokenTTL    = 7 * 24 * time.Hour
)

// AuthHandler handles JWT authentication endpoints.
type AuthHandler struct {
	state state.StateManager
}

// MakeAuthHandler creates a new AuthHandler.
func MakeAuthHandler(s state.StateManager) *AuthHandler {
	return &AuthHandler{state: s}
}

// Login handles POST /api/v1/auth/login.
// Body: {"username":"...", "password":"...", "admin": true/false}
// If admin=true → bcrypt check against ADMIN_PASSWORD_HASH env var.
// If admin=false → verify via Proxmox /access/ticket.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	secret := h.state.GetSettings().JWTSecret
	if secret == "" {
		errNotConfigured(w)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		errBadRequest(w, "username and password are required")
		return
	}

	var isAdmin bool

	if req.Admin {
		hash := os.Getenv("ADMIN_PASSWORD_HASH")
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
		if err := verifyProxmoxCredentials(r.Context(), req.Username, req.Password); err != nil {
			logger.SecurityEvent("api_auth_user_fail").Str("username", req.Username).Err(err).Msg("Invalid Proxmox credentials")
			errUnauthorized(w)
			return
		}
		isAdmin = false
	}

	if err := issueTokens(w, secret, req.Username, isAdmin); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to issue JWT tokens")
		errInternal(w)
		return
	}

	logger.AuthEvent("api_login").Str("username", req.Username).Bool("is_admin", isAdmin).Msg("API login successful")
	writeJSON(w, AuthResponse{Username: req.Username, IsAdmin: isAdmin})
}

// Exchange handles POST /api/v1/auth/exchange.
// Reads the SCS session cookie, validates it, and issues JWT tokens.
// Used by the Vue app on load to exchange an existing session for JWT tokens.
func (h *AuthHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	secret := h.state.GetSettings().JWTSecret
	if secret == "" {
		errNotConfigured(w)
		return
	}

	sm := h.state.GetSessionManager()
	if sm == nil {
		errInternal(w)
		return
	}

	username, ok := sm.Get(r.Context(), "username").(string)
	if !ok || username == "" {
		errUnauthorized(w)
		return
	}
	isAdmin, _ := sm.Get(r.Context(), "is_admin").(bool)

	if err := issueTokens(w, secret, username, isAdmin); err != nil {
		errInternal(w)
		return
	}
	writeJSON(w, AuthResponse{Username: username, IsAdmin: isAdmin})
}

// Refresh handles POST /api/v1/auth/refresh.
// Reads the refresh_token cookie and issues a new access_token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	secret := h.state.GetSettings().JWTSecret
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

	setTokenCookie(w, secret, accessTokenCookie, claims.Username, claims.IsAdmin, accessTokenTTL)
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
	env := os.Getenv("PVMSS_ENV")
	secure := env == "production" || env == "prod"

	http.SetCookie(w, &http.Cookie{Name: accessTokenCookie, Value: "", MaxAge: -1, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: refreshTokenCookie, Value: "", MaxAge: -1, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
	writeJSON(w, map[string]bool{"ok": true})
}

// ProxmoxAdminLogin handles POST /api/v1/auth/proxmox-admin-login.
// Body: {"username":"user@pve","password":"..."}
// Authenticates via Proxmox @pve realm, requires PVEAdmin or PVMSS_Admin role, issues admin JWT.
func (h *AuthHandler) ProxmoxAdminLogin(w http.ResponseWriter, r *http.Request) {
	secret := h.state.GetSettings().JWTSecret
	if secret == "" {
		errNotConfigured(w)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		errBadRequest(w, "username and password are required")
		return
	}

	pxClient, err := proxmox.MakeRestyClientCookieAuthFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
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

	if err := issueTokens(w, secret, ticketResp.Username, true); err != nil {
		errInternal(w)
		return
	}

	logger.AuthEvent("api_pve_admin_login").Str("username", ticketResp.Username).Msg("PVE admin login successful")
	writeJSON(w, AuthResponse{Username: ticketResp.Username, IsAdmin: true})
}

// issueTokens creates and sets both access_token and refresh_token cookies.
func issueTokens(w http.ResponseWriter, secret, username string, isAdmin bool) error {
	setTokenCookie(w, secret, accessTokenCookie, username, isAdmin, accessTokenTTL)
	setTokenCookie(w, secret, refreshTokenCookie, username, isAdmin, refreshTokenTTL)
	return nil
}

// setTokenCookie mints a signed JWT and writes it as an HttpOnly SameSite=Strict cookie.
func setTokenCookie(w http.ResponseWriter, secret, name, username string, isAdmin bool, ttl time.Duration) {
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

	env := os.Getenv("PVMSS_ENV")
	secure := env == "production" || env == "prod"

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

// verifyProxmoxCredentials POSTs to /access/ticket to confirm user credentials.
func verifyProxmoxCredentials(ctx context.Context, username, password string) error {
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		return fmt.Errorf("no proxmox client: %w", err)
	}
	if !strings.Contains(username, "@") {
		username = username + "@pve"
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
