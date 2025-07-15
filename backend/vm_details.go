package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/Telmate/proxmox-api-go/proxmox"
)

// vmActionHandler handles VM actions: start, stop, shutdown, reset
func vmActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	action := r.FormValue("action")
	vmid := r.FormValue("vmid")
	node := r.FormValue("node")
	log.Info().Str("handler", "vmActionHandler").Str("action", action).Str("vmid", vmid).Str("node", node).Msg("Received VM action request")
	missing := []string{}
	if action == "" { missing = append(missing, "action") }
	if vmid == "" { missing = append(missing, "vmid") }
	if node == "" { missing = append(missing, "node") }
	if len(missing) > 0 {
		errMsg := "Missing parameters: " + strings.Join(missing, ", ")
		log.Error().Str("handler", "vmActionHandler").Msg(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	var id int
	var err error
	id, err = strconv.Atoi(vmid)
	if err != nil {
		errMsg := "Invalid VMID, must be integer: " + vmid
		log.Error().Str("handler", "vmActionHandler").Str("vmid", vmid).Msg(errMsg)
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
		return
	default:
		http.Error(w, "Unknown action: "+action, http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Error().Err(err).Str("action", action).Str("node", node).Str("vmid", vmid).Msg("VM action failed")
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
		log.Error().Err(err).Str("node", node).Str("vmid", vmid).Msg("Failed to get VM config")
		data["Error"] = "Failed to fetch VM config."
		renderTemplate(w, r, "vm_details.html", data)
		return
	}
	
	// Fetch VM status
	statusPath := fmt.Sprintf("/nodes/%s/qemu/%s/status/current", node, vmid)
	status, err := proxmoxClient.GetWithContext(ctx, statusPath)
	if err != nil {
		log.Error().Err(err).Str("node", node).Str("vmid", vmid).Msg("Failed to get VM status")
		data["Error"] = "Failed to fetch VM status."
		renderTemplate(w, r, "vm_details.html", data)
		return
	}

	// Extract details
	cfg := config["data"].(map[string]interface{})
	st := status["data"].(map[string]interface{})

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
	// RAM (configured)
	if mem, ok := cfg["memory"].(float64); ok {
		data["RAM"] = formatMemory(mem * 1024 * 1024) // Proxmox reports in MB
	}
	// Disks (count and total size)
	diskCount := 0
	diskTotal := float64(0)
	for k, v := range cfg {
		if len(k) > 4 && k[:4] == "virt" {
			diskCount++
			if disk, ok := v.(string); ok {
				// Try to parse size from disk string, e.g. "local-lvm:vm-100-disk-0,size=32G"
				if parts := parseDiskSize(disk); parts != 0 {
					diskTotal += float64(parts)
				}
			}
		}
	}
	data["DiskCount"] = diskCount
	data["DiskTotalSize"] = fmt.Sprintf("%.1f GB", diskTotal)
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

// parseDiskSize extracts the size in GB from a Proxmox disk config string
func parseDiskSize(disk string) int {
	// e.g. "local-lvm:vm-100-disk-0,size=32G"
	for _, part := range strings.Split(disk, ",") {
		if strings.HasPrefix(part, "size=") {
			sizeStr := strings.TrimPrefix(part, "size=")
			if strings.HasSuffix(sizeStr, "G") {
				sizeStr = strings.TrimSuffix(sizeStr, "G")
				if val, err := strconv.Atoi(sizeStr); err == nil {
					return val
				}
			}
		}
	}
	return 0
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
