package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/state"
)

// InitHandlers initialise tous les gestionnaires et configure les routes
func InitHandlers() http.Handler {
	// Créer un nouveau routeur
	router := httprouter.New()

	// S'assurer que le tag par défaut existe
	if err := EnsureDefaultTag(); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to ensure default tag")
	}

	// Initialiser les gestionnaires
	authHandler := NewAuthHandler()
	adminHandler := NewAdminHandler()
	vmHandler := NewVMHandler()
	storageHandler := NewStorageHandler()
	searchHandler := NewSearchHandler()
	docsHandler := NewDocsHandler()
	healthHandler := NewHealthHandler()
	settingsHandler := NewSettingsHandler()
	tagsHandler := NewTagsHandler()
	tagsUIHandler := NewTagsUIHandler()

	// Configurer les routes
	setupRoutes(router, authHandler, adminHandler, vmHandler, storageHandler, searchHandler, docsHandler, healthHandler, settingsHandler, tagsHandler, tagsUIHandler)

	// Configurer le gestionnaire de fichiers statiques
	setupStaticFiles(router)

	// Appliquer le middleware de session
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()

	// Wrapper le routeur avec le middleware de session pour que les données de session
	// soient disponibles dans le contexte de la requête
	return sessionManager.LoadAndSave(router)
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
	tagsUIHandler *TagsUIHandler,
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
	tagsUIHandler.RegisterRoutes(router)

	// Route d'accueil
	router.GET("/", IndexRouterHandler)
}

// setupStaticFiles configure le serveur de fichiers statiques
func setupStaticFiles(router *httprouter.Router) {
	// Servir les fichiers statiques depuis le répertoire frontend
	router.ServeFiles("/css/*filepath", http.Dir("frontend/css"))
	router.ServeFiles("/js/*filepath", http.Dir("frontend/js"))
}
