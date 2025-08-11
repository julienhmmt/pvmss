package handlers

import (
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// StateManager is an interface that both real and mock state managers implement
type StateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
	GetSettings() *state.AppSettings
	SetSettings(settings *state.AppSettings) error
}

// StorageHandler gère les routes liées au stockage
type StorageHandler struct {
	stateManager StateManager
}

// NewStorageHandler crée une nouvelle instance de StorageHandler
func NewStorageHandler(stateManager state.StateManager) *StorageHandler {
	return &StorageHandler{
		stateManager: stateManager,
	}
}

// StoragePageHandler gère la page de gestion du stockage
func (h *StorageHandler) StoragePageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "StorageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	// Récupérer le client Proxmox
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Récupérer les paramètres
	node := r.URL.Query().Get("node")
	if node == "" {
		// Try to detect first available node automatically
		if nodes, err := proxmox.GetNodeNames(h.stateManager.GetProxmoxClient()); err == nil && len(nodes) > 0 {
			node = nodes[0]
		}
	}

	// Récupérer les paramètres
	settings := h.stateManager.GetSettings()

	// Initialiser la liste si elle est nulle pour éviter les erreurs
	if settings.EnabledStorages == nil {
		settings.EnabledStorages = []string{}
	}

	// Déterminer si la configuration manuelle est utilisée
	useManualConfig := len(settings.EnabledStorages) > 0

	// Créer une map des stockages activés
	enabledStoragesMap := make(map[string]bool, len(settings.EnabledStorages))
	for _, storageName := range settings.EnabledStorages {
		enabledStoragesMap[storageName] = true
	}

	// Récupérer la configuration globale des stockages (pour Content/Type)
	globalStorages, err := proxmox.GetStorages(client)
	if err != nil {
		log.Error().Err(err).Msg("Erreur lors de la récupération de la configuration des stockages")
		http.Error(w, "Erreur lors de la récupération des stockages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Indexer la config globale par nom de stockage
	cfgByName := make(map[string]proxmox.Storage, len(globalStorages))
	for _, s := range globalStorages {
		cfgByName[s.Storage] = s
	}

	// Récupérer les stockages du nœud avec statistiques courantes
	nodeStorages, err := proxmox.GetNodeStorages(client, node)
	if err != nil {
		log.Error().Err(err).Msg("Erreur lors de la récupération des stockages du nœud")
		http.Error(w, "Erreur lors de la récupération des stockages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log le nombre de stockages trouvés avant filtrage
	log.Info().Int("global_count", len(globalStorages)).Int("node_count", len(nodeStorages)).Msg("Stockages récupérés depuis Proxmox")

	// Convertir le résultat en format attendu par le template (avec filtrage)
	storageMaps := make([]map[string]interface{}, 0, len(nodeStorages))

	// Convertir chaque stockage en map pour le template
	for i, storage := range nodeStorages {
		// Log des détails du stockage pour le débogage
		log.Debug().
			Int("index", i).
			Str("storage", storage.Storage).
			Str("type", storage.Type).
			Str("used", storage.Used.String()).
			Str("total", storage.Total.String()).
			Msg("Traitement du stockage")

		// Enrichir avec la config globale si disponible (Content/Type/Description)
		if cfg, ok := cfgByName[storage.Storage]; ok {
			if storage.Content == "" && cfg.Content != "" {
				storage.Content = cfg.Content
			}
			if storage.Type == "" && cfg.Type != "" {
				storage.Type = cfg.Type
			}
			if storage.Description == "" && cfg.Description != "" {
				storage.Description = cfg.Description
			}
		}

		// Filtrer uniquement les stockages pouvant héberger des disques VM
		// Règle: exclure PBS; inclure si content contient "images" OU si content vide mais type dans la whitelist
		if strings.EqualFold(storage.Type, "pbs") {
			log.Debug().Str("storage", storage.Storage).Msg("Skipping storage: PBS type")
			continue
		}
		canHoldVMDisks := false
		if storage.Content != "" && strings.Contains(storage.Content, "images") {
			canHoldVMDisks = true
		} else if storage.Content == "" {
			switch strings.ToLower(storage.Type) {
			case "dir", "lvm", "lvmthin", "zfs", "rbd", "ceph", "cephfs", "nfs", "glusterfs":
				canHoldVMDisks = true
			}
		}
		if !canHoldVMDisks {
			log.Debug().Str("storage", storage.Storage).Str("type", storage.Type).Str("content", storage.Content).Msg("Skipping storage: not VM-disk capable")
			continue
		}

		// Convertir Used et Total en int64 pour le template
		used, _ := storage.Used.Int64()
		total, _ := storage.Total.Int64()
		percent := 0
		if total > 0 {
			percent = int((used * 100) / total)
		}

		// Créer la map pour le template
		s := map[string]interface{}{
			"Storage":       storage.Storage,
			"Type":          storage.Type,
			"Used":          used,
			"Total":         total,
			"Description":   storage.Description,
			"Enabled":       !useManualConfig || enabledStoragesMap[storage.Storage],
			"Content":       storage.Content,
			"UsedPercent":   percent,
		}

		// Ajouter des champs optionnels s'ils sont présents
		if storage.Avail.String() != "" {
			avail, _ := storage.Avail.Int64()
			s["Available"] = avail
		}
		if storage.Content != "" {
			s["Content"] = storage.Content
		}

		storageMaps = append(storageMaps, s)
	}

	log.Debug().Interface("storages", nodeStorages).Msg("Storages récupérés")

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Title":           "Gestion du stockage",
		"Node":            node,
		"Storages":        storageMaps,
		"EnabledStorages": settings.EnabledStorages,
		"EnabledMap":      enabledStoragesMap,
	}

	// Log des données envoyées au template pour le débogage
	log.Debug().Interface("template_data", data).Msg("Data being sent to storage template")

	// Ajouter les traductions
	i18n.LocalizePage(w, r, data)
	renderTemplateInternal(w, r, "storage", data)
}

// UpdateStorageHandler gère la mise à jour des stockages activés
func (h *StorageHandler) UpdateStorageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "UpdateStorageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Lire les paramètres du formulaire
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Erreur lors de l'analyse du formulaire", http.StatusBadRequest)
		return
	}

	// Récupérer les stockages cochés depuis le formulaire
	enabledStoragesList := r.Form["enabled_storages"]

	// Mettre à jour les paramètres
	settings := h.stateManager.GetSettings()

	// Mettre à jour la liste des stockages activés
	settings.EnabledStorages = enabledStoragesList

	// Sauvegarder les paramètres
	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Erreur lors de la sauvegarde des paramètres")
		http.Error(w, "Erreur lors de la sauvegarde des paramètres", http.StatusInternalServerError)
		return
	}

	// Rediriger vers la page de gestion des stockages
	http.Redirect(w, r, "/admin/storage?success=true", http.StatusSeeOther)
}

// RegisterRoutes enregistre les routes liées au stockage
func (h *StorageHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/admin/storage", h.StoragePageHandler)
	router.POST("/admin/storage/update", h.UpdateStorageHandler)
}
