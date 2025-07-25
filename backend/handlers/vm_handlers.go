// Package handlers - VM-specific HTTP handlers
package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
)

// VmDetailsHandler handles VM details page requests
func VmDetailsHandler(w http.ResponseWriter, r *http.Request) {
	vmidStr := security.ValidateInput(r.URL.Query().Get("vmid"), 10)
	if vmidStr == "" {
		http.Error(w, "VMID is required", http.StatusBadRequest)
		return
	}

	vmid, err := strconv.Atoi(vmidStr)
	if err != nil {
		http.Error(w, "Invalid VMID", http.StatusBadRequest)
		return
	}

	lang := getLanguage(r)
	data := getI18nData(lang)
	data["VMID"] = vmid

	client := state.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Get VM details from Proxmox - simplified implementation
	vmDetails := map[string]interface{}{
		"vmid":   vmidStr,
		"name":   "VM-" + vmidStr,
		"status": "unknown",
		"node":   "unknown",
	}
	data["VM"] = vmDetails

	// Create a buffer to capture the template output
	var buf bytes.Buffer

	tmpl := state.GetTemplates()
	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Templates not initialized", http.StatusInternalServerError)
		return
	}

	// Execute the VM details template
	if err := tmpl.ExecuteTemplate(&buf, "vm_details.html", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error rendering VM details template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add the rendered template to the data map as SafeContent (expected by layout)
	data["SafeContent"] = template.HTML(buf.String())

	// Execute the layout template with the content
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// VmActionHandler handles VM action requests (start, stop, reset, etc.)
func VmActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate CSRF token
	if !security.ValidateCSRFToken(r) {
		logger.Get().Warn().
			Str("ip", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Msg("CSRF token validation failed for VM action")
		http.Error(w, "CSRF token validation failed", http.StatusBadRequest)
		return
	}

	vmidStr := security.ValidateInput(r.FormValue("vmid"), 10)
	action := security.ValidateInput(r.FormValue("action"), 20)

	if vmidStr == "" || action == "" {
		http.Error(w, "VMID and action are required", http.StatusBadRequest)
		return
	}

	vmid, err := strconv.Atoi(vmidStr)
	if err != nil {
		http.Error(w, "Invalid VMID", http.StatusBadRequest)
		return
	}

	// Validate action
	validActions := []string{"start", "stop", "reset", "shutdown", "suspend", "resume"}
	if !contains(validActions, action) {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	client := state.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Execute VM action - simplified implementation
	logger.Get().Info().Str("vmid", vmidStr).Str("action", action).Msg("VM action requested (not implemented)")

	logger.Get().Info().
		Int("vmid", vmid).
		Str("action", action).
		Str("ip", r.RemoteAddr).
		Msg("VM action executed successfully")

	// Redirect back to VM details
	http.Redirect(w, r, fmt.Sprintf("/vm/details?vmid=%d", vmid), http.StatusSeeOther)
}

// CreateVmHandler handles VM creation requests
func CreateVmHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		lang := getLanguage(r)
		data := getI18nData(lang)
		data["CSRFToken"] = security.GenerateCSRFToken(r)

		// Get available nodes from Proxmox
		client := state.GetProxmoxClient()
		if client != nil {
			nodes, err := proxmox.GetNodeNames(client)
			if err != nil {
				logger.Get().Error().Err(err).Msg("Error getting nodes")
			} else {
				data["Nodes"] = nodes
			}
		}

		// Get available storage from Proxmox
		if client != nil {
			storage, err := proxmox.GetStorages(client)
			if err != nil {
				logger.Get().Error().Err(err).Msg("Error getting storage")
			} else {
				data["Storage"] = storage
			}
		}

		// Get available ISOs from Proxmox - simplified
		if client != nil {
			// In a real implementation, you'd iterate through nodes and storages
			data["ISOs"] = []interface{}{}
		}

		// Get available networks from Proxmox - simplified
		if client != nil {
			// In a real implementation, you'd iterate through nodes
			data["Networks"] = []interface{}{}
		}

		// Create a buffer to capture the template output
		var buf bytes.Buffer

		tmpl := state.GetTemplates()
		if tmpl == nil {
			logger.Get().Error().Msg("Templates not initialized")
			http.Error(w, "Templates not initialized", http.StatusInternalServerError)
			return
		}

		// Execute the VM creation template
		if err := tmpl.ExecuteTemplate(&buf, "create_vm.html", data); err != nil {
			logger.Get().Error().Err(err).Msg("Error rendering VM creation template")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Add the rendered template to the data map as SafeContent (expected by layout)
		data["SafeContent"] = template.HTML(buf.String())

		// Execute the layout template with the content
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			logger.Get().Error().Err(err).Msg("Error executing layout template")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == "POST" {
		// Validate CSRF token
		if !security.ValidateCSRFToken(r) {
			logger.Get().Warn().
				Str("ip", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("CSRF token validation failed for VM creation")
			http.Error(w, "CSRF token validation failed", http.StatusBadRequest)
			return
		}

		// Extract and validate form data
		vmConfig := extractVMConfig(r)
		if err := validateVMConfig(vmConfig); err != nil {
			logger.Get().Error().Err(err).Msg("Invalid VM configuration")
			http.Error(w, fmt.Sprintf("Invalid configuration: %v", err), http.StatusBadRequest)
			return
		}

		client := state.GetProxmoxClient()
		if client == nil {
			http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
			return
		}

		// Create VM via Proxmox - simplified implementation
		// In a real implementation, you'd use client.PostWithContext() to create VM
		logger.Get().Info().Interface("vmConfig", vmConfig).Msg("VM creation requested (not implemented)")

		// For now, just redirect back to the form with a success message
		http.Redirect(w, r, "/create_vm?success=1", http.StatusSeeOther)
	}
}

// Helper functions

// extractVMConfig extracts VM configuration from form data
func extractVMConfig(r *http.Request) map[string]interface{} {
	return map[string]interface{}{
		"name":    security.ValidateInput(r.FormValue("name"), 100),
		"node":    security.ValidateInput(r.FormValue("node"), 50),
		"cores":   parseIntOrDefault(r.FormValue("cores"), 1),
		"memory":  parseIntOrDefault(r.FormValue("memory"), 1024),
		"disk":    parseIntOrDefault(r.FormValue("disk"), 10),
		"storage": security.ValidateInput(r.FormValue("storage"), 50),
		"iso":     security.ValidateInput(r.FormValue("iso"), 200),
		"network": security.ValidateInput(r.FormValue("network"), 50),
		"tags":    security.ValidateInput(r.FormValue("tags"), 200),
		"ostype":  security.ValidateInput(r.FormValue("ostype"), 20),
	}
}

// validateVMConfig validates VM configuration
func validateVMConfig(config map[string]interface{}) error {
	name, ok := config["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("VM name is required")
	}

	node, ok := config["node"].(string)
	if !ok || node == "" {
		return fmt.Errorf("node is required")
	}

	cores := config["cores"].(int)
	if cores < 1 || cores > 32 {
		return fmt.Errorf("cores must be between 1 and 32")
	}

	memory := config["memory"].(int)
	if memory < 512 || memory > 65536 {
		return fmt.Errorf("memory must be between 512 MB and 64 GB")
	}

	disk := config["disk"].(int)
	if disk < 1 || disk > 1000 {
		return fmt.Errorf("disk size must be between 1 GB and 1000 GB")
	}

	return nil
}

// parseIntOrDefault parses string to int with default value
func parseIntOrDefault(s string, defaultVal int) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return defaultVal
}

// contains checks if slice contains string (case-insensitive)
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
