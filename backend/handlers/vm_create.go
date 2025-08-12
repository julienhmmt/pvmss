package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
)

// CreateVMPage renders the VM creation form using values from settings.json (ISOs, VMBRs, Tags, Limits)
func (h *VMHandler) CreateVMPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	sm := h.stateManager
	settings := sm.GetSettings()
	data := map[string]interface{}{
		"Title":         "Create VM",
		"ISOs":          settings.ISOs,
		"Bridges":       settings.VMBRs,
		"AvailableTags": settings.Tags,
		"Limits":        settings.Limits,
		// Empty form values for initial render
		"FormData": map[string]interface{}{
			"Tags": []string{"pvmss"},
		},
	}

	// Add i18n data
	i18n.LocalizePage(w, r, data)
	renderTemplateInternal(w, r, "create_vm", data)
}

// CreateVMHandler handles POST /api/vm/create to create a VM in Proxmox
func (h *VMHandler) CreateVMHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "CreateVMHandler").Logger()
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Error().Err(err).Msg("failed to parse form")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	// Extract fields
	name := r.FormValue("name")
	desc := r.FormValue("description")
	vmidStr := r.FormValue("vmid")
	sockets := r.FormValue("sockets")
	cores := r.FormValue("cores")
	memory := r.FormValue("memory") // MB
	diskSizeGB := r.FormValue("disk_size")
	iso := r.FormValue("iso") // settings provides full volid or path string
	bridge := r.FormValue("bridge")
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

	if name == "" || sockets == "" || cores == "" || memory == "" || diskSizeGB == "" || bridge == "" {
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not initialized")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Determine node: pick the first available node
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil || len(nodes) == 0 {
		log.Error().Err(err).Msg("unable to get Proxmox nodes")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Proxmox.ConnectionError"), http.StatusBadGateway)
		return
	}
	node := nodes[0]

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
			localizer := i18n.GetLocalizer(r)
			http.Error(w, i18n.Localize(localizer, "Error.InternalServer"), http.StatusInternalServerError)
			return
		}
		vmid = v
	}

	// Build Proxmox create parameters
	params := map[string]string{
		"vmid":    strconv.Itoa(vmid),
		"name":    name,
		"sockets": sockets,
		"cores":   cores,
		"memory":  memory, // MB
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
		if storage != "" && diskSizeGB != "" {
			params["scsi0"] = storage + ":" + diskSizeGB
			params["scsihw"] = "virtio-scsi-pci"
		}
	}

	// Perform API call: POST /nodes/{node}/qemu
	path := "/nodes/" + url.PathEscape(node) + "/qemu"
	if _, err := client.PostFormWithContext(ctx, path, params); err != nil {
		log.Error().Err(err).Str("node", node).Msg("VM create API call failed")
		localizer := i18n.GetLocalizer(r)
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
	http.Redirect(w, r, "/vm/details/"+strconv.Itoa(vmid)+"?refresh=1", http.StatusSeeOther)
}
