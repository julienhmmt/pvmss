package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/middleware"
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
				RenderErrorPage(w, r, http.StatusInternalServerError, "Internal server error")
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

// buildPublicMiddleware assembles the middleware stack for public/static routes.
func buildPublicMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := next
		handler = securityMiddleware.Headers(handler)
		handler = recoverMiddleware(handler)
		return handler
	}
}

// buildAPIMiddleware assembles the middleware stack for /api/v1/ routes.
// JWT-authenticated; no session or CSRF needed.
func buildAPIMiddleware(sm state.StateManager, rateLimiter *middleware.Limiter, isTestEnv bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := next
		handler = stateManagerContextMiddleware(sm)(handler)
		if !isTestEnv {
			handler = middleware.RateLimitMiddleware(rateLimiter)(handler)
		}
		handler = securityMiddleware.Headers(handler)
		handler = maxBodySizeMiddleware(handler, int64(constants.MaxFormSize))
		handler = recoverMiddleware(handler)
		return handler
	}
}

// isAPIPath returns true when the request path is a JWT-authenticated API route (/api/v1/).
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
	for _, prefix := range []string{"/components/", "/_app/", "/noVNC-1.6.0/"} {
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

// resolveWithinBase resolves relPath against baseDir and ensures the resulting absolute
// path stays within baseDir. It returns the absolute path and true on success, or
// an empty string and false if the path escapes baseDir or cannot be resolved.
func resolveWithinBase(baseDir, relPath string) (string, bool) {
	baseClean := filepath.Clean(baseDir)
	baseAbs, err := filepath.Abs(baseClean)
	if err != nil {
		return "", false
	}

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
	defer func() {
		if rec := recover(); rec != nil {
			logger.Get().Error().Interface("panic", rec).Str("path", r.URL.Path).Msg("Panic recovered in serveSPA")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(JSONErrorResponse{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Internal server error",
			})
		}
	}()

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")

	decodedPath, err := url.PathUnescape(r.URL.Path)
	if err != nil || strings.Contains(decodedPath, "\x00") || strings.Contains(decodedPath, "..") || strings.Contains(decodedPath, "\\") {
		http.NotFound(w, r)
		return
	}

	relPath := strings.TrimPrefix(decodedPath, "/")
	filePathAbs, ok := resolveWithinBase(spaDir, relPath)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if info, err := os.Stat(filePathAbs); err == nil && !info.IsDir() {
		http.ServeFile(w, r, filePathAbs)
		return
	}
	http.ServeFile(w, r, spaIndexPath)
}
