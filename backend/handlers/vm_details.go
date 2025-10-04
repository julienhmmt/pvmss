package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
	"pvmss/state"
)

// VMStateManager defines the minimal state contract needed by VM details.
// Provides access to Proxmox client and application settings.
type VMStateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
	GetSettings() *state.AppSettings
	GetProxmoxStatus() (bool, string)
}

// VMHandler handles VM-related pages and API endpoints
type VMHandler struct {
	stateManager VMStateManager
}

// NewVMHandler creates a new VMHandler
func NewVMHandler(stateManager VMStateManager) *VMHandler {
	return &VMHandler{
		stateManager: stateManager,
	}
}

// RegisterRoutes registers VM-related routes
func (h *VMHandler) RegisterRoutes(router *httprouter.Router) {
	// VM creation routes
	router.GET("/vm/create", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.CreateVMPage(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.POST("/api/vm/create", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.CreateVMHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// VM details and actions routes
	router.GET("/vm/details/:vmid", h.VMDetailsHandler)
	router.POST("/vm/update/description", RequireAuthHandle(h.UpdateVMDescriptionHandler))
	router.POST("/vm/update/tags", RequireAuthHandle(h.UpdateVMTagsHandler))
	router.POST("/vm/action", RequireAuthHandle(h.VMActionHandler))

	// VM deletion routes
	router.GET("/vm/delete/:vmid", RequireAuthHandle(h.VMDeleteConfirmHandler))
	router.POST("/vm/delete", RequireAuthHandle(h.VMDeleteHandler))

	// VM console routes
	router.POST("/api/vm/vnc-ticket", RequireAuthHandle(h.GetVNCTicketHandler))
	router.GET("/vm/console/websocket", RequireAuthHandle(h.VMConsoleWebSocketHandler))
}

// VMDetailsHandler displays detailed information about a specific VM
func (h *VMHandler) VMDetailsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("VMDetailsHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodGet) {
		return
	}

	vmid := ps.ByName("vmid")
	if vmid == "" {
		log.Error().Msg("VM ID is required")
		http.Error(w, "VM ID is required", http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	stateManager := getStateManager(r)
	client := stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// If 'refresh=1' is present, proactively invalidate caches for nodes and VM lists
	// to avoid race conditions right after VM creation where cached lists don't include the new VM yet.
	if r.URL.Query().Get("refresh") == "1" {
		// Invalidate node list
		client.InvalidateCache("/nodes")
		if nodes, err := proxmox.GetNodeNamesWithContext(r.Context(), client); err == nil {
			for _, n := range nodes {
				client.InvalidateCache("/nodes/" + n + "/qemu")
			}
		} else {
			log.Warn().Err(err).Msg("Unable to get nodes while invalidating cache for refresh")
		}
	}

	// Get all VMs and find the one we want
	vms, err := proxmox.GetVMsWithContext(r.Context(), client)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Msg("Failed to get VMs")
		http.Error(w, "Failed to get VMs", http.StatusInternalServerError)
		return
	}

	// Find the VM by ID
	var vm *proxmox.VM
	for i := range vms {
		if vms[i].VMID == vmidInt {
			vm = &vms[i]
			break
		}
	}

	if vm == nil {
		// As a fallback (especially right after creation), try to locate the VM by querying
		// each node's current status endpoint which is uncached per-VM.
		if nodes, err := proxmox.GetNodeNamesWithContext(r.Context(), client); err == nil {
			for _, n := range nodes {
				if cur, err2 := proxmox.GetVMCurrentWithContext(r.Context(), client, n, vmidInt); err2 == nil && cur != nil {
					// Found the VM on node 'n', synthesize a minimal VM struct for display
					vm = &proxmox.VM{
						VMID:   vmidInt,
						Node:   n,
						Name:   cur.Name,
						Status: cur.Status,
						CPUs:   cur.CPUs,
						MaxMem: cur.MaxMem,
						Mem:    cur.Mem,
					}
					break
				}
			}
		} else {
			log.Warn().Err(err).Msg("Unable to get nodes for VM fallback lookup")
		}

		if vm == nil {
			log.Error().Int("vmid", vmidInt).Msg("VM not found")
			http.Error(w, "VM not found", http.StatusNotFound)
			return
		}
	}

	// Get VM config to fetch description and tags
	var tags []string
	var description string
	var networkBridges []string
	// First attempt using vm.Node (fast path)
	cfg, cfgErr := proxmox.GetVMConfigWithContext(r.Context(), client, vm.Node, vm.VMID)
	if cfgErr != nil {
		// Log and attempt a robust fallback by discovering the node
		log.Warn().Err(cfgErr).Str("node", vm.Node).Int("vmid", vm.VMID).Msg("Primary VM config fetch failed, attempting node discovery fallback")
		if nodes, nErr := proxmox.GetNodeNamesWithContext(r.Context(), client); nErr == nil {
			for _, n := range nodes {
				if altCfg, altErr := proxmox.GetVMConfigWithContext(r.Context(), client, n, vm.VMID); altErr == nil {
					cfg = altCfg
					vm.Node = n // update node for subsequent actions
					cfgErr = nil
					log.Info().Str("resolved_node", n).Int("vmid", vm.VMID).Msg("Resolved VM node via fallback and fetched config")
					break
				}
			}
		} else {
			log.Warn().Err(nErr).Msg("Unable to list nodes during VM config fallback")
		}
	}

	if cfgErr == nil && cfg != nil {
		if tagsStr, ok := cfg["tags"].(string); ok && tagsStr != "" {
			parts := strings.Split(tagsStr, ";")
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					tags = append(tags, p)
				}
			}
		}
		if desc, ok := cfg["description"].(string); ok {
			description = desc
		}
		networkBridges = proxmox.ExtractNetworkBridges(cfg)
	} else if cfgErr != nil {
		log.Warn().Err(cfgErr).Int("vmid", vm.VMID).Msg("Unable to fetch VM config; description and tags may be empty")
	}

	// Get CSRF token
	handlerCtx := NewHandlerContext(w, r, "VMDetailsHandler")
	csrfToken, _ := handlerCtx.GetCSRFToken()

	// Check for edit modes
	showDescriptionEditor := r.URL.Query().Get("edit") == "description"
	showTagsEditor := r.URL.Query().Get("edit") == "tags"

	// Get available tags from settings
	settings := stateManager.GetSettings()
	allTags := settings.Tags

	// Format network bridges as string
	networkBridgesStr := ""
	if len(networkBridges) > 0 {
		networkBridgesStr = strings.Join(networkBridges, ", ")
	}

	// Process description as markdown
	descriptionHTML := ""
	if description != "" {
		descriptionHTML = string(markdown.ToHTML([]byte(description), nil, nil))
	}

	// Build custom data for template
	custom := map[string]interface{}{
		"VM":                    vm,
		"Tags":                  strings.Join(tags, ", "),
		"Description":           description,
		"DescriptionHTML":       descriptionHTML,
		"NetworkBridges":        networkBridgesStr,
		"CSRFToken":             csrfToken,
		"ShowDescriptionEditor": showDescriptionEditor,
		"ShowTagsEditor":        showTagsEditor,
		"CurrentTags":           tags,
		"AllTags":               allTags,
		"FormattedMaxMem":       FormatBytes(vm.MaxMem),
		"FormattedMaxDisk":      FormatBytes(vm.MaxDisk),
		"FormattedMem":          FormatBytes(vm.Mem),
		"FormattedUptime":       FormatUptime(vm.Uptime, r),
	}

	// Render using standardized user page helper to include Success/Warning/Error messages
	th := NewTemplateHelpers()
	th.RenderUserPage(w, r, "vm_details", "VM Details", stateManager, custom)
}
