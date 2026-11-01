//nolint:wsl_v5 // endpoint handlers keep validation and contract mapping adjacent
package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/adoption"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"sync"
	"time"
)

// defaultRecoveryScanCooldown bounds how often GET /admin/recovery hits
// Proxmox: recovery is a one-shot migration assistant, not a monitoring page,
// so repeat page loads within the window serve the cached report.
const defaultRecoveryScanCooldown = 5 * time.Minute

// cachedRecoveryReport is the last report served for one cluster.
type cachedRecoveryReport struct {
	report recoveryReportDTO
	at     time.Time
}

// AdminRecovery serves the adoption report and adopt endpoints backing the
// /admin/recovery page: it surfaces cluster-side leftovers from a previous
// PVMSS instance and re-registers the ones the administrator selects.
type AdminRecovery struct {
	auth             *Auth
	clients          cluster.ClientProvider
	source           inventory.LookupSource
	projection       *inventory.Projection
	store            *store.Store
	log              *slog.Logger
	trustedProxyHops int
	scanCooldown     time.Duration
	mu               sync.Mutex
	cache            map[string]cachedRecoveryReport
}

// AdminRecoveryDeps groups the collaborators NewAdminRecovery needs.
type AdminRecoveryDeps struct {
	Auth       *Auth
	Clients    cluster.ClientProvider
	Source     inventory.LookupSource
	Projection *inventory.Projection
	Store      *store.Store
	Log        *slog.Logger
	// ScanCooldown caps automatic rescans; <= 0 uses the 5-minute default.
	ScanCooldown time.Duration
}

// NewAdminRecovery creates the handler with per-request client and projection
// resolution keyed on the cluster parameter, mirroring AdminPools.
func NewAdminRecovery(deps AdminRecoveryDeps) *AdminRecovery {
	cooldown := deps.ScanCooldown
	if cooldown <= 0 {
		cooldown = defaultRecoveryScanCooldown
	}
	return &AdminRecovery{auth: deps.Auth, clients: deps.Clients, source: deps.Source, projection: deps.Projection, store: deps.Store, log: deps.Log, scanCooldown: cooldown, cache: map[string]cachedRecoveryReport{}}
}

// recoveryVMRefDTO is the stable JSON shape for one VM row in the report.
type recoveryVMRefDTO struct {
	VMID   int      `json:"vmid"`
	Name   string   `json:"name"`
	Node   string   `json:"node"`
	Status string   `json:"status"`
	Pool   string   `json:"pool,omitempty"`
	Tags   []string `json:"tags"`
}

type recoveryPoolDTO struct {
	Name    string             `json:"name"`
	Comment string             `json:"comment"`
	Members []recoveryVMRefDTO `json:"members"`
}

type recoveryTagDTO struct {
	Name    string `json:"name"`
	VMCount int    `json:"vmCount"`
	Color   string `json:"color"`
}

// recoveryReportDTO is the stable JSON contract for
// GET /api/v1/admin/recovery and POST /api/v1/admin/recovery/scan.
type recoveryReportDTO struct {
	Cluster   string             `json:"cluster"`
	ScannedAt string             `json:"scannedAt"`
	Pools     []recoveryPoolDTO  `json:"pools"`
	LooseVMs  []recoveryVMRefDTO `json:"looseVMs"`
	Tags      []recoveryTagDTO   `json:"tags"`
}

type adoptRequest struct {
	Cluster string   `json:"cluster"`
	Pools   []string `json:"pools"`
	Tags    []string `json:"tags"`
}

type scanRequest struct {
	Cluster string `json:"cluster"`
}

// adoptResultDTO is the stable JSON contract for
// POST /api/v1/admin/recovery.
type adoptResultDTO struct {
	Pools []string         `json:"pools"`
	Tags  []recoveryTagDTO `json:"tags"`
}

// ServeReport handles GET /api/v1/admin/recovery?cluster=. The scan queries
// Proxmox, so reports are cached per cluster: a cached report is served while
// the cooldown holds, and a report with nothing left to adopt is pinned
// forever - only POST /scan forces a fresh cluster read.
func (h *AdminRecovery) ServeReport(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminActor(w, r); !ok {
		return
	}
	clusterName, err := h.resolveClusterName(r)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	if report, ok := h.cachedReport(clusterName); ok {
		writeAdminJSON(w, http.StatusOK, report)
		return
	}
	report, err := h.scan(w, r, clusterName)
	if err != nil {
		return
	}
	dto := reportDTO(clusterName, report)
	h.storeCachedReport(clusterName, dto)
	writeAdminJSON(w, http.StatusOK, dto)
}

// ServeScan handles POST /api/v1/admin/recovery/scan: the explicit rescan
// button. It is a mutation-guarded route so it gets CSRF and rate limiting,
// and its result replaces the cached report.
func (h *AdminRecovery) ServeScan(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminActor(w, r); !ok {
		return
	}
	var req scanRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}
	clusterName, clusterErr := ResolveClusterValue(req.Cluster, h.clients)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}
	report, err := h.scan(w, r, clusterName)
	if err != nil {
		return
	}
	dto := reportDTO(clusterName, report)
	h.storeCachedReport(clusterName, dto)
	writeAdminJSON(w, http.StatusOK, dto)
}

// ServeAdopt handles POST /api/v1/admin/recovery. The body names the orphan
// pools to mark managed and the tags to recreate in catalog_tags; unknown
// names are ignored so a stale report cannot corrupt other clusters.
func (h *AdminRecovery) ServeAdopt(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminActor(w, r); !ok {
		return
	}
	var req adoptRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}
	clusterName, clusterErr := ResolveClusterValue(req.Cluster, h.clients)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}
	client, projection, err := h.adoptionDeps(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	result, err := adoption.Adopt(r.Context(), client, projection, h.store, clusterName, adoption.Selection{Pools: req.Pools, Tags: req.Tags})
	if err != nil {
		h.writeRecoveryError(w, err)
		return
	}
	h.recordAdminAction(r, "admin.recovery.adopt", "cluster", clusterName,
		fmt.Sprintf("adopted %d pool(s) and %d tag(s) on cluster %s", len(result.Pools), len(result.Tags), clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, "pools": result.Pools, "tags": result.Tags}})
	h.invalidateCache(clusterName)
	writeAdminJSON(w, http.StatusOK, adoptResultDTO{Pools: result.Pools, Tags: adoptedTagDTOs(result.Tags)})
}

// cachedReport returns the cached report when it is still fresh, or forever
// when nothing adoptable remains (empty means the one-shot recovery is done).
func (h *AdminRecovery) cachedReport(clusterName string) (recoveryReportDTO, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry, ok := h.cache[clusterName]
	if !ok {
		return recoveryReportDTO{}, false
	}
	if len(entry.report.Pools) == 0 && len(entry.report.Tags) == 0 {
		return entry.report, true
	}
	if time.Since(entry.at) < h.scanCooldown {
		return entry.report, true
	}
	return recoveryReportDTO{}, false
}

func (h *AdminRecovery) storeCachedReport(clusterName string, report recoveryReportDTO) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache[clusterName] = cachedRecoveryReport{report: report, at: time.Now()}
}

func (h *AdminRecovery) invalidateCache(clusterName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.cache, clusterName)
}

// scan resolves the deps and runs the adoption scan, writing the error
// response itself so both endpoints share one mapping.
func (h *AdminRecovery) scan(w http.ResponseWriter, r *http.Request, clusterName string) (adoption.Report, error) {
	client, projection, err := h.adoptionDeps(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return adoption.Report{}, err
	}
	report, err := adoption.Scan(r.Context(), client, projection, h.store, clusterName)
	if err != nil {
		h.writeRecoveryError(w, err)
		return adoption.Report{}, err
	}
	return report, nil
}

func (h *AdminRecovery) adoptionDeps(clusterName string) (cluster.Client, *inventory.Projection, error) {
	client, err := h.clientFor(clusterName)
	if err != nil {
		return nil, nil, err
	}
	projection, err := h.projectionFor(clusterName)
	if err != nil {
		return nil, nil, err
	}
	return client, projection, nil
}

// writeRecoveryError maps scan/adopt failures: the projection gap is a 503,
// a cluster read failure is a 502, anything else is a 500.
func (h *AdminRecovery) writeRecoveryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pools.ErrProjectionNotReady):
		writeAdminError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
	case errors.Is(err, cluster.ErrUnreachable):
		SetErrorMsg(w, "cluster list pools failed", err)
		writeAdminError(w, http.StatusBadGateway, "cluster_unreachable", "failed to list pools")
	default:
		SetErrorMsg(w, "admin recovery failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

func reportDTO(clusterName string, report adoption.Report) recoveryReportDTO {
	poolDTOs := make([]recoveryPoolDTO, len(report.Pools))
	for i, pool := range report.Pools {
		poolDTOs[i] = recoveryPoolDTO{Name: pool.Name, Comment: pool.Comment, Members: vmRefDTOs(pool.Members)}
	}
	return recoveryReportDTO{
		Cluster:   clusterName,
		ScannedAt: time.Now().UTC().Format(time.RFC3339),
		Pools:     poolDTOs,
		LooseVMs:  vmRefDTOs(report.LooseVMs),
		Tags:      orphanTagDTOs(report.Tags),
	}
}

func vmRefDTOs(refs []adoption.VMRef) []recoveryVMRefDTO {
	out := make([]recoveryVMRefDTO, len(refs))
	for i, ref := range refs {
		tags := ref.Tags
		if tags == nil {
			tags = []string{}
		}
		out[i] = recoveryVMRefDTO{VMID: ref.VMID, Name: ref.Name, Node: ref.Node, Status: ref.Status, Pool: ref.Pool, Tags: tags}
	}
	return out
}

func orphanTagDTOs(tags []adoption.OrphanTag) []recoveryTagDTO {
	out := make([]recoveryTagDTO, len(tags))
	for i, tag := range tags {
		out[i] = recoveryTagDTO{Name: tag.Name, VMCount: tag.VMCount, Color: tag.Color}
	}
	return out
}

func adoptedTagDTOs(tags []adoption.AdoptedTag) []recoveryTagDTO {
	out := make([]recoveryTagDTO, len(tags))
	for i, tag := range tags {
		out[i] = recoveryTagDTO{Name: tag.Name, Color: tag.Color}
	}
	return out
}

// clientFor resolves the cluster.Client for clusterName; when empty, the
// first registered cluster is used (mirrors AdminPools).
func (h *AdminRecovery) clientFor(clusterName string) (cluster.Client, error) {
	if h.clients == nil {
		return nil, cluster.ErrClusterNotFound
	}
	if clusterName == "" {
		names := h.clients.List()
		if len(names) == 0 {
			return nil, cluster.ErrClusterNotFound
		}
		clusterName = names[0]
	}
	return h.clients.Client(clusterName)
}

// projectionFor resolves the inventory.Projection for clusterName, falling
// back to the single bound projection when source is not a Registry.
func (h *AdminRecovery) projectionFor(clusterName string) (*inventory.Projection, error) {
	registry, ok := h.source.(*inventory.Registry)
	if !ok {
		return h.projection, nil
	}
	if clusterName == "" {
		names := h.clients.List()
		if len(names) == 0 {
			return nil, cluster.ErrClusterNotFound
		}
		clusterName = names[0]
	}
	return registry.Projection(clusterName)
}

// resolveClusterName resolves the ?cluster= query parameter, falling back to
// the first available cluster (mirrors AdminPools).
func (h *AdminRecovery) resolveClusterName(r *http.Request) (string, error) {
	clusterName := queryCluster(r)
	if clusterName != "" {
		return clusterName, nil
	}
	if h.clients == nil {
		return "", nil
	}
	names := h.clients.List()
	if len(names) == 0 {
		return "", cluster.ErrClusterNotFound
	}
	return names[0], nil
}

func (h *AdminRecovery) adminActor(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	actor, err := h.auth.Principal(r)
	if err != nil {
		writeAdminError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return auth.Identity{}, false
	}
	if !actor.IsAdmin {
		writeAdminError(w, http.StatusForbidden, "forbidden", msgAdminOnly)
		return auth.Identity{}, false
	}
	return actor, true
}

// SetTrustedProxyHops configures how many X-Forwarded-For hops to trust for
// client IP extraction used in audit log entries.
func (h *AdminRecovery) SetTrustedProxyHops(n int) {
	h.trustedProxyHops = n
}

func (h *AdminRecovery) recordAdminAction(r *http.Request, action, targetType, targetID, summary string, changes []any) {
	actor, _ := h.auth.Principal(r)
	if err := h.store.RecordAdminAction(r.Context(), actor.Username, action, targetType, targetID, detailJSON(summary, changes), clientIP(r, h.trustedProxyHops)); err != nil {
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to record admin action", "component", "httpapi", "action", action, "error", err)
	}
}
