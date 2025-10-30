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

	"github.com/julienschmidt/httprouter"
	"pvmss/constants"
	"pvmss/logger"
	"pvmss/middleware"
	"pvmss/security"
	securityMiddleware "pvmss/security/middleware"
	"pvmss/state"
)

// SetFrontendPath stores the frontend path in the state manager
func SetFrontendPath(path string) {
	// Deprecated: This function is kept for backward compatibility
	// The frontend path should now be stored in StateManager
	// This will be set during initialization in main.go
}

// maxBodySizeMiddleware limits the size of request bodies globally
func maxBodySizeMiddleware(next http.Handler, maxSize int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
		}
		next.ServeHTTP(w, r)
	})
}

// getFrontendPath returns the frontend path from the state manager
func getFrontendPath(sm state.StateManager) string {
	if sm == nil {
		return ""
	}
	return sm.GetFrontendPath()
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
	rateLimiter := middleware.NewRateLimiter(constants.RateLimitWindow, constants.RateLimitCleanup)
	rateLimiter.AddRule("POST", "/login", middleware.Rule{
		Capacity: constants.LoginRateLimitCapacity,
		Refill:   constants.LoginRateLimitRefill,
	})
	rateLimiter.AddRule("POST", "/admin/login", middleware.Rule{
		Capacity: constants.LoginRateLimitCapacity,
		Refill:   constants.LoginRateLimitRefill,
	})

	// Ensure default tag exists
	if err := EnsureDefaultTag(stateManager); err != nil {
		log.Error().Err(err).Msg("Failed to ensure default tag")
	}

	if stateManager == nil {
		log.Fatal().Msg("State manager not initialized")
	}

	// Initialize all handlers
	adminHandler := NewAdminHandler(stateManager)
	adminVMsHandler := NewAdminVMsHandler(stateManager)
	authHandler := NewAuthHandler(stateManager)
	diskHandler := NewDiskHandler(stateManager)
	docsHandler := NewDocsHandler()
	healthHandler := NewHealthHandler(stateManager)
	languageHandler := NewLanguageHandler()
	profileHandler := NewProfileHandler(stateManager)
	searchHandler := NewSearchHandler(stateManager)
	settingsHandler := NewSettingsHandler(stateManager)
	storageHandler := NewStorageHandler(stateManager)
	tagsHandler := NewTagsHandler(stateManager)
	userPoolHandler := NewUserPoolHandler(stateManager)
	vmCreateHandler := NewVMCreateOptimizedHandler(stateManager)
	vmHandler := NewVMHandler(stateManager)
	vmbrHandler := NewVMBRHandler(stateManager)

	// Configure routes
	setupRoutes(
		adminHandler,
		adminVMsHandler,
		authHandler,
		diskHandler,
		docsHandler,
		healthHandler,
		languageHandler,
		profileHandler,
		router,
		searchHandler,
		settingsHandler,
		storageHandler,
		tagsHandler,
		userPoolHandler,
		vmCreateHandler,
		vmHandler,
		vmbrHandler,
	)

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
	setupStaticFiles(router, stateManager)

	// Create a new ServeMux to route requests to different middleware stacks.
	// This allows us to have separate middleware for public/static routes vs. the main application.
	mux := http.NewServeMux()

	// Public/Static Middleware Chain (no session)
	var publicHandler http.Handler = router
	publicHandler = recoverMiddleware(publicHandler)

	// Main App Middleware Chain (with session, CSRF, etc.)
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
	// Limit request body size globally for the application to mitigate DoS via large uploads
	appHandler = maxBodySizeMiddleware(appHandler, int64(constants.MaxFormSize))
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

// handlerRegistrar interface for handlers that can register routes
type handlerRegistrar interface {
	RegisterRoutes(router *httprouter.Router)
}

// setupRoutes configures all application routes
func setupRoutes(
	adminHandler *AdminHandler,
	adminVMsHandler *AdminVMsHandler,
	authHandler *AuthHandler,
	diskHandler *DiskHandler,
	docsHandler *DocsHandler,
	healthHandler *HealthHandler,
	languageHandler *LanguageHandler,
	profileHandler *ProfileHandler,
	router *httprouter.Router,
	searchHandler *SearchHandler,
	settingsHandler *SettingsHandler,
	storageHandler *StorageHandler,
	tagsHandler *TagsHandler,
	userPoolHandler *UserPoolHandler,
	vmCreateHandler *VMCreateOptimizedHandler,
	vmHandler *VMHandler,
	vmbrHandler *VMBRHandler,
) {
	// Register routes for all handlers
	handlers := []handlerRegistrar{
		adminHandler,
		adminVMsHandler,
		authHandler,
		diskHandler,
		docsHandler,
		healthHandler,
		languageHandler,
		profileHandler,
		searchHandler,
		settingsHandler,
		storageHandler,
		tagsHandler,
		userPoolHandler,
		vmCreateHandler,
		vmHandler,
		vmbrHandler,
	}

	for _, h := range handlers {
		h.RegisterRoutes(router)
	}

	// Register additional routes for settings handler
	settingsHandler.RegisterISORoutes(router)
	settingsHandler.RegisterLimitsRoutes(router)

	// Home route
	router.GET("/", IndexRouterHandler)
}

// setupStaticFiles configures the static file server
// registerStaticHandler registers both GET and HEAD handlers for a static route
func registerStaticHandler(router *httprouter.Router, path string, handler http.Handler) {
	router.Handler(http.MethodGet, path, handler)
	router.Handler(http.MethodHead, path, handler)
}

// createCachedFileServer creates a file server with caching for a subdirectory
func createCachedFileServer(basePath, subdir string) http.Handler {
	return withStaticCaching(http.FileServer(http.Dir(filepath.Join(basePath, subdir))))
}

func setupStaticFiles(router *httprouter.Router, stateManager state.StateManager) {
	basePath := getFrontendPath(stateManager)

	// Create CSS handler for optimized CSS serving
	cssHandler := NewCSSHandler(basePath)

	// Configure routes - CSS uses custom handler, others use file servers
	registerStaticHandler(router, "/css/*filepath", http.HandlerFunc(cssHandler.ServeCSS))
	registerStaticHandler(router, "/js/*filepath", http.StripPrefix("/js/", createCachedFileServer(basePath, "js")))
	registerStaticHandler(router, "/webfonts/*filepath", http.StripPrefix("/webfonts/", createCachedFileServer(basePath, "webfonts")))
	registerStaticHandler(router, "/components/*filepath", http.StripPrefix("/components/", createCachedFileServer(basePath, "components")))
	registerStaticHandler(router, "/favicon.ico", http.HandlerFunc(serveFavicon))

	logger.Get().Info().Str("path", basePath).Msg("Static file serving configured for css, js, components, webfonts")
}

// isStaticPath returns true when the request is for a static asset we serve directly
func isStaticPath(p string) bool {
	if p == "/favicon.ico" {
		return true
	}
	for _, prefix := range []string{"/css/", "/js/", "/webfonts/", "/components/"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// maskSensitiveValue masks sensitive data for logging (shows only first 8 chars)
func maskSensitiveValue(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:8] + "..." + fmt.Sprintf("[%d chars]", len(value))
}

// sessionDebugMiddleware is a debug middleware for sessions
// Only active when DEBUG_SESSIONS environment variable is set to "true"
func sessionDebugMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only enable detailed logging if explicitly requested
		debugEnabled := os.Getenv("DEBUG_SESSIONS") == "true"

		if !debugEnabled {
			// Skip all debug logging and proceed
			next.ServeHTTP(w, r)
			return
		}

		log := CreateHandlerLogger("sessionDebugMiddleware", r).With().
			Str("remote_addr", r.RemoteAddr).
			Logger()

		// Log request headers (excluding sensitive ones)
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

		// Detailed cookie logging with masked values
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

		// Log summary before processing
		log.Debug().
			Int("header_count", len(headers)).
			Int("cookie_count", cookieCount).
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
