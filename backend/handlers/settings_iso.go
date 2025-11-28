package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"

	appI18n "pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
)

// ISOEntry represents an ISO file entry
type ISOEntry struct {
	Node    string      `json:"node"`
	Storage string      `json:"storage"`
	Volid   string      `json:"volid"`
	Size    interface{} `json:"size"`
	Format  string      `json:"format"`
	Enabled bool        `json:"enabled,omitempty"`
}

// NodeISOGroup represents grouped ISO entries per node for easier template rendering
type NodeISOGroup struct {
	Node string     `json:"node"`
	ISOs []ISOEntry `json:"isos"`
}

// isoPerStorageTimeout defines the maximum duration allowed for querying a single
// storage's ISO content. This prevents a slow or unresponsive backend (e.g. cephfs)
// from blocking or cancelling the entire ISO listing.
const isoPerStorageTimeout = 15 * time.Second

// fetchAllISOs retrieves all ISOs from all nodes and storages using sequential processing.
// This prevents slow storages like cephfs from cancelling the entire operation.
// It also returns the number of storages that failed to list their ISO content and a list of failed storage details.
func (h *SettingsHandler) fetchAllISOs(ctx context.Context, checkEnabled bool) ([]ISOEntry, int, []string, error) {
	log := logger.Get().With().Str("function", "fetchAllISOs").Logger()

	// Check if incoming context is already cancelled
	if ctx.Err() != nil {
		log.Error().Err(ctx.Err()).Msg("Incoming context is already cancelled before ISO fetch")
		return nil, 0, nil, fmt.Errorf("incoming context cancelled: %w", ctx.Err())
	}
	log.Debug().
		Str("component", "settings_iso").
		Str("operation", "fetch_isos").
		Str("reason", "context_valid").
		Msg("Starting ISO fetch with valid context")

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to create resty client for ISO retrieval: %w", err)
	}

	// Process storages sequentially to prevent cephfs and other slow storages
	// from cancelling the entire operation with context cancellation
	// This ensures that even if one storage is slow/fails, others will still be processed

	// Fetch nodes and storages sequentially first
	nodes, err := proxmox.GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	storages, err := proxmox.GetStoragesResty(ctx, restyClient)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to get storages: %w", err)
	}

	enabledSet := make(map[string]struct{})
	if checkEnabled {
		if settings := h.stateManager.GetSettings(); settings != nil {
			for _, enabledISO := range settings.ISOs {
				enabledSet[enabledISO] = struct{}{}
			}
		}
	}

	allISOs := make([]ISOEntry, 0)
	failedStorages := 0
	failedStorageDetails := make([]string, 0)

	// Process each storage sequentially with individual timeouts
	// This prevents a single slow cephfs storage from blocking the entire operation
	for _, nodeName := range nodes {
		for _, storage := range storages {
			// Check if storage is available on this node and supports ISO
			// Fix: Properly parse the Nodes field which can be comma-separated
			isNodeInStorage := storage.Nodes == "" || isStorageAvailableOnNode(storage.Nodes, nodeName)
			supportsISO := containsISO(storage.Content)

			logger.Get().Debug().
				Str("node", nodeName).
				Str("storage", storage.Storage).
				Str("storage_type", storage.Type).
				Str("nodes_field", storage.Nodes).
				Bool("is_node_in_storage", isNodeInStorage).
				Bool("supports_iso", supportsISO).
				Str("content_field", storage.Content).
				Msg("Checking storage availability and ISO support")

			if !isNodeInStorage || !supportsISO {
				logger.Get().Debug().
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Str("reason", "storage_not_available_or_no_iso_support").
					Msg("Skipping storage - not available on node or doesn't support ISO")
				continue
			}

			// Create a fresh context for each storage to avoid context cancellation issues
			// Use background context with timeout instead of inheriting from parent
			storageCtx, storageCancel := context.WithTimeout(context.Background(), isoPerStorageTimeout)

			logger.Get().Debug().
				Str("node", nodeName).
				Str("storage", storage.Storage).
				Str("storage_type", storage.Type).
				Dur("timeout", isoPerStorageTimeout).
				Msg("Fetching ISO list from storage")

			isoList, err := proxmox.GetISOListResty(storageCtx, restyClient, nodeName, storage.Storage)
			storageCancel()

			if err != nil {
				failedStorages++
				// Collect failed storage details
				detail := fmt.Sprintf("%s (%s)", storage.Storage, nodeName)
				failedStorageDetails = append(failedStorageDetails, detail)

				// Check if this is a context cancellation (likely timeout for cephfs)
				if ctxErr := storageCtx.Err(); ctxErr != nil {
					logger.Get().Warn().
						Err(err).
						Str("node", nodeName).
						Str("storage", storage.Storage).
						Str("storage_type", storage.Type).
						Str("content", storage.Content).
						Bool("is_context_cancelled", true).
						Dur("timeout", isoPerStorageTimeout).
						Msg("Storage ISO fetch timed out (likely cephfs or slow storage), skipping but continuing with other storages")
				} else {
					logger.Get().Warn().
						Err(err).
						Str("node", nodeName).
						Str("storage", storage.Storage).
						Str("storage_type", storage.Type).
						Str("content", storage.Content).
						Bool("is_context_cancelled", false).
						Msg("Failed to get ISO list for storage, skipping but continuing with other storages")
				}
				continue
			}

			logger.Get().Info().
				Str("node", nodeName).
				Str("storage", storage.Storage).
				Int("iso_count", len(isoList)).
				Msg("Successfully fetched ISO list from storage")

			// Validate ISO list before processing
			if len(isoList) == 0 {
				logger.Get().Debug().
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Msg("Storage returned empty ISO list - this is normal if no ISOs are present")
				continue
			}

			for _, iso := range isoList {
				// Validate ISO entry before adding
				if iso.VolID == "" {
					logger.Get().Warn().
						Str("node", nodeName).
						Str("storage", storage.Storage).
						Msg("Skipping ISO entry with empty VolID")
					continue
				}

				entry := ISOEntry{
					Node:    nodeName,
					Storage: storage.Storage,
					Volid:   iso.VolID,
					Size:    iso.Size,
					Format:  iso.Format,
				}

				if _, ok := enabledSet[iso.VolID]; ok {
					entry.Enabled = true
				}

				allISOs = append(allISOs, entry)

				logger.Get().Debug().
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Str("volid", iso.VolID).
					Int64("size", iso.Size).
					Str("format", iso.Format).
					Bool("enabled", entry.Enabled).
					Msg("Added ISO entry to list")
			}
		}
	}

	// Log summary of storage processing
	logger.Get().Info().
		Int("total_storages_processed", len(nodes)*len(storages)).
		Int("failed_storages", failedStorages).
		Int("successful_isos", len(allISOs)).
		Msg("ISO fetch completed - slow storages (like cephfs) handled gracefully")

	return allISOs, failedStorages, failedStorageDetails, nil
}

// ISOPageHandler renders the ISO management page (server-rendered, no JS required)
func (h *SettingsHandler) ISOPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ISOPageHandler", r)

	settings := h.stateManager.GetSettings()
	enabledMap := make(map[string]bool)
	if settings != nil {
		for _, v := range settings.ISOs {
			enabledMap[v] = true
		}
	}

	// Success banner via query params
	query := r.URL.Query()
	success := query.Get("success") != ""
	act := query.Get("action")
	isoName := query.Get("iso")
	var successMsg string
	if success {
		localizer := appI18n.GetLocalizerFromRequest(r)
		isoDisplay := isoName
		if isoDisplay != "" {
			isoDisplay = filepath.Base(isoDisplay)
		}

		var messageKey string
		switch act {
		case "enable":
			messageKey = "Admin.ISO.ToggleEnabled"
		case "disable":
			messageKey = "Admin.ISO.ToggleDisabled"
		}

		if messageKey != "" {
			localized, err := localizer.Localize(&goi18n.LocalizeConfig{
				MessageID:      messageKey,
				TemplateData:   map[string]string{"Name": isoDisplay},
				PluralCount:    nil,
				DefaultMessage: nil,
			})
			if err == nil {
				successMsg = localized
			}
		}

		if successMsg == "" {
			localized, err := localizer.Localize(&goi18n.LocalizeConfig{MessageID: "Admin.ISO.ToggleUpdated"})
			if err == nil {
				successMsg = localized
			}
		}

		if successMsg == "" {
			switch act {
			case "enable":
				successMsg = fmt.Sprintf("ISO \"%s\" enabled", isoDisplay)
			case "disable":
				successMsg = fmt.Sprintf("ISO \"%s\" disabled", isoDisplay)
			default:
				successMsg = "ISO settings updated"
			}
		}
	}

	opts := []TemplateOption{
		WithAdminActive("iso"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("TitleKey", "Admin.ISO.Title"),
		WithData("ISOsList", []ISOInfo{}),
		WithData("EnabledISOs", enabledMap),
		WithData("AllISOs", []ISOEntry{}),
		WithData("ISOGroupByNode", []NodeISOGroup{}),
	}

	if successMsg != "" {
		opts = append(opts, WithSuccess(successMsg))
	}

	data := NewTemplateDataWithOptions("", opts...).ToMap()

	// Return early if Proxmox not connected
	if !data["ProxmoxConnected"].(bool) {
		data["Warning"] = true
		data["WarningMessage"] = appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Error.ProxmoxConnectionError")
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	// Fetch all ISOs with enabled check
	isos, failedStorages, failedStorageDetails, err := h.fetchAllISOs(ctx, true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch ISOs for page")
		data["Warning"] = true
		data["WarningMessage"] = appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Error.FailedToGetResources")
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	// If some storages failed to list their content, show a non-blocking warning.
	if failedStorages > 0 {
		data["Warning"] = true
		if len(failedStorageDetails) > 0 {
			if len(failedStorageDetails) == 1 {
				// Single storage failure - more specific message
				parts := strings.Split(failedStorageDetails[0], " (")
				storageName := parts[0]
				nodeName := strings.TrimSuffix(parts[1], ")")

				localizer := appI18n.GetLocalizerFromRequest(r)
				data["WarningMessage"] = fmt.Sprintf("%s: %s",
					appI18n.Localize(localizer, "Admin.ISO.PartialStorageFailure"),
					fmt.Sprintf(appI18n.Localize(localizer, "Admin.ISO.StorageUnavailableOnNode"), storageName, nodeName))
			} else {
				// Multiple storage failures
				details := strings.Join(failedStorageDetails, ", ")
				data["WarningMessage"] = fmt.Sprintf("%s: %s",
					appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Admin.ISO.PartialStorageFailure"),
					details)
			}
		} else {
			data["WarningMessage"] = fmt.Sprintf("%s: %d storage(s) failed",
				appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Admin.ISO.PartialStorageFailure"),
				failedStorages)
		}
	}

	sort.Slice(isos, func(i, j int) bool {
		nodeI := strings.ToLower(isos[i].Node)
		nodeJ := strings.ToLower(isos[j].Node)
		if nodeI == nodeJ {
			nameI := strings.ToLower(filepath.Base(isos[i].Volid))
			nameJ := strings.ToLower(filepath.Base(isos[j].Volid))
			if nameI == nameJ {
				return strings.ToLower(isos[i].Storage) < strings.ToLower(isos[j].Storage)
			}
			return nameI < nameJ
		}
		return nodeI < nodeJ
	})

	// Validate final ISO list before passing to template
	logger.Get().Info().
		Int("total_isos_before_sort", len(isos)).
		Int("total_isos_after_sort", len(isos)).
		Msg("ISO list sorted and validated")

	data["AllISOs"] = isos

	if len(isos) > 0 {
		groups := make([]NodeISOGroup, 0)
		currentNode := isos[0].Node
		currentGroup := NodeISOGroup{Node: currentNode, ISOs: []ISOEntry{}}

		logger.Get().Debug().
			Str("first_node", currentNode).
			Msg("Starting ISO grouping by node")

		for _, iso := range isos {
			if iso.Node != currentNode {
				groups = append(groups, currentGroup)
				currentNode = iso.Node
				currentGroup = NodeISOGroup{Node: currentNode, ISOs: []ISOEntry{}}
				logger.Get().Debug().
					Str("new_node", currentNode).
					Msg("Created new node group")
			}
			currentGroup.ISOs = append(currentGroup.ISOs, iso)
		}
		groups = append(groups, currentGroup)
		data["ISOGroupByNode"] = groups

		logger.Get().Info().
			Int("node_groups", len(groups)).
			Msg("ISOs grouped by node for template rendering")
	} else {
		logger.Get().Warn().
			Msg("No ISOs found to display - template will show empty state")
	}

	log.Debug().
		Int("iso_count", len(isos)).
		Str("component", "settings_iso").
		Str("operation", "render_iso_page").
		Str("reason", "page_rendered").
		Msg("ISO page rendered")
	renderTemplateInternal(w, r, "admin_iso", data)
}

// ToggleISOHandler toggles a single ISO enabled state (auto-save per click, no JS)
func (h *SettingsHandler) ToggleISOHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ToggleISOHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	volid := strings.TrimSpace(r.FormValue("volid"))
	action := strings.TrimSpace(r.FormValue("action"))

	if volid == "" {
		log.Error().Msg("Missing volid parameter")
		http.Error(w, appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Error.MissingRequiredParameters"), http.StatusBadRequest)
		return
	}

	if action == "" {
		log.Error().Msg("Missing action parameter")
		http.Error(w, appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Error.MissingRequiredParameters"), http.StatusBadRequest)
		return
	}

	// Convert action to enabled boolean
	var enabled bool
	switch action {
	case "enable":
		enabled = true
	case "disable":
		enabled = false
	default:
		log.Error().Str("action", action).Msg("Invalid action parameter")
		http.Error(w, appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Error.InvalidFormData"), http.StatusBadRequest)
		return
	}

	log.Debug().
		Str("volid", volid).
		Bool("enabled", enabled).
		Str("component", "settings_iso").
		Str("operation", "toggle_iso").
		Str("reason", "iso_toggle").
		Msg("Toggling ISO")

	// Update settings
	settings := h.stateManager.GetSettings()
	if settings == nil {
		log.Error().Msg("Settings not available")
		http.Error(w, appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Error.SettingsUnavailable"), http.StatusInternalServerError)
		return
	}

	// Create a new slice for ISOs
	var newISOs []string
	found := false
	for _, iso := range settings.ISOs {
		if iso == volid {
			found = true
			if enabled {
				newISOs = append(newISOs, iso) // Keep it
			}
			// If not enabled, we skip adding it (remove it)
		} else {
			newISOs = append(newISOs, iso) // Keep other ISOs
		}
	}

	// If we want to enable it and it wasn't found, add it
	if enabled && !found {
		newISOs = append(newISOs, volid)
	}

	// Update settings
	settings.ISOs = newISOs
	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		http.Error(w, appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	log.Info().Str("volid", volid).Bool("enabled", enabled).Msg("ISO toggle completed")

	params := url.Values{}
	params.Set("success", "1")
	params.Set("action", action)
	if volid != "" {
		params.Set("iso", filepath.Base(volid))
	}

	redirectURL := "/admin/iso"
	if encoded := params.Encode(); encoded != "" {
		redirectURL = redirectURL + "?" + encoded
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// RegisterISORoutes registers ISO-related routes
func (h *SettingsHandler) RegisterISORoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin ISO routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/iso", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page":   h.ISOPageHandler,
		"toggle": h.ToggleISOHandler,
	})
}

// containsISO checks if a storage content type can contain ISOs
func containsISO(content string) bool {
	// Content types are separated by commas
	for _, part := range strings.Split(content, ",") {
		if strings.TrimSpace(part) == "iso" {
			return true
		}
	}
	return false
}

// isStorageAvailableOnNode checks if a storage is available on a specific node
// The Nodes field can be comma-separated (e.g., "node1,node2,node3") or empty for shared storages
func isStorageAvailableOnNode(nodesField, nodeName string) bool {
	if nodesField == "" {
		// Empty nodes field means storage is available on all nodes (shared storage)
		return true
	}

	// Split the nodes field by comma and check each node
	nodes := strings.Split(nodesField, ",")
	for _, node := range nodes {
		if strings.TrimSpace(node) == nodeName {
			return true
		}
	}

	return false
}
