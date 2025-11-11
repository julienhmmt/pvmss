package proxmox

import (
	"context"

	"pvmss/logger"
)

// ClusterInfo represents information about a Proxmox cluster
type ClusterInfo struct {
	IsCluster   bool   `json:"isCluster"`
	ClusterName string `json:"clusterName"`
	NodeCount   int    `json:"nodeCount"`
}

// ClusterStatusItem represents a single item in the cluster status response
type ClusterStatusItem struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Nodes   int    `json:"nodes,omitempty"`
	Quorate int    `json:"quorate,omitempty"`
	NodeID  int    `json:"nodeid,omitempty"`
	Online  int    `json:"online,omitempty"`
	Local   int    `json:"local,omitempty"`
	IP      string `json:"ip,omitempty"`
	Level   string `json:"level,omitempty"`
	Version int    `json:"version,omitempty"`
}

// GetClusterStatus retrieves cluster status information from the Proxmox API
// This endpoint provides information about whether we're in a cluster or standalone mode
func GetClusterStatus(ctx context.Context, client ClientInterface) (*ClusterInfo, error) {
	var response ListResponse[ClusterStatusItem]
	if err := client.GetJSON(ctx, "/cluster/status", &response); err != nil {
		logger.Get().Debug().Err(err).Msg("Failed to get cluster status from Proxmox API")
		// If cluster status fails, we assume standalone mode
		return &ClusterInfo{IsCluster: false, ClusterName: "", NodeCount: 0}, nil
	}

	clusterInfo := &ClusterInfo{
		IsCluster:   false,
		ClusterName: "",
		NodeCount:   0,
	}

	nodeCount := 0
	for _, item := range response.Data {
		if item.Type == "cluster" {
			// This is the cluster information entry
			clusterInfo.IsCluster = true
			clusterInfo.ClusterName = item.Name
			clusterInfo.NodeCount = item.Nodes
			logger.Get().Info().
				Str("cluster_name", item.Name).
				Int("nodes", item.Nodes).
				Msg("Proxmox cluster detected")
			break
		} else if item.Type == "node" {
			// Count individual nodes (fallback if cluster entry not found)
			nodeCount++
		}
	}

	// If we didn't find a cluster entry but have multiple nodes, it's still a cluster
	if !clusterInfo.IsCluster && nodeCount > 1 {
		clusterInfo.IsCluster = true
		clusterInfo.NodeCount = nodeCount
		clusterInfo.ClusterName = "Unknown" // We don't have the cluster name from individual nodes
		logger.Get().Info().
			Int("nodes", nodeCount).
			Msg("Proxmox cluster detected (multiple nodes)")
	} else if !clusterInfo.IsCluster && nodeCount == 1 {
		// Single node - standalone mode
		clusterInfo.NodeCount = 1
		logger.Get().Info().Msg("Proxmox standalone mode detected")
	}

	return clusterInfo, nil
}

// GetClusterStatusResty retrieves cluster status information using resty client
func GetClusterStatusResty(ctx context.Context, client *RestyClient) (*ClusterInfo, error) {
	var response ListResponse[ClusterStatusItem]
	if err := client.Get(ctx, "/cluster/status", &response); err != nil {
		logger.Get().Debug().Err(err).Msg("Failed to get cluster status from Proxmox API (resty)")
		// If cluster status fails, we assume standalone mode
		return &ClusterInfo{IsCluster: false, ClusterName: "", NodeCount: 0}, nil
	}

	clusterInfo := &ClusterInfo{
		IsCluster:   false,
		ClusterName: "",
		NodeCount:   0,
	}

	nodeCount := 0
	for _, item := range response.Data {
		if item.Type == "cluster" {
			// This is the cluster information entry
			clusterInfo.IsCluster = true
			clusterInfo.ClusterName = item.Name
			clusterInfo.NodeCount = item.Nodes
			logger.Get().Info().
				Str("cluster_name", item.Name).
				Int("nodes", item.Nodes).
				Msg("Proxmox cluster detected")
			break
		} else if item.Type == "node" {
			// Count individual nodes (fallback if cluster entry not found)
			nodeCount++
		}
	}

	// If we didn't find a cluster entry but have multiple nodes, it's still a cluster
	if !clusterInfo.IsCluster && nodeCount > 1 {
		clusterInfo.IsCluster = true
		clusterInfo.NodeCount = nodeCount
		clusterInfo.ClusterName = "Unknown" // We don't have the cluster name from individual nodes
		logger.Get().Info().
			Int("nodes", nodeCount).
			Msg("Proxmox cluster detected (multiple nodes)")
	} else if !clusterInfo.IsCluster && nodeCount == 1 {
		// Single node - standalone mode
		clusterInfo.NodeCount = 1
		logger.Get().Info().Msg("Proxmox standalone mode detected")
	}

	return clusterInfo, nil
}
