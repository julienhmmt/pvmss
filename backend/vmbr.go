package main

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	"pvmss/backend/proxmox"
)

// VMBRInfo holds information about a Proxmox network bridge.

type VMBRInfo struct {
	Name        string `json:"name"`
	Node        string `json:"node"`
	Description string `json:"description"`
}

// allVmbrsHandler fetches all VMBRs from all nodes.
func allVmbrsHandler(w http.ResponseWriter, r *http.Request) {
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

	nodes, err := proxmox.GetNodeNames(client)
	if err != nil {
		http.Error(w, "Failed to retrieve nodes from Proxmox", http.StatusInternalServerError)
		return
	}

	var allVMBRs []VMBRInfo
	uniqueVMBRs := make(map[string]bool)

	for _, node := range nodes {
		log.Info().Msgf("Querying node '%s' for network bridges...", node)
		networkResult, err := proxmox.GetVMBRs(client, node)
		if err != nil {
			log.Warn().Err(err).Msgf("Could not get network list for node %s.", node)
			continue
		}

		if data, ok := networkResult["data"].([]interface{}); ok {
			for _, item := range data {
				if netItem, ok := item.(map[string]interface{}); ok {
					if ntype, ok := netItem["type"].(string); ok && ntype == "bridge" {
						iface := netItem["iface"].(string)
						if !uniqueVMBRs[iface] {
							description := "N/A"
							if comment, ok := netItem["comment"].(string); ok {
								description = comment
							} else if comment, ok := netItem["comments"].(string); ok {
								description = comment
							}

							vmbr := VMBRInfo{
								Name:        iface,
								Node:        node,
								Description: description,
							}
							allVMBRs = append(allVMBRs, vmbr)
							uniqueVMBRs[iface] = true
						}
					}
				}
			}
		}
	}

	log.Info().Msgf("Found %d total VMBRs.", len(allVMBRs))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		VMBRs []VMBRInfo `json:"vmbrs"`
	}{VMBRs: allVMBRs})
}
