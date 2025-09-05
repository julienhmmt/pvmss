package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/security"
)

// CreateVMPage renders the VM creation form using values from settings.json (ISOs, VMBRs, Tags, Limits)
func (h *VMHandler) CreateVMPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	sm := h.stateManager
	settings := sm.GetSettings()
	// Get nodes list (best effort)
	nodes := []string{}
	activeNode := ""
	if client := sm.GetProxmoxClient(); client != nil {
		if list, err := proxmox.GetNodeNamesWithContext(r.Context(), client); err == nil && len(list) > 0 {
			nodes = list
			activeNode = list[0]
		}
	}

	// Get username from session to pre-fill pool
	defaultPool := ""
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if username, ok := sessionManager.Get(r.Context(), "username").(string); ok && username != "" {
			defaultPool = fmt.Sprintf("pvmss_%s", username)
		}
	}

	data := map[string]interface{}{
		"Title":         "Create VM",
		"ISOs":          settings.ISOs,
		"Bridges":       settings.VMBRs,
		"AvailableTags": settings.Tags,
		"Limits":        settings.Limits,
		"Nodes":         nodes,
		"ActiveNode":    activeNode,
		"DefaultPool":   defaultPool,
		// Empty form values for initial render
		"FormData": map[string]interface{}{
			"Tags": []string{"pvmss"},
		},
	}

	// Proxmox connection status for template (also provided by middleware, but ensure here)
	if sm != nil {
		connected, message := sm.GetProxmoxStatus()
		data["ProxmoxConnected"] = connected
		if !connected {
			data["ProxmoxError"] = message
		}
	}

	// Add i18n data
	RenderTemplate(w, r, "create_vm", data)
}

// CreateVMHandler handles POST /api/vm/create to create a VM in Proxmox
func (h *VMHandler) CreateVMHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CreateVMHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Extract fields
	name := r.FormValue("name")
	desc := r.FormValue("description")
	vmidStr := r.FormValue("vmid")
	socketsStr := r.FormValue("sockets")
	coresStr := r.FormValue("cores")
	memoryStr := r.FormValue("memory") // MB
	diskSizeGBStr := r.FormValue("disk_size")
	iso := r.FormValue("iso") // settings provides full volid or path string
	bridge := r.FormValue("bridge")
	selectedNode := r.FormValue("node")
	pool := r.FormValue("pool")
	tags := r.Form["tags"]
	// Ensure mandatory tag "pvmss" is present and deduplicate
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags)+1)
	// Always include pvmss first
	if _, ok := seen["pvmss"]; !ok {
		seen["pvmss"] = struct{}{}
		out = append(out, "pvmss")
	}
	for _, t := range tags {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	tags = out

	if name == "" || socketsStr == "" || coresStr == "" || memoryStr == "" || diskSizeGBStr == "" || bridge == "" {
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not initialized")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Determine node: use selected if provided, otherwise pick the first available node
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil || len(nodes) == 0 {
		log.Error().Err(err).Msg("unable to get Proxmox nodes")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Proxmox.ConnectionError"), http.StatusBadGateway)
		return
	}
	node := nodes[0]
	if selectedNode != "" {
		// ensure selected node exists
		for _, n := range nodes {
			if n == selectedNode {
				node = selectedNode
				break
			}
		}
	}

	// Parse numeric fields
	sockets, err := strconv.Atoi(socketsStr)
	if err != nil {
		http.Error(w, "invalid sockets", http.StatusBadRequest)
		return
	}
	cores, err := strconv.Atoi(coresStr)
	if err != nil {
		http.Error(w, "invalid cores", http.StatusBadRequest)
		return
	}
	memoryMB, err := strconv.Atoi(memoryStr)
	if err != nil {
		http.Error(w, "invalid memory", http.StatusBadRequest)
		return
	}
	diskSizeGB, err := strconv.Atoi(diskSizeGBStr)
	if err != nil {
		http.Error(w, "invalid disk size", http.StatusBadRequest)
		return
	}

	// Validate against settings limits (vm and optional node-specific)
	if settings := h.stateManager.GetSettings(); settings != nil && settings.Limits != nil {
		// Helper to read min/max from map
		readMinMax := func(m map[string]interface{}, key string) (min, max int, ok bool) {
			if raw, exists := m[key]; exists {
				if mm, ok2 := raw.(map[string]interface{}); ok2 {
					vMin, vMax := 0, 0
					if v, ok3 := mm["min"]; ok3 {
						if f, ok4 := v.(float64); ok4 {
							vMin = int(f)
						}
					}
					if v, ok3 := mm["max"]; ok3 {
						if f, ok4 := v.(float64); ok4 {
							vMax = int(f)
						}
					}
					return vMin, vMax, true
				}
			}
			return 0, 0, false
		}

		// VM limits
		if rawVM, ok := settings.Limits["vm"].(map[string]interface{}); ok {
			if min, max, ok2 := readMinMax(rawVM, "sockets"); ok2 {
				if sockets < min || sockets > max {
					http.Error(w, fmt.Sprintf("sockets must be between %d and %d", min, max), http.StatusBadRequest)
					return
				}
			}
			if min, max, ok2 := readMinMax(rawVM, "cores"); ok2 {
				if cores < min || cores > max {
					http.Error(w, fmt.Sprintf("cores must be between %d and %d", min, max), http.StatusBadRequest)
					return
				}
			}
			if minGB, maxGB, ok2 := readMinMax(rawVM, "ram"); ok2 {
				minMB := minGB * 1024
				maxMB := maxGB * 1024
				if memoryMB < minMB || memoryMB > maxMB {
					http.Error(w, fmt.Sprintf("memory must be between %d and %d MB", minMB, maxMB), http.StatusBadRequest)
					return
				}
			}
			if min, max, ok2 := readMinMax(rawVM, "disk"); ok2 {
				if diskSizeGB < min || diskSizeGB > max {
					http.Error(w, fmt.Sprintf("disk size must be between %d and %d GB", min, max), http.StatusBadRequest)
					return
				}
			}
		}

		// Node-specific caps (optional)
		if rawNodes, ok := settings.Limits["nodes"].(map[string]interface{}); ok {
			if rawNode, ok2 := rawNodes[node].(map[string]interface{}); ok2 {
				if _, max, ok3 := readMinMax(rawNode, "sockets"); ok3 {
					// Enforce only upper bound from node limits; VM lower bound is validated earlier
					if sockets > max {
						http.Error(w, fmt.Sprintf("sockets exceed node '%s' max (%d)", node, max), http.StatusBadRequest)
						return
					}
				}
				if _, max, ok3 := readMinMax(rawNode, "cores"); ok3 {
					// Enforce only upper bound from node limits; VM lower bound is validated earlier
					if cores > max {
						http.Error(w, fmt.Sprintf("cores exceed node '%s' max (%d)", node, max), http.StatusBadRequest)
						return
					}
				}
				if _, maxGB, ok3 := readMinMax(rawNode, "ram"); ok3 {
					// Enforce only upper bound from node limits; VM lower bound is validated earlier
					maxMB := maxGB * 1024
					if memoryMB > maxMB {
						http.Error(w, fmt.Sprintf("memory exceeds node '%s' max (%d MB)", node, maxMB), http.StatusBadRequest)
						return
					}
				}
			}
		}
	}

	// Ensure VMID
	vmid := 0
	if vmidStr != "" {
		if v, err := strconv.Atoi(vmidStr); err == nil {
			vmid = v
		}
	}
	if vmid == 0 {
		v, err := proxmox.GetNextVMID(ctx, client)
		if err != nil {
			log.Error().Err(err).Msg("failed to get next VMID")
			localizer := i18n.GetLocalizerFromRequest(r)
			http.Error(w, i18n.Localize(localizer, "Error.InternalServer"), http.StatusInternalServerError)
			return
		}
		vmid = v
	}

	// Build Proxmox create parameters
	params := map[string]string{
		"vmid":    strconv.Itoa(vmid),
		"name":    name,
		"sockets": strconv.Itoa(sockets),
		"cores":   strconv.Itoa(cores),
		"memory":  strconv.Itoa(memoryMB), // MB
	}

	// Assign to pool if provided
	if pool != "" {
		params["pool"] = pool
	}

	// Tags (Proxmox supports 'tags': csv)
	if len(tags) > 0 {
		params["tags"] = strings.Join(tags, ",")
	}
	if desc != "" {
		params["description"] = desc
	}

	// Attach ISO if provided (ide2 with media=cdrom)
	if iso != "" {
		// Expect iso to be a Proxmox volid like 'local:iso/debian.iso'
		params["ide2"] = iso + ",media=cdrom"
		// Set boot order to cdrom first then disk
		params["boot"] = "order=ide2;scsi0"
	}

	// Network: virtio on selected bridge
	if bridge != "" {
		params["net0"] = "virtio,bridge=" + bridge
	}

	// Disk: allocate on first enabled storage when available
	if settings := h.stateManager.GetSettings(); settings != nil {
		storage := ""
		if len(settings.EnabledStorages) > 0 {
			storage = settings.EnabledStorages[0]
		}
		if storage != "" && diskSizeGBStr != "" {
			params["scsi0"] = storage + ":" + strconv.Itoa(diskSizeGB)
			params["scsihw"] = "virtio-scsi-pci"
		}
	}

	// Perform API call: POST /nodes/{node}/qemu
	path := "/nodes/" + url.PathEscape(node) + "/qemu"

	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}

	if _, err := client.PostFormWithContext(ctx, path, values); err != nil {
		log.Error().Err(err).Str("node", node).Msg("VM create API call failed")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Proxmox.ConnectionError"), http.StatusBadGateway)
		return
	}

	// Optional: ensure VM is running. Query current status and start if needed.
	if cur, err := proxmox.GetVMCurrentWithContext(ctx, client, node, vmid); err != nil {
		log.Warn().Err(err).Int("vmid", vmid).Str("node", node).Msg("Could not fetch VM current status after creation")
	} else if strings.ToLower(cur.Status) != "running" {
		if _, err := proxmox.VMActionWithContext(ctx, client, node, strconv.Itoa(vmid), "start"); err != nil {
			log.Warn().Err(err).Int("vmid", vmid).Str("node", node).Msg("Failed to start VM after creation")
		} else {
			log.Info().Int("vmid", vmid).Str("node", node).Msg("VM started after creation")
		}
	}

	// Redirect to details
	redirectURL := "/vm/details/" + strconv.Itoa(vmid) + "?refresh=1"
	if lang := i18n.GetLanguage(r); lang != "" && lang != i18n.DefaultLang {
		redirectURL += "&lang=" + lang
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
