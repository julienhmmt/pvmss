package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/sync/errgroup"

	"pvmss/constants"
	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/state"
)

// VMHandler handles VM-related pages and API endpoints
type VMHandler struct {
	stateManager state.StateManager
}

// NewVMHandler creates a new VMHandler
func NewVMHandler(stateManager state.StateManager) *VMHandler {
	return &VMHandler{stateManager: stateManager}
}

// RegisterRoutes registers VM-related routes
func (h *VMHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/vm/details/:vmid", RequireAuthHandle(h.VMDetailsHandler))

	// API routes for dynamic updates
	router.GET("/api/vm/:vmid/metrics", RequireAuthHandle(h.VMMetricsHandler))

	router.POST("/api/vm/validate/vmid", RequireAuthHandle(h.ValidateVMIDHandler))
	router.POST("/api/vm/validate/name", RequireAuthHandle(h.ValidateVMNameHandler))
	router.POST("/api/vm/validate/vlan_tag", RequireAuthHandle(h.ValidateVLANHandler))

	router.POST("/vm/update/description", SecureFormHandler("UpdateVMDescription",
		RequireAuthHandle(h.UpdateVMDescriptionHandler),
	))
	router.POST("/vm/update/tags", SecureFormHandler("UpdateVMTags",
		RequireAuthHandle(h.UpdateVMTagsHandler),
	))
	router.POST("/vm/update/resources", SecureFormHandler("UpdateVMResources",
		RequireAuthHandle(h.UpdateVMResourcesHandler),
	))
	router.POST("/vm/toggle/network", SecureFormHandler("ToggleNetworkCard",
		RequireAuthHandle(h.ToggleNetworkCardHandler),
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

// VMStateManager defines the minimal state contract needed by VM details.
// Provides access to Proxmox client and application settings.
type VMStateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
	GetSettings() *state.AppSettings
	GetProxmoxStatus() (bool, string)
}

// VMHandler handles VM-related pages and API endpoints
// TODO Telmate migration: this handler still relies on Telmate-based helpers (guest agent data, cache invalidation). Replace them with Resty-based helpers and drop the Telmate cache.
func (h *VMHandler) VMDetailsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("VMDetailsHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodGet) {
		return
	}

	vmid := ps.ByName("vmid")
	if vmid == "" {
		log.Error().Msg("VM ID is required")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.MissingRequiredFields"), http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.Generic"), http.StatusBadRequest)
		return
	}

	stateManager := getStateManager(r)
	client := stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ProxmoxClientUnavailable"), http.StatusServiceUnavailable)
		return
	}

	if r.URL.Query().Get("refresh") == "1" {
		client.InvalidateCache("/nodes")
		if nodes, err := proxmox.GetNodeNamesWithContext(r.Context(), client); err == nil {
			for _, n := range nodes {
				client.InvalidateCache("/nodes/" + n + "/qemu")
			}
		} else {
			log.Warn().
				Err(err).
				Str("component", "vm_details").
				Str("operation", "invalidate_cache_refresh").
				Str("reason", "nodes_fetch_failed").
				Msg("Unable to get nodes while invalidating cache for refresh")
		}
	}

	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ServerConfigError"), http.StatusInternalServerError)
		return
	}

	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Msg("Failed to get VMs (resty)")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.FailedToGetResources"), http.StatusInternalServerError)
		return
	}

	var vm *proxmox.VM
	for i := range vms {
		if vms[i].VMID == vmidInt {
			vm = &vms[i]
			break
		}
	}

	if vm == nil {
		if nodes, err := proxmox.GetNodeNamesResty(r.Context(), restyClient); err == nil {
			g, ctx := errgroup.WithContext(r.Context())
			var mu sync.Mutex
			for _, n := range nodes {
				node := n
				g.Go(func() error {
					cur, err2 := proxmox.GetVMCurrentResty(ctx, restyClient, node, vmidInt)
					if err2 != nil || cur == nil {
						return nil
					}
					mu.Lock()
					if vm == nil {
						vm = &proxmox.VM{
							VMID:   vmidInt,
							Node:   node,
							Name:   cur.Name,
							Status: cur.Status,
							CPUs:   cur.CPUs,
							MaxMem: cur.MaxMem,
							Mem:    cur.Mem,
						}
					}
					mu.Unlock()
					return nil
				})
			}
			_ = g.Wait()
		} else {
			log.Warn().
				Err(err).
				Str("component", "vm_details").
				Str("operation", "vm_fallback_lookup").
				Str("reason", "nodes_fetch_failed").
				Msg("Unable to get nodes for VM fallback lookup")
		}

		if vm == nil {
			log.Error().Int("vmid", vmidInt).Msg("VM not found")
			http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.NotFound"), http.StatusNotFound)
			return
		}
	}

	var description string
	var efiEnabled bool
	var efiStorage string
	var networkBridges []string
	var networkInterfaces []proxmox.NetworkInterface
	var tags []string
	var tpmEnabled bool
	var currentISO string
	var hasCDROM bool
	cloudInitData := map[string]string{
		"user":       "",
		"sshKeys":    "",
		"ipConfig":   "",
		"nameserver": "",
		"cicustom":   "",
	}
	cloudInitEnabled := false
	cfg, cfgErr := proxmox.GetVMConfigResty(r.Context(), restyClient, vm.Node, vm.VMID)
	if cfgErr != nil {
		log.Warn().
			Err(cfgErr).
			Str("component", "vm_details").
			Str("operation", "fetch_vm_config").
			Str("reason", "primary_fetch_failed").
			Str("node", vm.Node).
			Int("vmid", vm.VMID).
			Msg("Primary VM config fetch failed, attempting node discovery fallback")
		if nodes, nErr := proxmox.GetNodeNamesResty(r.Context(), restyClient); nErr == nil {
			g, ctx := errgroup.WithContext(r.Context())
			var mu2 sync.Mutex
			for _, n := range nodes {
				node := n
				g.Go(func() error {
					if altCfg, altErr := proxmox.GetVMConfigResty(ctx, restyClient, node, vm.VMID); altErr == nil && altCfg != nil {
						mu2.Lock()
						if cfgErr != nil { // still unresolved
							cfg = altCfg
							vm.Node = node
							cfgErr = nil
						}
						mu2.Unlock()
					}
					return nil
				})
			}
			_ = g.Wait()
			if cfgErr == nil {
				log.Info().Str("resolved_node", vm.Node).Int("vmid", vm.VMID).Msg("Resolved VM node via fallback and fetched config")
			}
		} else {
			log.Warn().
				Err(nErr).
				Str("component", "vm_details").
				Str("operation", "vm_config_fallback").
				Str("reason", "node_list_failed").
				Msg("Unable to list nodes during VM config fallback")
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

		// Detect EFI
		if bios, ok := cfg["bios"].(string); ok && strings.ToLower(strings.TrimSpace(bios)) == "ovmf" {
			efiEnabled = true
		}
		if rawEFI, ok := cfg["efidisk0"].(string); ok && strings.TrimSpace(rawEFI) != "" {
			efiEnabled = true
			first := rawEFI
			if idx := strings.Index(rawEFI, ","); idx >= 0 {
				first = rawEFI[:idx]
			}
			if parts := strings.SplitN(first, ":", 2); len(parts) == 2 {
				efiStorage = strings.TrimSpace(parts[0])
			}
		}

		// Detect TPM
		if rawTPM, ok := cfg["tpmstate0"].(string); ok && strings.TrimSpace(rawTPM) != "" {
			tpmEnabled = true
		}

		// Detect CD-ROM ISO (ide2) - All VMs have CD-ROM by default in PVMSS
		if ide2, ok := cfg["ide2"].(string); ok && strings.TrimSpace(ide2) != "" {
			hasCDROM = true
			// Parse ide2 format: "storage:iso,media=cdrom"
			parts := strings.SplitN(ide2, ":", 2)
			if len(parts) == 2 {
				isoPart := strings.TrimSpace(parts[1])
				// Remove ",media=cdrom" suffix if present
				if idx := strings.Index(isoPart, ",media=cdrom"); idx >= 0 {
					isoPart = strings.TrimSpace(isoPart[:idx])
				}
				currentISO = isoPart
			}
		} else {
			// No ide2 config means VM has empty CD-ROM drive
			hasCDROM = true
		}

		// Extract cloud-init configuration from main VM config
		if ciuser, ok := cfg["ciuser"].(string); ok && ciuser != "" {
			cloudInitData["user"] = ciuser
			cloudInitEnabled = true
		}
		if sshkeys, ok := cfg["sshkeys"].(string); ok && sshkeys != "" {
			// Decode URL-encoded SSH keys (Proxmox stores them URL-encoded)
			decodedKeys, err := url.QueryUnescape(sshkeys)
			if err != nil {
				// If decoding fails, use the original value
				decodedKeys = sshkeys
			}
			cloudInitData["sshKeys"] = decodedKeys
			cloudInitEnabled = true
		}
		if ipconfig0, ok := cfg["ipconfig0"].(string); ok && ipconfig0 != "" {
			cloudInitData["ipConfig"] = ipconfig0
			cloudInitEnabled = true
		}
		if nameserver, ok := cfg["nameserver"].(string); ok && nameserver != "" {
			cloudInitData["nameserver"] = nameserver
			cloudInitEnabled = true
		}
		if cicustom, ok := cfg["cicustom"].(string); ok && cicustom != "" {
			cloudInitData["cicustom"] = cicustom
			cloudInitEnabled = true
			// Try to find matching template from cicustom
			// New format: user=<storage>:snippets/pvmss-<vm_name>-<template_id>.yml
			// Old format: user=<storage>:snippets/pvmss-<template_id>.yml
			if strings.Contains(cicustom, "pvmss-") && stateManager.GetSettings() != nil {
				templates := stateManager.GetSettings().CloudInitTemplates
				for _, template := range templates {
					// Check if the cicustom path ends with the template ID
					if strings.Contains(cicustom, "-"+template.ID+".yml") || strings.Contains(cicustom, "-"+template.ID+".yaml") {
						cloudInitData["templateName"] = template.Name
						cloudInitData["templateYAML"] = template.YAMLContent
						break
					}
				}
			}
		}
		// Check for cloud-init drive (ide2=<storage>:cloudinit or similar)
		for key, val := range cfg {
			if strVal, ok := val.(string); ok && strings.Contains(strVal, ":cloudinit") {
				cloudInitEnabled = true
				log.Debug().Str("drive", key).Str("value", strVal).Msg("Cloud-init drive detected")
				break
			}
		}

		if vm.Status == "running" && len(networkInterfaces) > 0 && !isGuestAgentUnavailableCached(vm.Node, vm.VMID) {
			if cachedIfaces, found := getGuestAgentIPsFromCache(vm.Node, vm.VMID); found {
				proxmox.EnrichNetworkInterfacesWithIPs(networkInterfaces, cachedIfaces)
				log.Debug().
					Int("vmid", vm.VMID).
					Str("component", "vm_details").
					Str("operation", "guest_agent_network").
					Str("reason", "cache_hit").
					Msg("Using cached guest agent network info")
			} else {
				guestCtx, cancel := context.WithTimeout(r.Context(), constants.GuestAgentTimeout)
				defer cancel()
				if guestIfaces, err := proxmox.GetGuestAgentNetworkInterfaces(guestCtx, client, vm.Node, vm.VMID); err == nil {
					proxmox.EnrichNetworkInterfacesWithIPs(networkInterfaces, guestIfaces)
					cacheGuestAgentIPs(vm.Node, vm.VMID, guestIfaces)
					log.Debug().
						Int("vmid", vm.VMID).
						Str("component", "vm_details").
						Str("operation", "guest_agent_network").
						Str("reason", "fetch_success").
						Msg("Fetched and cached guest agent network info")
				} else {
					cacheGuestAgentUnavailable(vm.Node, vm.VMID)
					log.Debug().
						Err(err).
						Int("vmid", vm.VMID).
						Str("component", "vm_details").
						Str("operation", "guest_agent_network").
						Str("reason", "unavailable_cached").
						Msg("Guest agent network info not available (cached unavailability)")
				}
			}
		}
	} else if cfgErr != nil {
		log.Warn().
			Err(cfgErr).
			Str("component", "vm_details").
			Str("operation", "fetch_vm_config").
			Str("reason", "config_fetch_failed").
			Int("vmid", vm.VMID).
			Msg("Unable to fetch VM config; description and tags may be empty")
	}

	handlerCtx := NewHandlerContext(w, r, "VMDetailsHandler")
	csrfToken, _ := handlerCtx.GetCSRFToken()

	showDescriptionEditor := r.URL.Query().Get("edit") == "description"
	showResourcesEditor := r.URL.Query().Get("edit") == "resources"
	showTagsEditor := r.URL.Query().Get("edit") == "tags"
	isNewlyCreated := r.URL.Query().Get("created") == "1"

	settings := stateManager.GetSettings()
	var allTags []string
	if settings != nil && settings.Tags != nil {
		allTags = settings.Tags
	} else {
		allTags = []string{}
	}

	networkBridgesStr := ""
	if len(networkBridges) > 0 {
		networkBridgesStr = strings.Join(networkBridges, ", ")
	}

	descriptionHTML := ""
	if description != "" {
		descriptionHTML = string(markdown.ToHTML([]byte(description), nil, nil))
	}

	var availableVMBRs []string
	availableVMBRSet := make(map[string]struct{})
	var currentCores = 1
	var currentSockets = 1
	var currentVMBR string
	var currentMemoryMB = vm.MaxMem / (1024 * 1024)

	// Compute explicit VM RAM limits for template (MB) from settings
	// If settings are missing, fall back to current VM memory to avoid hardcoded constants
	vmRamMinMB, vmRamMaxMB := 0, 0
	if settings != nil {
		// Stored in GB in settings, convert to MB
		vmRamMinMB = settings.Limits.VM.RAM.Min * 1024
		vmRamMaxMB = settings.Limits.VM.RAM.Max * 1024
	}
	// Fallback without hardcoded values: constrain to current memory
	if vmRamMinMB <= 0 {
		vmRamMinMB = int(currentMemoryMB)
	}
	if vmRamMaxMB <= 0 {
		vmRamMaxMB = int(currentMemoryMB)
	}

	maxNetworkCards := state.MinNetworkCards
	if settings != nil {
		maxNetworkCards = settings.MaxNetworkCards
	}
	if maxNetworkCards <= 0 {
		maxNetworkCards = state.MinNetworkCards
	}
	networkCardsData := buildNetworkCardsData(cfg, maxNetworkCards)

	currentNetworkModel := networkCardsData[0].Model
	if currentNetworkModel == "" {
		currentNetworkModel = "virtio"
	}
	currentVMBR = networkCardsData[0].Bridge
	if currentVMBR == "" && len(networkBridges) > 0 {
		currentVMBR = networkBridges[0]
	}

	if showResourcesEditor {
		if vmbrs, err := proxmox.GetVMBRsResty(r.Context(), restyClient, vm.Node); err == nil {
			for _, vmbr := range vmbrs {
				iface := vmbr.Iface
				if _, exists := availableVMBRSet[iface]; !exists {
					availableVMBRSet[iface] = struct{}{}
					availableVMBRs = append(availableVMBRs, iface)
				}
			}
			if currentVMBR == "" && len(availableVMBRs) > 0 {
				currentVMBR = availableVMBRs[0]
			}
		} else {
			log.Warn().
				Err(err).
				Str("component", "vm_details").
				Str("operation", "fetch_vmbrs").
				Str("reason", "vmbrs_fetch_failed").
				Str("node", vm.Node).
				Msg("Failed to get VMBRs for resource editor")
		}
	}

	// Get available ISOs for CD-ROM editor - Use admin-approved ISOs from settings
	var availableISOs []string
	if showResourcesEditor {
		if settings != nil {
			// Use only ISOs approved by administrators from settings.json
			availableISOs = settings.ISOs
			log.Debug().
				Int("iso_count", len(availableISOs)).
				Str("component", "vm_details").
				Str("operation", "fetch_isos").
				Str("reason", "admin_approved").
				Msg("Using admin-approved ISOs from settings")
		} else {
			log.Warn().
				Str("component", "vm_details").
				Str("operation", "fetch_isos").
				Str("reason", "settings_unavailable").
				Msg("Settings not available, no ISOs will be shown")
		}
	}

	if cfg != nil {
		if socketsVal, ok := cfg["sockets"].(float64); ok {
			currentSockets = int(socketsVal)
		}
		if coresVal, ok := cfg["cores"].(float64); ok {
			currentCores = int(coresVal)
		}
	}

	agentStatusKey := "Unknown"
	agentStatusClass := "is-light"
	if stateManager != nil {
		if connected, _ := stateManager.GetProxmoxStatus(); !connected {
			agentStatusKey = "Offline"
			agentStatusClass = "is-warning is-light"
		} else if vm.Status == "running" {
			if cachedIfaces, found := getGuestAgentIPsFromCache(vm.Node, vm.VMID); found && len(cachedIfaces) > 0 {
				agentStatusKey = "Available"
				agentStatusClass = "is-success is-light"
			} else if isGuestAgentUnavailableCached(vm.Node, vm.VMID) {
				agentStatusKey = "Unavailable"
				agentStatusClass = "is-warning is-light"
			} else {
				// No cached data, check guest agent status in real-time
				log.Debug().
					Int("vmid", vm.VMID).
					Str("node", vm.Node).
					Str("component", "vm_details").
					Str("operation", "guest_agent_status").
					Str("reason", "realtime_check").
					Msg("Performing real-time guest agent status check (no cached data)")
				guestCtx, cancel := context.WithTimeout(r.Context(), constants.GuestAgentTimeout)
				defer cancel()
				if _, err := proxmox.GetGuestAgentNetworkInterfaces(guestCtx, client, vm.Node, vm.VMID); err == nil {
					log.Debug().
						Int("vmid", vm.VMID).
						Str("node", vm.Node).
						Str("component", "vm_details").
						Str("operation", "guest_agent_status").
						Str("reason", "realtime_success").
						Msg("Real-time guest agent check succeeded")
					agentStatusKey = "Available"
					agentStatusClass = "is-success is-light"
				} else {
					log.Debug().
						Err(err).
						Int("vmid", vm.VMID).
						Str("node", vm.Node).
						Str("component", "vm_details").
						Str("operation", "guest_agent_status").
						Str("reason", "realtime_failed").
						Msg("Real-time guest agent check failed")
					// Cache the unavailability to avoid repeated checks
					cacheGuestAgentUnavailable(vm.Node, vm.VMID)
					agentStatusKey = "Unavailable"
					agentStatusClass = "is-warning is-light"
				}
			}
		}
	}

	for _, card := range networkCardsData {
		if card.Bridge != "" {
			if _, exists := availableVMBRSet[card.Bridge]; !exists {
				availableVMBRSet[card.Bridge] = struct{}{}
				availableVMBRs = append(availableVMBRs, card.Bridge)
			}
		}
	}

	// Build disks visualization data
	disksData := buildDisksData(cfg)
	totalGB := 0
	for _, d := range disksData {
		if d.SizeGB > 0 {
			totalGB += d.SizeGB
		}
	}
	segments := make([]diskSegmentTemplate, 0, len(disksData))
	for _, d := range disksData {
		if d.SizeGB <= 0 || totalGB == 0 {
			continue
		}
		segments = append(segments, diskSegmentTemplate{
			Index:     d.Index,
			Storage:   d.Storage,
			Bus:       d.Bus,
			SizeGB:    d.SizeGB,
			Percent:   (float64(d.SizeGB) / float64(totalGB)) * 100.0,
			Color:     busColor(d.Bus),
			SizeLabel: formatSizeLabelGB(d.SizeGB),
		})
	}
	totalLabel := formatSizeLabelGB(totalGB)

	// Build bus legend info
	busSet := make(map[string]struct{})
	busNames := make([]string, 0, 4)
	for _, seg := range segments {
		if seg.Bus == "" {
			continue
		}
		if _, ok := busSet[seg.Bus]; !ok {
			busSet[seg.Bus] = struct{}{}
			busNames = append(busNames, seg.Bus)
		}
	}
	busNamesStr := strings.Join(busNames, ", ")

	// Ensure vm is non-nil before dereferencing in the template data
	if vm == nil {
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.NotFound"), http.StatusNotFound)
		return
	}

	custom := map[string]interface{}{
		"AllTags":               allTags,
		"AvailableVMBRs":        availableVMBRs,
		"AvailableISOs":         availableISOs,
		"CSRFToken":             csrfToken,
		"CloudInitEnabled":      cloudInitEnabled,
		"CloudInitData":         cloudInitData,
		"CurrentCores":          currentCores,
		"CurrentISO":            currentISO,
		"CurrentMemory":         currentMemoryMB,
		"CurrentNetworkModel":   currentNetworkModel,
		"CurrentSockets":        currentSockets,
		"CurrentTags":           tags,
		"CurrentVMBR":           currentVMBR,
		"Description":           description,
		"DescriptionHTML":       descriptionHTML,
		"Disks":                 disksData,
		"DisksTotalGB":          totalGB,
		"DisksTotalLabel":       totalLabel,
		"DiskSegments":          segments,
		"DiskBusCount":          len(busNames),
		"DiskBusNamesString":    busNamesStr,
		"EFIEnabled":            efiEnabled,
		"EFIStorage":            efiStorage,
		"FormattedMaxDisk":      FormatBytes(vm.MaxDisk),
		"FormattedMaxMem":       FormatBytes(vm.MaxMem),
		"FormattedMaxMemGB":     FormatMemoryGB(vm.MaxMem, true),
		"FormattedMem":          FormatBytes(vm.Mem),
		"FormattedMemGB":        FormatMemoryGB(vm.Mem, true),
		"FormattedUptime":       FormatUptime(vm.Uptime, r),
		"HasCDROM":              hasCDROM,
		"IsNewlyCreated":        isNewlyCreated,
		"Limits":                settings.Limits,
		"VMRamMinMB":            vmRamMinMB,
		"VMRamMaxMB":            vmRamMaxMB,
		"MaxNetworkCards":       maxNetworkCards,
		"NetworkBridges":        networkBridgesStr,
		"NetworkCards":          networkCardsData,
		"NetworkInterfaces":     networkInterfaces,
		"AgentStatusKey":        agentStatusKey,
		"AgentStatusClass":      agentStatusClass,
		"ShowDescriptionEditor": showDescriptionEditor,
		"ShowResourcesEditor":   showResourcesEditor,
		"ShowTagsEditor":        showTagsEditor,
		"TPMEnabled":            tpmEnabled,
		"Tags":                  strings.Join(tags, ", "),
		"VM":                    vm,
	}

	title := ""
	idLabel := "ID"
	if handlerCtx != nil {
		idLabel = handlerCtx.Translate("Common.ID")
	}
	if vm.Name != "" && vm.VMID > 0 {
		title = fmt.Sprintf("%s (%s %d)", vm.Name, idLabel, vm.VMID)
	} else if vm.VMID > 0 {
		title = fmt.Sprintf("%s %d", idLabel, vm.VMID)
	} else if vm.Name != "" {
		title = vm.Name
	}
	if title == "" {
		if handlerCtx != nil {
			title = handlerCtx.Translate("VMDetails.EditResourcesTitle")
		} else {
			title = "VM Details"
		}
	}

	th := NewTemplateHelpers()
	th.RenderUserPage(w, r, "vm_details", title, stateManager, custom)
}

// VMMetricsHandler returns VM metrics as JSON for dynamic updates
func (h *VMHandler) VMMetricsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("VMMetricsHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodGet) {
		return
	}

	vmid := ps.ByName("vmid")
	if vmid == "" {
		log.Error().Msg("VM ID is required")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.MissingRequiredFields"), http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.Generic"), http.StatusBadRequest)
		return
	}

	// Get Proxmox client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ProxmoxClientUnavailable"), http.StatusServiceUnavailable)
		return
	}

	// Get resty client for API calls
	restyClient, err := proxmox.NewRestyClientFromEnv(30 * time.Second)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ServerConfigError"), http.StatusInternalServerError)
		return
	}

	// Resolve node for VMID
	node := ""
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err == nil {
		for _, vm := range vms {
			if vm.VMID == vmidInt {
				node = vm.Node
				break
			}
		}
	}

	if node == "" {
		// Fallback: try to find node by iterating all nodes
		if nodes, err := proxmox.GetNodeNamesResty(r.Context(), restyClient); err == nil {
			for _, n := range nodes {
				if status, err := proxmox.GetVMCurrentResty(r.Context(), restyClient, n, vmidInt); err == nil && status != nil {
					node = n
					break
				}
			}
		}
	}

	if node == "" {
		log.Error().Int("vmid", vmidInt).Msg("VM not found on any node")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.NotFound"), http.StatusNotFound)
		return
	}

	// Get current VM status and metrics
	vmCurrent, err := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmidInt)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Str("node", node).Msg("Failed to get VM metrics")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.FailedToGetResources"), http.StatusInternalServerError)
		return
	}

	// Prepare response
	metrics := map[string]interface{}{
		"status": vmCurrent.Status,
		"cpu":    vmCurrent.CPU,
		"mem":    vmCurrent.Mem,
		"maxmem": vmCurrent.MaxMem,
	}

	// Set JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Error().Err(err).Msg("Failed to encode metrics response")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("vmid", vmidInt).
		Str("status", vmCurrent.Status).
		Float64("cpu", vmCurrent.CPU).
		Int64("mem", vmCurrent.Mem).
		Str("component", "vm_details").
		Str("operation", "serve_metrics").
		Str("reason", "metrics_served").
		Msg("VM metrics served")
}

// ValidationRequest represents a validation request payload
type ValidationRequest struct {
	Value string `json:"value"`
	Node  string `json:"node"`
	Pool  string `json:"pool"`
}

// ValidationResponse represents a validation response
type ValidationResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// ValidateVMIDHandler validates VM ID uniqueness
func (h *VMHandler) ValidateVMIDHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ValidateVMIDHandler", r)
	localizer := i18n.GetLocalizerFromRequest(r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode validation request")
		w.WriteHeader(http.StatusBadRequest)
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.InvalidRequest")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	vmidStr := strings.TrimSpace(req.Value)
	if vmidStr == "" {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMIDRequired")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// Validate format
	vmidInt, err := strconv.Atoi(vmidStr)
	if err != nil || vmidInt <= 0 || vmidInt > 999999999 {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMIDRange")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// Get Proxmox client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "Proxmox.ConnectionError")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// Check if VM ID already exists
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "Error.InternalServer")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// Try to get VM config - if it exists, VM ID is taken
	_, err = proxmox.GetVMConfigResty(ctx, restyClient, req.Node, vmidInt)
	if err == nil {
		// VM exists
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMIDExists")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// VM ID is available
	if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: true, Message: i18n.Localize(localizer, "VM.Create.Validation.VMIDAvailable")}); encodeErr != nil {
		log.Error().Err(encodeErr).Msg("Failed to encode error response")
	}
}

// ValidateVMNameHandler validates VM name
func (h *VMHandler) ValidateVMNameHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ValidateVMNameHandler", r)
	localizer := i18n.GetLocalizerFromRequest(r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode validation request")
		w.WriteHeader(http.StatusBadRequest)
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: "Invalid request"}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	name := strings.TrimSpace(req.Value)
	if name == "" {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMNameRequired")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// Validate length
	if len(name) < 1 || len(name) > 100 {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMNameLength")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// Validate format (basic - no special characters that could cause issues)
	if strings.ContainsAny(name, "<>\"'&") {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMNameInvalidChars")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// For now, just validate format - uniqueness check would require scanning all VMs
	// which could be expensive. We can add that later if needed.
	if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: true, Message: i18n.Localize(localizer, "VM.Create.Validation.VMNameValid")}); encodeErr != nil {
		log.Error().Err(encodeErr).Msg("Failed to encode error response")
	}
}

// ValidateVLANHandler validates VLAN tag values (1-4096)
func (h *VMHandler) ValidateVLANHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ValidateVLANHandler", r)
	localizer := i18n.GetLocalizerFromRequest(r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode validation request")
		w.WriteHeader(http.StatusBadRequest)
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: "Invalid request"}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	vlanStr := strings.TrimSpace(req.Value)
	if vlanStr == "" {
		// Empty VLAN is valid (optional field)
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: true, Message: i18n.Localize(localizer, "VM.Create.Validation.VLANValid")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// Check if value is numeric
	vlanID, err := strconv.Atoi(vlanStr)
	if err != nil {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VLANNumeric")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// Validate VLAN range (1-4096)
	if vlanID < 1 || vlanID > 4096 {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VLANRange")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	// VLAN is valid
	if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: true, Message: i18n.Localize(localizer, "VM.Create.Validation.VLANValid")}); encodeErr != nil {
		log.Error().Err(encodeErr).Msg("Failed to encode error response")
	}
}
