package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"pvmss/backend/proxmox"
)

// ISOInfo holds combined information about an ISO file.
type ISOInfo struct {
	Storage string `json:"storage"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	VolID   string `json:"volid"`
}

// allIsosHandler fetches all ISOs from all compatible storages across all nodes.
func allIsosHandler(w http.ResponseWriter, r *http.Request) {
	if proxmoxClient == nil {
		log.Error().Msg("Proxmox client not initialized")
		http.Error(w, "Failed to connect to Proxmox API", http.StatusInternalServerError)
		return
	}

	storages, err := getStorages()
	if err != nil {
		http.Error(w, "Failed to retrieve storages from Proxmox", http.StatusInternalServerError)
		return
	}

	nodes, err := proxmox.GetNodeNames(proxmoxClient)
	if err != nil {
		http.Error(w, "Failed to retrieve nodes from Proxmox", http.StatusInternalServerError)
		return
	}

	var allISOs []ISOInfo

	for _, storage := range storages {
		if !strings.Contains(storage.Content, "iso") {
			continue // Skip storages that don't support ISOs
		}

		// Determine which node to query. For local storage, it's specific. For shared, any node works.
		nodesToQuery := nodes
		if storage.Nodes != "" {
			nodesToQuery = strings.Split(storage.Nodes, ",")
		}

		for _, node := range nodesToQuery {
			log.Info().Str("storage", storage.Name).Str("node", node).Msg("Querying for ISOs")
			isoResult, err := proxmox.GetISOList(proxmoxClient, node, storage.Name)
			if err != nil {
				log.Warn().Err(err).Str("storage", storage.Name).Str("node", node).Msg("Could not get ISO list. This may be expected if the storage is not available on this node.")
				continue // Try next node
			}

			// isoResult is now a map[string]interface{}, so we can directly access its fields.
			data, dataOk := isoResult["data"].([]interface{})
			if !dataOk {
				log.Debug().Interface("isoResult", isoResult).Msg("Invalid ISO list data format, skipping")
				continue
			}
			
			for _, item := range data {
				isoItem, itemOk := item.(map[string]interface{})
				if !itemOk {
					log.Debug().Interface("item", item).Msg("ISO item is not a valid map, skipping")
					continue
				}
				
				ctype, ctypeOk := isoItem["content"].(string)
				if !ctypeOk || ctype != "iso" {
					// Si ce n'est pas un ISO ou type invalide, on ignore
					continue
				}
				
				volid, volidOk := isoItem["volid"].(string)
				if !volidOk {
					log.Debug().Interface("isoItem", isoItem).Msg("ISO item missing volid, skipping")
					continue
				}
				
				// Extract the filename from the volid (e.g., 'local:iso/file.iso' -> 'file.iso')
				nameParts := strings.Split(volid, "/")
				name := volid // Fallback to full volid
				if len(nameParts) > 1 {
					name = nameParts[1]
				}

				iso := ISOInfo{
					Storage: storage.Name,
					VolID:   volid,
					Name:    name,
				}
				
				if size, ok := isoItem["size"].(float64); ok {
					iso.Size = int64(size)
				}
				allISOs = append(allISOs, iso)
			}
			// If we successfully queried a storage, we don't need to query it on other nodes.
			break
		}
	}

	log.Info().Int("count", len(allISOs)).Msg("ISO images found")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		ISOs []ISOInfo `json:"isos"`
	}{ISOs: allISOs})
}
