package main

import (
	"context"
	"fmt"
	"pvmss/backend/proxmox"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func searchVM(client *proxmox.Client, vmid, name string) (interface{}, error) {
	log.Debug().Msg("Entering searchVM function")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vmList, err := client.GetVmList(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get VM list from Proxmox")
		return nil, err
	}

	var results []interface{}
	data, ok := vmList["data"].([]interface{})
	if !ok {
		log.Warn().Msg("VM list data is not in the expected format")
		return nil, nil
	}

	for _, vm := range data {
		vmMap, ok := vm.(map[string]interface{})
		if !ok {
			continue
		}

		// Filter by tag: must contain "pvmss"
		tags, ok := vmMap["tags"].(string)
		if !ok || !strings.Contains(tags, "pvmss") {
			continue
		}

		// Perform search
		if vmid != "" {
			vmIDStr := fmt.Sprintf("%.0f", vmMap["vmid"])
			if strings.Contains(vmIDStr, vmid) {
				results = append(results, vmMap)
			}
		} else if name != "" {
			vmName, ok := vmMap["name"].(string)
			if ok && strings.Contains(vmName, name) {
				results = append(results, vmMap)
			}
		}
	}

	if len(results) == 0 {
		log.Debug().Msg("No VMs found matching criteria")
		return nil, nil // No results found
	}

	log.Debug().Int("count", len(results)).Msg("Found matching VMs")
	return results, nil
}
