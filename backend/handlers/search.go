package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// SearchHandler gère les requêtes de recherche
type SearchHandler struct {
}

// NewSearchHandler crée un nouveau gestionnaire de recherche
func NewSearchHandler() *SearchHandler {
	return &SearchHandler{}
}

// SearchPageHandler gère les requêtes GET et POST pour la page de recherche
func (h *SearchHandler) SearchPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	data := make(map[string]interface{})

	// Pour les requêtes GET, afficher simplement le formulaire de recherche
	if r.Method == http.MethodGet {
		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Search.Title"]
		renderTemplateInternal(w, r, "search", data)
		return
	}

	// Pour les requêtes POST, effectuer la recherche
	if r.Method == http.MethodPost {
		vmid := r.FormValue("vmid")
		name := r.FormValue("name")

		logger.Get().Info().Str("vmid", vmid).Str("name", name).Msg("VM search")

		// Construire la chaîne de requête pour l'affichage
		var queryParts []string
		if vmid != "" {
			queryParts = append(queryParts, "VMID: "+vmid)
		}
		if name != "" {
			queryParts = append(queryParts, "Name: "+name)
		}
		data["Query"] = strings.Join(queryParts, ", ")

		// Récupérer le client Proxmox depuis l'état global
		client := state.GetGlobalState().GetProxmoxClient()
		if client == nil {
			logger.Get().Error().Msg("Proxmox client not available")
			data["Error"] = "Proxmox client not available"
			i18n.LocalizePage(w, r, data)
			data["Title"] = data["Search.Results"]
			renderTemplateInternal(w, r, "search", data)
			return
		}

		// Créer un contexte avec timeout pour la requête API
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Récupérer toutes les VMs depuis Proxmox
		vms, err := searchVMs(ctx, client, vmid, name)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to search VMs")
			data["Error"] = fmt.Sprintf("Failed to search VMs: %v", err)
			i18n.LocalizePage(w, r, data)
			data["Title"] = data["Search.Results"]
			renderTemplateInternal(w, r, "search", data)
			return
		}

		// Ajouter les résultats à la map de données
		data["Results"] = vms
		if len(vms) == 0 {
			data["NoResults"] = true
		}

		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Search.Results"]
		renderTemplateInternal(w, r, "search", data)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// searchVMs recherche les VMs selon les critères fournis
func searchVMs(ctx context.Context, client *proxmox.Client, vmidStr, name string) ([]map[string]interface{}, error) {
	// Récupérer toutes les VMs
	allVMs, err := proxmox.GetVMsWithContext(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs: %w", err)
	}

	// Si aucun critère de recherche n'est fourni, retourner toutes les VMs (limité à 20)
	if vmidStr == "" && name == "" {
		if len(allVMs) > 20 {
			return allVMs[:20], nil
		}
		return allVMs, nil
	}

	// Convertir vmid en entier si fourni
	var vmidInt int
	var vmidProvided bool
	if vmidStr != "" {
		vmidInt, err = strconv.Atoi(vmidStr)
		if err != nil {
			// Si vmid n'est pas un nombre valide, on considère qu'il n'est pas fourni
			logger.Get().Warn().Str("vmid", vmidStr).Msg("Invalid VMID format, ignoring this filter")
		} else {
			vmidProvided = true
		}
	}

	// Filtrer les VMs selon les critères
	results := make([]map[string]interface{}, 0)
	for _, vm := range allVMs {
		// Vérifier si la VM correspond aux critères de recherche
		if vmidProvided {
			// Vérifier si le VMID correspond
			vmidFloat, ok := vm["vmid"].(float64)
			if !ok || int(vmidFloat) != vmidInt {
				continue
			}
		}

		if name != "" {
			// Vérifier si le nom contient la chaîne de recherche (insensible à la casse)
			vmName, ok := vm["name"].(string)
			if !ok || !strings.Contains(strings.ToLower(vmName), strings.ToLower(name)) {
				continue
			}
		}

		// Ajouter la VM aux résultats
		results = append(results, vm)
	}

	return results, nil
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
