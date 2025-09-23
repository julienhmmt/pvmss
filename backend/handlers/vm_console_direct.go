// New handler in backend/handlers/vm_console_direct.go
package handlers

import (
	"context"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/proxmox"
)

// VMConsoleDirectHandler renders the console page and provides WebSocket connection details
func (h *VMHandler) VMConsoleDirectHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleDirectHandler", r)

	if !IsAuthenticated(r) {
		log.Warn().Msg("Console request rejected: not authenticated")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vmid := r.URL.Query().Get("vmid")
	vmname := r.URL.Query().Get("vmname")
	if vmid == "" || vmname == "" {
		log.Error().Str("vmid", vmid).Str("vmname", vmname).Msg("VM ID and name are required")
		http.Error(w, "VM ID and name are required", http.StatusBadRequest)
		return
	}

	log.Info().Str("vmid", vmid).Str("vmname", vmname).Msg("Console direct request received")

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().Msg("State manager not available")
		http.Error(w, "Service unavailable", http.StatusInternalServerError)
		return
	}

	// Use the existing API token client for now
	// The key insight is that we need to use the VNC ticket for authentication, not cookies
	client := stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, "Proxmox connection not available", http.StatusServiceUnavailable)
		return
	}

	// Get VM node
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	allVMs, err := proxmox.GetVMsWithContext(ctx, client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get VM list")
		http.Error(w, "Failed to get VM information", http.StatusInternalServerError)
		return
	}

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

	// Request VNC proxy ticket - this should work with API token if it has VM.Console permission
	ctx, cancel = context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	vncResponse, err := client.GetVNCProxy(ctx, actualNode, vmidInt)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Str("node", actualNode).Msg("Failed to get VNC proxy ticket")

		// Provide specific error messages
		if strings.Contains(err.Error(), "403") {
			http.Error(w, "API token lacks VM.Console permission. Please add VM.Console permission to your API token in Proxmox.", http.StatusForbidden)
			return
		} else if strings.Contains(err.Error(), "401") {
			http.Error(w, "Authentication failed. Please check your API token credentials.", http.StatusUnauthorized)
			return
		}

		http.Error(w, "Failed to get console access: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Extract VNC data
	var vncData map[string]interface{}
	if data, ok := vncResponse["data"].(map[string]interface{}); ok {
		vncData = data
	} else {
		vncData = vncResponse
	}

	vncTicket, ok := vncData["ticket"].(string)
	if !ok {
		log.Error().Interface("response", vncResponse).Msg("Invalid VNC response format")
		http.Error(w, "Invalid console response format", http.StatusInternalServerError)
		return
	}

	var vncPort int
	if portFloat, ok := vncData["port"].(float64); ok {
		vncPort = int(portFloat)
	} else if portStr, ok := vncData["port"].(string); ok {
		if portInt, err := strconv.Atoi(portStr); err == nil {
			vncPort = portInt
		} else {
			log.Error().Interface("response", vncResponse).Msg("Invalid port format")
			http.Error(w, "Invalid console port format", http.StatusInternalServerError)
			return
		}
	} else {
		log.Error().Interface("response", vncResponse).Msg("Missing port in response")
		http.Error(w, "Missing console port", http.StatusInternalServerError)
		return
	}

	// Get Proxmox host URL
	proxmoxHost := client.GetApiUrl()
	proxmoxHost = strings.TrimPrefix(proxmoxHost, "https://")
	proxmoxHost = strings.TrimSuffix(proxmoxHost, "/api2/json")

	log.Info().Str("proxmox_host", proxmoxHost).Int("port", vncPort).Str("node", actualNode).Int("vmid", vmidInt).Str("ticket", vncTicket[:20]+"...").Msg("VNC ticket obtained successfully")

	// For Proxmox's native noVNC to work, we need to establish a proper PVE session
	// The redirect approach works, but we need to handle authentication differently

	// Instead of redirecting to Proxmox, let's create a simple embedded iframe approach
	// or provide the user with the direct link and proper instructions

	// Render the console page with data
	tmplPath := "frontend/console_page.html"
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		tmplPath = "/app/frontend/console_page.html"
		if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
			log.Error().Msg("Console template not found")
			http.Error(w, "Console template not available", http.StatusInternalServerError)
			return
		}
	}

	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse console template")
		http.Error(w, "Failed to prepare console page", http.StatusInternalServerError)
		return
	}

	data := struct {
		Title  string
		Host   string
		Port   int
		Ticket string
		Node   string
		VMID   int
	}{
		Title:  "Console - " + vmname,
		Host:   proxmoxHost,
		Port:   vncPort,
		Ticket: vncTicket,
		Node:   actualNode,
		VMID:   vmidInt,
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Error().Err(err).Msg("Failed to render console page")
		http.Error(w, "Failed to display console page", http.StatusInternalServerError)
		return
	}
}
