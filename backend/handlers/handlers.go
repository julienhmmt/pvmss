package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/middleware"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// InitHandlers initializes all handlers and configures routes
// InitHandlers returns the HTTP handler and the underlying httprouter.Router.
// The router is exposed so that additional route groups (e.g. api/v1) can be
// registered before the server starts.
func InitHandlers(stateManager state.StateManager) (http.Handler, *httprouter.Router) {
	log := logger.Get().With().Str("component", "handlers").Logger()

	// Create a new router
	router := httprouter.New()

	// Configure rate limiter (disabled in automated test environment to avoid
	// interfering with functional route tests that perform repeated logins).
	isTestEnv := os.Getenv("GO_TEST_ENVIRONMENT") == "1"
	rateLimiter := middleware.MakeRateLimiter(constants.RateLimitWindow, constants.RateLimitCleanup)
	if !isTestEnv {
		rateLimiter.AddRule("POST", "/login", middleware.Rule{
			Capacity: constants.LoginRateLimitCapacity,
			Refill:   constants.LoginRateLimitRefill,
		})
		rateLimiter.AddRule("POST", "/admin/login", middleware.Rule{
			Capacity: constants.LoginRateLimitCapacity,
			Refill:   constants.LoginRateLimitRefill,
		})
		rateLimiter.AddRule("POST", "/admin/proxmox-login", middleware.Rule{
			Capacity: constants.LoginRateLimitCapacity,
			Refill:   constants.LoginRateLimitRefill,
		})
	}

	// Ensure default tag exists
	if err := EnsureDefaultTag(stateManager); err != nil {
		log.Error().Err(err).Msg("Failed to ensure default tag")
	}

	if stateManager == nil {
		log.Fatal().Msg("State manager not initialized")
	}

	// Initialize all handlers
	authHandler := MakeAuthHandler(stateManager)
	healthHandler := MakeHealthHandler(stateManager)
	languageHandler := MakeLanguageHandler()
	settingsHandler := MakeSettingsHandler(stateManager)
	tagsHandler := MakeTagsHandler(stateManager)

	// Configure routes
	setupRoutes(
		authHandler,
		healthHandler,
		languageHandler,
		router,
		settingsHandler,
		tagsHandler,
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
	publicHandler := buildPublicMiddleware()(router)

	// API Middleware Chain: session loading (needed by /api/v1/auth/exchange),
	// but NO CSRF validation (JSON APIs use JWT, not CSRF tokens).
	apiHandler := buildAPIMiddleware(stateManager, rateLimiter, isTestEnv)(router)

	// Main App Middleware Chain (with session, CSRF, etc.)
	appHandler := buildAppMiddleware(stateManager, rateLimiter, isTestEnv)(router)

	// SvelteKit admin SPA serving
	spaDir := filepath.Join(getFrontendPath(stateManager), "admin")
	spaIndexPath := filepath.Join(spaDir, "index.html")
	spaAvailable := false
	if _, err := os.Stat(spaIndexPath); err == nil {
		spaAvailable = true
		log.Info().Str("path", spaDir).Msg("SvelteKit admin SPA build found")
	} else {
		log.Info().Msg("SvelteKit admin SPA build not found — /admin/ will fall through to router")
	}

	// Route requests to the appropriate middleware chain.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case isStaticPath(r.URL.Path) || r.URL.Path == "/health":
			publicHandler.ServeHTTP(w, r)
		case isAPIPath(r.URL.Path):
			apiHandler.ServeHTTP(w, r)
		case isLegacyAPIPath(r.URL.Path):
			// Session-authenticated API routes (/api/settings, /api/vmbr, /api/health)
			// must go through the app middleware even when the SPA is available.
			appHandler.ServeHTTP(w, r)
		case spaAvailable:
			// Serve SvelteKit SPA for all other paths when available
			serveSPA(w, r, spaDir, spaIndexPath)
		default:
			// Fallback to legacy app handler only if SPA not available
			appHandler.ServeHTTP(w, r)
		}
	})

	var handler http.Handler = mux

	log.Info().Msg("HTTP handlers and middleware initialized")
	return handler, router
}

// handlerRegistrar interface for handlers that can register routes
type handlerRegistrar interface {
	RegisterRoutes(router *httprouter.Router)
}

// setupRoutes configures all application routes
func setupRoutes(
	authHandler *AuthHandler,
	healthHandler *HealthHandler,
	languageHandler *LanguageHandler,
	router *httprouter.Router,
	settingsHandler *SettingsHandler,
	tagsHandler *TagsHandler,
) {
	handlers := []handlerRegistrar{
		authHandler,
		healthHandler,
		languageHandler,
		settingsHandler,
		tagsHandler,
	}

	for _, h := range handlers {
		h.RegisterRoutes(router)
	}
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

	// noVNC library for the Svelte console component
	registerStaticHandler(router, "/components/*filepath", http.StripPrefix("/components/", createCachedFileServer(basePath, "components")))
	registerStaticHandler(router, "/favicon.ico", http.HandlerFunc(serveFavicon))

	logger.Get().Info().Str("path", basePath).Msg("Static file serving configured")
}

// setupStaticFiles configures the static file server
