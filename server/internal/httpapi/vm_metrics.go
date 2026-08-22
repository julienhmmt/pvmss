package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"strconv"
	"strings"
	"time"
)

// VMMetrics serves the VM metrics endpoints: GET .../metrics/history and
// GET .../metrics/stream. Both are resolved and ownership-checked the same
// way as every other /vms/{cluster}/{vmid}/... read (vm.Resolve()).
type VMMetrics struct {
	projection    *inventory.Projection
	resolver      vm.ClusterIndexResolver
	auth          *Auth
	reader        cluster.MetricsHistoryReader
	currentReader cluster.MetricsCurrentReader
	clients       cluster.ClientProvider
	log           *slog.Logger
}

const (
	metricsStreamTickInterval = 1 * time.Second
	metricsStreamRetryMs      = 3000
)

// NewVMMetrics creates the metrics handler bound to a single cluster. Use
// NewVMMetricsWithRegistry for multi-cluster deployments.
func NewVMMetrics(projection *inventory.Projection, authHandler *Auth, reader cluster.MetricsHistoryReader, currentReader cluster.MetricsCurrentReader, log *slog.Logger) *VMMetrics {
	return &VMMetrics{projection: projection, resolver: singleClusterResolver{projection: projection}, auth: authHandler, reader: reader, currentReader: currentReader, log: log}
}

// NewVMMetricsWithRegistry creates the metrics handler with per-request
// index and cluster reader resolution, keyed on the request's own :cluster
// path value — the fix for the cross-cluster leak this endpoint's
// single-client wiring surfaced.
func NewVMMetricsWithRegistry(source inventory.LookupSource, projection *inventory.Projection, authHandler *Auth, reader cluster.MetricsHistoryReader, currentReader cluster.MetricsCurrentReader, clients cluster.ClientProvider, log *slog.Logger) *VMMetrics {
	handler := NewVMMetrics(projection, authHandler, reader, currentReader, log)
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

// ServeHTTP dispatches the two metrics routes registered by NewRouter.
func (h *VMMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/metrics/stream") {
		h.handleStream(w, r)
		return
	}

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

func (h *VMMetrics) handleStream(w http.ResponseWriter, r *http.Request) {
	identity, clusterName, vmid, ok := h.requestTarget(w, r)
	if !ok {
		return
	}

	index, ok := loadClusterIndex(h.resolver, clusterName, func(status int, code, message string) { h.writeError(w, status, code, message) })
	if !ok {
		return
	}

	currentReader, err := resolveCapability(h.clients, h.currentReader, clusterName, "MetricsCurrentReader")
	if err != nil {
		h.writeError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}

	// Check the VM is running once before starting the stream. The client is
	// expected to close the connection when the VM stops, but this guards the
	// first tick against calling a Proxmox status endpoint on a stopped guest.
	entity, err := vm.Resolve(index, identity, clusterName, vmid)
	if err != nil {
		h.writeMetricsError(w, err)
		return
	}

	if entity.Status != cluster.VMRunning {
		h.writeError(w, http.StatusConflict, "vm_not_running", "metrics stream is only available while the VM is running")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)

	// The server's global WriteTimeout/ReadTimeout (main.go) bound ordinary
	// requests and silently kill a long-lived SSE ~WriteTimeout after the
	// request starts, regardless of how much data is still flowing. A live
	// metrics stream is exactly the kind of long-lived connection those
	// deadlines were never meant to bound. Clear both for this connection
	// only, mirroring the WebSocket deadline-clearing in vm_console.go.
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	// Configure the client's automatic reconnection interval.
	if _, err := fmt.Fprintf(w, "retry: %d\n\n", metricsStreamRetryMs); err != nil {
		h.log.Error("metrics stream: failed to write retry header", "component", "httpapi", "error", err)
		return
	}

	if err := rc.Flush(); err != nil {
		h.log.Error("metrics stream: failed to flush retry header", "component", "httpapi", "error", err)
		return
	}

	deps := vm.MetricsCurrentDependencies{Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Reader: currentReader}

	ticker := time.NewTicker(metricsStreamTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if err := h.writeStreamTick(r.Context(), w, rc, deps); err != nil {
				h.log.Debug("metrics stream closed", "component", "httpapi", "error", err)
				return
			}
		}
	}
}

func (h *VMMetrics) writeStreamTick(ctx context.Context, w http.ResponseWriter, rc *http.ResponseController, deps vm.MetricsCurrentDependencies) error {
	sample, err := vm.GetMetricsCurrent(ctx, deps)
	if err != nil {
		return fmt.Errorf("get metrics current: %w", err)
	}

	body, err := json.Marshal(metricsSampleFromModel(sample))
	if err != nil {
		return fmt.Errorf("marshal metrics tick: %w", err)
	}

	if _, err := fmt.Fprintf(w, "id: %s\ndata: %s\n\n", strconv.FormatInt(sample.Timestamp.UnixMilli(), 10), body); err != nil {
		return fmt.Errorf("write metrics tick: %w", err)
	}

	return rc.Flush()
}

func metricsSampleFromModel(s vm.MetricsSample) metricsSampleDTO {
	return metricsSampleDTO{
		Timestamp:    s.Timestamp.UTC().Format(time.RFC3339Nano),
		CPUPercent:   s.CPUPercent,
		MemoryUsed:   s.MemoryUsed,
		MemoryMax:    s.MemoryMax,
		DiskReadBps:  s.DiskReadBps,
		DiskWriteBps: s.DiskWriteBps,
		NetInBps:     s.NetInBps,
		NetOutBps:    s.NetOutBps,
	}
}

func sampleDTOsFromModel(samples []vm.MetricsSample) []metricsSampleDTO {
	dtos := make([]metricsSampleDTO, 0, len(samples))
	for _, s := range samples {
		dtos = append(dtos, metricsSampleFromModel(s))
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
		h.log.Error("vm metrics failed", "component", "httpapi", "error", err)
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
