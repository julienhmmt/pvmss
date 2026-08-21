package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"time"
)

// VMMetrics serves GET /api/v1/vms/:cluster/:vmid/metrics/history, resolved
// and ownership-checked the same way as every other VM read (vm.Resolve()
// via vm.GetMetricsHistory).
type VMMetrics struct {
	projection *inventory.Projection
	resolver   vm.ClusterIndexResolver
	auth       *Auth
	reader     cluster.MetricsHistoryReader
	clients    cluster.ClientProvider
	log        *slog.Logger
}

// NewVMMetrics creates the metrics handler bound to a single cluster. Use
// NewVMMetricsWithRegistry for multi-cluster deployments.
func NewVMMetrics(projection *inventory.Projection, authHandler *Auth, reader cluster.MetricsHistoryReader, log *slog.Logger) *VMMetrics {
	return &VMMetrics{projection: projection, resolver: singleClusterResolver{projection: projection}, auth: authHandler, reader: reader, log: log}
}

// NewVMMetricsWithRegistry creates the metrics handler with per-request
// index and cluster.MetricsHistoryReader resolution, keyed on the request's
// own :cluster path value — the fix for the cross-cluster leak this
// endpoint's single-client wiring surfaced.
func NewVMMetricsWithRegistry(source inventory.LookupSource, projection *inventory.Projection, authHandler *Auth, reader cluster.MetricsHistoryReader, clients cluster.ClientProvider, log *slog.Logger) *VMMetrics {
	handler := NewVMMetrics(projection, authHandler, reader, log)
	if registry, ok := source.(*inventory.Registry); ok {
		handler.resolver = registryResolver{registry: registry}
	}

	handler.clients = clients

	return handler
}

type metricsSampleDTO struct {
	Timestamp    string  `json:"timestamp"`
	CPUPercent   float64 `json:"cpuPercent"`
	MemoryUsed   int64   `json:"memoryUsedBytes"`
	MemoryMax    int64   `json:"memoryMaxBytes"`
	DiskReadBps  float64 `json:"diskReadBytesPerSec"`
	DiskWriteBps float64 `json:"diskWriteBytesPerSec"`
	NetInBps     float64 `json:"netInBytesPerSec"`
	NetOutBps    float64 `json:"netOutBytesPerSec"`
}

type metricsHistoryDTO struct {
	Range   string             `json:"range"`
	Samples []metricsSampleDTO `json:"samples"`
}

// ServeHTTP dispatches the one metrics route registered by NewRouter.
func (h *VMMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handleHistory(w, r)
}

func (h *VMMetrics) handleHistory(w http.ResponseWriter, r *http.Request) {
	identity, clusterName, vmid, ok := h.requestTarget(w, r)
	if !ok {
		return
	}

	timeframe, err := cluster.ParseMetricsTimeframe(r.URL.Query().Get("range"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_range", "range must be hour, day, or week")
		return
	}

	index, ok := loadClusterIndex(h.resolver, clusterName, func(status int, code, message string) { h.writeError(w, status, code, message) })
	if !ok {
		return
	}

	reader, err := resolveCapability(h.clients, h.reader, clusterName, "MetricsHistoryReader")
	if err != nil {
		h.writeError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}

	samples, err := vm.GetMetricsHistory(r.Context(), vm.MetricsDependencies{Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Reader: reader}, timeframe)
	if err != nil {
		h.writeMetricsError(w, err)
		return
	}

	h.writePayload(w, http.StatusOK, metricsHistoryDTO{Range: string(timeframe), Samples: sampleDTOsFromModel(samples)})
}

func sampleDTOsFromModel(samples []vm.MetricsSample) []metricsSampleDTO {
	dtos := make([]metricsSampleDTO, 0, len(samples))
	for _, s := range samples {
		dtos = append(dtos, metricsSampleDTO{
			Timestamp:    s.Timestamp.UTC().Format(time.RFC3339Nano),
			CPUPercent:   s.CPUPercent,
			MemoryUsed:   s.MemoryUsed,
			MemoryMax:    s.MemoryMax,
			DiskReadBps:  s.DiskReadBps,
			DiskWriteBps: s.DiskWriteBps,
			NetInBps:     s.NetInBps,
			NetOutBps:    s.NetOutBps,
		})
	}

	return dtos
}

func (h *VMMetrics) requestTarget(w http.ResponseWriter, r *http.Request) (auth.Identity, string, int, bool) {
	return parseVMRequestTarget(h.auth, r, func(status int, code, message string) { h.writeError(w, status, code, message) })
}

func (h *VMMetrics) writeMetricsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, cluster.ErrNotFound):
		h.writeError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
	default:
		h.log.Error("vm metrics history failed", "component", "httpapi", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

func (h *VMMetrics) writePayload(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write metrics response", "component", "httpapi", "error", err)
	}
}

func (h *VMMetrics) writeError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write metrics error", "component", "httpapi", "code", code, "error", err)
	}
}
