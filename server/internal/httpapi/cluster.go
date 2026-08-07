package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/inventory"
	"time"
)

// ClusterNodes serves GET /api/v1/cluster/nodes, reading from the inventory
// projection — never the cluster client directly (FR-002, SC-004). The
// handler is the literal fix AC02 exists for: reads no longer pay a
// per-request client call.
type ClusterNodes struct {
	projection *inventory.Projection
	log        *slog.Logger
}

// NewClusterNodes creates the handler for the given inventory projection.
func NewClusterNodes(projection *inventory.Projection, log *slog.Logger) *ClusterNodes {
	return &ClusterNodes{projection: projection, log: log}
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
	VMCount      int     `json:"vmCount"`
}

type clusterNodesResponse struct {
	Nodes       []nodeDTO `json:"nodes"`
	RefreshedAt string    `json:"refreshedAt"`
}

// clusterErrorEnvelope is the {code, message} shape used by this endpoint
// (contracts/cluster-refresh.md). message stays generic — driver detail goes
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

	idx := h.projection.Load()
	if idx == nil {
		// FR-009: never-refreshed is distinct from an empty list.
		if err := writeClusterError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet"); err != nil {
			h.log.Error("failed to write inventory_not_ready response", "component", "httpapi", "error", err)
		}

		return
	}

	resp := clusterNodesResponse{
		Nodes:       make([]nodeDTO, len(idx.Nodes)),
		RefreshedAt: idx.RefreshedAt.UTC().Format(time.RFC3339Nano),
	}
	for i, n := range idx.Nodes {
		resp.Nodes[i] = nodeDTO{
			Name:         n.Name,
			Status:       string(n.Status),
			CPUCores:     n.CPUCores,
			CPUUsage:     n.CPUUsage,
			MemoryTotal:  n.MemoryTotal,
			MemoryUsed:   n.MemoryUsed,
			StorageTotal: n.StorageTotal,
			StorageUsed:  n.StorageUsed,
			VMCount:      len(idx.ByNode[n.Name]),
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
