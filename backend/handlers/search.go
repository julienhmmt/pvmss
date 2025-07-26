package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/logger"
)

// SearchHandler gère les routes de recherche
type SearchHandler struct{}

// NewSearchHandler crée une nouvelle instance de SearchHandler
func NewSearchHandler() *SearchHandler {
	return &SearchHandler{}
}

// SearchPageHandler gère la page de recherche
func (h *SearchHandler) SearchPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Créer les données de base
	data := make(map[string]interface{})

	if r.Method == http.MethodGet {
		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Search.Title"]
		renderTemplateInternal(w, r, "search", data)
		return
	}

	if r.Method == http.MethodPost {
		vmid := r.FormValue("vmid")
		name := r.FormValue("name")

		logger.Get().Info().Str("vmid", vmid).Str("name", name).Msg("VM search")

		// Mettre à jour les données avec les résultats de la recherche
		data["Results"] = []map[string]string{}
		data["Query"] = map[string]string{"vmid": vmid, "name": name}

		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Search.Results"]
		renderTemplateInternal(w, r, "search", data)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// RegisterRoutes enregistre les routes de recherche
func (h *SearchHandler) RegisterRoutes(router *httprouter.Router) {
	// Page de recherche
	router.GET("/search", h.SearchPageHandler)
	router.POST("/search", h.SearchPageHandler)
}

// SearchHandlerFunc est une fonction wrapper pour compatibilité avec le code existant
func SearchHandlerFunc(w http.ResponseWriter, r *http.Request) {
	h := NewSearchHandler()
	h.SearchPageHandler(w, r, nil)
}
