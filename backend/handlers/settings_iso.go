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
	"golang.org/x/sync/errgroup"

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

// fetchAllISOs retrieves all ISOs from all nodes and storages using errgroup for concurrent API calls
func (h *SettingsHandler) fetchAllISOs(ctx context.Context, checkEnabled bool) ([]ISOEntry, error) {
	logger.Get().Debug().Msg("Starting ISO fetch from all nodes and storages")

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create resty client for ISO retrieval: %w", err)
	}

	// Use errgroup for concurrent API calls
	g, ctx := errgroup.WithContext(ctx)

	var nodes []string
	var storages []proxmox.Storage

	// Fetch nodes concurrently
	g.Go(func() error {
		var err error
		nodes, err = proxmox.GetNodeNamesResty(ctx, restyClient)
		if err != nil {
			return fmt.Errorf("failed to get nodes: %w", err)
		}
		logger.Get().Debug().Ints("node_count", []int{len(nodes)}).Msg("Retrieved node list")
		return nil
	})

	// Fetch storages concurrently
	g.Go(func() error {
		var err error
		storages, err = proxmox.GetStoragesResty(ctx, restyClient)
		if err != nil {
			return fmt.Errorf("failed to get storages: %w", err)
		}
		logger.Get().Debug().Ints("storage_count", []int{len(storages)}).Msg("Retrieved storage list")
		return nil
	})

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("failed to fetch nodes/storages for ISO retrieval: %w", err)
	}

	enabledSet := make(map[string]struct{})
	if checkEnabled {
		if settings := h.stateManager.GetSettings(); settings != nil {
			for _, enabledISO := range settings.ISOs {
				enabledSet[enabledISO] = struct{}{}
			}
		}
		logger.Get().Debug().Ints("enabled_iso_count", []int{len(enabledSet)}).Msg("Loaded enabled ISO settings")
	}

	allISOs := make([]ISOEntry, 0)
	isoCount := 0

	// For each node, get ISOs from each compatible storage
	for _, nodeName := range nodes {
		for _, storage := range storages {
			// Check if storage is available on this node and supports ISO
			isNodeInStorage := storage.Nodes == "" || strings.Contains(storage.Nodes, nodeName)
			if !isNodeInStorage || !containsISO(storage.Content) {
				continue
			}

			// Create a new context for this API call
			isoCtx, isoCancel := context.WithTimeout(ctx, 30*time.Second)

			isoList, err := proxmox.GetISOListResty(isoCtx, restyClient, nodeName, storage.Storage)
			if err != nil {
				logger.Get().Warn().Err(err).
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Str("storage_type", storage.Type).
					Msg("Failed to get ISO list for storage - continuing with other storages")
				isoCancel() // Cancel immediately on error
				continue
			}
			isoCancel() // Cancel immediately after successful API call

			logger.Get().Debug().
				Str("node", nodeName).
				Str("storage", storage.Storage).
				Str("storage_type", storage.Type).
				Int("iso_count", len(isoList)).
				Msg("Successfully retrieved ISO list from storage")

			for _, iso := range isoList {
				// Debug log for size field types (helpful for troubleshooting storage-specific issues)
				logger.Get().Debug().
					Str("volid", iso.VolID).
					Interface("size", iso.Size).
					Str("size_type", fmt.Sprintf("%T", iso.Size)).
					Msg("Processing ISO entry")

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
				isoCount++
			}
		}
	}

	logger.Get().Info().
		Int("total_iso_count", isoCount).
		Int("node_count", len(nodes)).
		Int("storage_count", len(storages)).
		Msg("Completed ISO fetch from all storages")

	return allISOs, nil
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
		data["Warning"] = appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Error.ProxmoxConnectionError")
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Fetch all ISOs with enabled check
	isos, err := h.fetchAllISOs(ctx, true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch ISOs for page")
		data["Warning"] = appI18n.Localize(appI18n.GetLocalizerFromRequest(r), "Error.FailedToGetResources")
		renderTemplateInternal(w, r, "admin_iso", data)
		return
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

	data["AllISOs"] = isos

	if len(isos) > 0 {
		groups := make([]NodeISOGroup, 0)
		currentNode := isos[0].Node
		currentGroup := NodeISOGroup{Node: currentNode, ISOs: []ISOEntry{}}
		for _, iso := range isos {
			if iso.Node != currentNode {
				groups = append(groups, currentGroup)
				currentNode = iso.Node
				currentGroup = NodeISOGroup{Node: currentNode, ISOs: []ISOEntry{}}
			}
			currentGroup.ISOs = append(currentGroup.ISOs, iso)
		}
		groups = append(groups, currentGroup)
		data["ISOGroupByNode"] = groups
	}

	log.Debug().Int("iso_count", len(isos)).Msg("ISO page rendered")
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

	log.Debug().Str("volid", volid).Bool("enabled", enabled).Msg("Toggling ISO")

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
