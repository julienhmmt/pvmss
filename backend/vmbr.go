package main

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"pvmss/proxmox"
	"pvmss/state"
)

// VMBRInfo holds information about a Proxmox network bridge.
type VMBRInfo struct {
	Name        string `json:"name"`
	Node        string `json:"node"`
	Description string `json:"description"`
}

// allVmbrsHandler fetches all VMBRs from all nodes.
func allVmbrsHandler(w http.ResponseWriter, r *http.Request) {
	// Get the Proxmox client from state
	proxmoxClient := state.GetProxmoxClient()
	if proxmoxClient == nil {
		log.Error().Msg("Proxmox client not initialized")
		http.Error(w, "Failed to connect to Proxmox API", http.StatusInternalServerError)
		return
	}

	nodes, err := proxmox.GetNodeNames(proxmoxClient)
	if err != nil {
		log.Error().Err(err).Msg("Failed to retrieve nodes from Proxmox")
		http.Error(w, "Failed to retrieve nodes from Proxmox", http.StatusInternalServerError)
		return
	}

	var allVMBRs []VMBRInfo
	uniqueVMBRs := make(map[string]bool)

	for _, node := range nodes {
		log.Info().Str("node", node).Msg("Querying node for network bridges")
		networkResult, err := proxmox.GetVMBRs(proxmoxClient, node)
		if err != nil {
			log.Warn().Err(err).Str("node", node).Msg("Could not get network list for node")
			continue
		}

		if data, ok := networkResult["data"].([]interface{}); ok {
			for _, item := range data {
				if netItem, ok := item.(map[string]interface{}); ok {
					// Vérifier le type de l'interface réseau
					ntype, ntypeOk := netItem["type"].(string)
					if !ntypeOk || ntype != "bridge" {
						continue
					}

					// Récupérer le nom de l'interface
					iface, ifaceOk := netItem["iface"].(string)
					if !ifaceOk {
						log.Debug().Interface("netItem", netItem).Msg("Network bridge missing iface name")
						continue
					}

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

	log.Info().Int("count", len(allVMBRs)).Msg("Found VMBRs")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		VMBRs []VMBRInfo `json:"vmbrs"`
	}{VMBRs: allVMBRs})
}
