package handlers

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/middleware"
	"pvmss/security"
	securityMiddleware "pvmss/security/middleware"
	"pvmss/state"
)

// Global variable to store frontend path for static file serving
// This is set during initialization and used for serving static files
var frontendPath string

// SetFrontendPath stores the frontend path for static file serving
func SetFrontendPath(path string) {
	frontendPath = path
}

// getFrontendPath returns the stored frontend path
func getFrontendPath() string {
	return frontendPath
}

// withStaticCaching wraps a static file handler to add strong caching headers.
// We use a long max-age with immutable as these assets are expected to be fingerprinted or rarely change.
func withStaticCaching(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do not cache if explicitly disabled upstream
		if w.Header().Get("Cache-Control") == "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
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
				RenderErrorPage(w, r, http.StatusInternalServerError, "Internal Server Error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// trailingSlashRedirectMiddleware redirects "/path/" to "/path" (excluding root and static assets)
// to avoid registering duplicate routes and reduce 404s with strict routers.
func trailingSlashRedirectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if len(p) > 1 && p[len(p)-1] == '/' {
			// Preserve static paths and directories under static mounts
			if isStaticPath(p) {
				next.ServeHTTP(w, r)
				return
			}
			// Only redirect idempotent requests
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				http.Redirect(w, r, p[:len(p)-1], http.StatusSeeOther)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// serveFavicon serves a tiny transparent PNG at /favicon.ico to satisfy browsers without touching sessions.
func serveFavicon(w http.ResponseWriter, r *http.Request) {
	// cache shorter than other assets to allow easy replacement
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

// InitHandlers initializes all handlers and configures routes
func InitHandlers(stateManager state.StateManager) http.Handler {
	log := logger.Get().With().Str("component", "handlers").Logger()

	// Create a new router
	router := httprouter.New()

	// Configure rate limiter
	rateLimiter := middleware.NewRateLimiter(5*time.Minute, 15*time.Minute)
	rateLimiter.AddRule("POST", "/login", middleware.Rule{Capacity: 5, Refill: 12 * time.Second})

	// Ensure default tag exists
	if err := EnsureDefaultTag(stateManager); err != nil {
		log.Error().Err(err).Msg("Failed to ensure default tag")
	}

	if stateManager == nil {
		log.Fatal().Msg("State manager not initialized")
	}

	// Initialize handlers
	authHandler := NewAuthHandler(stateManager)
	adminHandler := NewAdminHandler(stateManager)
	// wsHub := websocket.NewHub()
	// go wsHub.Run()
	vmHandler := NewVMHandler(stateManager)
	storageHandler := NewStorageHandler(stateManager)
	searchHandler := NewSearchHandler(stateManager)
	docsHandler := NewDocsHandler()
	healthHandler := NewHealthHandler(stateManager)
	settingsHandler := NewSettingsHandler(stateManager)
	tagsHandler := NewTagsHandler(stateManager)
	vmbrHandler := NewVMBRHandler(stateManager)
	userPoolHandler := NewUserPoolHandler(stateManager)

	// Configure routes
	setupRoutes(router, authHandler, adminHandler, vmHandler, storageHandler, searchHandler, docsHandler, healthHandler, settingsHandler, tagsHandler, vmbrHandler, userPoolHandler)

	// Friendly NotFound and MethodNotAllowed handlers (when state is available)
	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if getStateManager(r) != nil {
			RenderErrorPage(w, r, http.StatusNotFound, "Page not found")
			return
		}
		http.NotFound(w, r)
	})
	router.MethodNotAllowed = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if getStateManager(r) != nil {
			RenderErrorPage(w, r, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method not allowed"))
	})

	// Configure static files handler
	setupStaticFiles(router)

	// Create a new ServeMux to route requests to different middleware stacks.
	// This allows us to have separate middleware for public/static routes vs. the main application.
	mux := http.NewServeMux()

	// --- Public/Static Middleware Chain (no session) ---
	var publicHandler http.Handler = router
	publicHandler = recoverMiddleware(publicHandler)

	// --- Main App Middleware Chain (with session, CSRF, etc.) ---
	var appHandler http.Handler = router
	appHandler = stateManagerContextMiddleware(stateManager)(appHandler)

	sessionManager := stateManager.GetSessionManager()
	if sessionManager != nil {
		// Apply session-dependent middleware only to the app handler
		appHandler = security.CSRF(appHandler)
		appHandler = securityMiddleware.Headers(appHandler)
		appHandler = securityMiddleware.SessionMiddleware(sessionManager)(appHandler)
		appHandler = sessionDebugMiddleware(appHandler)
		appHandler = sessionManager.LoadAndSave(appHandler) // Outermost session middleware
	} else {
		log.Warn().Msg("Session manager not available, running with limited functionality")
	}

	// Apply middleware that should run for the main app but after sessions
	appHandler = middleware.ProxmoxStatusMiddlewareWithState(stateManager)(appHandler)
	appHandler = middleware.RateLimitMiddleware(rateLimiter)(appHandler)
	appHandler = trailingSlashRedirectMiddleware(appHandler)
	appHandler = recoverMiddleware(appHandler) // Innermost recovery for the app

	// Route requests to the appropriate middleware chain.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Route static assets and /health to the public handler (no session)
		if isStaticPath(r.URL.Path) || r.URL.Path == "/health" {
			publicHandler.ServeHTTP(w, r)
		} else {
			// All other requests go to the main app handler with the full middleware stack
			appHandler.ServeHTTP(w, r)
		}
	})

	var handler http.Handler = mux

	log.Info().Msg("HTTP handlers and middleware initialized")
	return handler
}

// stateManagerContextMiddleware adds the provided state manager to each request context
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

// setupRoutes configures all application routes
func setupRoutes(
	router *httprouter.Router,
	authHandler *AuthHandler,
	adminHandler *AdminHandler,
	vmHandler *VMHandler,
	storageHandler *StorageHandler,
	searchHandler *SearchHandler,
	docsHandler *DocsHandler,
	healthHandler *HealthHandler,
	settingsHandler *SettingsHandler,
	tagsHandler *TagsHandler,
	vmbrHandler *VMBRHandler,
	userPoolHandler *UserPoolHandler,
) {
	// Register routes for each handler
	authHandler.RegisterRoutes(router)
	adminHandler.RegisterRoutes(router)
	vmHandler.RegisterRoutes(router)
	storageHandler.RegisterRoutes(router)
	searchHandler.RegisterRoutes(router)
	docsHandler.RegisterRoutes(router)
	healthHandler.RegisterRoutes(router)
	settingsHandler.RegisterRoutes(router)
	settingsHandler.RegisterISORoutes(router)
	settingsHandler.RegisterLimitsRoutes(router)
	tagsHandler.RegisterRoutes(router)
	vmbrHandler.RegisterRoutes(router)
	userPoolHandler.RegisterRoutes(router)

	// Home route
	router.GET("/", IndexRouterHandler)
}

// setupStaticFiles configures the static file server
func setupStaticFiles(router *httprouter.Router) {
	// Get the base path of the frontend directory.
	basePath := getFrontendPath()

	// Create CSS handler for optimized CSS serving
	cssHandler := NewCSSHandler(basePath)

	// Create specific file handlers for other static subdirectories
	jsServer := withStaticCaching(http.FileServer(http.Dir(filepath.Join(basePath, "js"))))
	webfontsServer := withStaticCaching(http.FileServer(http.Dir(filepath.Join(basePath, "webfonts"))))
	componentsServer := withStaticCaching(http.FileServer(http.Dir(filepath.Join(basePath, "components"))))

	// Configure routes to serve CSS files using CSS handler
	router.Handler(http.MethodGet, "/css/*filepath", http.HandlerFunc(cssHandler.ServeCSS))
	router.Handler(http.MethodHead, "/css/*filepath", http.HandlerFunc(cssHandler.ServeCSS))
	router.Handler(http.MethodGet, "/js/*filepath", http.StripPrefix("/js/", jsServer))
	router.Handler(http.MethodHead, "/js/*filepath", http.StripPrefix("/js/", jsServer))
	router.Handler(http.MethodGet, "/webfonts/*filepath", http.StripPrefix("/webfonts/", webfontsServer))
	router.Handler(http.MethodHead, "/webfonts/*filepath", http.StripPrefix("/webfonts/", webfontsServer))
	router.Handler(http.MethodGet, "/components/*filepath", http.StripPrefix("/components/", componentsServer))
	router.Handler(http.MethodHead, "/components/*filepath", http.StripPrefix("/components/", componentsServer))
	router.Handler(http.MethodGet, "/favicon.ico", http.HandlerFunc(serveFavicon))
	router.Handler(http.MethodHead, "/favicon.ico", http.HandlerFunc(serveFavicon))

	logger.Get().Info().Str("path", basePath).Msg("Static file serving configured for css, js, components, webfonts")
}

// isStaticPath returns true when the request is for a static asset we serve directly
func isStaticPath(p string) bool {
	if p == "/favicon.ico" {
		return true
	}
	return hasAnyPrefix(p, "/css/", "/js/", "/webfonts/", "/components/")
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, pref := range prefixes {
		if len(s) >= len(pref) && s[:len(pref)] == pref {
			return true
		}
	}
	return false
}

// sessionDebugMiddleware is a debug middleware for sessions
func sessionDebugMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("remote_addr", r.RemoteAddr).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()

		// Log request headers
		headers := make(map[string]string)
		for name, values := range r.Header {
			headers[name] = values[0] // We only take the first value for simplicity
		}

		// Detailed cookie logging
		cookies := make(map[string]string)
		for _, cookie := range r.Cookies() {
			cookies[cookie.Name] = cookie.Value
			log.Debug().
				Str("cookie_name", cookie.Name).
				Str("value", cookie.Value).
				Str("path", cookie.Path).
				Str("domain", cookie.Domain).
				Bool("secure", cookie.Secure).
				Bool("http_only", cookie.HttpOnly).
				Msg("Cookie received in request")
		}

		// Log before the next middleware
		log.Debug().
			Interface("headers", headers).
			Interface("cookies", cookies).
			Msg("Request received - before processing")

		// Skip ResponseWriter wrapping for WebSocket requests to avoid hijacking issues
		isWebSocket := strings.ToLower(r.Header.Get("Upgrade")) == "websocket" ||
			strings.ToLower(r.Header.Get("Connection")) == "upgrade"

		if isWebSocket {
			// For WebSocket requests, skip the wrapper to preserve hijacking capability
			next.ServeHTTP(w, r)
			return
		}

		// Create a wrapper to capture response headers
		ww := &responseWriterWrapper{ResponseWriter: w, status: 200}

		// Call the next middleware
		next.ServeHTTP(ww, r)

		// Log response headers
		log.Debug().
			Int("status_code", ww.status).
			Interface("response_headers", ww.Header()).
			Msg("Response sent")

		// Specific log for session cookies in the response
		for _, cookie := range ww.Header()["Set-Cookie"] {
			log.Debug().
				Str("set_cookie", cookie).
				Msg("Cookie set in response")
		}
	})
}

// responseWriterWrapper is a wrapper to capture the HTTP status code
type responseWriterWrapper struct {
	http.ResponseWriter
	status int
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker interface for WebSocket support
func (w *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
}
