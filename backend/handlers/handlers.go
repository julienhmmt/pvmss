package handlers

import (
	"context"
	"encoding/base64"
	"net/http"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/metrics"
	"pvmss/middleware"
	"pvmss/security"
	securityMiddleware "pvmss/security/middleware"
	"pvmss/state"
	// "pvmss/backend/websocket"
)

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
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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

	// Configure rate limits for sensitive endpoints (e.g., POST /login)
	middleware.ConfigureLoginRateLimit()

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
	vmHandler := NewVMHandler(stateManager, nil)
	storageHandler := NewStorageHandler(stateManager)
	searchHandler := NewSearchHandler(stateManager)
	docsHandler := NewDocsHandler()
	healthHandler := NewHealthHandler(stateManager)
	settingsHandler := NewSettingsHandler(stateManager)
	tagsHandler := NewTagsHandler(stateManager)
	vmbrHandler := NewVMBRHandler(stateManager)
	themeHandler := NewThemeHandler(stateManager)

	// Configure routes
	setupRoutes(router, authHandler, adminHandler, vmHandler, storageHandler, searchHandler, docsHandler, healthHandler, settingsHandler, tagsHandler, vmbrHandler, themeHandler)

	// Configure static files handler
	setupStaticFiles(router)

	// Metrics endpoint
	router.Handler(http.MethodGet, "/metrics", metrics.Handler())
	router.Handler(http.MethodHead, "/metrics", metrics.Handler())

	// Create middleware chain
	var handler http.Handler = router

	// Inject state manager into request context for downstream usage
	handler = stateManagerContextMiddleware(stateManager)(handler)

	// Get the session manager
	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		log.Warn().Msg("Session manager is not available, running with limited functionality")
	} else {
		// Add our custom session middleware (diagnostics and headers/CSRF are applied after)
		handler = securityMiddleware.SessionMiddleware(handler)

		// Debug middleware (after session)
		handler = sessionDebugMiddleware(handler)

		// IMPORTANT: Order matters (outermost first in code, innermost executes first):
		// We want runtime order: LoadAndSave -> InjectSession -> CSRFGenerator -> CSRF -> Headers -> CSRFMiddleware -> router
		// Apply inner to outer accordingly (wrapping inside-out below):
		handler = securityMiddleware.CSRF(handler)
		handler = securityMiddleware.Headers(handler)
		handler = middleware.CSRFMiddleware(handler)
		handler = security.CSRFGeneratorMiddleware(handler)

		// Inject scs.SessionManager into context so security.GetSession finds it BEFORE CSRF/headers run at runtime
		handler = security.InjectSessionManagerMiddleware(sessionManager)(handler)
	}

	// Proxmox status middleware (after CSRF validation)
	handler = middleware.ProxmoxStatusMiddlewareWithState(stateManager)(handler)

	// Apply rate limiting (runs early)
	handler = middleware.RateLimitMiddleware(handler)

	// Normalize trailing slashes early to reduce duplicate route handlers
	handler = trailingSlashRedirectMiddleware(handler)

	// HTTP metrics middleware (near-outermost to observe complete response)
	handler = metrics.HTTPMetricsMiddleware(handler)

	// IMPORTANT: scs LoadAndSave must be the OUTERMOST wrapper so downstream middlewares see session data in context.
	// However, to avoid unnecessary session churn on static assets and health checks,
	// we bypass LoadAndSave for those paths.
	if sessionManager != nil {
		// Capture the current handler chain to avoid self-recursion in the closure
		baseHandler := handler
		// Pre-wrap once to avoid allocating per-request
		withSession := sessionManager.LoadAndSave(baseHandler)
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isStaticPath(r.URL.Path) || r.URL.Path == "/health" {
				baseHandler.ServeHTTP(w, r)
				return
			}
			withSession.ServeHTTP(w, r)
		})
		log.Info().Msgf("Session middleware enabled with conditional bypass for static/health; manager: %p", sessionManager)
	}

	// Global panic recovery (outermost) to avoid crashing the server on unexpected panics
	handler = recoverMiddleware(handler)

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

// setupRoutes configure toutes les routes de l'application
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
	themeHandler *ThemeHandler,
) {
	// Enregistrer les routes de chaque gestionnaire
	authHandler.RegisterRoutes(router)
	adminHandler.RegisterRoutes(router)
	vmHandler.RegisterRoutes(router)
	storageHandler.RegisterRoutes(router)
	searchHandler.RegisterRoutes(router)
	docsHandler.RegisterRoutes(router)
	healthHandler.RegisterRoutes(router)
	settingsHandler.RegisterRoutes(router)
	tagsHandler.RegisterRoutes(router)
	vmbrHandler.RegisterRoutes(router)
	themeHandler.RegisterRoutes(router)

	// Route d'accueil
	router.GET("/", IndexRouterHandler)
}

// setupStaticFiles configure le serveur de fichiers statiques
func setupStaticFiles(router *httprouter.Router) {
	// Récupère le chemin de base du répertoire frontend.
	basePath := state.GetTemplatesPath()

	// Crée des gestionnaires de fichiers spécifiques pour chaque sous-répertoire statique.
	cssServer := withStaticCaching(http.FileServer(http.Dir(filepath.Join(basePath, "css"))))
	jsServer := withStaticCaching(http.FileServer(http.Dir(filepath.Join(basePath, "js"))))
	imagesServer := withStaticCaching(http.FileServer(http.Dir(filepath.Join(basePath, "images"))))
	webfontsServer := withStaticCaching(http.FileServer(http.Dir(filepath.Join(basePath, "webfonts"))))

	// Configure les routes pour servir les fichiers statiques en utilisant StripPrefix.
	// Cela garantit que le serveur de fichiers reçoit le bon chemin relatif.
	router.Handler(http.MethodGet, "/css/*filepath", http.StripPrefix("/css/", cssServer))
	router.Handler(http.MethodHead, "/css/*filepath", http.StripPrefix("/css/", cssServer))
	router.Handler(http.MethodGet, "/js/*filepath", http.StripPrefix("/js/", jsServer))
	router.Handler(http.MethodHead, "/js/*filepath", http.StripPrefix("/js/", jsServer))
	router.Handler(http.MethodGet, "/images/*filepath", http.StripPrefix("/images/", imagesServer))
	router.Handler(http.MethodHead, "/images/*filepath", http.StripPrefix("/images/", imagesServer))
	router.Handler(http.MethodGet, "/webfonts/*filepath", http.StripPrefix("/webfonts/", webfontsServer))
	router.Handler(http.MethodHead, "/webfonts/*filepath", http.StripPrefix("/webfonts/", webfontsServer))
	router.Handler(http.MethodGet, "/favicon.ico", http.HandlerFunc(serveFavicon))
	router.Handler(http.MethodHead, "/favicon.ico", http.HandlerFunc(serveFavicon))

	logger.Get().Info().Str("path", basePath).Msg("Service des fichiers statiques configuré pour css, js, images et webfonts")
}

// isStaticPath returns true when the request is for a static asset we serve directly
func isStaticPath(p string) bool {
	if p == "/favicon.ico" {
		return true
	}
	return hasAnyPrefix(p, "/css/", "/js/", "/images/", "/webfonts/")
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, pref := range prefixes {
		if len(s) >= len(pref) && s[:len(pref)] == pref {
			return true
		}
	}
	return false
}

// sessionDebugMiddleware est un middleware de débogage pour les sessions
func sessionDebugMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("remote_addr", r.RemoteAddr).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()

		// Log des en-têtes de la requête
		headers := make(map[string]string)
		for name, values := range r.Header {
			headers[name] = values[0] // On ne prend que la première valeur pour simplifier
		}

		// Log détaillé des cookies
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
				Msg("Cookie reçu dans la requête")
		}

		// Log avant le prochain middleware
		log.Debug().
			Interface("headers", headers).
			Interface("cookies", cookies).
			Msg("Requête reçue - avant traitement")

		// Création d'un wrapper pour capturer les en-têtes de réponse
		ww := &responseWriterWrapper{ResponseWriter: w, status: 200}

		// Appel au prochain middleware
		next.ServeHTTP(ww, r)

		// Log des en-têtes de réponse
		log.Debug().
			Int("status_code", ww.status).
			Interface("response_headers", ww.Header()).
			Msg("Réponse envoyée")

		// Log spécifique pour les cookies de session dans la réponse
		for _, cookie := range ww.Header()["Set-Cookie"] {
			log.Debug().
				Str("set_cookie", cookie).
				Msg("Cookie défini dans la réponse")
		}
	})
}

// responseWriterWrapper est un wrapper pour capturer le code de statut HTTP
type responseWriterWrapper struct {
	http.ResponseWriter
	status int
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
