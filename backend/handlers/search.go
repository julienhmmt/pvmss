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
	stateManager state.StateManager
}

// NewSearchHandler crée un nouveau gestionnaire de recherche
func NewSearchHandler(sm state.StateManager) *SearchHandler {
	return &SearchHandler{stateManager: sm}
}

// SearchPageHandler gère les requêtes GET et POST pour la page de recherche
func (h *SearchHandler) SearchPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Créer un logger pour cette requête
	log := logger.Get().With().
		Str("handler", "SearchPageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Traitement de la requête de recherche")

	data := make(map[string]interface{})

	// Pour les requêtes GET, afficher simplement le formulaire de recherche
	if r.Method == http.MethodGet {
		log.Debug().Msg("Affichage du formulaire de recherche")
		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Search.Title"]
		renderTemplateInternal(w, r, "search", data)
		log.Info().Msg("Formulaire de recherche affiché avec succès")
		return
	}

	// Pour les requêtes POST, effectuer la recherche
	if r.Method == http.MethodPost {
		// Récupérer et valider les paramètres de recherche
		vmid := strings.TrimSpace(r.FormValue("vmid"))
		name := strings.TrimSpace(r.FormValue("name"))

		log.Info().
			Str("vmid", vmid).
			Str("name", name).
			Msg("Nouvelle recherche de VM")

		// Valider les entrées
		if vmid == "" && name == "" {
			log.Warn().Msg("Aucun critère de recherche fourni")
			data["Error"] = "Veuillez fournir au moins un critère de recherche (ID ou nom)"
			i18n.LocalizePage(w, r, data)
			data["Title"] = data["Search.Title"]
			renderTemplateInternal(w, r, "search", data)
			return
		}

		// Construire la chaîne de requête pour l'affichage
		var queryParts []string
		if vmid != "" {
			queryParts = append(queryParts, "VMID: "+vmid)
		}
		if name != "" {
			queryParts = append(queryParts, "Nom: "+name)
		}
		queryString := strings.Join(queryParts, ", ")
		data["Query"] = queryString

		log.Debug().
			Str("query", queryString).
			Msg("Critères de recherche formatés")

		// Récupérer le client Proxmox depuis le gestionnaire d'état injecté
		client := h.stateManager.GetProxmoxClient()
		if client == nil {
			errMsg := "Client Proxmox non disponible"
			log.Error().Msg(errMsg)
			data["Error"] = errMsg
			i18n.LocalizePage(w, r, data)
			data["Title"] = data["Search.Results"]
			renderTemplateInternal(w, r, "search", data)
			return
		}

		log.Debug().Msg("Client Proxmox récupéré avec succès")

		// Créer un contexte avec timeout pour la requête API
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		log.Debug().Msg("Lancement de la recherche de VMs")

		// Récupérer toutes les VMs depuis Proxmox
		vms, err := searchVMs(ctx, client, vmid, name)
		if err != nil {
			log.Error().
				Err(err).
				Msg("Échec de la recherche de VMs")

			data["Error"] = fmt.Sprintf("Échec de la recherche de VMs: %v", err)
			i18n.LocalizePage(w, r, data)
			data["Title"] = data["Search.Results"]
			renderTemplateInternal(w, r, "search", data)
			return
		}

		log.Info().
			Int("results_count", len(vms)).
			Msg("Recherche de VMs terminée avec succès")

		// Ajouter les résultats à la map de données
		data["Results"] = vms
		if len(vms) == 0 {
			log.Debug().Msg("Aucun résultat trouvé pour la recherche")
			data["NoResults"] = true
		} else {
			log.Debug().
				Int("vms_found", len(vms)).
				Msg("VMs trouvées avec succès")
		}

		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Search.Results"]

		log.Debug().Msg("Rendu de la page de résultats")
		renderTemplateInternal(w, r, "search", data)
		log.Info().Msg("Résultats de recherche affichés avec succès")
		return
	}

	// Méthode HTTP non autorisée
	log.Warn().
		Str("method", r.Method).
		Msg("Méthode HTTP non autorisée pour la route de recherche")

	http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
}

// searchVMs recherche les VMs selon les critères fournis
func searchVMs(ctx context.Context, clientInterface proxmox.ClientInterface, vmidStr, name string) ([]map[string]interface{}, error) {
	log := logger.Get().With().
		Str("handler", "searchVMs").
		Str("vmid", vmidStr).
		Str("name", name).
		Logger()

	// Get all nodes
	nodes, err := proxmox.GetNodeNames(clientInterface)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get node names")
		return nil, fmt.Errorf("failed to get node names: %w", err)
	}

	log.Debug().
		Strs("nodes", nodes).
		Int("nodes_count", len(nodes)).
		Msg("Liste des nœuds récupérée avec succès")

	// Récupérer toutes les VMs
	allVMs, err := proxmox.GetVMsWithContext(ctx, clientInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs: %w", err)
	}

	// Si aucun critère de recherche n'est fourni, retourner toutes les VMs (limité à 20)
	if vmidStr == "" && name == "" {
		results := make([]map[string]interface{}, 0, min(20, len(allVMs)))
		for i, vm := range allVMs {
			if i >= 20 {
				break
			}
			results = append(results, map[string]interface{}{
				"vmid":   vm.VMID,
				"name":   vm.Name,
				"node":   vm.Node,
				"status": "", // Status n'est pas disponible dans la structure VM de base
				"cpu":    vm.CPU,
				"memory": vm.Mem,
				"disk":   vm.MaxDisk,
			})
		}
		return results, nil
	}

	// Convertir vmid en entier si spécifié
	var vmid int
	if vmidStr != "" {
		var err error
		vmid, err = strconv.Atoi(vmidStr)
		if err != nil {
			errMsg := "ID de VM invalide"
			log.Error().
				Err(err).
				Str("vmid_input", vmidStr).
				Msg(errMsg)
			return nil, fmt.Errorf("%s: %v", errMsg, err)
		}
		log.Debug().
			Int("vmid_parsed", vmid).
			Msg("ID de VM parsé avec succès")
	}

	// Filtrer les VMs selon les critères
	var results []map[string]interface{}

	log.Debug().
		Int("total_vms_to_filter", len(allVMs)).
		Msg("Début du filtrage des VMs selon les critères")

	for _, vm := range allVMs {
		vmID := vm.VMID // VMID is already an int
		vmName := vm.Name
		node := vm.Node

		// Filtrer par VMID si spécifié
		if vmid > 0 {
			if vmID != vmid {
				continue
			}
			log.Debug().
				Int("vmid", vmID).
				Str("name", vmName).
				Str("node", node).
				Msg("VM correspondant au critère VMID")
		}

		// Filtrer par nom si spécifié
		if name != "" {
			if !strings.Contains(strings.ToLower(vmName), strings.ToLower(name)) {
				continue
			}
			log.Debug().
				Int("vmid", vmID).
				Str("name", vmName).
				Str("node", node).
				Msg("VM correspondant au critère de nom")
		}

		match := true

		if vmidStr != "" && vmID != vmid {
			match = false
		}

		if name != "" && !strings.Contains(strings.ToLower(vmName), strings.ToLower(name)) {
			match = false
		}

		if match {
			vmMap := map[string]interface{}{
				"vmid":   vm.VMID,
				"name":   vm.Name,
				"node":   vm.Node,
				"status": "", // Status n'est pas disponible dans la structure VM de base
				"cpu":    vm.CPU,
				"memory": vm.Mem,
				"disk":   vm.MaxDisk,
			}
			results = append(results, vmMap)

			log.Debug().
				Int("vmid", vmID).
				Str("name", vmName).
				Str("node", node).
				Int("filtered_count", len(results)).
				Msg("VM correspondante trouvée")
		}
	}

	log.Info().
		Int("matching_vms", len(results)).
		Int("total_vms_searched", len(allVMs)).
		Msg("Filtrage des VMs terminé avec succès")

	return results, nil
}

// RegisterRoutes enregistre les routes de recherche
func (h *SearchHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "SearchHandler").
		Str("function", "RegisterRoutes").
		Logger()

	if router == nil {
		log.Error().Msg("Le routeur est nul, impossible d'enregistrer les routes de recherche")
		return
	}

	log.Debug().Msg("Enregistrement des routes de recherche")

	router.GET("/search", h.SearchPageHandler)
	router.POST("/search", h.SearchPageHandler)

	log.Info().
		Strs("routes", []string{"GET /search", "POST /search"}).
		Msg("Routes de recherche enregistrées avec succès")
}

// SearchHandlerFunc est une fonction wrapper pour compatibilité avec le code existant
func SearchHandlerFunc(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("function", "SearchHandlerFunc").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	log.Debug().Msg("Appel du gestionnaire de recherche via la fonction wrapper")

	h := &SearchHandler{stateManager: getStateManager(r)}
	h.SearchPageHandler(w, r, nil)

	log.Debug().Msg("Traitement du gestionnaire de recherche terminé")
}
