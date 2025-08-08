package handlers

import (
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
func InitHandlers() http.Handler {
	log := logger.Get().With().Str("component", "handlers").Logger()

	// Create a new router
	router := httprouter.New()

	// Ensure default tag exists
	if err := EnsureDefaultTag(); err != nil {
		log.Error().Err(err).Msg("Failed to ensure default tag")
	}

	// Get the global state manager
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		log.Fatal().Msg("State manager not initialized")
	}

	// Initialize handlers
	authHandler := NewAuthHandler()
	adminHandler := NewAdminHandler()
	vmHandler := NewVMHandler(stateManager)
	storageHandler := NewStorageHandler(stateManager)
	searchHandler := NewSearchHandler()
	docsHandler := NewDocsHandler()
	healthHandler := NewHealthHandler(stateManager)
	settingsHandler := NewSettingsHandler()
	tagsHandler := NewTagsHandler()
	vmbrHandler := NewVMBRHandler()

	// Configure routes
	setupRoutes(router, authHandler, adminHandler, vmHandler, storageHandler, searchHandler, docsHandler, healthHandler, settingsHandler, tagsHandler, vmbrHandler)

	// Configure static files handler
	setupStaticFiles(router)

	// Create middleware chain
	var handler http.Handler = router

	// Get the session manager
	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		log.Warn().Msg("Session manager is not available, running with limited functionality")
	} else {
		// 1. Session middleware (MUST be the first middleware that touches the request)
		handler = sessionManager.LoadAndSave(handler)
		log.Info().Msgf("Session middleware enabled with session manager: %p", sessionManager)

		// 2. Add our custom session middleware to ensure session manager is in context
		handler = securityMiddleware.SessionMiddleware(handler)

		// 3. Debug middleware (after session middleware to have access to session)
		handler = sessionDebugMiddleware(handler)

		// 4. CSRF token generation middleware (must be after session middleware)
		handler = security.CSRFGeneratorMiddleware(handler)

		// 5. Add CSRF token to context for templates
		handler = middleware.CSRFMiddleware(handler)
	}

	// 6. Security headers middleware
	handler = securityMiddleware.Headers(handler)

	// 7. CSRF validation middleware (must be after token generation and headers)
	handler = securityMiddleware.CSRF(handler)

	// 8. Proxmox status middleware (after CSRF validation)
	handler = middleware.ProxmoxStatusMiddleware(handler)

	log.Info().Msg("HTTP handlers and middleware initialized")
	return handler
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
