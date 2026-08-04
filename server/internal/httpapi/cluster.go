package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"pvmss/server/internal/cluster"
)

// ClusterNodes serves GET /api/v1/cluster/nodes, identically whichever
// cluster.Client implementation is active (constitution XI).
type ClusterNodes struct {
	client cluster.Client
	log    *slog.Logger
}

// NewClusterNodes creates the handler for the given cluster client.
func NewClusterNodes(client cluster.Client, log *slog.Logger) *ClusterNodes {
	return &ClusterNodes{client: client, log: log}
}

type nodeDTO struct {
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	CPUCores     int     `json:"cpuCores"`
	CPUUsage     float64 `json:"cpuUsage"`
	MemoryTotal  int64   `json:"memoryTotal"`
	MemoryUsed   int64   `json:"memoryUsed"`
	StorageTotal int64   `json:"storageTotal"`
	StorageUsed  int64   `json:"storageUsed"`
}

type clusterNodesResponse struct {
	Nodes []nodeDTO `json:"nodes"`
}

// clusterErrorEnvelope is the {code, message} shape used by this endpoint
// (contracts/cluster-nodes.md). message stays generic — driver detail goes
// only to the structured server log (constitution XIII).
type clusterErrorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *ClusterNodes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		if err := writeClusterError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed"); err != nil {
			h.log.Error("failed to write method not allowed", "component", "httpapi", "error", err)
		}
		return
	}

	nodes, err := h.client.ListNodes(r.Context())
	if err != nil {
		if errors.Is(err, cluster.ErrUnreachable) {
			h.log.Error("cluster unreachable", "component", "httpapi", "error", err)
			if writeErr := writeClusterError(w, http.StatusBadGateway, "cluster_unreachable", "cluster is not reachable"); writeErr != nil {
				h.log.Error("failed to write cluster_unreachable response", "component", "httpapi", "error", writeErr)
			}
			return
		}
		h.log.Error("failed to list cluster nodes", "component", "httpapi", "error", err)
		if writeErr := writeClusterError(w, http.StatusInternalServerError, "internal_error", "internal server error"); writeErr != nil {
			h.log.Error("failed to write internal_error response", "component", "httpapi", "error", writeErr)
		}
		return
	}

	resp := clusterNodesResponse{Nodes: make([]nodeDTO, len(nodes))}
	for i, n := range nodes {
		resp.Nodes[i] = nodeDTO{
			Name:         n.Name,
			Status:       string(n.Status),
			CPUCores:     n.CPUCores,
			CPUUsage:     n.CPUUsage,
			MemoryTotal:  n.MemoryTotal,
			MemoryUsed:   n.MemoryUsed,
			StorageTotal: n.StorageTotal,
			StorageUsed:  n.StorageUsed,
		}
	}

	body, err := json.Marshal(resp)
	if err != nil {
		h.log.Error("failed to marshal cluster nodes response", "component", "httpapi", "error", err)
		if writeErr := writeClusterError(w, http.StatusInternalServerError, "internal_error", "internal server error"); writeErr != nil {
			h.log.Error("failed to write internal_error response", "component", "httpapi", "error", writeErr)
		}
		return
	}
	if err := writeJSON(w, http.StatusOK, body); err != nil {
		h.log.Error("failed to write cluster nodes response", "component", "httpapi", "error", err)
	}
}

func writeClusterError(w http.ResponseWriter, status int, code, message string) error {
	body, err := json.Marshal(clusterErrorEnvelope{Code: code, Message: message})
	if err != nil {
		return fmt.Errorf("marshal cluster error response: %w", err)
	}
	return writeJSON(w, status, body)
}
