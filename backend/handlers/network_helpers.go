package handlers

import (
	"context"
	"strings"
	"time"

	"pvmss/proxmox"
	"pvmss/state"
)

// buildVMBRDescription builds a description string from VMBR fields
func buildVMBRDescription(vmbr proxmox.VMBR) string {
	var parts []string

	if vmbr.BridgePorts != "" {
		parts = append(parts, "ports: "+vmbr.BridgePorts)
	}

	if vmbr.Address != "" {
		cidr := vmbr.Address
		if vmbr.Netmask != "" {
			cidr += "/" + vmbr.Netmask
		}
		parts = append(parts, "ip: "+cidr)
	}

	if vmbr.Gateway != "" {
		parts = append(parts, "gw: "+vmbr.Gateway)
	}

	if vmbr.Method != "" {
		parts = append(parts, "method: "+vmbr.Method)
	}

	if len(parts) > 0 {
		return strings.Join(parts, " | ")
	}

	// Fallback to comments if no other info
	return vmbr.Comments
}

// getVMBRInterface returns the interface name, preferring Iface over IfaceName
func getVMBRInterface(vmbr proxmox.VMBR) string {
	if vmbr.Iface != "" {
		return vmbr.Iface
	}
	return vmbr.IfaceName
}

// collectAllVMBRs retrieves VMBR bridge information across all nodes via Proxmox.
// It returns a slice of maps to minimize churn in existing templates/callers.
// Each map contains: node, iface, type, method, address, netmask, gateway, description("").
func collectAllVMBRs(sm state.StateManager) ([]map[string]string, error) {
	log := CreateHandlerLogger("collectAllVMBRs", nil)

	if sm == nil {
		log.Error().Msg("state manager is nil")
		return nil, nil
	}

	// Respect background connectivity monitor: short-circuit when offline
	if connected, _ := sm.GetProxmoxStatus(); !connected {
		log.Warn().Msg("Proxmox reported offline by background monitor; skipping VMBR collection")
		return []map[string]string{}, nil
	}

	client := sm.GetProxmoxClient()
	if client == nil {
		log.Warn().Msg("Proxmox client is not initialized; returning empty VMBR list")
		return []map[string]string{}, nil
	}

	// Use a short timeout to keep admin page responsive
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the interface directly to support real and mock clients
	nodeNames, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to retrieve Proxmox nodes")
		return []map[string]string{}, nil
	}
	log.Info().Int("node_count", len(nodeNames)).Msg("Discovered Proxmox nodes")

	allVMBRs := make([]map[string]string, 0)
	for _, node := range nodeNames {
		vmbrs, err := proxmox.GetVMBRsWithContext(ctx, client, node)
		if err != nil {
			log.Warn().Err(err).Str("node", node).Msg("Failed to get VMBRs for node; skipping")
			continue
		}
		log.Info().Str("node", node).Int("vmbr_count", len(vmbrs)).Msg("Fetched VMBRs for node")
		for _, vmbr := range vmbrs {
			if vmbr.Type == "bridge" { // keep parity with existing admin filtering
				allVMBRs = append(allVMBRs, map[string]string{
					"node":        node,
					"iface":       getVMBRInterface(vmbr),
					"type":        vmbr.Type,
					"method":      vmbr.Method,
					"address":     vmbr.Address,
					"netmask":     vmbr.Netmask,
					"gateway":     vmbr.Gateway,
					"description": buildVMBRDescription(vmbr),
				})
			}
		}
	}

	log.Debug().Int("total_vmbrs", len(allVMBRs)).Msg("Total VMBR bridges collected across all nodes")

	return allVMBRs, nil
}
