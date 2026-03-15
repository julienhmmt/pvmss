package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// buildVMBRDescription builds a description string from VMBR fields
func buildVMBRDescription(vmbr proxmox.VMBR) string {
	return strings.TrimSpace(vmbr.Comments)
}

// getVMBRInterface returns the interface name, preferring Iface over IfaceName
func getVMBRInterface(vmbr proxmox.VMBR) string {
	if vmbr.Iface != "" {
		return vmbr.Iface
	}
	return vmbr.IfaceName
}

// VMBR cache for offline nodes
var (
	vmbrCache    = make(map[string]cachedVMBRs)
	vmbrCacheMu  sync.Mutex
	vmbrCacheTTL = 15 * time.Second
)

type cachedVMBRs struct {
	items     []map[string]string
	expiresAt time.Time
}

// collectAllVMBRs retrieves VMBR bridge information across all nodes via Proxmox.
// It returns a slice of maps to minimize churn in existing templates/callers.
// Each map contains: node, iface, type, method, address, netmask, gateway, description("").
// Uses caching and fallback logic for offline nodes.
func collectAllVMBRs(ctx context.Context, sm state.StateManager) ([]map[string]string, error) {
	log := logger.Get().With().Str("component", "network_helpers").Logger()

	if sm == nil {
		log.Error().Msg("state manager is nil")
		return nil, nil
	}

	if snapshot := sm.GetProxmoxSnapshot(); snapshot != nil && len(snapshot.NetworkBridges) > 0 {
		vmbrs := vmbrsFromSnapshot(snapshot)
		if len(vmbrs) > 0 {
			log.Info().Int("vmbr_count", len(vmbrs)).Msg("Serving VMBRs from cached snapshot")
			return vmbrs, nil
		}
	}

	// Respect background connectivity monitor: short-circuit when offline
	if connected, _ := sm.GetProxmoxStatus(); !connected {
		log.Warn().
			Str("component", "network_helpers").
			Str("operation", "collect_vmb rs").
			Str("reason", "proxmox_offline").
			Str("fallback", "cache").
			Msg("Proxmox offline; using cache fallback")
		return collectVMBRsFromCache(), nil
	}

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "network_helpers").
			Str("operation", "collect_vmb rs").
			Str("reason", "resty_client_failed").
			Str("fallback", "cache").
			Msg("Failed to create resty client; using cache fallback")
		return collectVMBRsFromCache(), nil
	}

	// Get nodes using resty
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	nodeNames, err := proxmox.GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "network_helpers").
			Str("operation", "collect_vmb rs").
			Str("reason", "nodes_retrieval_failed").
			Str("fallback", "cache").
			Msg("Unable to retrieve Proxmox nodes; using cache fallback")
		return collectVMBRsFromCache(), nil
	}
	log.Info().Int("node_count", len(nodeNames)).Msg("Discovered Proxmox nodes")

	allVMBRs := make([]map[string]string, 0)
	successCount := 0
	fallbackCount := 0
	var mu sync.Mutex
	sem := make(chan struct{}, 6)
	g, gctx := errgroup.WithContext(ctx)

	for _, node := range nodeNames {
		nodeName := node
		g.Go(func() error {
			// throttle
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-gctx.Done():
				return gctx.Err()
			}

			vmbrs, err := getVMBRsFromNode(gctx, nodeName, restyClient)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				log.Warn().
					Err(err).
					Str("component", "network_helpers").
					Str("operation", "collect_vmb rs").
					Str("node", nodeName).
					Str("reason", "node_vmb rs_failed").
					Str("fallback", "cache").
					Msg("Failed to get VMBRs from node; using cache fallback")
				cachedVMBRs := getCachedVMBRsForNode(nodeName)
				if len(cachedVMBRs) > 0 {
					allVMBRs = append(allVMBRs, cachedVMBRs...)
					fallbackCount++
					log.Info().Str("node", nodeName).Int("cached_count", len(cachedVMBRs)).Msg("Using cached VMBRs for offline node")
				}
				return nil
			}
			allVMBRs = append(allVMBRs, vmbrs...)
			successCount++
			log.Info().Str("node", nodeName).Int("vmbr_count", len(vmbrs)).Msg("Fetched fresh VMBRs for node")
			return nil
		})
	}
	_ = g.Wait()

	log.Info().Int("online_nodes", successCount).Int("fallback_nodes", fallbackCount).Int("total_vmbrs", len(allVMBRs)).Msg("VMBR collection completed")

	return allVMBRs, nil
}

// getVMBRsFromNode fetches VMBRs from a specific node and caches them
func getVMBRsFromNode(ctx context.Context, node string, restyClient *proxmox.RestyClient) ([]map[string]string, error) {
	log := logger.Get().With().Str("component", "network_helpers").Logger()

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	vmbrs, err := proxmox.GetVMBRsResty(ctx, restyClient, node)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMBRs for node %s: %w", node, err)
	}

	// Process VMBRs
	result := make([]map[string]string, 0, len(vmbrs))
	for _, vmbr := range vmbrs {
		if vmbr.Type == "bridge" {
			result = append(result, map[string]string{
				"node":        node,
				"iface":       getVMBRInterface(vmbr),
				"type":        vmbr.Type,
				"method":      vmbr.Method,
				"address":     vmbr.Address,
				"netmask":     vmbr.Netmask,
				"gateway":     vmbr.Gateway,
				"description": buildVMBRDescription(vmbr),
				"isFromCache": "false", // Fresh data from online node
			})
		}
	}

	// Update cache
	vmbrCacheMu.Lock()
	vmbrCache[node] = cachedVMBRs{items: result, expiresAt: time.Now().Add(vmbrCacheTTL)}
	vmbrCacheMu.Unlock()
	log.Debug().
		Str("node", node).
		Int("items", len(result)).
		Dur("ttl", vmbrCacheTTL).
		Str("component", "network_helpers").
		Str("operation", "update_cache").
		Str("reason", "cache_updated").
		Msg("VMBR cache updated")

	return result, nil
}

// getCachedVMBRsForNode returns cached VMBRs for a specific node
func getCachedVMBRsForNode(node string) []map[string]string {
	vmbrCacheMu.Lock()
	defer vmbrCacheMu.Unlock()

	if cached, ok := vmbrCache[node]; ok && time.Now().Before(cached.expiresAt) {
		// Mark as from cache
		result := make([]map[string]string, len(cached.items))
		for i, item := range cached.items {
			cpy := make(map[string]string)
			for k, v := range item {
				cpy[k] = v
			}
			cpy["isFromCache"] = "true" // Mark as cached data
			result[i] = cpy
		}
		return result
	}
	return []map[string]string{}
}

// collectVMBRsFromCache returns all cached VMBRs from all nodes
func collectVMBRsFromCache() []map[string]string {
	vmbrCacheMu.Lock()
	defer vmbrCacheMu.Unlock()

	allVMBRs := make([]map[string]string, 0)
	for node, cached := range vmbrCache {
		if time.Now().Before(cached.expiresAt) {
			allVMBRs = append(allVMBRs, cached.items...)
			logger.Get().Debug().Str("node", node).Int("count", len(cached.items)).Msg("Using cached VMBRs")
		}
	}
	return allVMBRs
}

// vmbrsFromSnapshot flattens the cached snapshot bridges into template-friendly maps.
func vmbrsFromSnapshot(snapshot *state.ProxmoxClusterSnapshot) []map[string]string {
	if snapshot == nil {
		return []map[string]string{}
	}
	result := make([]map[string]string, 0)
	for nodeName, vmbrs := range snapshot.NetworkBridges {
		for _, vmbr := range vmbrs {
			if vmbr.Type != "bridge" {
				continue
			}
			result = append(result, map[string]string{
				"node":        nodeName,
				"iface":       getVMBRInterface(vmbr),
				"type":        vmbr.Type,
				"method":      vmbr.Method,
				"address":     vmbr.Address,
				"netmask":     vmbr.Netmask,
				"gateway":     vmbr.Gateway,
				"description": buildVMBRDescription(vmbr),
				"isFromCache": "true",
			})
		}
	}
	return result
}
