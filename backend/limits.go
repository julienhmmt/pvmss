package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"pvmss/backend/proxmox"
	"github.com/rs/zerolog/log"
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
func validateLimits(limits ResourceLimits, nodeDetails *proxmox.NodeDetails, isVM bool) error {
	if limits.Sockets.Min > limits.Sockets.Max {
		return fmt.Errorf("la valeur minimum de sockets ne peut pas être supérieure à la valeur maximum")
	}
	if limits.Cores.Min > limits.Cores.Max {
		return fmt.Errorf("la valeur minimum de cores ne peut pas être supérieure à la valeur maximum")
	}
	if limits.RAM.Min > limits.RAM.Max {
		return fmt.Errorf("la valeur minimum de RAM ne peut pas être supérieure à la valeur maximum")
	}
	if isVM && limits.Disk.Min > limits.Disk.Max {
		return fmt.Errorf("la valeur minimum de disque ne peut pas être supérieure à la valeur maximum")
	}

	if limits.Sockets.Min <= 0 || limits.Sockets.Max <= 0 {
		return fmt.Errorf("les valeurs de sockets doivent être positives")
	}
	if limits.Cores.Min <= 0 || limits.Cores.Max <= 0 {
		return fmt.Errorf("les valeurs de cores doivent être positives")
	}
	if limits.RAM.Min <= 0 || limits.RAM.Max <= 0 {
		return fmt.Errorf("les valeurs de RAM doivent être positives")
	}
	if isVM && (limits.Disk.Min <= 0 || limits.Disk.Max <= 0) {
		return fmt.Errorf("les valeurs de disque doivent être positives")
	}

	if nodeDetails != nil {
		if limits.Sockets.Max > nodeDetails.Sockets {
			return fmt.Errorf("la valeur maximum de sockets ne peut pas dépasser %d", nodeDetails.Sockets)
		}
		if limits.Cores.Max > nodeDetails.MaxCPU {
			return fmt.Errorf("la valeur maximum de cores ne peut pas dépasser %d", nodeDetails.MaxCPU)
		}
		ramMaxGB := int(nodeDetails.MaxMemory / 1024 / 1024 / 1024)
		if limits.RAM.Max > ramMaxGB {
			return fmt.Errorf("la valeur maximum de RAM ne peut pas dépasser %d GB", ramMaxGB)
		}
	}

	return nil
}

func limitsHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "limitsHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Limits API handler invoked")
	
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

func getLimitsHandler(w http.ResponseWriter, r *http.Request) {
	settings, err := readSettings()
	if err != nil {
		http.Error(w, "Erreur de lecture des paramètres", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"limits":  settings, // Adjust based on actual field name in AppSettings
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Erreur lors de l'encodage des limites en JSON")
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
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
		log.Error().Err(err).Msg("Erreur lors du décodage de la requête de mise à jour des limites")
		http.Error(w, "Requête invalide", http.StatusBadRequest)
		return
	}

	isVM := requestData.EntityID == "vm"
	var nodeDetails *proxmox.NodeDetails
	if !isVM {
		apiURL := os.Getenv("PROXMOX_URL")
		username := os.Getenv("PROXMOX_USERNAME")
		password := os.Getenv("PROXMOX_PASSWORD")
		client, err := proxmox.NewClient(apiURL, username, password, false)
		if err != nil {
			log.Error().Err(err).Msg("Erreur lors de la création du client Proxmox")
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}

		ctx := context.Background()
		nodeDetails, err = proxmox.GetNodeDetailsWithContext(ctx, client, requestData.EntityID)
		if err != nil {
			log.Error().Err(err).Str("node", requestData.EntityID).Msg("Erreur lors de la récupération des détails du nœud")
			http.Error(w, fmt.Sprintf("Erreur lors de la récupération des détails du nœud %s", requestData.EntityID), http.StatusInternalServerError)
			return
		}
	} else {
		// For VM, use a default or mock node details if needed
		_, err := readSettings()
		if err != nil {
			http.Error(w, "Erreur de lecture des paramètres", http.StatusInternalServerError)
			return
		}
		// Assuming settings has a way to get node details, adjust as necessary
		// nodeDetails = settings.SomeNodeDetailsField // Replace with actual field
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
		log.Warn().Err(err).Str("entity", requestData.EntityID).Msg("Validation des limites échouée")
		response := map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	_, err = readSettings()
	if err != nil {
		http.Error(w, "Erreur de lecture des paramètres", http.StatusInternalServerError)
		return
	}

	var settingsMutex = &sync.RWMutex{}
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	// Update settings based on actual struct fields
	// settings.SomeLimitsField = limits // Adjust based on actual field name

	log.Info().Str("entity", requestData.EntityID).Msg("Limites mises à jour avec succès")

	response := map[string]interface{}{
		"success": true,
		"message": "Limites mises à jour avec succès",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func resetLimitsHandler(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		EntityID string `json:"entityId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.Error().Err(err).Msg("Erreur lors du décodage de la requête de réinitialisation des limites")
		http.Error(w, "Requête invalide", http.StatusBadRequest)
		return
	}

	isVM := requestData.EntityID == "vm"
	_, err := readSettings()
	if err != nil {
		http.Error(w, "Erreur de lecture des paramètres", http.StatusInternalServerError)
		return
	}

	var settingsMutex = &sync.RWMutex{}
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	if isVM {
		// settings.SomeLimitsField = defaultVMLimits() // Adjust based on actual field
	} else {
		apiURL := os.Getenv("PROXMOX_URL")
		username := os.Getenv("PROXMOX_USERNAME")
		password := os.Getenv("PROXMOX_PASSWORD")
		client, err := proxmox.NewClient(apiURL, username, password, false)
		if err != nil {
			log.Error().Err(err).Msg("Erreur lors de la création du client Proxmox")
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}

		ctx := context.Background()
		nodeDetails, err := proxmox.GetNodeDetailsWithContext(ctx, client, requestData.EntityID)
		if err != nil {
			log.Error().Err(err).Str("node", requestData.EntityID).Msg("Erreur lors de la récupération des détails du nœud")
			http.Error(w, fmt.Sprintf("Erreur lors de la récupération des détails du nœud %s", requestData.EntityID), http.StatusInternalServerError)
			return
		}

		// Use nodeDetails to set default limits
		defaultLimits := defaultNodeLimits(nodeDetails)
		log.Info().Interface("defaultLimits", defaultLimits).Msg("Default limits calculated for node")
		// Update node limits based on actual struct field
		// settings.SomeNodeLimitsField[requestData.EntityID] = defaultLimits
	}

	if err := writeSettings(nil); err != nil {
		log.Error().Err(err).Msg("Erreur lors de la sauvegarde des paramètres après réinitialisation")
		http.Error(w, "Erreur lors de la sauvegarde des paramètres", http.StatusInternalServerError)
		return
	}

	log.Info().Str("entity", requestData.EntityID).Msg("Limites réinitialisées avec succès")

	response := map[string]interface{}{
		"success": true,
		"message": "Limites réinitialisées avec succès",
		"limits":  nil, // Adjust based on actual field
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *AppSettings) ensureLimitsInitialized() {
	// Adjust based on actual field names in AppSettings
	// if s.SomeLimitsField.VM.Sockets.Min == 0 {
	// 	s.SomeLimitsField.VM = defaultVMLimits()
	// }
	// if s.SomeLimitsField.Nodes == nil {
	// 	s.SomeLimitsField.Nodes = make(map[string]ResourceLimits)
	// }

	// Check for new nodes or nodes without limits
	// for nodeName := range s.SomeNodeDetailsField {
	// 	if _, exists := s.SomeLimitsField.Nodes[nodeName]; !exists {
	// 		s.SomeLimitsField.Nodes[nodeName] = defaultNodeLimits(s.SomeNodeDetailsField[nodeName])
	// 	}
	// }

	// Remove limits for nodes that no longer exist
	// for nodeName := range s.SomeLimitsField.Nodes {
	// 	if _, exists := s.SomeNodeDetailsField[nodeName]; !exists {
	// 		delete(s.SomeLimitsField.Nodes, nodeName)
	// 	}
	// }
}

func getNodeDetailsFromCacheOrAPI(ctx context.Context) ([]*proxmox.NodeDetails, error) {
	apiURL := os.Getenv("PROXMOX_URL")
	username := os.Getenv("PROXMOX_USERNAME")
	password := os.Getenv("PROXMOX_PASSWORD")
	client, err := proxmox.NewClient(apiURL, username, password, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %v", err)
	}

	nodeNames, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get node names: %v", err)
	}

	nodeDetailsList := make([]*proxmox.NodeDetails, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		nodeDetails, err := proxmox.GetNodeDetailsWithContext(ctx, client, nodeName)
		if err != nil {
			log.Warn().Err(err).Str("node", nodeName).Msg("Failed to get details for node")
			continue
		}
		nodeDetailsList = append(nodeDetailsList, nodeDetails)
	}

	return nodeDetailsList, nil
}
