package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
	"pvmss/state"
)

// VMStateManager defines the minimal state contract needed by VM details.
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

	// VM action handlers
	router.POST("/vm/update-description", h.UpdateVMDescriptionHandler)
	router.POST("/vm/update-tags", h.UpdateVMTagsHandler)
	router.POST("/vm/action", h.VMActionHandler)
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
		log.Error().Int("vmid", vmidInt).Msg("VM not found")
		http.Error(w, "VM not found", http.StatusNotFound)
		return
	}

	// Get VM config to fetch description and tags
	var tags []string
	if cfg, err := proxmox.GetVMConfigWithContext(r.Context(), client, vm.Node, vm.VMID); err == nil {
		if tagsStr, ok := cfg["tags"].(string); ok && tagsStr != "" {
			parts := strings.Split(tagsStr, ";")
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					tags = append(tags, p)
				}
			}
		}
	}

	// Render template
	data := map[string]interface{}{
		"Title":  "VM Details",
		"VM":     vm,
		"VMTags": tags,
	}

	renderTemplateInternal(w, r, "vm_details", data)
}
