package main

import (
	"encoding/json"
	"net/http"
	"os"
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

	storages, err := getStorages()
	if err != nil {
		http.Error(w, "Failed to retrieve storages from Proxmox", http.StatusInternalServerError)
		return
	}

	nodes, err := proxmox.GetNodeNames(client)
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
			log.Info().Msgf("Querying storage '%s' on node '%s' for ISOs...", storage.Name, node)
			isoResult, err := proxmox.GetISOList(client, node, storage.Name)
			if err != nil {
				log.Warn().Err(err).Msgf("Could not get ISO list for storage %s on node %s. This may be expected if the storage is not available on this node.", storage.Name, node)
				continue // Try next node
			}

			// isoResult is now a map[string]interface{}, so we can directly access its fields.
			if data, ok := isoResult["data"].([]interface{}); ok {
				for _, item := range data {
					if isoItem, ok := item.(map[string]interface{}); ok {
						if ctype, ok := isoItem["content"].(string); ok && ctype == "iso" {
							volid := isoItem["volid"].(string)
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
					}
				}
			}
			// If we successfully queried a storage, we don't need to query it on other nodes.
			break
		}
	}

	log.Info().Msgf("Found %d total ISO images.", len(allISOs))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		ISOs []ISOInfo `json:"isos"`
	}{ISOs: allISOs})
}
