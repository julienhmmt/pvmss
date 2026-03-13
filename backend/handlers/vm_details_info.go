package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/sync/errgroup"

	"pvmss/constants"
	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/state"
)

// VMDetailsHandler renders the VM details page with resource, network, disk and metadata information.
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
	var snapshots []proxmox.VMSnapshot
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
						if cfgErr != nil {
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

	var currentSnapshotName string
	if cfgErr == nil && cfg != nil {
		// Fetch snapshots after config is successfully retrieved
		if snapshotsData, snapErr := proxmox.GetVMSnapshotsResty(r.Context(), restyClient, vm.Node, strconv.Itoa(vm.VMID)); snapErr == nil {
			// Filter out the "current" pseudo-snapshot which represents the current VM state
			for _, snap := range snapshotsData {
				if snap.Name == "current" {
					// Store the current snapshot name (which is the parent of the current state)
					currentSnapshotName = snap.Parent
				} else {
					snapshots = append(snapshots, snap)
				}
			}
		} else {
			log.Warn().
				Err(snapErr).
				Str("component", "vm_details").
				Str("operation", "fetch_snapshots").
				Str("reason", "snapshot_fetch_failed").
				Str("node", vm.Node).
				Int("vmid", vm.VMID).
				Msg("Failed to fetch VM snapshots")
		}

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

		if rawTPM, ok := cfg["tpmstate0"].(string); ok && strings.TrimSpace(rawTPM) != "" {
			tpmEnabled = true
		}

		if ide2, ok := cfg["ide2"].(string); ok && strings.TrimSpace(ide2) != "" {
			hasCDROM = true
			parts := strings.SplitN(ide2, ":", 2)
			if len(parts) == 2 {
				isoPart := strings.TrimSpace(parts[1])
				if idx := strings.Index(isoPart, ",media=cdrom"); idx >= 0 {
					isoPart = strings.TrimSpace(isoPart[:idx])
				}
				currentISO = isoPart
			}
		} else {
			hasCDROM = true
		}

		if ciuser, ok := cfg["ciuser"].(string); ok && ciuser != "" {
			cloudInitData["user"] = ciuser
			cloudInitEnabled = true
		}
		if sshkeys, ok := cfg["sshkeys"].(string); ok && sshkeys != "" {
			decodedKeys, err := url.QueryUnescape(sshkeys)
			if err != nil {
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
			if strings.Contains(cicustom, "pvmss-") && stateManager.GetSettings() != nil {
				templates := stateManager.GetSettings().CloudInitTemplates
				for _, template := range templates {
					if strings.Contains(cicustom, "-"+template.ID+".yml") || strings.Contains(cicustom, "-"+template.ID+".yaml") {
						cloudInitData["templateName"] = template.Name
						cloudInitData["templateYAML"] = template.YAMLContent
						break
					}
				}
			}
		}
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

	handlerCtx := HandlerContextWith(w, r, "VMDetailsHandler")
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

	vmRamMinMB, vmRamMaxMB := 0, 0
	if settings != nil {
		vmRamMinMB = settings.Limits.VM.RAM.Min * 1024
		vmRamMaxMB = settings.Limits.VM.RAM.Max * 1024
	}
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

	var availableISOs []string
	if showResourcesEditor {
		if settings != nil {
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

	if vm == nil {
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.NotFound"), http.StatusNotFound)
		return
	}

	// Handle success/error messages from query parameters
	localizer := i18n.GetLocalizerFromRequest(r)
	success := false
	successMessage := ""
	errorMessage := ""

	if successParam := r.URL.Query().Get("success"); successParam != "" {
		success = true
		// Check for success_msg first (from RedirectWithSuccess)
		if msg := r.URL.Query().Get("success_msg"); msg != "" {
			successMessage = msg
		} else {
			// Fallback to old parameter-based messages
			switch successParam {
			case "snapshot_created":
				successMessage = i18n.Localize(localizer, "VMDetails.Snapshots.CreatedSuccess")
			case "snapshot_updated":
				successMessage = i18n.Localize(localizer, "VMDetails.Snapshots.UpdatedSuccess")
			case "snapshot_deleted":
				successMessage = i18n.Localize(localizer, "VMDetails.Snapshots.DeletedSuccess")
			case "snapshot_rollback":
				successMessage = i18n.Localize(localizer, "VMDetails.Snapshots.RollbackSuccess")
			}
		}
	}

	if errorParam := r.URL.Query().Get("error"); errorParam != "" {
		switch errorParam {
		case "create_failed":
			errorMessage = i18n.Localize(localizer, "VMDetails.Snapshots.CreateFailed")
		case "update_failed":
			errorMessage = i18n.Localize(localizer, "VMDetails.Snapshots.UpdateFailed")
		case "delete_failed":
			errorMessage = i18n.Localize(localizer, "VMDetails.Snapshots.DeleteFailed")
		case "rollback_failed":
			errorMessage = i18n.Localize(localizer, "VMDetails.Snapshots.RollbackFailed")
		case "max_snapshots_reached":
			errorMessage = i18n.Localize(localizer, "VMDetails.Snapshots.LimitReached")
		case "invalid_snapshot_name":
			errorMessage = i18n.Localize(localizer, "VMDetails.Snapshots.InvalidName")
		case "missing_parameters":
			errorMessage = i18n.Localize(localizer, "Error.MissingRequiredFields")
		case "client_error":
			errorMessage = i18n.Localize(localizer, "Error.ServerConfigError")
		}
	}

	custom := map[string]interface{}{
		"Success":               success,
		"SuccessMessage":        successMessage,
		"ErrorMessage":          errorMessage,
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
		"Lang":                  getLangFromRequest(r),
		"AgentStatusKey":        agentStatusKey,
		"AgentStatusClass":      agentStatusClass,
		"ShowDescriptionEditor": showDescriptionEditor,
		"ShowResourcesEditor":   showResourcesEditor,
		"ShowTagsEditor":        showTagsEditor,
		"TPMEnabled":            tpmEnabled,
		"Tags":                  strings.Join(tags, ", "),
		"VM":                    vm,
		"Snapshots":             snapshots,
		"CurrentSnapshotName":   currentSnapshotName,
		"MaxSnapshotsPerVM":     settings.Limits.MaxSnapshots,
	}

	connected, _ := stateManager.GetProxmoxStatus()
	custom["ProxmoxConnected"] = connected
	custom["IsAdmin"] = IsAdmin(r)
	custom["Username"] = getUsernameFromSession(r)

	renderVMDetailsTempl(w, r, custom)
}
