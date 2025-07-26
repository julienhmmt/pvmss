package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"pvmss/i18n"
)

// StorageHandler gère les routes liées au stockage
type StorageHandler struct{}

// NewStorageHandler crée une nouvelle instance de StorageHandler
func NewStorageHandler() *StorageHandler {
	return &StorageHandler{}
}

// StoragePageHandler gère la page de gestion du stockage
func (h *StorageHandler) StoragePageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Mock temporaire pour la liste des stockages
	storages := []map[string]interface{}{
		{"storage": "local", "type": "directory", "total": 500, "used": 200, "avail": 300},
		{"storage": "local-lvm", "type": "lvm", "total": 1000, "used": 300, "avail": 700},
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Storages": storages,
	}

	// Ajouter les traductions
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["Storage.Title"]

	renderTemplateInternal(w, r, "storage", data)
}

// StorageAPIHandler gère les appels API pour les opérations sur le stockage
func (h *StorageHandler) StorageAPIHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Exemple d'implémentation d'une API pour le stockage
	response := map[string]interface{}{
		"status":  "success",
		"message": "Storage API endpoint",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RegisterRoutes enregistre les routes liées au stockage
func (h *StorageHandler) RegisterRoutes(router *httprouter.Router) {
	// Page de gestion du stockage (protégée par authentification)
	router.GET("/storage", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.StoragePageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// API de gestion du stockage (protégée par authentification)
	router.GET("/api/storage", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.StorageAPIHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
}
