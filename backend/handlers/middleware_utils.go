package handlers

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/middleware"
	"pvmss/security"
	securityMiddleware "pvmss/security/middleware"
	"pvmss/state"
)

// maxBodySizeMiddleware limits the size of request bodies globally.
func maxBodySizeMiddleware(next http.Handler, maxSize int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
		}
		next.ServeHTTP(w, r)
	})
}

// recoverMiddleware ensures the server returns 500 instead of crashing on unexpected panics.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Get().Error().Interface("panic", rec).Str("path", r.URL.Path).Msg("Unhandled panic recovered")
				RenderErrorPageWithI18n(w, r, http.StatusInternalServerError, "Error.InternalServer", "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// trailingSlashRedirectMiddleware redirects "/path/" to "/path" (excluding root and static assets).
func trailingSlashRedirectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if len(p) > 1 && p[len(p)-1] == '/' {
			if isStaticPath(p) {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				http.Redirect(w, r, p[:len(p)-1], http.StatusSeeOther)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// stateManagerContextMiddleware adds the provided state manager to each request context.
func stateManagerContextMiddleware(sm state.StateManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sm != nil {
				ctx := context.WithValue(r.Context(), StateManagerKey, sm)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// snapshotRefreshMiddleware triggers an asynchronous Proxmox snapshot refresh on page navigation.
func snapshotRefreshMiddleware(sm state.StateManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sm != nil && r.Method == http.MethodGet {
				sm.RequestSnapshotRefresh()
			}
			next.ServeHTTP(w, r)
		})
	}
}

// sessionDebugMiddleware is a debug middleware for sessions (enabled via DEBUG_SESSIONS=true).
func sessionDebugMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("DEBUG_SESSIONS") != "true" {
			next.ServeHTTP(w, r)
			return
		}

		log := CreateHandlerLogger("sessionDebugMiddleware", r).With().
			Str("remote_addr", r.RemoteAddr).
			Logger()

		sensitiveHeaders := map[string]bool{
			"authorization": true,
			"cookie":        true,
			"x-csrf-token":  true,
		}

		headers := make(map[string]string)
		for name, values := range r.Header {
			nameLower := strings.ToLower(name)
			if sensitiveHeaders[nameLower] {
				headers[name] = maskSensitiveValue(values[0])
			} else {
				headers[name] = values[0]
			}
		}

		cookieCount := len(r.Cookies())
		for _, cookie := range r.Cookies() {
			log.Debug().
				Str("cookie_name", cookie.Name).
				Str("value_preview", maskSensitiveValue(cookie.Value)).
				Str("path", cookie.Path).
				Str("domain", cookie.Domain).
				Bool("secure", cookie.Secure).
				Bool("http_only", cookie.HttpOnly).
				Msg("Cookie received in request")
		}

		log.Debug().
			Int("header_count", len(headers)).
			Int("cookie_count", cookieCount).
			Msg("Request received - before processing")

		isWebSocket := strings.ToLower(r.Header.Get("Upgrade")) == "websocket" ||
			strings.ToLower(r.Header.Get("Connection")) == "upgrade"
		if isWebSocket {
			next.ServeHTTP(w, r)
			return
		}

		ww := &responseWriterWrapper{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)

		log.Debug().
			Int("status_code", ww.status).
			Interface("response_headers", ww.Header()).
			Msg("Response sent")

		for _, cookie := range ww.Header()["Set-Cookie"] {
			log.Debug().Str("set_cookie", cookie).Msg("Cookie set in response")
		}
	})
}

// responseWriterWrapper captures status code for logging.
type responseWriterWrapper struct {
	http.ResponseWriter
	status int
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker interface for WebSocket support.
func (w *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
}

// maskSensitiveValue masks sensitive data for logging (shows only first 8 chars).
func maskSensitiveValue(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:8] + "..." + fmt.Sprintf("[%d chars]", len(value))
}

// buildAppMiddleware assembles the middleware stack for the main app.
func buildAppMiddleware(sm state.StateManager, rateLimiter *middleware.Limiter, isTestEnv bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := next
		handler = stateManagerContextMiddleware(sm)(handler)
		handler = snapshotRefreshMiddleware(sm)(handler)

		sessionManager := sm.GetSessionManager()
		if sessionManager != nil {
			handler = security.CSRF(handler)
			handler = securityMiddleware.Headers(handler)
			handler = securityMiddleware.SessionMiddleware(sessionManager)(handler)
			handler = sessionDebugMiddleware(handler)
			handler = sessionManager.LoadAndSave(handler)
		}
		handler = middleware.ProxmoxStatusMiddlewareWithState(sm)(handler)
		if !isTestEnv {
			handler = middleware.RateLimitMiddleware(rateLimiter)(handler)
		}
		handler = trailingSlashRedirectMiddleware(handler)
		handler = maxBodySizeMiddleware(handler, int64(constants.MaxFormSize))
		handler = recoverMiddleware(handler)
		return handler
	}
}

// buildPublicMiddleware assembles the middleware stack for public/static routes.
func buildPublicMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := recoverMiddleware(next)
		return handler
	}
}

// buildAPIMiddleware assembles the middleware stack for /api/v1/ routes.
// It loads sessions (needed by /api/v1/auth/exchange) but skips CSRF validation
// because JSON API clients authenticate via JWT, not CSRF tokens.
func buildAPIMiddleware(sm state.StateManager, rateLimiter *middleware.Limiter, isTestEnv bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := next
		handler = stateManagerContextMiddleware(sm)(handler)

		sessionManager := sm.GetSessionManager()
		if sessionManager != nil {
			handler = sessionManager.LoadAndSave(handler)
		}
		if !isTestEnv {
			handler = middleware.RateLimitMiddleware(rateLimiter)(handler)
		}
		handler = maxBodySizeMiddleware(handler, int64(constants.MaxFormSize))
		handler = recoverMiddleware(handler)
		return handler
	}
}

// isAPIPath returns true when the request path is a JWT-authenticated API route (/api/v1/).
// Routes under /api/vm/ and /api/settings/ are session-authenticated and handled by appHandler.
func isAPIPath(p string) bool {
	return strings.HasPrefix(p, "/api/v1/")
}

// withStaticCaching wraps a static file handler to add strong caching headers.
func withStaticCaching(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if w.Header().Get("Cache-Control") == "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		next.ServeHTTP(w, r)
	})
}

// isStaticPath returns true when the request is for a static asset we serve directly.
func isStaticPath(p string) bool {
	if p == "/favicon.ico" {
		return true
	}
	for _, prefix := range []string{"/css/", "/js/", "/webfonts/", "/components/", "/vendor/", "/src/"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// serveFavicon serves a tiny transparent PNG at /favicon.ico.
func serveFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "image/png")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	const b64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9YfP2dQAAAAASUVORK5CYII="
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// getFrontendPath returns the frontend path from the state manager.
func getFrontendPath(sm state.StateManager) string {
	if sm == nil {
		return ""
	}
	return sm.GetFrontendPath()
}

// isSPAStaticAsset returns true for SvelteKit build assets (JS, CSS, etc.)
// that should be served without authentication (needed by the login page).
func isSPAStaticAsset(p string) bool {
	return strings.HasPrefix(p, "/admin/_app/")
}

// isSPAPath returns true for admin page routes that should be served by the
// SvelteKit admin SPA (after authentication). Login routes are excluded so the
// server-rendered login form is used.
func isSPAPath(p string) bool {
	if p == "/admin/login" || p == "/admin/proxmox-login" {
		return false
	}
	// Static assets are handled separately without auth.
	if isSPAStaticAsset(p) {
		return false
	}
	return strings.HasPrefix(p, "/admin/") || p == "/admin"
}

// resolveWithinBase resolves relPath against baseDir and ensures the resulting absolute
// path stays within baseDir. It returns the absolute path and true on success, or
// an empty string and false if the path escapes baseDir or cannot be resolved.
func resolveWithinBase(baseDir, relPath string) (string, bool) {
	// Normalize the base directory and ensure it is absolute.
	baseClean := filepath.Clean(baseDir)
	baseAbs, err := filepath.Abs(baseClean)
	if err != nil {
		return "", false
	}

	// Normalize the relative path and ensure it is not absolute or volume-rooted.
	cleanRel := filepath.Clean(relPath)
	if cleanRel == "." {
		cleanRel = ""
	}
	if cleanRel != "" && (filepath.IsAbs(cleanRel) || filepath.VolumeName(cleanRel) != "") {
		return "", false
	}

	joined := filepath.Join(baseAbs, cleanRel)
	targetAbs, err := filepath.Abs(joined)
	if err != nil {
		return "", false
	}
	// Ensure the resolved path stays within the base directory to prevent path traversal.
	baseWithSep := baseAbs
	if !strings.HasSuffix(baseWithSep, string(os.PathSeparator)) {
		baseWithSep += string(os.PathSeparator)
	}
	if targetAbs != baseAbs && !strings.HasPrefix(targetAbs, baseWithSep) {
		return "", false
	}
	return targetAbs, true
}

// serveSPA serves the SvelteKit SPA. Static assets (files with extensions) are served
// directly from the build directory; all other paths get the fallback index.html.
func serveSPA(w http.ResponseWriter, r *http.Request, spaDir, spaIndexPath string) {
	// Strip /admin prefix to find the file in the build directory
	relPath := strings.TrimPrefix(r.URL.Path, "/admin")
	if relPath == "" {
		relPath = "/"
	}
	// Resolve the requested path within the SPA directory and ensure it cannot escape.
	filePathAbs, ok := resolveWithinBase(spaDir, relPath)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Try to serve the file directly (JS, CSS, images, etc.)
	if info, err := os.Stat(filePathAbs); err == nil && !info.IsDir() {
		http.ServeFile(w, r, filePathAbs)
		return
	}
	// SPA fallback: serve index.html for all routes
	http.ServeFile(w, r, spaIndexPath)
}
