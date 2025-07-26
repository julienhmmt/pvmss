package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// HealthHandler gère les points de terminaison de santé et d'API
type HealthHandler struct{}

// NewHealthHandler crée une nouvelle instance de HealthHandler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthCheckHandler gère les requêtes de vérification de santé
func (h *HealthHandler) HealthCheckHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Mock: considérer Proxmox et session comme OK (à remplacer par logique réelle si besoin)
	proxmoxStatus := "ok"
	sessionStatus := "ok"

	// Préparer la réponse
	response := map[string]interface{}{
		"status":  "ok",
		"version": "1.0.0",
		"services": map[string]string{
			"proxmox": proxmoxStatus,
			"session": sessionStatus,
		},
	}

	// Renvoyer la réponse en JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIDocsHandler fournit la documentation de l'API
func (h *HealthHandler) APIDocsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Documentation Swagger/OpenAPI simplifiée
	docs := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]string{
			"title":       "PVMSS API",
			"description": "API for Proxmox Virtual Machine Self-Service",
			"version":     "1.0.0",
		},
		"paths": map[string]interface{}{
			"/api/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Health check",
					"description": "Returns the health status of the application",
					"responses": map[string]interface{}{
						"200": map[string]string{
							"description": "Application is healthy",
						},
					},
				},
			},
			"/api/vms": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List VMs",
					"description": "Returns a list of virtual machines",
					"security":    []map[string][]string{{"bearerAuth": {}}},
					"responses": map[string]interface{}{
						"200": map[string]string{
							"description": "A list of VMs",
						},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]string{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
	}

	// Renvoyer la documentation en JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(docs)
}

// NotFoundHandler gère les routes non trouvées
func (h *HealthHandler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		// Réponse JSON pour les routes API
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "The requested resource was not found",
		})
	} else {
		// Réponse HTML pour les routes web
		data := map[string]interface{}{
			"Title":        "Page non trouvée",
			"ErrorMessage": "La page que vous recherchez n'existe pas ou a été déplacée.",
		}
		w.WriteHeader(http.StatusNotFound)
		renderTemplateInternal(w, r, "error.html", data)
	}
}

// MethodNotAllowedHandler gère les méthodes HTTP non autorisées
func (h *HealthHandler) MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		// Réponse JSON pour les routes API
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Method Not Allowed",
			"message": "The requested method is not allowed for this resource",
		})
	} else {
		// Réponse HTML pour les routes web
		data := map[string]interface{}{
			"Title":        "Méthode non autorisée",
			"ErrorMessage": "La méthode de requête n'est pas autorisée pour cette ressource.",
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
		renderTemplateInternal(w, r, "error.html", data)
	}
}

// RegisterRoutes enregistre les routes de santé et d'API
func (h *HealthHandler) RegisterRoutes(router *httprouter.Router) {
	// Points de terminaison de santé
	router.GET("/health", h.HealthCheckHandler)
	router.GET("/api/health", h.HealthCheckHandler)

	// Documentation de l'API
	router.GET("/api/docs", h.APIDocsHandler)

	// Gestion des erreurs
	router.NotFound = http.HandlerFunc(h.NotFoundHandler)
	router.MethodNotAllowed = http.HandlerFunc(h.MethodNotAllowedHandler)
}
