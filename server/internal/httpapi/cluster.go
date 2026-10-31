package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/vm"
	"slices"
	"time"
)

// ClusterNodes serves GET /api/v1/cluster/nodes, reading from the inventory
// projection - never the cluster client directly. The
// handler is the literal fix exists for: reads no longer pay a
// per-request client call.
type ClusterNodes struct {
	projection *inventory.Projection
	registry   *inventory.Registry
	auth       *Auth
	log        *slog.Logger
}

// NewClusterNodes creates the handler for the given inventory projection.
func NewClusterNodes(projection *inventory.Projection, log *slog.Logger) *ClusterNodes {
	return &ClusterNodes{projection: projection, log: log}
}

// NewClusterNodesWithRegistry creates the handler that scopes nodes per
// request cluster: a non-admin always sees the cluster it signed in to
// (?cluster= is ignored); an admin may name one via ?cluster= and without it
// receives the union of every cluster's nodes, each row tagged with its
// cluster.
func NewClusterNodesWithRegistry(registry *inventory.Registry, authHandler *Auth, log *slog.Logger) *ClusterNodes {
	return &ClusterNodes{registry: registry, auth: authHandler, log: log}
}

type nodeDTO struct {
	Name         string  `json:"name"`
	Cluster      string  `json:"cluster"`
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

// clusterErrorEnvelope is the {code, message} shape used by this endpoint.
// message stays generic - driver detail goes
// only to the structured server log.
type clusterErrorEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Privilege string `json:"privilege,omitempty"`
	Path      string `json:"path,omitempty"`
}

func (h *ClusterNodes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")

		if err := writeClusterError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed"); err != nil {
			logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write method not allowed", "component", "httpapi", "error", err)
		}

		return
	}

	if h.registry != nil && h.auth != nil {
		h.serveScoped(w, r)
		return
	}

	h.serveProjection(w, r)
}

// serveScoped resolves the caller's cluster and answers from that cluster's
// index (or the union of all indexes for an admin without ?cluster=).
func (h *ClusterNodes) serveScoped(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		writeClusterErrorLogged(w, r, h.log, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	requested := ""
	if identity.IsAdmin {
		requested = r.URL.Query().Get("cluster")
	}

	clusterName, _ := vm.TargetCluster(identity, requested)

	if clusterName == "" {
		h.serveUnion(w, r)
		return
	}

	index, err := h.registry.Index(clusterName)
	if err != nil {
		writeClusterErrorLogged(w, r, h.log, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}

	if index == nil {
		h.writeNotReady(w, r)
		return
	}

	h.writeNodes(w, r, nodeDTOs(index, clusterName), index.RefreshedAt)
}

// serveUnion answers an admin's unscoped request with every cluster's nodes,
// each row carrying its cluster name. RefreshedAt reports the oldest
// populated index - the least fresh data being served.
func (h *ClusterNodes) serveUnion(w http.ResponseWriter, r *http.Request) {
	indexes := h.registry.All()

	names := make([]string, 0, len(indexes))
	for name := range indexes {
		names = append(names, name)
	}

	slices.Sort(names)

	nodes := make([]nodeDTO, 0)
	loaded := 0

	var refreshedAt time.Time

	for _, name := range names {
		index := indexes[name]
		if index == nil {
			continue
		}

		nodes = append(nodes, nodeDTOs(index, name)...)
		loaded++

		if refreshedAt.IsZero() || index.RefreshedAt.Before(refreshedAt) {
			refreshedAt = index.RefreshedAt
		}
	}

	if loaded == 0 {
		h.writeNotReady(w, r)
		return
	}

	h.writeNodes(w, r, nodes, refreshedAt)
}

// serveProjection is the legacy single-projection path kept for tests that
// construct the handler directly without a registry.
func (h *ClusterNodes) serveProjection(w http.ResponseWriter, r *http.Request) {
	idx := h.projection.Load()
	if idx == nil {
		h.writeNotReady(w, r)
		return
	}

	h.writeNodes(w, r, nodeDTOs(idx, ""), idx.RefreshedAt)
}

func (h *ClusterNodes) writeNotReady(w http.ResponseWriter, r *http.Request) {
	// never-refreshed is distinct from an empty list.
	if err := writeClusterError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet"); err != nil {
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write inventory_not_ready response", "component", "httpapi", "error", err)
	}
}

func (h *ClusterNodes) writeNodes(w http.ResponseWriter, r *http.Request, nodes []nodeDTO, refreshedAt time.Time) {
	resp := clusterNodesResponse{
		Nodes:       nodes,
		RefreshedAt: refreshedAt.UTC().Format(time.RFC3339Nano),
	}

	body, err := json.Marshal(resp)
	if err != nil {
		SetErrorMsg(w, "failed to marshal cluster nodes response", err)

		if writeErr := writeClusterError(w, http.StatusInternalServerError, "internal_error", "internal server error"); writeErr != nil {
			logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write internal_error response", "component", "httpapi", "error", writeErr)
		}

		return
	}

	if err := writeJSON(w, http.StatusOK, body); err != nil {
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write cluster nodes response", "component", "httpapi", "error", err)
	}
}

func nodeDTOs(index *inventory.Index, clusterName string) []nodeDTO {
	nodes := make([]nodeDTO, len(index.Nodes))
	for i, n := range index.Nodes {
		nodes[i] = nodeDTO{
			Name:         n.Name,
			Cluster:      clusterName,
			Status:       string(n.Status),
			CPUCores:     n.CPUCores,
			CPUUsage:     n.CPUUsage,
			MemoryTotal:  n.MemoryTotal,
			MemoryUsed:   n.MemoryUsed,
			StorageTotal: n.StorageTotal,
			StorageUsed:  n.StorageUsed,
			VMCount:      len(index.ByNode[n.Name]),
		}
	}

	return nodes
}

// writeClusterErrorLogged collapses the repeated writeClusterError + warn
// boilerplate of the scoped cluster handlers.
func writeClusterErrorLogged(w http.ResponseWriter, r *http.Request, log *slog.Logger, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		logctx.FromOr(r.Context(), log).WarnContext(r.Context(), "failed to write cluster error response", "component", "httpapi", "error", err)
	}
}

func writeClusterError(w http.ResponseWriter, status int, code, message string) error {
	body, err := json.Marshal(clusterErrorEnvelope{Code: code, Message: message})
	if err != nil {
		return fmt.Errorf("marshal cluster error response: %w", err)
	}

	return writeJSON(w, status, body)
}
