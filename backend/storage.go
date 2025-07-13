package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"pvmss/backend/proxmox"
)

// Storage represents a Proxmox storage.
type Storage struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

// storageHandler routes the requests to the appropriate handler.
func storageHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getStoragesHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getStoragesHandler handles fetching all storages.
func getStoragesHandler(w http.ResponseWriter, r *http.Request) {
	apiURL := os.Getenv("PROXMOX_URL")
	apiTokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
	apiTokenSecret := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecure := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	client, err := proxmox.NewClient(apiURL, apiTokenID, apiTokenSecret, insecure)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create proxmox client")
		http.Error(w, "Failed to connect to Proxmox API", http.StatusInternalServerError)
		return
	}

	storagesResult, err := proxmox.GetStorages(client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get storage list")
		http.Error(w, "Failed to retrieve storages from Proxmox", http.StatusInternalServerError)
		return
	}

	var allStorages []Storage
	if storagesMap, ok := storagesResult.(map[string]interface{}); ok {
		if data, ok := storagesMap["data"].([]interface{}); ok {
			for _, item := range data {
				if storageItem, ok := item.(map[string]interface{}); ok {
					allStorages = append(allStorages, Storage{
						Name:    storageItem["storage"].(string),
						Type:    storageItem["type"].(string),
						Content: storageItem["content"].(string),
					})
				}
			}
		}
	}

	// Filter for storages that can store virtual machine disks ('images' or 'rootdir')
	var vmStorages []Storage
	for _, s := range allStorages {
		if strings.Contains(s.Content, "images") || strings.Contains(s.Content, "rootdir") {
			vmStorages = append(vmStorages, s)
		}
	}

	log.Info().Msgf("Found %d storages, %d can be used for VMs", len(allStorages), len(vmStorages))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Storages []Storage `json:"storages"`
	}{Storages: vmStorages})
}
