package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"pvmss/backend/proxmox"
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

// getLimitsHandler returns the current resource limits for all nodes.
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

	// Get node information from the cache or from Proxmox API if needed
	nodeDetails, err := getNodeDetailsFromCacheOrAPI(r.Context(), settings)
	if err != nil {
		// Log the error but continue - we can still return the existing limits
		log.Warn().Err(err).Msg("Failed to get node details, will use existing limits only")
	}

	// For any new nodes discovered, create default limits
	if len(nodeDetails) > 0 {
		for _, node := range nodeDetails {
			// Only create limits if they don't exist yet
			if _, exists := settings.Limits[node.Node]; !exists {
				maxCPU := node.MaxCPU
				maxMemoryGB := int(node.MaxMemory / (1024 * 1024 * 1024))
				
				// Set default limits (min=1, max=node capacity)
				settings.Limits[node.Node] = NodeLimits{
					Sockets: MinMax{Min: 1, Max: maxCPU},
					Cores:   MinMax{Min: 1, Max: maxCPU},
					RAM:     MinMax{Min: 1, Max: maxMemoryGB},
				}
				log.Info().Str("node", node.Node).Msg("Created default limits for new node")
			}
		}
	}

	// Ensure reasonable defaults for all limits
	updated := ensureReasonableLimits(settings.Limits)
	
	// Persist any adjustments so future loads are correct
	if updated {
		if err := writeSettings(settings); err != nil {
			log.Error().Err(err).Msg("Failed to persist defaulted limits")
		}
	}

	// Return the limits
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settings.Limits); err != nil {
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

// updateLimitsHandler handles updating the resource limits for a specific node.
func updateLimitsHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Node    string `json:"node"`
		Sockets MinMax `json:"sockets"`
		Cores   MinMax `json:"cores"`
		RAM     MinMax `json:"ram"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the node parameter
	if payload.Node == "" {
		log.Error().Msg("Node name is missing in request")
		http.Error(w, "Node name is required", http.StatusBadRequest)
		return
	}

	log.Info().Str("node", payload.Node).Interface("limits", payload).Msg("Updating limits")
	
	// Get current settings
	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings for update")
		http.Error(w, "Failed to read settings for update", http.StatusInternalServerError)
		return
	}

	// Initialize limits map if it doesn't exist
	if settings.Limits == nil {
		settings.Limits = make(map[string]NodeLimits)
	}

	// Get node capacity constraints - either from cache or from Proxmox API
	// Optionally get the node details to validate against real capacity
	nodeCapacities, err := getNodeCapacities(r.Context(), payload.Node)
	validationMessages := validateAndAdjustLimits(&payload, nodeCapacities)

	// Update limits in settings
	limits := NodeLimits{
		Sockets: payload.Sockets,
		Cores:   payload.Cores,
		RAM:     payload.RAM,
	}

	settings.Limits[payload.Node] = limits

	if err := writeSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to write updated settings")
		http.Error(w, "Failed to write updated settings", http.StatusInternalServerError)
		return
	}

	// Return success with any validation messages
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"success": true,
		"message": "Limits updated successfully",
		"limits":  limits,
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
	log.Info().Str("node", nodeName).Int("sockets", result.Sockets).Int("cores", result.Cores).Int("maxCPU", result.MaxCPU).Int("maxMemoryGB", result.MaxMemory).Msg("Retrieved node capacity details")

	return result, nil
}

// getDefaultCapacities returns sensible minimal default values for node capacities
func getDefaultCapacities() *NodeCapacities {
	return &NodeCapacities{
		MaxCPU:    1,  // Start with minimal capacity
		MaxMemory: 1,  // Start with minimal capacity (1 GB)
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

	return validationMessages
}
