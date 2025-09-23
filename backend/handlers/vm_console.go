package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
)

// VMConsoleHandler handles VM console requests
func (h *VMHandler) VMConsoleHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleHandler", r)

	log.Info().Str("method", r.Method).Str("path", r.URL.Path).Msg("Console request received")

	// Check authentication manually instead of using RequireAuthHandle
	// to avoid middleware issues with AJAX requests
	if !IsAuthenticated(r) {
		log.Warn().Msg("Console request rejected: not authenticated")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	vmid := r.FormValue("vmid")
	vmname := r.FormValue("vmname") // The frontend sends vmname
	if vmid == "" || vmname == "" {
		log.Error().Str("vmid", vmid).Str("vmname", vmname).Msg("VM ID and name are required")
		http.Error(w, "VM ID and name are required", http.StatusBadRequest)
		return
	}

	log.Info().Str("vmid", vmid).Str("vmname", vmname).Msg("Processing console request")

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	// Get the state manager to access the Proxmox client
	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().Msg("State manager not available")
		http.Error(w, "Service unavailable", http.StatusInternalServerError)
		return
	}

	client := stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, "Proxmox connection not available", http.StatusServiceUnavailable)
		return
	}

	// Get all VMs to find the one with the matching name and get its node
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	allVMs, err := proxmox.GetVMsWithContext(ctx, client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get VM list")
		http.Error(w, "Failed to get VM information", http.StatusInternalServerError)
		return
	}

	// Find the VM by name and get its node
	var actualNode string
	for _, vm := range allVMs {
		if vm.Name == vmname && vm.VMID == vmidInt {
			actualNode = vm.Node
			break
		}
	}

	if actualNode == "" {
		log.Error().Str("vmname", vmname).Int("vmid", vmidInt).Msg("VM not found")
		http.Error(w, "VM not found", http.StatusNotFound)
		return
	}

	log.Info().Int("vmid", vmidInt).Str("node", actualNode).Msg("Requesting VNC proxy ticket")

	// Request VNC proxy ticket using the existing client (API token auth)
	ctx, cancel = context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	vncResponse, err := client.GetVNCProxy(ctx, actualNode, vmidInt)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Str("node", actualNode).Msg("Failed to get VNC proxy ticket")

		// Provide helpful error messages based on error type
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "permission") {
			http.Error(w, "Insufficient permissions for console access. Please ensure your user or API token has 'VM.Console' permission in Proxmox.", http.StatusForbidden)
			return
		} else if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "authentication") {
			http.Error(w, "Authentication failed for console access. Please check your credentials or API token.", http.StatusUnauthorized)
			return
		}

		http.Error(w, "Failed to get console access: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Extract the VNC ticket and port from the response
	// Proxmox API returns data wrapped in a "data" field
	var vncData map[string]interface{}
	if data, ok := vncResponse["data"].(map[string]interface{}); ok {
		vncData = data
	} else {
		// Fallback: try to use the response directly if no "data" wrapper
		vncData = vncResponse
	}

	vncTicket, ok := vncData["ticket"].(string)
	if !ok {
		log.Error().Interface("response", vncResponse).Interface("vncData", vncData).Msg("Invalid VNC response format")
		http.Error(w, "Invalid console response format", http.StatusInternalServerError)
		return
	}

	vncPort, ok := vncData["port"].(float64)
	if !ok {
		// Try as string (Proxmox sometimes returns port as string)
		if portStr, ok := vncData["port"].(string); ok {
			if portInt, err := strconv.Atoi(portStr); err == nil {
				vncPort = float64(portInt)
			} else {
				log.Error().Interface("response", vncResponse).Interface("vncData", vncData).Str("portStr", portStr).Msg("Invalid VNC response format - port not a valid number")
				http.Error(w, "Invalid console response format", http.StatusInternalServerError)
				return
			}
		} else {
			log.Error().Interface("response", vncResponse).Interface("vncData", vncData).Msg("Invalid VNC response format - missing port")
			http.Error(w, "Invalid console response format", http.StatusInternalServerError)
			return
		}
	}

	// Get the Proxmox host URL for constructing the console URL
	proxmoxHost := client.GetApiUrl()
	proxmoxHost = strings.TrimPrefix(proxmoxHost, "https://")
	proxmoxHost = strings.TrimSuffix(proxmoxHost, "/api2/json")

	log.Info().Int("vmid", vmidInt).Str("node", actualNode).Int("port", int(vncPort)).Msg("VNC proxy ticket obtained successfully")

	// Determine authentication method for frontend
	authMethod := "api_token"
	if client.GetPVEAuthCookie() != "" {
		authMethod = "cookie"
	}

	// Return the console information as JSON
	response := map[string]interface{}{
		"success":    true,
		"ticket":     vncTicket,
		"port":       int(vncPort),
		"host":       proxmoxHost,
		"node":       actualNode,
		"vmid":       vmidInt,
		"authMethod": authMethod,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode console response")
		http.Error(w, "Failed to prepare console response", http.StatusInternalServerError)
		return
	}
}
