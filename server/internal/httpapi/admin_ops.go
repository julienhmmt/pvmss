//nolint:wsl_v5 // endpoint handlers keep validation and contract mapping adjacent
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strconv"
	"time"
)

// maxAuditPageSize is the upper bound on audit log page size. Matches the
// default MaxListPageSize used by T04's VM list (contracts/admin-ops.md).
const maxAuditPageSize = 100

// defaultClusterName is the single-cluster name used throughout the v0.4
// codebase. Centralized here so goconst does not flag the shared literal.
const defaultClusterName = "default"

// AdminOps serves the T14 admin exploitation endpoints: the audit log read,
// the dashboard aggregate, the database export/import, and the app info with
// redaction. Every /api/v1/admin/* route is wrapped by Auth.RequireAdmin
// (FR-016); the public version endpoint is not.
type AdminOps struct {
	auth       *Auth
	store      *store.Store
	client     cluster.Client
	projection *inventory.Projection
	version    string
	log        *slog.Logger
}

// NewAdminOps creates the handler for all T14 admin exploitation endpoints.
// The projection feeds the dashboard's node/VM counts and storage occupancy
// (from Index.StoragesByNode) and the appinfo's per-cluster refresh state.
// The version string is surfaced in the dashboard and the public version
// endpoint.
func NewAdminOps(authHandler *Auth, st *store.Store, client cluster.Client, projection *inventory.Projection, version string, log *slog.Logger) *AdminOps {
	return &AdminOps{
		auth:       authHandler,
		store:      st,
		client:     client,
		projection: projection,
		version:    version,
		log:        log,
	}
}

type auditEntryDTO struct {
	ID        int64  `json:"id"`
	Actor     string `json:"actor"`
	Cluster   string `json:"cluster"`
	VMID      int    `json:"vmid"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
}

type auditPageDTO struct {
	Items    []auditEntryDTO `json:"items"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
}

// ServeAudit handles GET /api/v1/admin/audit (FR-001/FR-002).
func (h *AdminOps) ServeAudit(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseAuditFilter(w, r)
	if !ok {
		return
	}

	result, err := h.store.ListAuditLog(r.Context(), filter)
	if err != nil {
		h.log.Error("admin audit list failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	items := make([]auditEntryDTO, len(result.Items))
	for i, e := range result.Items {
		items[i] = auditEntryDTO{
			ID:        e.ID,
			Actor:     e.Actor,
			Cluster:   e.Cluster,
			VMID:      e.VMID,
			Action:    e.Action,
			Timestamp: e.Timestamp.UTC().Format(time.RFC3339),
		}
	}

	writeAdminJSON(w, http.StatusOK, auditPageDTO{
		Items:    items,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	})
}

// parseAuditFilter extracts the AuditFilter from query parameters. It returns
// ok=false if the page size exceeds the maximum (the error response is already
// written in that case).
func parseAuditFilter(w http.ResponseWriter, r *http.Request) (store.AuditFilter, bool) {
	q := r.URL.Query()

	pageSize, err := strconv.Atoi(q.Get("pageSize"))
	if err != nil || pageSize < 1 {
		pageSize = 20
	}

	if pageSize > maxAuditPageSize {
		writeAdminError(w, http.StatusBadRequest, "page_size_too_large", "pageSize exceeds the maximum of 100")
		return store.AuditFilter{}, false
	}

	page, err := strconv.Atoi(q.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	filter := store.AuditFilter{
		Cluster:  q.Get("cluster"),
		Actor:    q.Get("actor"),
		Action:   q.Get("action"),
		Page:     page,
		PageSize: pageSize,
	}

	if vmidStr := q.Get("vmid"); vmidStr != "" {
		vmid, err := strconv.Atoi(vmidStr)
		if err != nil {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", "vmid must be an integer")
			return store.AuditFilter{}, false
		}
		filter.VMID = &vmid
	}

	if fromStr := q.Get("from"); fromStr != "" {
		from, err := time.Parse(time.RFC3339Nano, fromStr)
		if err != nil {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", "from must be a valid RFC3339 timestamp")
			return store.AuditFilter{}, false
		}
		filter.From = &from
	}

	if toStr := q.Get("to"); toStr != "" {
		to, err := time.Parse(time.RFC3339Nano, toStr)
		if err != nil {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", "to must be a valid RFC3339 timestamp")
			return store.AuditFilter{}, false
		}
		filter.To = &to
	}

	return filter, true
}

type nodeSummaryDTO struct {
	Name             string  `json:"name"`
	Status           string  `json:"status"`
	VMCount          int     `json:"vmCount"`
	CPUCores         int     `json:"cpuCores"`
	CPUUsage         float64 `json:"cpuUsage"`
	MemoryTotalBytes int64   `json:"memoryTotalBytes"`
	MemoryUsedBytes  int64   `json:"memoryUsedBytes"`
}

type vmStatusCountsDTO struct {
	Running int `json:"running"`
	Paused  int `json:"paused"`
	Stopped int `json:"stopped"`
	Other   int `json:"other"`
}

type dashboardDTO struct {
	Nodes          []nodeSummaryDTO  `json:"nodes"`
	NodeCount      int               `json:"nodeCount"`
	VMCount        int               `json:"vmCount"`
	VMStatusCounts vmStatusCountsDTO `json:"vmStatusCounts"`
	Version        string            `json:"version"`
	RefreshedAt    string            `json:"refreshedAt"`
}

// ServeDashboard handles GET /api/v1/admin/dashboard (FR-004/FR-005/FR-006).
// Only Proxmox nodes that host at least one PVMSS-managed VM are surfaced;
// per-node CPU/RAM usage and VM counts come from the in-memory Index, and VM
// status counts come from Index.ByVMID. No cluster.Client call is made,
// satisfying SC-003 and constitution IV.
func (h *AdminOps) ServeDashboard(w http.ResponseWriter, _ *http.Request) {
	idx := h.projection.Load()
	if idx == nil {
		writeAdminError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}

	// Build a set of node names that host at least one PVMSS-managed VM.
	usedNodes := make(map[string]struct{}, len(idx.ByNode))
	for name := range idx.ByNode {
		usedNodes[name] = struct{}{}
	}

	nodes := make([]nodeSummaryDTO, 0, len(usedNodes))
	for _, n := range idx.Nodes {
		if _, ok := usedNodes[n.Name]; !ok {
			continue
		}
		nodes = append(nodes, nodeSummaryDTO{
			Name:             n.Name,
			Status:           string(n.Status),
			VMCount:          len(idx.ByNode[n.Name]),
			CPUCores:         n.CPUCores,
			CPUUsage:         n.CPUUsage,
			MemoryTotalBytes: n.MemoryTotal,
			MemoryUsedBytes:  n.MemoryUsed,
		})
	}

	// Stable sort by node name so the dashboard order is deterministic.
	sortNodeSummaries(nodes)

	var counts vmStatusCountsDTO
	for _, vm := range idx.ByVMID {
		switch vm.Status {
		case cluster.VMRunning:
			counts.Running++
		case cluster.VMPaused:
			counts.Paused++
		case cluster.VMStopped:
			counts.Stopped++
		default:
			counts.Other++
		}
	}

	writeAdminJSON(w, http.StatusOK, dashboardDTO{
		Nodes:          nodes,
		NodeCount:      len(nodes),
		VMCount:        len(idx.ByVMID),
		VMStatusCounts: counts,
		Version:        h.version,
		RefreshedAt:    idx.RefreshedAt.UTC().Format(time.RFC3339),
	})
}

// sortNodeSummaries sorts dashboard node summaries by name in place. Kept as a
// helper so the handler body stays linear.
func sortNodeSummaries(nodes []nodeSummaryDTO) {
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j-1].Name > nodes[j].Name; j-- {
			nodes[j-1], nodes[j] = nodes[j], nodes[j-1]
		}
	}
}

// ServeDBExport handles GET /api/v1/admin/db/export (FR-007). Streams a
// VACUUM INTO-produced snapshot as a binary file download.
func (h *AdminOps) ServeDBExport(w http.ResponseWriter, r *http.Request) {
	filename := "pvmss-" + time.Now().UTC().Format("20060102-150405") + ".db"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	if err := h.store.ExportDatabase(r.Context(), w); err != nil {
		h.log.Error("admin db export failed", "component", "httpapi", "error", err)
		// Headers already sent — the best we can do is log; the client will
		// see a truncated stream.
		return
	}
}

// --- Database import (US3) ---

type importPreviewDTO struct {
	StagingToken  string               `json:"stagingToken"`
	ExpiresAt     string               `json:"expiresAt"`
	Tables        []store.TablePreview `json:"tables"`
	IgnoredTables []string             `json:"ignoredTables"`
}

type importConfirmRequest struct {
	StagingToken string `json:"stagingToken"`
}

type importResultDTO struct {
	Status string               `json:"status"`
	Tables []store.TablePreview `json:"tables"`
}

// ServeDBImport handles POST /api/v1/admin/db/import (FR-008/FR-009).
// Accepts a multipart file upload, validates it, and returns a preview
// without writing anything to the live database.
func (h *AdminOps) ServeDBImport(w http.ResponseWriter, r *http.Request) {
	// Limit upload size to 50 MiB — a configuration database is small.
	// MaxBytesReader wraps the body so ParseMultipartForm cannot read beyond
	// the limit (gosec G120: unbounded form parsing).
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	//nolint:gosec // body is bounded by MaxBytesReader above
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_upload", "could not parse multipart upload")
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	file, _, err := r.FormFile("file")
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_upload", "missing 'file' field in multipart upload")
		return
	}
	defer func() { _ = file.Close() }()

	preview, err := h.store.ValidateImport(r.Context(), file)
	if err != nil {
		if errors.Is(err, store.ErrInvalidDatabase) {
			writeAdminError(w, http.StatusBadRequest, "invalid_database", "uploaded file is not a valid SQLite database")
			return
		}
		h.log.Error("admin db import validate failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	writeAdminJSON(w, http.StatusOK, importPreviewDTO{
		StagingToken:  preview.StagingToken,
		ExpiresAt:     preview.ExpiresAt.UTC().Format(time.RFC3339),
		Tables:        preview.Tables,
		IgnoredTables: preview.IgnoredTables,
	})
}

// ServeDBImportConfirm handles POST /api/v1/admin/db/import/confirm
// (FR-010/FR-012).
func (h *AdminOps) ServeDBImportConfirm(w http.ResponseWriter, r *http.Request) {
	var req importConfirmRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if req.StagingToken == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "stagingToken is required")
		return
	}

	result, err := h.store.ConfirmImport(r.Context(), req.StagingToken)
	if err != nil {
		if store.IsNotFound(err) {
			writeAdminError(w, http.StatusNotFound, "not_found", "staging token not found")
			return
		}
		if store.IsExpired(err) {
			writeAdminError(w, http.StatusGone, "expired", "import preview expired — upload again")
			return
		}
		h.log.Error("admin db import confirm failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "import_failed", "import failed, no changes were applied")
		return
	}

	writeAdminJSON(w, http.StatusOK, importResultDTO{Status: "restored", Tables: result.Tables})
}

type configFieldDTO struct {
	Name     string  `json:"name"`
	Value    *string `json:"value"`
	Redacted bool    `json:"redacted"`
}

type clusterHealthDTO struct {
	Name                 string `json:"name"`
	RefreshedAt          string `json:"refreshedAt"`
	LastRefreshSucceeded bool   `json:"lastRefreshSucceeded"`
}

type appInfoDTO struct {
	Version  string             `json:"version"`
	Config   []configFieldDTO   `json:"config"`
	Clusters []clusterHealthDTO `json:"clusters"`
}

// ServeAppInfo handles GET /api/v1/admin/appinfo (FR-013/FR-014).
func (h *AdminOps) ServeAppInfo(w http.ResponseWriter, _ *http.Request) {
	// The Configuration is loaded fresh from the environment so the admin
	// sees the current effective config, not a stale snapshot.
	cfg, err := config.Load()
	if err != nil {
		// If the env is broken, show what we can — the redaction logic does
		// not depend on a valid config, only on the field values.
		cfg = config.Configuration{}
	}

	fields := cfg.Redacted()
	configDTOs := make([]configFieldDTO, 0, len(fields))
	for _, f := range fields {
		dto := configFieldDTO{Name: f.Name, Redacted: f.Redacted}
		if !f.Redacted {
			dto.Value = &f.Value
		}
		// Redacted fields: Value stays nil → JSON null (contracts/admin-ops.md).
		configDTOs = append(configDTOs, dto)
	}

	// Per-cluster health from the projection's refresh state.
	idx := h.projection.Load()
	clusters := make([]clusterHealthDTO, 0, 1)
	if idx != nil {
		clusters = append(clusters, clusterHealthDTO{
			Name:                 defaultClusterName,
			RefreshedAt:          idx.RefreshedAt.UTC().Format(time.RFC3339),
			LastRefreshSucceeded: !idx.RefreshedAt.IsZero(),
		})
	}

	writeAdminJSON(w, http.StatusOK, appInfoDTO{
		Version:  h.version,
		Config:   configDTOs,
		Clusters: clusters,
	})
}

type publicVersionDTO struct {
	Version string `json:"version"`
}

// ServePublicVersion handles GET /api/v1/public/version (FR-015). No
// authentication required — the version alone is visible in the public
// footer (X17).
func (h *AdminOps) ServePublicVersion(w http.ResponseWriter, _ *http.Request) {
	body, err := json.Marshal(publicVersionDTO{Version: h.version})
	if err != nil {
		_ = writeJSON(w, http.StatusInternalServerError, []byte(`{"code":"internal_error"}`))
		return
	}
	_ = writeJSON(w, http.StatusOK, body)
}
