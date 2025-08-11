package handlers

import (
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// collectAllVMBRs retrieves VMBR bridge information across all nodes via Proxmox.
// It returns a slice of maps to minimize churn in existing templates/callers.
// Each map contains: node, iface, type, method, address, netmask, gateway, description("").
func collectAllVMBRs(sm state.StateManager) ([]map[string]string, error) {
	log := logger.Get().With().Str("helper", "collectAllVMBRs").Logger()

	if sm == nil {
		log.Error().Msg("state manager is nil")
		return nil, nil
	}

	client := sm.GetProxmoxClient()
	if client == nil {
		log.Warn().Msg("Proxmox client is not initialized; returning empty VMBR list")
		return []map[string]string{}, nil
	}

	// Use the interface directly to support real and mock clients
	nodeNames, err := proxmox.GetNodeNames(client)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to retrieve Proxmox nodes")
		return []map[string]string{}, nil
	}
	log.Info().Int("node_count", len(nodeNames)).Msg("Discovered Proxmox nodes")

	allVMBRs := make([]map[string]string, 0)
	for _, node := range nodeNames {
		vmbrs, err := proxmox.GetVMBRs(client, node)
		if err != nil {
			log.Warn().Err(err).Str("node", node).Msg("Failed to get VMBRs for node; skipping")
			continue
		}
		log.Info().Str("node", node).Int("vmbr_count", len(vmbrs)).Msg("Fetched VMBRs for node")
		for _, vmbr := range vmbrs {
			if vmbr.Type == "bridge" { // keep parity with existing admin filtering
				// Fallbacks: some Proxmox versions expose interface as "name" instead of "iface"
				iface := vmbr.Iface
				if iface == "" && vmbr.IfaceName != "" {
					iface = vmbr.IfaceName
				}

				// Build a readable description from available fields
				desc := ""
				if vmbr.BridgePorts != "" {
					desc = "ports: " + vmbr.BridgePorts
				}
				if vmbr.Address != "" {
					if desc != "" {
						desc += " | "
					}
					cidr := vmbr.Address
					if vmbr.Netmask != "" {
						cidr += "/" + vmbr.Netmask
					}
					desc += "ip: " + cidr
				}
				if vmbr.Gateway != "" {
					if desc != "" {
						desc += " | "
					}
					desc += "gw: " + vmbr.Gateway
				}
				if vmbr.Method != "" {
					if desc != "" {
						desc += " | "
					}
					desc += "method: " + vmbr.Method
				}
				if desc == "" && vmbr.Comments != "" {
					desc = vmbr.Comments
				}

				allVMBRs = append(allVMBRs, map[string]string{
					"node":        node,
					"iface":       iface,
					"type":        vmbr.Type,
					"method":      vmbr.Method,
					"address":     vmbr.Address,
					"netmask":     vmbr.Netmask,
					"gateway":     vmbr.Gateway,
					"description": desc,
				})
			}
		}
	}

	log.Debug().Int("total_vmbrs", len(allVMBRs)).Msg("Total VMBR bridges collected across all nodes")

	return allVMBRs, nil
}
