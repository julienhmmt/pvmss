package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"pvmss/logger"
	"pvmss/proxmox"
)

// ResourceLimits defines the min and max values for a resource
type ResourceLimits struct {
	Sockets ResourceMinMax
	Cores   ResourceMinMax
	RAM     ResourceMinMax
	Disk    ResourceMinMax // Only used for VM limits
}

// ResourceMinMax holds minimum and maximum values for a resource
type ResourceMinMax struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// LimitsSettings holds the limits configuration for VM and nodes
type LimitsSettings struct {
	VM    ResourceLimits            `json:"vm"`
	Nodes map[string]ResourceLimits `json:"nodes"`
}

// Default limits for VM
func defaultVMLimits() ResourceLimits {
	return ResourceLimits{
		Sockets: ResourceMinMax{Min: 1, Max: 8},
		Cores:   ResourceMinMax{Min: 1, Max: 32},
		RAM:     ResourceMinMax{Min: 1, Max: 128},    // in GB
		Disk:    ResourceMinMax{Min: 10, Max: 1000},  // in GB
	}
}

// Default limits for a node (will be adjusted based on node capacity)
func defaultNodeLimits(nodeDetails *proxmox.NodeDetails) ResourceLimits {
	sockets := 1
	if nodeDetails.Sockets > 0 {
		sockets = nodeDetails.Sockets
	}
	return ResourceLimits{
		Sockets: ResourceMinMax{Min: 1, Max: sockets},
		Cores:   ResourceMinMax{Min: 1, Max: nodeDetails.MaxCPU},
		RAM:     ResourceMinMax{Min: 1, Max: int(nodeDetails.MaxMemory / 1024 / 1024 / 1024)}, // Convert bytes to GB
		Disk:    ResourceMinMax{Min: 0, Max: 0}, // Not used for nodes
	}
}

// Validate limits against node capacity and rules
// validateLimits checks if the specified limits are valid against node capacity and rules
func validateLimits(limits ResourceLimits, nodeDetails *proxmox.NodeDetails, isVM bool) error {
	// Check min values are not greater than max values
	if limits.Sockets.Min > limits.Sockets.Max {
		return fmt.Errorf("socket minimum value cannot be greater than maximum value")
	}
	if limits.Cores.Min > limits.Cores.Max {
		return fmt.Errorf("cores minimum value cannot be greater than maximum value")
	}
	if limits.RAM.Min > limits.RAM.Max {
		return fmt.Errorf("RAM minimum value cannot be greater than maximum value")
	}
	if isVM && limits.Disk.Min > limits.Disk.Max {
		return fmt.Errorf("disk minimum value cannot be greater than maximum value")
	}

	// Check all values are positive
	if limits.Sockets.Min <= 0 || limits.Sockets.Max <= 0 {
		return fmt.Errorf("socket values must be positive")
	}
	if limits.Cores.Min <= 0 || limits.Cores.Max <= 0 {
		return fmt.Errorf("core values must be positive")
	}
	if limits.RAM.Min <= 0 || limits.RAM.Max <= 0 {
		return fmt.Errorf("RAM values must be positive")
	}
	if isVM && (limits.Disk.Min <= 0 || limits.Disk.Max <= 0) {
		return fmt.Errorf("disk values must be positive")
	}

	// Check against node capacity if provided
	if nodeDetails != nil {
		if limits.Sockets.Max > nodeDetails.Sockets {
			return fmt.Errorf("maximum socket value cannot exceed %d", nodeDetails.Sockets)
		}
		if limits.Cores.Max > nodeDetails.MaxCPU {
			return fmt.Errorf("maximum core value cannot exceed %d", nodeDetails.MaxCPU)
		}
		ramMaxGB := int(nodeDetails.MaxMemory / 1024 / 1024 / 1024)
		if limits.RAM.Max > ramMaxGB {
			return fmt.Errorf("maximum RAM value cannot exceed %d GB", ramMaxGB)
		}
	}

	return nil
}

func limitsHandler(w http.ResponseWriter, r *http.Request) {
	logger.Get().Info().
		Str("handler", "limitsHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Limits API handler invoked")
	
	switch r.Method {
	case http.MethodGet:
		getLimitsHandler(w, r)
	case http.MethodPost:
		updateLimitsHandler(w, r)
	case http.MethodPut:
		resetLimitsHandler(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, PUT")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getLimitsHandler(w http.ResponseWriter, _ *http.Request) {
	logger.Get().Info().Msg("Getting resource limits")
	settings, err := readSettings()
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to read settings")
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"limits":  settings.Limits,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Get().Error().Err(err).Msg("Error encoding limits to JSON")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func updateLimitsHandler(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		EntityID string          `json:"entityId"`
		Sockets  ResourceMinMax  `json:"sockets"`
		Cores    ResourceMinMax  `json:"cores"`
		RAM      ResourceMinMax  `json:"ram"`
		Disk     ResourceMinMax  `json:"disk"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		logger.Get().Error().Err(err).Msg("Error decoding limits update request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	isVM := requestData.EntityID == "vm"
	var nodeDetails *proxmox.NodeDetails
	if !isVM {
		apiURL := os.Getenv("PROXMOX_URL")
		tokenName := os.Getenv("PROXMOX_API_TOKEN_NAME")
		tokenValue := os.Getenv("PROXMOX_API_TOKEN_VALUE")
		verifySSL := os.Getenv("PROXMOX_VERIFY_SSL") != "false"
		client, err := proxmox.NewClient(apiURL, tokenName, tokenValue, !verifySSL)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Error creating Proxmox client")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ctx := context.Background()
		nodeDetails, err = proxmox.GetNodeDetailsWithContext(ctx, client, requestData.EntityID)
		if err != nil {
			logger.Get().Error().Err(err).Str("node", requestData.EntityID).Msg("Error retrieving node details")
			http.Error(w, fmt.Sprintf("Error retrieving node details for %s", requestData.EntityID), http.StatusInternalServerError)
			return
		}
	} else {
		// For VM, use a default or mock node details if needed
		_, err := readSettings()
		if err != nil {
			logger.Get().Error().Err(err).Msg("Error reading settings")
			http.Error(w, "Error reading settings", http.StatusInternalServerError)
			return
		}
		// Placeholder for VM reference node details
		nodeDetails = &proxmox.NodeDetails{Sockets: 8, MaxCPU: 32, MaxMemory: 128 * 1024 * 1024 * 1024}
	}

	limits := ResourceLimits{
		Sockets: requestData.Sockets,
		Cores:   requestData.Cores,
		RAM:     requestData.RAM,
		Disk:    requestData.Disk,
	}

	err := validateLimits(limits, nodeDetails, isVM)
	if err != nil {
		logger.Get().Warn().Err(err).Str("entity", requestData.EntityID).Msg("Limits validation failed")
		response := map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	settings, err := readSettings()
	if err != nil {
		logger.Get().Error().Err(err).Msg("Error reading settings")
		http.Error(w, "Error reading settings", http.StatusInternalServerError)
		return
	}

	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	// Update settings based on limits type
	if isVM {
		// Update VM limits
		settings.Limits["vm"] = VMLimit{
			Sockets: MinMax{Min: limits.Sockets.Min, Max: limits.Sockets.Max},
			Cores:   MinMax{Min: limits.Cores.Min, Max: limits.Cores.Max},
			RAM:     MinMax{Min: limits.RAM.Min, Max: limits.RAM.Max},
			Disk:    MinMax{Min: limits.Disk.Min, Max: limits.Disk.Max},
		}
	} else {
		// Update node limits
		// Ensure nodes map exists
		nodes, ok := settings.Limits["nodes"].(map[string]interface{})
		if !ok {
			settings.Limits["nodes"] = make(map[string]interface{})
			nodes = settings.Limits["nodes"].(map[string]interface{})
		}
		
		// Update node limits
		nodes[requestData.EntityID] = NodeLimit{
			Sockets: MinMax{Min: limits.Sockets.Min, Max: limits.Sockets.Max},
			Cores:   MinMax{Min: limits.Cores.Min, Max: limits.Cores.Max},
			RAM:     MinMax{Min: limits.RAM.Min, Max: limits.RAM.Max},
		}
	}

	// Save updated settings
	if err := writeSettings(settings); err != nil {
		logger.Get().Error().Err(err).Msg("Error saving settings after limit update")
		http.Error(w, "Error saving settings", http.StatusInternalServerError)
		return
	}

	logger.Get().Info().Str("entity", requestData.EntityID).Msg("Limits successfully updated")

	response := map[string]interface{}{
		"success": true,
		"message": "Limits successfully updated",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func resetLimitsHandler(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		EntityID string `json:"entity_id"` // "vm" or node name
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		logger.Get().Error().Err(err).Msg("Error decoding limits reset request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	isVM := requestData.EntityID == "vm"
	settings, err := readSettings()
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to read settings")
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	if isVM {
		// Reset VM limits to defaults
		defLimits := defaultVMLimits()
		vm := VMLimit{
			Sockets: MinMax{Min: defLimits.Sockets.Min, Max: defLimits.Sockets.Max},
			Cores:   MinMax{Min: defLimits.Cores.Min, Max: defLimits.Cores.Max},
			RAM:     MinMax{Min: defLimits.RAM.Min, Max: defLimits.RAM.Max},
			Disk:    MinMax{Min: defLimits.Disk.Min, Max: defLimits.Disk.Max},
		}
		settings.Limits["vm"] = vm
	} else {
		// Reset node limits to defaults
		apiURL := os.Getenv("PROXMOX_URL")
		tokenName := os.Getenv("PROXMOX_API_TOKEN_NAME")
		tokenValue := os.Getenv("PROXMOX_API_TOKEN_VALUE")
		verifySSL := os.Getenv("PROXMOX_VERIFY_SSL") != "false"
		client, err := proxmox.NewClient(apiURL, tokenName, tokenValue, !verifySSL)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Error creating Proxmox client")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ctx := context.Background()
		nodeDetails, err := proxmox.GetNodeDetailsWithContext(ctx, client, requestData.EntityID)
		if err != nil {
			logger.Get().Error().Err(err).Str("node", requestData.EntityID).Msg("Error retrieving node details")
			http.Error(w, fmt.Sprintf("Error retrieving node details for %s", requestData.EntityID), http.StatusInternalServerError)
			return
		}

		// Use nodeDetails to set default limits
		defLimits := defaultNodeLimits(nodeDetails)
		logger.Get().Debug().Interface("defaultLimits", defLimits).Str("node", requestData.EntityID).Msg("Default limits calculated for node")
		
		// Ensure nodes map exists in settings
		if _, exists := settings.Limits["nodes"]; !exists {
			settings.Limits["nodes"] = make(map[string]interface{})
		}
		
		// Get the nodes map
		nodes, ok := settings.Limits["nodes"].(map[string]interface{})
		if !ok {
			settings.Limits["nodes"] = make(map[string]interface{})
			nodes = settings.Limits["nodes"].(map[string]interface{})
		}
		
		// Update node limits
		nodes[requestData.EntityID] = NodeLimit{
			Sockets: MinMax{Min: defLimits.Sockets.Min, Max: defLimits.Sockets.Max},
			Cores:   MinMax{Min: defLimits.Cores.Min, Max: defLimits.Cores.Max},
			RAM:     MinMax{Min: defLimits.RAM.Min, Max: defLimits.RAM.Max},
		}
	}

	if err := writeSettings(settings); err != nil {
		logger.Get().Error().Err(err).Msg("Error saving settings after reset")
		http.Error(w, "Error saving settings", http.StatusInternalServerError)
		return
	}

	logger.Get().Info().Str("entity", requestData.EntityID).Msg("Limits successfully reset")

	var limitsValue interface{}
	if isVM {
		limitsValue = settings.Limits["vm"]
	} else {
		nodes, ok := settings.Limits["nodes"].(map[string]interface{})
		if !ok {
			limitsValue = nil
		} else {
			limitsValue = nodes[requestData.EntityID]
		}
	}
	
	response := map[string]interface{}{
		"success": true,
		"message": "Limits successfully reset",
		"limits":  limitsValue,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
