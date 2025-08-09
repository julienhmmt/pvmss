package handlers

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/middleware"
	"pvmss/security"
	securityMiddleware "pvmss/security/middleware"
	"pvmss/state"
)

// InitHandlers initializes all handlers and configures routes
func InitHandlers(stateManager state.StateManager) http.Handler {
	log := logger.Get().With().Str("component", "handlers").Logger()

	// Create a new router
	router := httprouter.New()

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
	vmHandler := NewVMHandler(stateManager)
	storageHandler := NewStorageHandler(stateManager)
	searchHandler := NewSearchHandler(stateManager)
	docsHandler := NewDocsHandler()
	healthHandler := NewHealthHandler(stateManager)
	settingsHandler := NewSettingsHandler(stateManager)
	tagsHandler := NewTagsHandler(stateManager)
	vmbrHandler := NewVMBRHandler(stateManager)

	// Configure routes
	setupRoutes(router, authHandler, adminHandler, vmHandler, storageHandler, searchHandler, docsHandler, healthHandler, settingsHandler, tagsHandler, vmbrHandler)

	// Configure static files handler
	setupStaticFiles(router)

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

	// IMPORTANT: scs LoadAndSave must be the OUTERMOST wrapper so downstream middlewares see session data in context
	if sessionManager != nil {
		handler = sessionManager.LoadAndSave(handler)
		log.Info().Msgf("Session middleware enabled with session manager: %p", sessionManager)
	}

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

	// Route d'accueil
	router.GET("/", IndexRouterHandler)
}

// setupStaticFiles configure le serveur de fichiers statiques
func setupStaticFiles(router *httprouter.Router) {
	// Récupère le chemin de base du répertoire frontend.
	basePath := state.GetTemplatesPath()

	// Crée des gestionnaires de fichiers spécifiques pour chaque sous-répertoire statique.
	cssServer := http.FileServer(http.Dir(filepath.Join(basePath, "css")))
	jsServer := http.FileServer(http.Dir(filepath.Join(basePath, "js")))
	imagesServer := http.FileServer(http.Dir(filepath.Join(basePath, "images")))
	webfontsServer := http.FileServer(http.Dir(filepath.Join(basePath, "webfonts")))

	// Configure les routes pour servir les fichiers statiques en utilisant StripPrefix.
	// Cela garantit que le serveur de fichiers reçoit le bon chemin relatif.
	router.Handler(http.MethodGet, "/css/*filepath", http.StripPrefix("/css/", cssServer))
	router.Handler(http.MethodGet, "/js/*filepath", http.StripPrefix("/js/", jsServer))
	router.Handler(http.MethodGet, "/images/*filepath", http.StripPrefix("/images/", imagesServer))
	router.Handler(http.MethodGet, "/webfonts/*filepath", http.StripPrefix("/webfonts/", webfontsServer))

	logger.Get().Info().Str("path", basePath).Msg("Service des fichiers statiques configuré pour css, js, images et webfonts")
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
