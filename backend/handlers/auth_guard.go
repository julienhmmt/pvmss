package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
)

// IsAuthenticated checks if the user is authenticated.
func IsAuthenticated(r *http.Request) bool {
	log := CreateHandlerLogger("IsAuthenticated", r).With().Str("remote_addr", r.RemoteAddr).Logger()
	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().Msg("State manager not available in IsAuthenticated")
		return false
	}
	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		log.Error().Msg("Session manager not available in IsAuthenticated")
		return false
	}
	sessionToken := sessionManager.Token(r.Context())
	log.Debug().Str("session_token", sessionToken).Msg("Session token in IsAuthenticated")
	authenticated, ok := sessionManager.Get(r.Context(), "authenticated").(bool)
	sessionData := map[string]interface{}{
		"authenticated_found": ok,
		"authenticated_value": authenticated,
	}
	if username, ok := sessionManager.Get(r.Context(), "username").(string); ok {
		sessionData["username"] = username
	}
	if isAdmin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok {
		sessionData["is_admin"] = isAdmin
	}
	if !ok || !authenticated {
		log.Debug().Bool("authenticated", false).Interface("session_data", sessionData).Str("session_id", sessionToken).Msg("Access denied: user not authenticated")
		return false
	}
	log.Debug().Bool("authenticated", true).Interface("session_data", sessionData).Str("session_id", sessionToken).Msg("Access granted: user authenticated")
	return true
}

// IsAdmin checks if the current user is an admin.
func IsAdmin(r *http.Request) bool {
	log := CreateHandlerLogger("IsAdmin", r)
	stateManager := getStateManager(r)
	if stateManager == nil {
		return false
	}
	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		return false
	}
	isAdmin, ok := sessionManager.Get(r.Context(), "is_admin").(bool)
	if !ok || !isAdmin {
		log.Debug().Bool("is_admin", false).Msg("User is authenticated but not admin")
		return false
	}
	log.Debug().Bool("is_admin", true).Str("session_id", sessionManager.Token(r.Context())).Msg("Admin access verified")
	return true
}

// RequireAuth enforces authentication for protected routes.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := CreateHandlerLogger("RequireAuth", r).With().Str("remote_addr", r.RemoteAddr).Logger()
		if !IsAuthenticated(r) {
			log.Info().Msg("Authentication required, redirecting to login")
			returnURL := r.URL.Path
			if r.URL.RawQuery != "" {
				returnURL = returnURL + "?" + r.URL.RawQuery
			}
			if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/vm/update/description") {
				if err := r.ParseForm(); err == nil {
					if vmid := r.FormValue("vmid"); vmid != "" {
						returnURL = "/vm/details/" + vmid + "?edit=description"
					} else {
						returnURL = "/vm/create"
					}
				} else {
					returnURL = "/vm/create"
				}
			}
			setNoCacheHeaders(w)
			loginURL := "/login?return=" + url.QueryEscape(returnURL)
			if strings.HasPrefix(r.URL.Path, "/vm/update/description") {
				loginURL += "&warning=login_required&context=update_description"
			}
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}
		setSecurityHeaders(w, r)
		next.ServeHTTP(w, r)
	}
}

// RequireAdminAuth enforces admin authentication for admin routes.
func RequireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := CreateHandlerLogger("RequireAdminAuth", r).With().Str("remote_addr", r.RemoteAddr).Logger()
		if !IsAdmin(r) {
			log.Info().Msg("Admin authentication required")
			if IsAuthenticated(r) {
				logger.SecurityEvent("admin_access_denied").Str("method", r.Method).Str("path", r.URL.Path).Str("client_ip", r.RemoteAddr).Msg("Authenticated user attempted to access admin area without privileges")
				RenderErrorPage(w, r, http.StatusForbidden, "Access Denied: Admin privileges required")
				return
			}
			returnURL := r.URL.Path
			if r.URL.RawQuery != "" {
				returnURL = returnURL + "?" + r.URL.RawQuery
			}
			setNoCacheHeaders(w)
			http.Redirect(w, r, "/admin/login?return="+url.QueryEscape(returnURL), http.StatusSeeOther)
			return
		}
		setSecurityHeaders(w, r)
		next.ServeHTTP(w, r)
	}
}

// RequireAuthHandleWS wraps a handler with authentication for WebSockets.
func RequireAuthHandleWS(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		stateManager := getStateManager(r)
		if stateManager == nil {
			http.Error(w, "INTERNAL_SERVER_ERROR", http.StatusInternalServerError)
			return
		}

		if !IsAuthenticated(r) {
			log := CreateHandlerLogger("RequireAuthHandleWS", r)
			log.Warn().Msg("WebSocket connection rejected: not authenticated")
			http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
			return
		}
		h(w, r, ps)
	}
}

// RequireAuthHandle adapts a httprouter.Handle with the RequireAuth middleware.
func RequireAuthHandle(h func(http.ResponseWriter, *http.Request, httprouter.Params)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		wrapped := func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ParamsKey, ps)
			h(w, r.WithContext(ctx), ps)
		}
		RequireAuth(wrapped)(w, r)
	}
}
