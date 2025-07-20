package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"pvmss/logger"
)

// LimitsSettings represents the resource limits configuration for VMs and nodes.
// It contains default limits for VMs and per-node specific limits.
type LimitsSettings struct {
	// VM contains the default resource limits for virtual machines
	VM    ResourceLimits            `json:"vm"`

	// Nodes contains node-specific resource limits, keyed by node name
	Nodes map[string]ResourceLimits `json:"nodes,omitempty"`
}

// defaultVMLimits returns the default resource limits for virtual machines.
// These defaults provide reasonable starting points for new VMs:
// - 1-1 vCPU sockets
// - 1-2 CPU cores
// - 1-4 GB RAM
// - 1-16 GB disk (optional, only for VMs)
func defaultVMLimits() ResourceLimits {
	return ResourceLimits{
		Sockets: MinMax{Min: 1, Max: 1},
		Cores:   MinMax{Min: 1, Max: 2},
		RAM:     MinMax{Min: 1, Max: 4}, // GB
		Disk:    &MinMax{Min: 1, Max: 16}, // GB
	}
}

// defaultNodeLimits returns the default resource limits for Proxmox nodes.
// These defaults provide reasonable starting points for node resources:
// - 1-1 vCPU sockets
// - 1-2 CPU cores
// - 1-4 GB RAM
// No disk limits by default for nodes
func defaultNodeLimits() ResourceLimits {
	return ResourceLimits{
		Sockets: MinMax{Min: 1, Max: 1},
		Cores:   MinMax{Min: 1, Max: 2},
		RAM:     MinMax{Min: 1, Max: 4}, // GB
		Disk:    nil, // Nodes don't have disk limits
	}
}

// resourceLimitsToMap converts a ResourceLimits struct to a map[string]interface{} for JSON serialization.
// This is used when saving limits to the settings file.
// The output format matches the expected structure in the frontend.
func resourceLimitsToMap(limits ResourceLimits) map[string]interface{} {
	result := map[string]interface{}{
		"sockets": map[string]int{
			"min": limits.Sockets.Min,
			"max": limits.Sockets.Max,
		},
		"cores": map[string]int{
			"min": limits.Cores.Min,
			"max": limits.Cores.Max,
		},
		"ram": map[string]int{
			"min": limits.RAM.Min,
			"max": limits.RAM.Max,
		},
	}

	if limits.Disk != nil {
		result["disk"] = map[string]int{
			"min": limits.Disk.Min,
			"max": limits.Disk.Max,
		}
	}

	return result
}

// convertToResourceLimits converts a raw map of limits (from JSON) to a properly typed ResourceLimits struct.
// It handles type assertions and provides detailed error messages if the input format is invalid.
// The function is lenient with the disk field, making it optional since nodes don't have disk limits.
//
// Parameters:
//   - rawLimits: map containing the raw limit values from JSON (e.g., {"sockets": {"min":1, "max":2}})
//
// Returns:
//   - ResourceLimits: The parsed and validated limits
//   - error: Descriptive error if parsing fails
func convertToResourceLimits(rawLimits map[string]interface{}) (ResourceLimits, error) {
	logger := logger.Get()
	logger.Debug().Interface("rawLimits", rawLimits).Msg("Converting raw limits to ResourceLimits")
	
	var limits ResourceLimits

	// extractMinMax is a helper function that extracts MinMax values from a nested map.
	// It handles type assertions and provides detailed error messages.
	//
	// Parameters:
	//   - data: The parent map containing the MinMax values
	//   - key: The key in the parent map that contains the MinMax values
	//
	// Returns:
	//   - MinMax: The parsed MinMax values
	//   - error: If the MinMax values are missing or invalid
	extractMinMax := func(data map[string]interface{}, key string) (MinMax, error) {
		if val, ok := data[key].(map[string]interface{}); ok {
			min, minOk := val["min"].(float64)
			max, maxOk := val["max"].(float64)
			if minOk && maxOk {
				return MinMax{Min: int(min), Max: int(max)}, nil
			}
		}
		return MinMax{}, fmt.Errorf("invalid %s format", key)
	}

	// Extract Sockets
	sockets, err := extractMinMax(rawLimits, "sockets")
	if err != nil {
		return limits, fmt.Errorf("invalid sockets: %v", err)
	}
	limits.Sockets = sockets

	// Extract Cores
	cores, err := extractMinMax(rawLimits, "cores")
	if err != nil {
		return limits, fmt.Errorf("invalid cores: %v", err)
	}
	limits.Cores = cores

	// Extract RAM
	ram, err := extractMinMax(rawLimits, "ram")
	if err != nil {
		return limits, fmt.Errorf("invalid RAM: %v", err)
	}
	limits.RAM = ram

	// Extract Disk if it exists (for VMs)
	if _, ok := rawLimits["disk"].(map[string]interface{}); ok {
		disk, err := extractMinMax(rawLimits, "disk")
		if err != nil {
			return limits, fmt.Errorf("invalid disk: %v", err)
		}
		limits.Disk = &disk
	}

	return limits, nil
}

// limitsHandler is the main HTTP handler for the limits API endpoint.
// It routes requests to the appropriate handler based on the HTTP method:
//   - GET: Retrieve current limits
//   - POST: Update limits
//   - PUT: Reset limits to defaults
//
// All other methods return a 405 Method Not Allowed response.
func limitsHandler(w http.ResponseWriter, r *http.Request) {
	logger := logger.Get()
	logger.Info().
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

// getLimitsHandler handles GET requests to retrieve the current resource limits.
// It reads the current settings and returns them in a structured JSON format.
// If an error occurs while reading or parsing the settings, it returns a 500 Internal Server Error.
func getLimitsHandler(w http.ResponseWriter, _ *http.Request) {
	logger := logger.Get()
	logger.Info().Msg("Getting resource limits")
	settings, err := readSettings()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to read settings")
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	// Convert raw limits to proper structure
	response := map[string]interface{}{
		"success": true,
		"limits":  make(map[string]interface{}),
	}

	// Process VM limits
	if vmRaw, ok := settings.Limits["vm"].(map[string]interface{}); ok {
		logger.Debug().
			Interface("raw_vm_limits", vmRaw).
			Msg("Converting VM limits from raw format")
			
		vmLimits, err := convertToResourceLimits(vmRaw)
		if err != nil {
			logger.Error().
				Err(err).
				Interface("raw_vm_limits", vmRaw).
				Msg("Failed to convert VM limits")
			http.Error(w, "Failed to process VM limits", http.StatusInternalServerError)
			return
		}
		
		logger.Debug().
			Interface("converted_vm_limits", vmLimits).
			Msg("Successfully converted VM limits")
			
		response["limits"].(map[string]interface{})["vm"] = vmLimits
	} else {
		logger.Warn().Msg("No VM limits found in settings")
	}

	// Process node limits
	if nodesRaw, ok := settings.Limits["nodes"].(map[string]interface{}); ok {
		logger.Debug().
			Int("node_count", len(nodesRaw)).
			Msg("Processing node limits")
			
		nodeLimits := make(map[string]ResourceLimits)
		for nodeID, nodeRaw := range nodesRaw {
			if nodeData, ok := nodeRaw.(map[string]interface{}); ok {
				logger.Debug().
					Str("node_id", nodeID).
					Interface("raw_limits", nodeData).
					Msg("Converting node limits")
					
				nodeLimit, err := convertToResourceLimits(nodeData)
				if err != nil {
					logger.Error().
						Err(err).
						Str("node_id", nodeID).
						Interface("raw_limits", nodeData).
						Msg("Failed to convert node limits")
					continue
				}
				
				logger.Debug().
					Str("node_id", nodeID).
					Interface("converted_limits", nodeLimit).
					Msg("Successfully converted node limits")
					
				nodeLimits[nodeID] = nodeLimit
			} else {
				logger.Warn().
					Str("node_id", nodeID).
					Interface("invalid_node_data", nodeRaw).
					Msg("Invalid node data format")
			}
		}
		
		logger.Debug().
			Int("processed_nodes", len(nodeLimits)).
			Msg("Finished processing node limits")
			
		response["limits"].(map[string]interface{})["nodes"] = nodeLimits
	} else {
		logger.Debug().Msg("No node limits found in settings")
	}

	// Log the final response before sending
	logger.Debug().
		Interface("response_data", response).
		Msg("Sending response with limits")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error().
			Err(err).
			Interface("response_data", response).
			Msg("Error encoding limits to JSON")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	logger.Info().
		Int("vm_limits_size", len(settings.Limits["vm"].(map[string]interface{}))).
		Int("node_limits_count", len(settings.Limits["nodes"].(map[string]interface{}))).
		Msg("Successfully sent limits response")
}

// updateLimitsHandler handles POST requests to update resource limits.
// It expects a JSON payload with the following structure:
// {
//   "entityId": "vm" or node name,
//   "sockets": {"min": 1, "max": 2},
//   "cores": {"min": 1, "max": 4},
//   "ram": {"min": 1, "max": 8},
//   "disk": {"min": 10, "max": 100}  // Optional, only for VMs
// }
//
// On success, it returns the updated limits in the response.
// Returns 400 for invalid requests and 500 for server errors.
func updateLimitsHandler(w http.ResponseWriter, r *http.Request) {
	logger := logger.Get()
	
	var requestData struct {
		EntityID string `json:"entityId"`
		Sockets  MinMax  `json:"sockets"`
		Cores    MinMax  `json:"cores"`
		RAM      MinMax  `json:"ram"`
		Disk     MinMax  `json:"disk"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		logger.Error().Err(err).Msg("Error decoding limits update request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Charger les paramètres actuels
	settings, err := readSettings()
	if err != nil {
		logger.Error().Err(err).Msg("Error reading settings")
		http.Error(w, "Error reading settings", http.StatusInternalServerError)
		return
	}

	// Initialiser la structure des limites si elle n'existe pas
	if settings.Limits == nil {
		settings.Limits = make(map[string]interface{})
	}

	// Convertir les limites en format de sauvegarde
	limitsMap := map[string]interface{}{
		"sockets": map[string]int{"min": requestData.Sockets.Min, "max": requestData.Sockets.Max},
		"cores":   map[string]int{"min": requestData.Cores.Min, "max": requestData.Cores.Max},
		"ram":     map[string]int{"min": requestData.RAM.Min, "max": requestData.RAM.Max},
	}

	// Pour les VMs, ajouter le disque
	if requestData.EntityID == "vm" {
		limitsMap["disk"] = map[string]int{"min": requestData.Disk.Min, "max": requestData.Disk.Max}
	}

	// Mettre à jour les limites pour l'entité spécifiée
	if requestData.EntityID == "vm" {
		settings.Limits["vm"] = limitsMap
	} else {
		nodes, ok := settings.Limits["nodes"].(map[string]interface{})
		if !ok {
			nodes = make(map[string]interface{})
			settings.Limits["nodes"] = nodes
		}
		nodes[requestData.EntityID] = limitsMap
	}

	// Sauvegarder les paramètres
	if err := writeSettings(settings); err != nil {
		logger.Error().Err(err).Msg("Error saving settings")
		http.Error(w, "Error saving settings", http.StatusInternalServerError)
		return
	}

	logger.Info().
		Str("entity", requestData.EntityID).
		Interface("limits", limitsMap).
		Msg("Limits updated successfully")

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Limits updated successfully",
		"limits":  limitsMap,
	})
}

// resetLimitsHandler handles PUT requests to reset resource limits to their default values.
// It expects a JSON payload with the following structure:
// {
//   "entityId": "vm" or node name
// }
//
// For VMs, it resets to defaultVMLimits().
// For nodes, it resets to defaultNodeLimits().
//
// On success, it returns the reset limits in the response.
// Returns 400 for invalid requests and 500 for server errors.
func resetLimitsHandler(w http.ResponseWriter, r *http.Request) {
	logger := logger.Get()
	logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Reset limits handler called")

	var requestData struct {
		EntityID string `json:"entity_id"` // "vm" or node name
	}

	// Log the raw request body for debugging
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Restore body for decoding
	
	logger.Debug().
		Str("raw_request_body", string(bodyBytes)).
		Msg("Raw reset limits request body")

	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&requestData); err != nil {
		logger.Error().
			Err(err).
			Str("request_body", string(bodyBytes)).
			Msg("Error decoding limits reset request")
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	
	logger.Debug().
		Str("entity_id", requestData.EntityID).
		Msg("Decoded reset limits request")

	isVM := requestData.EntityID == "vm"
	
	logger.Debug().
		Bool("is_vm", isVM).
		Msg("Determined entity type")

	settings, err := readSettings()
	if err != nil {
		logger.Error().
			Err(err).
			Str("entity_id", requestData.EntityID).
			Msg("Failed to read settings")
		http.Error(w, "Failed to read settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	logger.Debug().
		Interface("current_settings_limits", settings.Limits).
		Msg("Read current settings")

	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	if isVM {
		// Reset VM limits to defaults
		logger.Debug().Msg("Resetting VM limits to defaults")
		defLimits := defaultVMLimits()
		
		logger.Debug().
			Interface("default_vm_limits", defLimits).
			Msg("Default VM limits generated")
		
		settings.Limits["vm"] = resourceLimitsToMap(defLimits)
		
		logger.Debug().
			Interface("updated_vm_limits", settings.Limits["vm"]).
			Msg("Updated VM limits in settings")
	} else {
		// Reset node limits to defaults
		logger.Debug().
			Str("node", requestData.EntityID).
			Msg("Resetting node limits to defaults")
		
		// Use default limits for node
		defLimits := defaultNodeLimits()
		
		logger.Debug().
			Interface("default_node_limits", defLimits).
			Str("node", requestData.EntityID).
			Msg("Default limits calculated for node")
		
		// Ensure nodes map exists in settings
		nodes, ok := settings.Limits["nodes"].(map[string]interface{})
		if !ok {
			logger.Debug().Msg("Initializing empty nodes map in settings")
			settings.Limits["nodes"] = make(map[string]interface{})
			nodes = settings.Limits["nodes"].(map[string]interface{})
		}
		
		// Convert limits to map and update node limits
		nodeLimitsMap := resourceLimitsToMap(defLimits)
		nodes[requestData.EntityID] = nodeLimitsMap
		
		logger.Debug().
			Str("node", requestData.EntityID).
			Interface("node_limits", nodeLimitsMap).
			Msg("Updated node limits in settings")
	}

	// Log settings before saving
	logger.Debug().
		Str("entity", requestData.EntityID).
		Interface("settings_before_save", settings).
		Msg("Saving settings after reset")

	if err := writeSettings(settings); err != nil {
		logger.Error().
			Err(err).
			Str("entity", requestData.EntityID).
			Interface("settings", settings).
			Msg("Error saving settings after reset")
		http.Error(w, "Error saving settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info().
		Str("entity", requestData.EntityID).
		Msg("Successfully reset and saved limits")

	w.Header().Set("Content-Type", "application/json")
	// Get the updated limits to include in the response
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
	
	// Build the response with the updated limits
	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully reset limits for %s", requestData.EntityID),
		"limits":  limitsValue,
	}

	// Send the response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error().
			Err(err).
			Interface("response", response).
			Msg("Error encoding response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
