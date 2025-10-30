package handlers

import (
	"context"
	"fmt"
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

// VMHandler handles VM-related pages and API endpoints
type VMHandler struct {
	stateManager state.StateManager
}

// diskSegmentTemplate is used by the UI to render a stacked bar per disk
type diskSegmentTemplate struct {
	Index     string
	Storage   string
	Bus       string
	SizeGB    int
	Percent   float64
	Color     string
	SizeLabel string
}

func busColor(bus string) string {
	switch bus {
	case "virtio":
		return "#f80" // primary orange
	case "scsi":
		return "#48c774" // success green
	case "sata":
		return "#ffdd57" // warning yellow
	case "ide":
		return "#7a7a7a" // grey
	default:
		return "#b5b5b5"
	}
}

func formatSizeLabelGB(sizeGB int) string {
	if sizeGB >= 1024 {
		tb := float64(sizeGB) / 1024.0
		// format with max 1 decimal when not integer
		if sizeGB%1024 == 0 {
			return fmt.Sprintf("%d TB", sizeGB/1024)
		}
		return fmt.Sprintf("%.1f TB", tb)
	}
	return fmt.Sprintf("%d GB", sizeGB)
}

// NewVMHandler creates a new VMHandler
func NewVMHandler(stateManager state.StateManager) *VMHandler {
	return &VMHandler{stateManager: stateManager}
}

// RegisterRoutes registers VM-related routes
func (h *VMHandler) RegisterRoutes(router *httprouter.Router) {
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

// containsString checks if target exists in items
func containsString(items []string, target string) bool {
	for _, it := range items {
		if it == target {
			return true
		}
	}
	return false
}

// Network cards display helpers
type networkCardTemplateData struct {
	Index    string
	Bridge   string
	Model    string
	MAC      string
	Exists   bool
	Options  []string
	LinkDown bool // true = disabled, false = enabled
}

var networkModelKeys = []string{"virtio", "e1000", "e1000e", "rtl8139", "vmxnet3"}

func parseNetworkConfig(raw string) (model, mac, bridge string, options []string, linkDown bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			key := strings.TrimSpace(kv[0])
			value := ""
			if len(kv) > 1 {
				value = strings.TrimSpace(kv[1])
			}
			switch {
			case key == "model":
				model = value
			case key == "bridge":
				bridge = value
			case key == "link_down":
				linkDown = (value == "1" || value == "true")
			case containsString(networkModelKeys, key):
				model = key
				mac = strings.ToUpper(value)
			default:
				options = append(options, part)
			}
		} else if containsString(networkModelKeys, part) {
			model = part
		} else if part == "link_down" {
			linkDown = true
		} else {
			options = append(options, part)
		}
	}
	if model == "" {
		model = "virtio"
	}
	return
}

// diskTemplateData represents a single disk entry for the template
type diskTemplateData struct {
	Index     string // e.g., virtio0, scsi1
	Bus       string // virtio, scsi, sata, ide
	Number    int    // index number
	Storage   string // storage name before ':'
	SizeGB    int    // size in GB if parseable, 0 otherwise
	SizeLabel string // human label GB/TB
	Color     string // color derived from bus
	Raw       string // raw value from config
	Exists    bool
}

// buildDisksData extracts disk definitions from VM config for all supported buses
func buildDisksData(cfg map[string]interface{}) []diskTemplateData {
	if cfg == nil {
		return nil
	}
	// Bus order for display (VirtIO first as it's the default and most common)
	busOrder := []string{state.DiskBusVirtIO, state.DiskBusSCSI, state.DiskBusSATA, state.DiskBusIDE}

	disks := make([]diskTemplateData, 0, 8)
	for _, bus := range busOrder {
		max := state.GetMaxDisksForBus(bus)
		for i := 0; i < max; i++ {
			key := fmt.Sprintf("%s%d", bus, i)
			raw := ""
			if v, ok := cfg[key].(string); ok {
				raw = strings.TrimSpace(v)
			}
			if raw == "" {
				continue
			}
			// Skip CD-ROM drives (e.g., ide2: <iso>,media=cdrom)
			if strings.Contains(strings.ToLower(raw), "media=cdrom") {
				continue
			}
			storage := ""
			sizeGB := 0
			first := raw
			if idx := strings.Index(raw, ","); idx >= 0 {
				first = raw[:idx]
			}
			if parts := strings.SplitN(first, ":", 2); len(parts) == 2 {
				storage = strings.TrimSpace(parts[0])
				if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					sizeGB = n
				}
			}
			// Try to parse size option like size=20G, size=512M, size=1T
			if sizeGB == 0 && strings.Contains(raw, "size=") {
				for _, seg := range strings.Split(raw, ",") {
					seg = strings.TrimSpace(seg)
					if strings.HasPrefix(seg, "size=") {
						val := strings.TrimPrefix(seg, "size=")
						val = strings.TrimSpace(val)
						// Normalize suffix
						suffix := ""
						numStr := val
						if len(val) > 0 {
							last := val[len(val)-1]
							if last == 'G' || last == 'g' || last == 'M' || last == 'm' || last == 'T' || last == 't' {
								suffix = strings.ToUpper(string(last))
								numStr = strings.TrimSpace(val[:len(val)-1])
							}
						}
						if n, err := strconv.ParseFloat(numStr, 64); err == nil {
							switch suffix {
							case "M":
								sizeGB = int(n / 1024.0)
								if sizeGB == 0 && n > 0 {
									sizeGB = 1
								}
							case "T":
								sizeGB = int(n * 1024.0)
							default: // "" or G
								sizeGB = int(n)
							}
						}
						break
					}
				}
			}
			disks = append(disks, diskTemplateData{
				Index:     key,
				Bus:       bus,
				Number:    i,
				Storage:   storage,
				SizeGB:    sizeGB,
				SizeLabel: formatSizeLabelGB(sizeGB),
				Color:     busColor(bus),
				Raw:       raw,
				Exists:    true,
			})
		}
	}
	return disks
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

func buildNetworkCardsData(cfg map[string]interface{}, maxCards int) []networkCardTemplateData {
	if maxCards <= 0 {
		maxCards = 1
	}
	cards := make([]networkCardTemplateData, maxCards)
	for i := 0; i < maxCards; i++ {
		key := fmt.Sprintf("net%d", i)
		rawVal := ""
		if cfg != nil {
			if netVal, ok := cfg[key].(string); ok {
				rawVal = netVal
			}
		}
		model, mac, bridge, opts, linkDown := parseNetworkConfig(rawVal)
		cards[i] = networkCardTemplateData{
			Index:    key,
			Bridge:   bridge,
			Model:    model,
			MAC:      mac,
			Exists:   rawVal != "",
			Options:  opts,
			LinkDown: linkDown,
		}
	}
	return cards
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

// InvalidateGuestAgentCache removes a specific VM's guest agent cache entries
// This should be called when VM configuration changes (e.g., network model update)
func InvalidateGuestAgentCache(node string, vmid int) {
	key := node + ":" + strconv.Itoa(vmid)

	// Remove from unavailable cache
	guestAgentUnavailableCacheMutex.Lock()
	delete(guestAgentUnavailableCache, key)
	guestAgentUnavailableCacheMutex.Unlock()

	// Remove from IP cache
	guestAgentIPCacheMutex.Lock()
	delete(guestAgentIPCache, key)
	guestAgentIPCacheMutex.Unlock()

	logger.Get().Debug().Str("node", node).Int("vmid", vmid).Msg("Guest agent cache invalidated for VM")
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

	if r.URL.Query().Get("refresh") == "1" {
		client.InvalidateCache("/nodes")
		if nodes, err := proxmox.GetNodeNamesWithContext(r.Context(), client); err == nil {
			for _, n := range nodes {
				client.InvalidateCache("/nodes/" + n + "/qemu")
			}
		} else {
			log.Warn().Err(err).Msg("Unable to get nodes while invalidating cache for refresh")
		}
	}

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

	var vm *proxmox.VM
	for i := range vms {
		if vms[i].VMID == vmidInt {
			vm = &vms[i]
			break
		}
	}

	if vm == nil {
		if nodes, err := proxmox.GetNodeNamesResty(r.Context(), restyClient); err == nil {
			for _, n := range nodes {
				if cur, err2 := proxmox.GetVMCurrentResty(r.Context(), restyClient, n, vmidInt); err2 == nil && cur != nil {
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

	var description string
	var efiEnabled bool
	var efiStorage string
	var networkBridges []string
	var networkInterfaces []proxmox.NetworkInterface
	var tags []string
	var tpmEnabled bool
	cfg, cfgErr := proxmox.GetVMConfigResty(r.Context(), restyClient, vm.Node, vm.VMID)
	if cfgErr != nil {
		log.Warn().Err(cfgErr).Str("node", vm.Node).Int("vmid", vm.VMID).Msg("Primary VM config fetch failed, attempting node discovery fallback")
		if nodes, nErr := proxmox.GetNodeNamesResty(r.Context(), restyClient); nErr == nil {
			for _, n := range nodes {
				if altCfg, altErr := proxmox.GetVMConfigResty(r.Context(), restyClient, n, vm.VMID); altErr == nil {
					cfg = altCfg
					vm.Node = n
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

		if vm.Status == "running" && len(networkInterfaces) > 0 && !isGuestAgentUnavailableCached(vm.Node, vm.VMID) {
			if cachedIfaces, found := getGuestAgentIPsFromCache(vm.Node, vm.VMID); found {
				proxmox.EnrichNetworkInterfacesWithIPs(networkInterfaces, cachedIfaces)
				log.Debug().Int("vmid", vm.VMID).Msg("Using cached guest agent network info")
			} else {
				guestCtx, cancel := context.WithTimeout(r.Context(), constants.GuestAgentTimeout)
				defer cancel()
				if guestIfaces, err := proxmox.GetGuestAgentNetworkInterfaces(guestCtx, client, vm.Node, vm.VMID); err == nil {
					proxmox.EnrichNetworkInterfacesWithIPs(networkInterfaces, guestIfaces)
					cacheGuestAgentIPs(vm.Node, vm.VMID, guestIfaces)
					log.Debug().Int("vmid", vm.VMID).Msg("Fetched and cached guest agent network info")
				} else {
					cacheGuestAgentUnavailable(vm.Node, vm.VMID)
					log.Debug().Err(err).Int("vmid", vm.VMID).Msg("Guest agent network info not available (cached unavailability)")
				}
			}
		}
	} else if cfgErr != nil {
		log.Warn().Err(cfgErr).Int("vmid", vm.VMID).Msg("Unable to fetch VM config; description and tags may be empty")
	}

	handlerCtx := NewHandlerContext(w, r, "VMDetailsHandler")
	csrfToken, _ := handlerCtx.GetCSRFToken()

	showDescriptionEditor := r.URL.Query().Get("edit") == "description"
	showResourcesEditor := r.URL.Query().Get("edit") == "resources"
	showTagsEditor := r.URL.Query().Get("edit") == "tags"

	settings := stateManager.GetSettings()
	allTags := settings.Tags

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

	maxNetworkCards := settings.MaxNetworkCards
	if maxNetworkCards <= 0 {
		maxNetworkCards = 1
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
			log.Warn().Err(err).Str("node", vm.Node).Msg("Failed to get VMBRs for resource editor")
		}

		if cfg != nil {
			if socketsVal, ok := cfg["sockets"].(float64); ok {
				currentSockets = int(socketsVal)
			}
			if coresVal, ok := cfg["cores"].(float64); ok {
				currentCores = int(coresVal)
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
		http.Error(w, "VM not found", http.StatusNotFound)
		return
	}

	custom := map[string]interface{}{
		"AllTags":               allTags,
		"AvailableVMBRs":        availableVMBRs,
		"CSRFToken":             csrfToken,
		"CurrentCores":          currentCores,
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
		"Limits":                settings.Limits,
		"MaxNetworkCards":       maxNetworkCards,
		"NetworkBridges":        networkBridgesStr,
		"NetworkCards":          networkCardsData,
		"NetworkInterfaces":     networkInterfaces,
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
