package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// guestAgentCache stores VMs without guest agent to avoid repeated slow API calls
var (
	guestAgentUnavailableCache      = make(map[string]time.Time)
	guestAgentUnavailableCacheMutex sync.RWMutex
	guestAgentIPCache               = make(map[string]guestAgentCacheEntry)
	guestAgentIPCacheMutex          sync.RWMutex
)

// guestAgentCacheEntry stores cached guest agent network information
type guestAgentCacheEntry struct {
	interfaces []proxmox.GuestAgentNetworkInterface
	expiry     time.Time
}

// isGuestAgentUnavailableCached checks if a VM is cached as having no guest agent
func isGuestAgentUnavailableCached(node string, vmid int) bool {
	key := node + ":" + strconv.Itoa(vmid)
	guestAgentUnavailableCacheMutex.RLock()
	defer guestAgentUnavailableCacheMutex.RUnlock()

	if expiry, found := guestAgentUnavailableCache[key]; found {
		if time.Now().Before(expiry) {
			return true
		}
		// Entry expired, will be removed later
	}
	return false
}

// cacheGuestAgentUnavailable marks a VM as having no guest agent
func cacheGuestAgentUnavailable(node string, vmid int) {
	key := node + ":" + strconv.Itoa(vmid)
	guestAgentUnavailableCacheMutex.Lock()
	defer guestAgentUnavailableCacheMutex.Unlock()
	guestAgentUnavailableCache[key] = time.Now().Add(constants.GuestAgentCacheTTL)
}

// getGuestAgentIPsFromCache retrieves cached guest agent network interfaces
func getGuestAgentIPsFromCache(node string, vmid int) ([]proxmox.GuestAgentNetworkInterface, bool) {
	key := node + ":" + strconv.Itoa(vmid)
	guestAgentIPCacheMutex.RLock()
	defer guestAgentIPCacheMutex.RUnlock()

	if entry, found := guestAgentIPCache[key]; found {
		if time.Now().Before(entry.expiry) {
			return entry.interfaces, true
		}
		// Entry expired, will be removed later
	}
	return nil, false
}

// cacheGuestAgentIPs stores guest agent network interfaces in cache
func cacheGuestAgentIPs(node string, vmid int, interfaces []proxmox.GuestAgentNetworkInterface) {
	key := node + ":" + strconv.Itoa(vmid)
	guestAgentIPCacheMutex.Lock()
	defer guestAgentIPCacheMutex.Unlock()

	guestAgentIPCache[key] = guestAgentCacheEntry{
		interfaces: interfaces,
		expiry:     time.Now().Add(constants.GuestAgentCacheTTL),
	}
}

// CleanExpiredGuestAgentCache removes expired entries from both guest agent caches.
// This function is called periodically by the state manager to prevent cache growth.
func CleanExpiredGuestAgentCache() {
	now := time.Now()

	unavailableCount := 0
	ipCount := 0

	// Clean unavailable cache
	guestAgentUnavailableCacheMutex.Lock()
	for key, expiry := range guestAgentUnavailableCache {
		if now.After(expiry) {
			delete(guestAgentUnavailableCache, key)
			unavailableCount++
		}
	}
	unavailableSize := len(guestAgentUnavailableCache)
	guestAgentUnavailableCacheMutex.Unlock()

	// Clean IP cache
	guestAgentIPCacheMutex.Lock()
	for key, entry := range guestAgentIPCache {
		if now.After(entry.expiry) {
			delete(guestAgentIPCache, key)
			ipCount++
		}
	}
	ipSize := len(guestAgentIPCache)
	guestAgentIPCacheMutex.Unlock()

	// Log cleanup results if any entries were removed
	if unavailableCount > 0 || ipCount > 0 {
		logger.Get().Debug().
			Int("unavailable_expired", unavailableCount).
			Int("unavailable_remaining", unavailableSize).
			Int("ip_expired", ipCount).
			Int("ip_remaining", ipSize).
			Msg("Guest agent cache cleanup completed")
	}
}

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

	// VM creation with CSRF protection
	router.POST("/api/vm/create", SecureFormHandler("CreateVM",
		HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			h.CreateVMHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	// VM details and actions routes
	router.GET("/vm/details/:vmid", RequireAuthHandle(h.VMDetailsHandler))

	router.POST("/vm/update/description", SecureFormHandler("UpdateVMDescription",
		RequireAuthHandle(h.UpdateVMDescriptionHandler),
	))
	router.POST("/vm/update/tags", SecureFormHandler("UpdateVMTags",
		RequireAuthHandle(h.UpdateVMTagsHandler),
	))
	router.POST("/vm/update/resources", SecureFormHandler("UpdateVMResources",
		RequireAuthHandle(h.UpdateVMResourcesHandler),
	))
	router.POST("/vm/action", SecureFormHandler("VMAction",
		RequireAuthHandle(h.VMActionHandler),
	))

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

	// Get all VMs and find the one we want using resty
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		http.Error(w, "Failed to create API client", http.StatusInternalServerError)
		return
	}

	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Msg("Failed to get VMs (resty)")
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
		if nodes, err := proxmox.GetNodeNamesResty(r.Context(), restyClient); err == nil {
			for _, n := range nodes {
				if cur, err2 := proxmox.GetVMCurrentResty(r.Context(), restyClient, n, vmidInt); err2 == nil && cur != nil {
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
	var networkInterfaces []proxmox.NetworkInterface
	// First attempt using vm.Node (fast path) with resty
	cfg, cfgErr := proxmox.GetVMConfigResty(r.Context(), restyClient, vm.Node, vm.VMID)
	if cfgErr != nil {
		// Log and attempt a robust fallback by discovering the node
		log.Warn().Err(cfgErr).Str("node", vm.Node).Int("vmid", vm.VMID).Msg("Primary VM config fetch failed, attempting node discovery fallback")
		if nodes, nErr := proxmox.GetNodeNamesResty(r.Context(), restyClient); nErr == nil {
			for _, n := range nodes {
				if altCfg, altErr := proxmox.GetVMConfigResty(r.Context(), restyClient, n, vm.VMID); altErr == nil {
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
		networkInterfaces = proxmox.ExtractNetworkInterfaces(cfg)

		// Try to enrich network interfaces with IP addresses from guest agent (only if VM is running)
		// Use cache-first approach to avoid repeated slow API calls
		if vm.Status == "running" && len(networkInterfaces) > 0 && !isGuestAgentUnavailableCached(vm.Node, vm.VMID) {
			// Try cache first
			if cachedIfaces, found := getGuestAgentIPsFromCache(vm.Node, vm.VMID); found {
				proxmox.EnrichNetworkInterfacesWithIPs(networkInterfaces, cachedIfaces)
				log.Debug().Int("vmid", vm.VMID).Msg("Using cached guest agent network info")
			} else {
				// Cache miss - fetch from API with short timeout
				guestCtx, cancel := context.WithTimeout(r.Context(), constants.GuestAgentTimeout)
				defer cancel()

				if guestIfaces, err := proxmox.GetGuestAgentNetworkInterfaces(guestCtx, client, vm.Node, vm.VMID); err == nil {
					proxmox.EnrichNetworkInterfacesWithIPs(networkInterfaces, guestIfaces)
					// Cache successful result
					cacheGuestAgentIPs(vm.Node, vm.VMID, guestIfaces)
					log.Debug().Int("vmid", vm.VMID).Msg("Fetched and cached guest agent network info")
				} else {
					// Guest agent not available - cache this result to avoid repeated slow calls
					cacheGuestAgentUnavailable(vm.Node, vm.VMID)
					log.Debug().Err(err).Int("vmid", vm.VMID).Msg("Guest agent network info not available (cached unavailability)")
				}
			}
		}
	} else if cfgErr != nil {
		log.Warn().Err(cfgErr).Int("vmid", vm.VMID).Msg("Unable to fetch VM config; description and tags may be empty")
	}

	// Get CSRF token
	handlerCtx := NewHandlerContext(w, r, "VMDetailsHandler")
	csrfToken, _ := handlerCtx.GetCSRFToken()

	// Check for edit modes
	showDescriptionEditor := r.URL.Query().Get("edit") == "description"
	showResourcesEditor := r.URL.Query().Get("edit") == "resources"
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

	// Get available VMBRs for the node when editing resources
	var availableVMBRs []string
	var currentCores = 1
	var currentSockets = 1
	var currentVMBR string
	var currentMemoryMB = vm.MaxMem / 1024 // Convert bytes to MB

	if showResourcesEditor {
		if vmbrs, err := proxmox.GetVMBRsResty(r.Context(), restyClient, vm.Node); err == nil {
			// Convert VMBR structs to strings (iface names)
			for _, vmbr := range vmbrs {
				availableVMBRs = append(availableVMBRs, vmbr.Iface)
			}
			// Get current VMBR from network bridges
			if len(networkBridges) > 0 {
				currentVMBR = networkBridges[0]
			}
		} else {
			log.Warn().Err(err).Str("node", vm.Node).Msg("Failed to get VMBRs for resource editor")
		}

		// Extract sockets and cores from VM config
		if cfg != nil {
			if socketsVal, ok := cfg["sockets"].(float64); ok {
				currentSockets = int(socketsVal)
			}
			if coresVal, ok := cfg["cores"].(float64); ok {
				currentCores = int(coresVal)
			}
		}
	}

	// Build custom data for template
	custom := map[string]interface{}{
		"AllTags":               allTags,
		"AvailableVMBRs":        availableVMBRs,
		"CSRFToken":             csrfToken,
		"CurrentCores":          currentCores,
		"CurrentMemory":         currentMemoryMB,
		"CurrentSockets":        currentSockets,
		"CurrentTags":           tags,
		"CurrentVMBR":           currentVMBR,
		"Description":           description,
		"DescriptionHTML":       descriptionHTML,
		"FormattedMaxDisk":      FormatBytes(vm.MaxDisk),
		"FormattedMaxMem":       FormatBytes(vm.MaxMem),
		"FormattedMem":          FormatBytes(vm.Mem),
		"FormattedUptime":       FormatUptime(vm.Uptime, r),
		"Limits":                settings.Limits,
		"NetworkBridges":        networkBridgesStr,
		"NetworkInterfaces":     networkInterfaces,
		"ShowDescriptionEditor": showDescriptionEditor,
		"ShowResourcesEditor":   showResourcesEditor,
		"ShowTagsEditor":        showTagsEditor,
		"Tags":                  strings.Join(tags, ", "),
		"VM":                    vm,
	}

	// Render using standardized user page helper to include Success/Warning/Error messages
	th := NewTemplateHelpers()
	th.RenderUserPage(w, r, "vm_details", "VM Details", stateManager, custom)
}
