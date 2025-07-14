package main

import (
	"context"
	"fmt"
	"pvmss/backend/proxmox"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// getStringValue safely retrieves a string from a map, handling both string and float64 types.
func getStringValue(vmMap map[string]interface{}, key string) (string, bool) {
	rawValue, present := vmMap[key]
	if !present {
		return "", false
	}

	switch value := rawValue.(type) {
	case string:
		return value, true
	case float64:
		return fmt.Sprintf("%.0f", value), true
	default:
		// Not a string or float64, so we can't treat it as a searchable string
		return "", false
	}
}

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
		tagsStr, ok := getStringValue(vmMap, "tags")
		if !ok || !strings.Contains(tagsStr, "pvmss") {
			continue
		}

		// Perform search
		if vmid != "" {
			vmIDStr, ok := getStringValue(vmMap, "vmid")
			if ok && strings.Contains(vmIDStr, vmid) {
				results = append(results, vmMap)
			}
		} else if name != "" {
			vmNameStr, ok := getStringValue(vmMap, "name")
			if ok && strings.Contains(vmNameStr, name) {
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
