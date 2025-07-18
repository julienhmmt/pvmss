package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"pvmss/backend/proxmox"
)

// Storage represents a Proxmox storage.
type Storage struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Nodes   string `json:"nodes"`
}

// storageHandler routes the requests to the appropriate handler.
func storageHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "storageHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Storage handler invoked")
	switch r.Method {
	case http.MethodGet:
		getStoragesHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getStorages is a helper function to fetch and parse storages from Proxmox.
func getStorages() ([]Storage, error) {
	// Use the global proxmox client
	if proxmoxClient == nil {
		log.Error().Msg("Proxmox client not initialized")
		return nil, fmt.Errorf("proxmox client not initialized")
	}

	storagesResult, err := proxmox.GetStorages(proxmoxClient)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get storage list")
		return nil, err
	}

	var allStorages []Storage
	storagesMap, mapOk := storagesResult.(map[string]interface{})
	if !mapOk {
		log.Error().Interface("result", storagesResult).Msg("Invalid storages result format")
		return nil, fmt.Errorf("invalid storages result format")
	}
	
	data, dataOk := storagesMap["data"].([]interface{})
	if !dataOk {
		log.Error().Interface("storagesMap", storagesMap).Msg("Invalid storages data format")
		return nil, fmt.Errorf("invalid storages data format")
	}
	
	for _, item := range data {
		storageItem, itemOk := item.(map[string]interface{})
		if !itemOk {
			log.Debug().Interface("item", item).Msg("Storage item is not a valid map, skipping")
			continue
		}
		
		// Récupérer les propriétés de stockage avec gestion des erreurs de type
		storageName, nameOk := storageItem["storage"].(string)
		storageType, typeOk := storageItem["type"].(string)
		storageContent, contentOk := storageItem["content"].(string)
		
		if !nameOk || !typeOk || !contentOk {
			log.Debug().Interface("storageItem", storageItem).Msg("Storage item missing required fields, skipping")
			continue
		}
		
		storage := Storage{
			Name:    storageName,
			Type:    storageType,
			Content: storageContent,
		}
		
		if nodes, ok := storageItem["nodes"].(string); ok {
			storage.Nodes = nodes
		}
		allStorages = append(allStorages, storage)
	}
	return allStorages, nil
}

// getStoragesHandler handles fetching all storages.
func getStoragesHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "getStoragesHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Fetching all storages")
	allStorages, err := getStorages()
	if err != nil {
		http.Error(w, "Failed to retrieve storages from Proxmox", http.StatusInternalServerError)
		return
	}

	// Filter for storages that can store virtual machine disks ('images' or 'rootdir')
	var vmStorages []Storage
	for _, s := range allStorages {
		if strings.Contains(s.Content, "images") || strings.Contains(s.Content, "rootdir") {
			vmStorages = append(vmStorages, s)
		}
	}
	log.Debug().Int("total", len(allStorages)).Int("vmEligible", len(vmStorages)).Msg("Filtered storages for VM eligibility")

	log.Info().Int("total", len(allStorages)).Int("vmCount", len(vmStorages)).Msg("Storage filtering completed")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Storages []Storage `json:"storages"`
	}{Storages: vmStorages})
}
