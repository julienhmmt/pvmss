package apiv1

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"pvmss/constants"
	"pvmss/database"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// SetupHandler handles the first-run setup wizard API endpoints.
// Mutating routes are only reachable when IsBootstrapComplete() returns false;
// the Status endpoint is always public.
type SetupHandler struct {
	state state.StateManager
	db    database.DB
}

// MakeSetupHandler creates a new SetupHandler.
func MakeSetupHandler(s state.StateManager, db database.DB) *SetupHandler {
	return &SetupHandler{state: s, db: db}
}

// SetupStatusResponse is the response for GET /api/v1/setup/status.
type SetupStatusResponse struct {
	Complete   bool   `json:"complete"`
	ProxmoxOK  bool   `json:"proxmox_ok"`
	Offline    bool   `json:"offline"`
	ProxmoxURL string `json:"proxmox_url,omitempty"`
}

// SetupConnectionTestResponse is the response for POST /api/v1/setup/test-connection.
type SetupConnectionTestResponse struct {
	OK         bool   `json:"ok"`
	ProxmoxURL string `json:"proxmox_url"`
	Error      string `json:"error,omitempty"`
}

// SetupProxmoxDataResponse is the response for GET /api/v1/setup/proxmox-data.
type SetupProxmoxDataResponse struct {
	Nodes    []string `json:"nodes"`
	Storages []string `json:"storages"`
	ISOs     []string `json:"isos"`
	VMBRs    []string `json:"vmbrs"`
}

// SetupCompleteRequest is the body for POST /api/v1/setup/complete.
type SetupCompleteRequest struct {
	EnabledNodes    []string          `json:"enabled_nodes"`
	EnabledStorages []string          `json:"enabled_storages"`
	EnabledISOs     []string          `json:"enabled_isos"`
	EnabledVMBRs    []string          `json:"enabled_vmbrs"`
	Limits          SetupLimitsConfig `json:"limits"`
}

// SetupLimitsConfig holds the resource limits chosen during the setup wizard.
type SetupLimitsConfig struct {
	MaxVMs          int  `json:"max_vms"`
	MaxVMPerUser    int  `json:"max_vm_per_user"`
	MaxNetworkCards int  `json:"max_network_cards"`
	MaxDiskPerVM    int  `json:"max_disk_per_vm"`
	MaxSnapshots    int  `json:"max_snapshots"`
	AllowCustomYAML bool `json:"allow_custom_yaml"`
}

// RequireSetupIncompleteForTest exposes requireSetupIncomplete for white-box
// testing in the apiv1_test package.
func RequireSetupIncompleteForTest(db database.DB, next http.HandlerFunc) http.Handler {
	return requireSetupIncomplete(db, next)
}

// requireSetupIncomplete is a middleware that returns 404 when bootstrap is
// already complete, preventing re-entry into the setup wizard (T128).
func requireSetupIncomplete(db database.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		complete, err := db.IsBootstrapComplete()
		if err != nil {
			logger.Get().Error().Err(err).Msg("setup: failed to check bootstrap status")
			errInternal(w)
			return
		}
		if complete {
			errNotFound(w, "setup is no longer available")
			return
		}
		next(w, r)
	}
}

// Status handles GET /api/v1/setup/status.
// Always public — returns bootstrap completion state and Proxmox connectivity.
func (h *SetupHandler) Status(w http.ResponseWriter, _ *http.Request) {
	complete, err := h.db.IsBootstrapComplete()
	if err != nil {
		logger.Get().Error().Err(err).Msg("setup: failed to check bootstrap status")
		errInternal(w)
		return
	}
	proxmoxOK, _ := h.state.GetProxmoxStatus()
	envCfg := h.state.GetEnvConfig()
	resp := SetupStatusResponse{
		Complete:  complete,
		ProxmoxOK: proxmoxOK,
		Offline:   envCfg.Offline,
	}
	if !complete {
		resp.ProxmoxURL = envCfg.ProxmoxURL
	}
	writeJSON(w, resp)
}

// TestConnection handles POST /api/v1/setup/test-connection.
// Verifies Proxmox connectivity using the env var credentials (T126).
func (h *SetupHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	envCfg := h.state.GetEnvConfig()
	if envCfg.Offline {
		writeJSON(w, SetupConnectionTestResponse{
			OK:    false,
			Error: "offline mode enabled — Proxmox API calls are disabled",
		})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 10*time.Second)
	if err != nil {
		writeJSON(w, SetupConnectionTestResponse{
			OK:         false,
			ProxmoxURL: envCfg.ProxmoxURL,
			Error:      err.Error(),
		})
		return
	}
	var versionResp struct {
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := restyClient.Get(r.Context(), "/version", &versionResp); err != nil {
		writeJSON(w, SetupConnectionTestResponse{
			OK:         false,
			ProxmoxURL: envCfg.ProxmoxURL,
			Error:      err.Error(),
		})
		return
	}
	writeJSON(w, SetupConnectionTestResponse{
		OK:         true,
		ProxmoxURL: envCfg.ProxmoxURL,
	})
}

// ProxmoxData handles GET /api/v1/setup/proxmox-data.
// Returns available nodes, storages, ISOs and VMBRs for wizard steps 2–3 (T126).
func (h *SetupHandler) ProxmoxData(w http.ResponseWriter, r *http.Request) {
	empty := SetupProxmoxDataResponse{
		Nodes:    []string{},
		Storages: []string{},
		ISOs:     []string{},
		VMBRs:    []string{},
	}
	envCfg := h.state.GetEnvConfig()
	if envCfg.Offline {
		writeJSON(w, empty)
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 15*time.Second)
	if err != nil {
		writeJSON(w, empty)
		return
	}
	nodeNames, err := proxmox.GetNodeNamesResty(r.Context(), restyClient)
	if err != nil || len(nodeNames) == 0 {
		writeJSON(w, empty)
		return
	}
	storageNames := collectStorageNames(r.Context(), restyClient)
	isoNames := collectISONames(r.Context(), restyClient, nodeNames)
	vmbrNames := collectVMBRNames(r.Context(), restyClient, nodeNames)
	writeJSON(w, SetupProxmoxDataResponse{
		Nodes:    nodeNames,
		Storages: storageNames,
		ISOs:     isoNames,
		VMBRs:    vmbrNames,
	})
}

// collectStorageNames returns unique storage names from the Proxmox /storage endpoint.
func collectStorageNames(ctx context.Context, client *proxmox.RestyClient) []string {
	storages, err := proxmox.GetStoragesResty(ctx, client)
	if err != nil {
		return []string{}
	}
	names := make([]string, 0, len(storages))
	for _, s := range storages {
		names = append(names, s.Storage)
	}
	return names
}

// collectISONames returns unique ISO VolIDs across all nodes and ISO-capable storages.
func collectISONames(ctx context.Context, client *proxmox.RestyClient, nodeNames []string) []string {
	storages, err := proxmox.GetStoragesResty(ctx, client)
	if err != nil {
		return []string{}
	}
	seen := make(map[string]bool)
	var names []string
	for _, node := range nodeNames {
		for _, s := range storages {
			if !isoStorageSupportsISO(s.Content) || !isoStorageAvailableOnNode(s.Nodes, node) {
				continue
			}
			isoCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			isos, err := proxmox.GetISOListResty(isoCtx, client, node, s.Storage)
			cancel()
			if err != nil {
				continue
			}
			for _, iso := range isos {
				if iso.VolID == "" || seen[iso.VolID] {
					continue
				}
				seen[iso.VolID] = true
				name := iso.VolID
				if idx := strings.LastIndex(name, "/"); idx >= 0 {
					name = name[idx+1:]
				}
				names = append(names, name)
			}
		}
	}
	return names
}

// collectVMBRNames returns deduplicated bridge interface names across all provided nodes.
func collectVMBRNames(ctx context.Context, client *proxmox.RestyClient, nodeNames []string) []string {
	seen := make(map[string]bool)
	var names []string
	for _, node := range nodeNames {
		bridges, err := proxmox.GetVMBRsResty(ctx, client, node)
		if err != nil {
			continue
		}
		for _, b := range bridges {
			if seen[b.Iface] {
				continue
			}
			seen[b.Iface] = true
			names = append(names, b.Iface)
		}
	}
	return names
}

// Complete handles POST /api/v1/setup/complete.
// Persists the initial configuration chosen in the wizard and marks bootstrap
// complete so the app becomes fully operational (T127).
func (h *SetupHandler) Complete(w http.ResponseWriter, r *http.Request) {
	var req SetupCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	log := logger.Get()
	if len(req.EnabledNodes) > 0 {
		if err := h.state.SetEnabledNodes(req.EnabledNodes, "setup"); err != nil {
			log.Error().Err(err).Msg("setup: failed to save enabled nodes")
			errInternal(w)
			return
		}
	}
	if len(req.EnabledStorages) > 0 {
		if err := h.state.SetEnabledStorages(req.EnabledStorages, "setup"); err != nil {
			log.Error().Err(err).Msg("setup: failed to save enabled storages")
			errInternal(w)
			return
		}
	}
	if len(req.EnabledISOs) > 0 {
		if err := h.state.SetEnabledISOs(req.EnabledISOs, "setup"); err != nil {
			log.Error().Err(err).Msg("setup: failed to save enabled ISOs")
			errInternal(w)
			return
		}
	}
	if len(req.EnabledVMBRs) > 0 {
		if err := h.state.SetEnabledVMBRs(req.EnabledVMBRs, "setup"); err != nil {
			log.Error().Err(err).Msg("setup: failed to save enabled VMBRs")
			errInternal(w)
			return
		}
	}
	limits := &database.VMLimits{
		MaxVMs:          req.Limits.MaxVMs,
		MaxVMPerUser:    req.Limits.MaxVMPerUser,
		MaxNetworkCards: req.Limits.MaxNetworkCards,
		MaxDiskPerVM:    req.Limits.MaxDiskPerVM,
		MaxSnapshots:    req.Limits.MaxSnapshots,
		AllowCustomYAML: req.Limits.AllowCustomYAML,
	}
	if err := h.state.SetVMLimits(limits, "setup"); err != nil {
		log.Error().Err(err).Msg("setup: failed to save VM limits")
		errInternal(w)
		return
	}
	if err := h.db.CompleteBootstrap(constants.AppVersion); err != nil {
		log.Error().Err(err).Msg("setup: failed to complete bootstrap")
		errInternal(w)
		return
	}
	log.Info().Msg("First-run setup wizard completed")
	writeJSON(w, map[string]bool{"success": true})
}
