package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Telmate/proxmox-api-go/proxmox"
	"pvmss/logger"
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
	proxmoxClient.InvalidateCache(statusPath)
	status, err := proxmoxClient.GetWithContext(ctx, statusPath)
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
	action := r.FormValue("action")
	vmid := r.FormValue("vmid")
	node := r.FormValue("node")
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
		_, err = proxmoxClient.StartVm(ctx, ref)
	case "stop":
		_, err = proxmoxClient.StopVm(ctx, ref)
	case "shutdown":
		_, err = proxmoxClient.ShutdownVm(ctx, ref)
	case "reset":
		_, err = proxmoxClient.ResetVm(ctx, ref)
	case "reboot":
		_, err = proxmoxClient.RebootVm(ctx, ref)
	default:
		http.Error(w, "Unknown action: "+action, http.StatusBadRequest)
		return
	}
	// Invalidate cache for this VM status after any action
	statusPath := fmt.Sprintf("/nodes/%s/qemu/%s/status/current", node, vmid)
	proxmoxClient.InvalidateCache(statusPath)

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
	vmid := r.URL.Query().Get("vmid")
	node := r.URL.Query().Get("node")
	lang := r.URL.Query().Get("lang")
	data := map[string]interface{}{"Lang": lang, "Node": node}

	if vmid == "" || node == "" {
		http.Error(w, "Missing vmid or node parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Fetch VM config
	configPath := fmt.Sprintf("/nodes/%s/qemu/%s/config", node, vmid)
	config, err := proxmoxClient.GetWithContext(ctx, configPath)
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
	status, err := proxmoxClient.GetWithContext(ctx, statusPath)
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
