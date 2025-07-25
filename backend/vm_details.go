package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Telmate/proxmox-api-go/proxmox"
	"pvmss/logger"
	"pvmss/state"
)

// apiVmStatusHandler returns the latest status of a VM (no cache)
func apiVmStatusHandler(w http.ResponseWriter, r *http.Request) {
	vmid := r.URL.Query().Get("vmid")
	node := r.URL.Query().Get("node")
	if vmid == "" || node == "" {
		http.Error(w, "Missing vmid or node parameter", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	statusPath := fmt.Sprintf("/nodes/%s/qemu/%s/status/current", node, vmid)
	client := state.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not initialized", http.StatusInternalServerError)
		return
	}
	client.InvalidateCache(statusPath)
	status, err := client.GetWithContext(ctx, statusPath)
	if err != nil {
		http.Error(w, "Failed to fetch status: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// vmActionHandler handles VM actions: start, stop, shutdown, reset
func vmActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Get Proxmox client from state
	client := state.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not initialized", http.StatusInternalServerError)
		return
	}
	action := validateInput(r.FormValue("action"), 20)
	vmid := validateInput(r.FormValue("vmid"), 10)
	node := validateInput(r.FormValue("node"), 50)
	logger.Get().Info().
		Str("handler", "vmActionHandler").
		Str("action", action).
		Str("vmid", vmid).
		Str("node", node).
		Msg("Received VM action request")
	missing := []string{}
	if action == "" {
		missing = append(missing, "action")
	}
	if vmid == "" {
		missing = append(missing, "vmid")
	}
	if node == "" {
		missing = append(missing, "node")
	}
	if len(missing) > 0 {
		errMsg := "Missing parameters: " + strings.Join(missing, ", ")
		logger.Get().Error().
			Str("handler", "vmActionHandler").
			Msg(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	var id int
	var err error
	id, err = strconv.Atoi(vmid)
	if err != nil {
		errMsg := "Invalid VMID, must be integer: " + vmid
		logger.Get().Error().
			Str("handler", "vmActionHandler").
			Str("vmid", vmid).
			Msg(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ref := proxmox.NewVmRef(proxmox.GuestID(id))
	switch action {
	case "start":
		_, err = client.StartVm(ctx, ref)
	case "stop":
		_, err = client.StopVm(ctx, ref)
	case "shutdown":
		_, err = client.ShutdownVm(ctx, ref)
	case "reset":
		_, err = client.ResetVm(ctx, ref)
	case "reboot":
		_, err = client.RebootVm(ctx, ref)
	default:
		http.Error(w, "Unknown action: "+action, http.StatusBadRequest)
		return
	}
	// Invalidate cache for this VM status after any action
	statusPath := fmt.Sprintf("/nodes/%s/qemu/%s/status/current", node, vmid)
	client.InvalidateCache(statusPath)

	if err != nil {
		logger.Get().Error().
			Err(err).
			Str("action", action).
			Str("node", node).
			Str("vmid", vmid).
			Msg("VM action failed")
		http.Error(w, "VM action failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// vmDetailsHandler serves the VM details page
func vmDetailsHandler(w http.ResponseWriter, r *http.Request) {
	vmid := validateInput(r.URL.Query().Get("vmid"), 10)
	node := validateInput(r.URL.Query().Get("node"), 50)
	lang := r.URL.Query().Get("lang")
	data := map[string]interface{}{"Lang": lang, "Node": node}

	logger.Get().Info().
		Str("handler", "vmDetailsHandler").
		Str("vmid", vmid).
		Str("node", node).
		Msg("VM details request")

	if vmid == "" || node == "" {
		logger.Get().Error().
			Str("handler", "vmDetailsHandler").
			Msg("Missing required parameters")
		http.Error(w, "Missing vmid or node parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Get Proxmox client from state
	client := state.GetProxmoxClient()
	if client == nil {
		data["Error"] = "Proxmox client not initialized"
		renderTemplate(w, r, "vm_details.html", data)
		return
	}

	// Fetch VM config
	configPath := fmt.Sprintf("/nodes/%s/qemu/%s/config", node, vmid)
	config, err := client.GetWithContext(ctx, configPath)
	if err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Str("vmid", vmid).
			Msg("Failed to get VM config")
		data["Error"] = "Failed to fetch VM config."
		renderTemplate(w, r, "vm_details.html", data)
		return
	}

	// Fetch VM status
	statusPath := fmt.Sprintf("/nodes/%s/qemu/%s/status/current", node, vmid)
	status, err := client.GetWithContext(ctx, statusPath)
	if err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Str("vmid", vmid).
			Msg("Failed to get VM status")
		data["Error"] = "Failed to fetch VM status."
		renderTemplate(w, r, "vm_details.html", data)
		return
	}

	// Extract details
	cfg, cfgOk := config["data"].(map[string]interface{})
	if !cfgOk {
		logger.Get().Error().
			Str("node", node).
			Str("vmid", vmid).
			Msg("Invalid VM config format")
		data["Error"] = "Failed to parse VM configuration format."
		renderTemplate(w, r, "vm_details.html", data)
		return
	}

	st, stOk := status["data"].(map[string]interface{})
	if !stOk {
		logger.Get().Error().
			Str("node", node).
			Str("vmid", vmid).
			Msg("Invalid VM status format")
		data["Error"] = "Failed to parse VM status format."
		renderTemplate(w, r, "vm_details.html", data)
		return
	}

	// VM Name & ID
	data["VMName"] = cfg["name"]
	data["VMID"] = vmid
	// Status
	data["Status"] = st["status"]
	// Uptime (format seconds to human)
	if uptime, ok := st["uptime"].(float64); ok {
		data["Uptime"] = formatUptime(int64(uptime))
	}
	// Sockets & Cores
	data["Sockets"] = cfg["sockets"]
	data["Cores"] = cfg["cores"]
	// RAM (actual usage and total)
	ramUsed := float64(0)
	ramTotal := float64(0)
	if v, ok := st["mem"].(float64); ok {
		ramUsed = v
	}
	if v, ok := st["maxmem"].(float64); ok {
		ramTotal = v
	}
	if ramTotal > 0 {
		data["RAM"] = fmt.Sprintf("%s / %s", formatMemory(ramUsed), formatMemory(ramTotal))
	} else if mem, ok := cfg["memory"].(float64); ok {
		// fallback: only configured
		data["RAM"] = formatMemory(mem * 1024 * 1024)
	} else {
		data["RAM"] = "-"
	}

	// Disk (actual usage and total)
	diskUsed := float64(0)
	diskTotal := float64(0)
	if v, ok := st["disk"].(float64); ok {
		diskUsed = v
	}
	if v, ok := st["maxdisk"].(float64); ok {
		diskTotal = v
	}
	if diskTotal > 0 {
		data["DiskTotalSize"] = fmt.Sprintf("%s / %s", formatMemory(diskUsed), formatMemory(diskTotal))
	} else {
		data["DiskTotalSize"] = "-"
	}
	// Optionally, count disks from config (not always reliable)
	diskCount := 0
	for k := range cfg {
		if len(k) > 4 && k[:4] == "virt" {
			diskCount++
		}
	}
	data["DiskCount"] = diskCount
	// Network bridges
	netBridges := ""
	for k, v := range cfg {
		if len(k) > 3 && k[:3] == "net" {
			if netstr, ok := v.(string); ok {
				if bridge := parseBridge(netstr); bridge != "" {
					netBridges += bridge + " "
				}
			}
		}
	}
	data["NetworkBridges"] = netBridges
	// Description
	data["Description"] = cfg["description"]

	renderTemplate(w, r, "vm_details.html", data)
}

// formatMemory converts bytes to human readable format (KB, MB, GB, TB)
func formatMemory(bytes float64) string {
	if bytes <= 0 {
		return "0 B"
	}

	const unit = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	exp := int(math.Log(bytes) / math.Log(unit))
	if exp >= len(sizes) {
		exp = len(sizes) - 1
	}
	size := bytes / math.Pow(unit, float64(exp))
	return fmt.Sprintf("%.1f %s", size, sizes[exp])
}

// formatUptime converts seconds to human readable
func formatUptime(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// renderTemplate is a helper function to render HTML templates
func renderTemplate(w http.ResponseWriter, r *http.Request, tmpl string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}

	// Get templates from state
	templates := state.GetTemplates()
	if templates == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Templates not initialized", http.StatusInternalServerError)
		return
	}

	// Add i18n data to template context
	localizePage(w, r, data)

	// Create a buffer to capture the template output
	var buf bytes.Buffer

	// Execute the specific template
	err := templates.ExecuteTemplate(&buf, tmpl, data)
	if err != nil {
		logger.Get().Error().Err(err).Str("template", tmpl).Msg("Error executing template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add the rendered template to the data map as SafeContent (expected by layout)
	data["SafeContent"] = template.HTML(buf.String())

	// Execute the layout template with the content
	err = templates.ExecuteTemplate(w, "layout", data)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// parseBridge extracts the bridge name from a Proxmox network config string
func parseBridge(netstr string) string {
	// e.g. "virtio=XX:XX:XX:XX:XX:XX,bridge=vmbr0,firewall=1"
	for _, part := range strings.Split(netstr, ",") {
		if strings.HasPrefix(part, "bridge=") {
			return strings.TrimPrefix(part, "bridge=")
		}
	}
	return ""
}
