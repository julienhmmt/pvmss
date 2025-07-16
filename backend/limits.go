package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"pvmss/backend/proxmox"

	"github.com/rs/zerolog/log"
)

// max returns the greater of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// limitsHandler routes the requests to the appropriate handler based on the HTTP method.
func limitsHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "limitsHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Limits handler invoked")
	switch r.Method {
	case http.MethodGet:
		getLimitsHandler(w, r)
	case http.MethodPost:
		updateLimitsHandler(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getLimitsHandler returns the current resource limits for all nodes and VM defaults.
func getLimitsHandler(w http.ResponseWriter, r *http.Request) {
	// Read settings from the settings.json file
	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings")
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	// Initialize limits map if it doesn't exist
	if settings.Limits == nil {
		settings.Limits = make(map[string]NodeLimits)
	}

	// Ensure we have reasonable default values for all nodes
	nodeDetails, err := getNodeDetailsFromCacheOrAPI(r.Context(), settings)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get node details, using existing limits")
	} else {
		// Update limits for all known nodes (except 'vm' which is our defaults)
		for _, node := range nodeDetails {
			if node.Node != "vm" {
				if _, exists := settings.Limits[node.Node]; !exists {
					settings.Limits[node.Node] = settings.Limits["vm"] // Use VM defaults for new nodes
				}
			}
		}

		// Save the updated settings with new nodes
		if err := writeSettings(settings); err != nil {
			log.Error().Err(err).Msg("Failed to save updated settings")
		}
	}

	// Ensure limits are reasonable
	if !ensureReasonableLimits(settings.Limits) {
		log.Warn().Msg("Some limits were adjusted to be more reasonable")
	}

	// Prepare response with VM defaults and node limits
	response := struct {
		VM    NodeLimits            `json:"vm"`
		Nodes map[string]NodeLimits `json:"nodes"`
	}{
		VM:    settings.Limits["vm"],
		Nodes: make(map[string]NodeLimits),
	}

	// Return the limits
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode limits response")
	}
}

// ensureReasonableLimits validates and corrects any unreasonable limit values
func ensureReasonableLimits(limits map[string]NodeLimits) bool {
	updated := false

	for nodeName, lim := range limits {
		limUpdated := false
		// sockets defaults and corrections
		if lim.Sockets.Min < 1 {
			lim.Sockets.Min = 1
			limUpdated = true
		}
		if lim.Sockets.Max < lim.Sockets.Min {
			// default to min or 1 if zero
			lim.Sockets.Max = lim.Sockets.Min
			limUpdated = true
		}
		// cores defaults and corrections
		if lim.Cores.Min < 1 {
			lim.Cores.Min = 1
			limUpdated = true
		}
		if lim.Cores.Max < lim.Cores.Min {
			// if unset (0) use 2, otherwise at least min
			if lim.Cores.Max == 0 {
				lim.Cores.Max = max(2, lim.Cores.Min)
			} else {
				lim.Cores.Max = lim.Cores.Min
			}
			limUpdated = true
		}
		// ram
		if lim.RAM.Min == 0 && lim.RAM.Max == 0 {
			lim.RAM.Min = 1
			lim.RAM.Max = 2
			limUpdated = true
		}
		if limUpdated {
			limits[nodeName] = lim
			updated = true
		}
	}

	return updated
}

// getNodeDetailsFromCacheOrAPI fetches node details either from cache or from Proxmox API
func getNodeDetailsFromCacheOrAPI(ctx context.Context, settings *AppSettings) ([]*proxmox.NodeDetails, error) {
	// Get node information from Proxmox API
	apiURL := os.Getenv("PROXMOX_URL")
	apiTokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
	apiTokenSecret := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecure := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	// If API credentials are missing, we can't connect
	if apiURL == "" || apiTokenID == "" || apiTokenSecret == "" {
		return nil, fmt.Errorf("proxmox API credentials not configured")
	}

	// Initialize client
	client, err := proxmox.NewClientWithOptions(apiURL, apiTokenID, apiTokenSecret, insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	// Create timeout if context is nil
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	// Get node names
	nodeNames, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get node names: %w", err)
	}

	// Get details for each node
	var nodeDetails []*proxmox.NodeDetails
	for _, nodeName := range nodeNames {
		detail, err := proxmox.GetNodeDetailsWithContext(ctx, client, nodeName)
		if err != nil {
			log.Warn().Err(err).Str("node", nodeName).Msg("Failed to get details for node")
			continue
		}
		nodeDetails = append(nodeDetails, detail)
	}

	if len(nodeDetails) == 0 {
		return nil, fmt.Errorf("no node details could be retrieved")
	}

	return nodeDetails, nil
}

// updateLimitsHandler handles updating the resource limits for VMs and nodes.
func updateLimitsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the request body
	var payload struct {
		VM    VMLimits              `json:"vm"`
		Nodes map[string]NodeLimits `json:"nodes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate min <= max for all fields
	if payload.VM.Sockets.Min > payload.VM.Sockets.Max {
		http.Error(w, "Sockets min cannot exceed max", http.StatusBadRequest)
		return
	}
	if payload.VM.Cores.Min > payload.VM.Cores.Max {
		http.Error(w, "Cores min cannot exceed max", http.StatusBadRequest)
		return
	}
	if payload.VM.RAM.Min > payload.VM.RAM.Max {
		http.Error(w, "RAM min cannot exceed max", http.StatusBadRequest)
		return
	}
	if payload.VM.Disk.Min > payload.VM.Disk.Max {
		http.Error(w, "Disk min cannot exceed max", http.StatusBadRequest)
		return
	}

	// Read current settings
	settings, err := readSettings()
	if err != nil {
		http.Error(w, "Failed to read settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Initialize limits map if it doesn't exist
	if settings.Limits == nil {
		settings.Limits = make(map[string]NodeLimits)
	}

	// Update VM defaults (stored with key "vm")
	settings.Limits["vm"] = NodeLimits{
		Sockets: payload.VM.Sockets,
		Cores:   payload.VM.Cores,
		RAM:     payload.VM.RAM,
		Disk:    payload.VM.Disk,
	}

	// Update node-specific limits
	var validationMessages []string

	for nodeName, limits := range payload.Nodes {
		// Get node capacities for validation
		capacities, err := getNodeCapacities(r.Context(), nodeName)
		if err != nil {
			log.Warn().Err(err).Str("node", nodeName).Msg("Failed to get node capacities, using defaults")
			capacities = getDefaultCapacities()
		}

		// Create a temporary payload for validation
		nodePayload := struct {
			Node    string `json:"node"`
			Sockets MinMax `json:"sockets"`
			Cores   MinMax `json:"cores"`
			RAM     MinMax `json:"ram"`
			Disk    MinMax `json:"disk"`
		}{
			Node:    nodeName,
			Sockets: limits.Sockets,
			Cores:   limits.Cores,
			RAM:     limits.RAM,
			Disk:    limits.Disk,
		}

		// Validate and adjust limits
		msgs := validateAndAdjustLimits(&nodePayload, capacities)
		validationMessages = append(validationMessages, msgs...)

		// Update the node limits
		settings.Limits[nodeName] = NodeLimits{
			Sockets: nodePayload.Sockets,
			Cores:   nodePayload.Cores,
			RAM:     nodePayload.RAM,
			Disk:    nodePayload.Disk,
		}
	}

	// Save the updated settings
	if err := writeSettings(settings); err != nil {
		http.Error(w, "Failed to save settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Prepare response
	response := map[string]interface{}{
		"success": true,
		"message": "Limits updated successfully",
	}

	// Include validation messages if there were any adjustments
	if len(validationMessages) > 0 {
		response["validationMessages"] = validationMessages
	}

	json.NewEncoder(w).Encode(response)
}

// NodeCapacities holds information about a node's resource capacities
type NodeCapacities struct {
	Cores     int
	MaxCPU    int
	MaxMemory int
	Sockets   int
	MaxDiskGB int
}

// getNodeCapacities retrieves resource capacity information for a node
func getNodeCapacities(ctx context.Context, nodeName string) (*NodeCapacities, error) {
	// Initialize with empty values - we'll determine defaults only if we can't get real values
	result := &NodeCapacities{}

	// Get actual node capacity from Proxmox API
	apiURL := os.Getenv("PROXMOX_URL")
	apiTokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
	apiTokenSecret := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecure := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	// If any API credentials are missing, use calculated defaults
	if apiURL == "" || apiTokenID == "" || apiTokenSecret == "" {
		log.Warn().Msg("Proxmox API credentials not set, will use minimal capacity values")
		return getDefaultCapacities(), nil
	}

	// Create client
	client, err := proxmox.NewClientWithOptions(apiURL, apiTokenID, apiTokenSecret, insecure)
	if err != nil {
		log.Warn().Err(err).Msg("Could not create Proxmox client, will use minimal capacity values")
		return getDefaultCapacities(), nil
	}

	// Create context with timeout if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	// Get node details to validate limits against actual capacity
	nodeDetails, err := proxmox.GetNodeDetailsWithContext(ctx, client, nodeName)
	if err != nil {
		log.Warn().Err(err).Str("node", nodeName).Msg("Could not get node details, will use minimal capacity values")
		return getDefaultCapacities(), nil
	}

	// Successfully got node details
	result.MaxCPU = nodeDetails.MaxCPU
	result.Sockets = nodeDetails.Sockets
	result.Cores = nodeDetails.MaxCPU / nodeDetails.Sockets // Cores per socket
	result.MaxMemory = int(nodeDetails.MaxMemory / (1024 * 1024 * 1024))
	result.MaxDiskGB = int(nodeDetails.MaxDisk / (1024 * 1024 * 1024)) // Convert bytes to GB
	log.Info().
		Str("node", nodeName).
		Int("sockets", result.Sockets).
		Int("cores", result.Cores).
		Int("maxCPU", result.MaxCPU).
		Int("maxMemoryGB", result.MaxMemory).
		Int("maxDiskGB", result.MaxDiskGB).
		Msg("Retrieved node capacity details")

	return result, nil
}

// getDefaultCapacities returns sensible minimal default values for node capacities
func getDefaultCapacities() *NodeCapacities {
	return &NodeCapacities{
		MaxCPU:    1, // Start with minimal capacity
		MaxMemory: 1, // Start with minimal capacity (1 GB)
		Sockets:   1,
		Cores:     1,
	}
}

// validateAndAdjustLimits validates and adjusts limits against node capacities
func validateAndAdjustLimits(payload *struct {
	Node    string `json:"node"`
	Sockets MinMax `json:"sockets"`
	Cores   MinMax `json:"cores"`
	RAM     MinMax `json:"ram"`
	Disk    MinMax `json:"disk"`
}, capacities *NodeCapacities) []string {
	validationMessages := make([]string, 0)
	maxSockets := 1 // Default minimum values
	maxCoresPerSocket := 1
	maxMemoryGB := 1

	if capacities != nil {
		maxSockets = capacities.Sockets
		if maxSockets < 1 {
			maxSockets = 1 // Ensure at least 1 socket
		}

		if capacities.Cores > 0 {
			maxCoresPerSocket = capacities.Cores
		} else if capacities.MaxCPU >= maxSockets {
			maxCoresPerSocket = capacities.MaxCPU / maxSockets
		}

		maxMemoryGB = capacities.MaxMemory
		if maxMemoryGB < 1 {
			maxMemoryGB = 1 // Ensure at least 1 GB
		}
	}

	// Check max sockets against node capacity
	if payload.Sockets.Max > maxSockets {
		log.Info().Str("node", payload.Node).Int("requested", payload.Sockets.Max).Int("max", maxSockets).Msg("Limiting max sockets to node capacity")
		validationMessages = append(validationMessages, fmt.Sprintf("Max sockets limited to %d (node capacity)", maxSockets))
		payload.Sockets.Max = maxSockets
	}

	// Check max cores against node capacity (per socket)
	if payload.Cores.Max > maxCoresPerSocket {
		log.Info().Str("node", payload.Node).Int("requested", payload.Cores.Max).Int("max", maxCoresPerSocket).Msg("Limiting max cores to node capacity per socket")
		validationMessages = append(validationMessages, fmt.Sprintf("Max cores limited to %d per socket (node capacity)", maxCoresPerSocket))
		payload.Cores.Max = maxCoresPerSocket
	}

	// Check max RAM against node capacity
	if payload.RAM.Max > maxMemoryGB {
		log.Info().Str("node", payload.Node).Int("requested", payload.RAM.Max).Int("max", maxMemoryGB).Msg("Limiting max RAM to node capacity")
		validationMessages = append(validationMessages, fmt.Sprintf("Max RAM limited to %d GB (node capacity)", maxMemoryGB))
		payload.RAM.Max = maxMemoryGB
	}

	// Ensure min values for sockets and cores are at least 1
	if payload.Sockets.Min < 1 {
		validationMessages = append(validationMessages, "Min sockets must be at least 1, adjusted accordingly")
		payload.Sockets.Min = 1
	}
	if payload.Cores.Min < 1 {
		validationMessages = append(validationMessages, "Min cores must be at least 1, adjusted accordingly")
		payload.Cores.Min = 1
	}
	// RAM can be 0 but not negative
	if payload.RAM.Min < 0 {
		validationMessages = append(validationMessages, "Min RAM cannot be negative, set to 0")
		payload.RAM.Min = 0
	}

	// Ensure min values don't exceed max values
	if payload.Sockets.Min > payload.Sockets.Max {
		validationMessages = append(validationMessages, "Min sockets cannot exceed max, adjusted accordingly")
		payload.Sockets.Min = payload.Sockets.Max
	}
	if payload.Cores.Min > payload.Cores.Max {
		validationMessages = append(validationMessages, "Min cores cannot exceed max, adjusted accordingly")
		payload.Cores.Min = payload.Cores.Max
	}
	if payload.RAM.Min > payload.RAM.Max {
		validationMessages = append(validationMessages, "Min RAM cannot exceed max, adjusted accordingly")
		payload.RAM.Min = payload.RAM.Max
	}

	// Validate disk limits
	if payload.Disk.Min < 1 {
		validationMessages = append(validationMessages, "Minimum disk size must be at least 1GB, adjusted")
		payload.Disk.Min = 1
	}
	if payload.Disk.Max < 1 {
		validationMessages = append(validationMessages, "Maximum disk size must be at least 1GB, adjusted")
		payload.Disk.Max = 1
	}
	if payload.Disk.Min > payload.Disk.Max {
		validationMessages = append(validationMessages, "Min disk size cannot exceed max, adjusted accordingly")
		payload.Disk.Min = payload.Disk.Max
	}

	// If we have node capacity information, validate against it
	if capacities != nil && capacities.MaxDiskGB > 0 {
		// Ensure max disk doesn't exceed node capacity
		if payload.Disk.Max > capacities.MaxDiskGB {
			validationMessages = append(validationMessages, fmt.Sprintf("Maximum disk size (%dGB) exceeds node capacity (%dGB), adjusted", payload.Disk.Max, capacities.MaxDiskGB))
			payload.Disk.Max = capacities.MaxDiskGB
		}
		// Ensure min disk doesn't exceed the adjusted max
		if payload.Disk.Min > payload.Disk.Max {
			payload.Disk.Min = payload.Disk.Max
		}
	}

	return validationMessages
}
