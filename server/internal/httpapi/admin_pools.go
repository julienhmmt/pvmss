//nolint:wsl_v5 // endpoint handlers keep validation and contract mapping adjacent
package httpapi

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"slices"
)

// AdminPools serves the admin pool list, provisioning, and cascade endpoints.
type AdminPools struct {
	auth             *Auth
	client           cluster.Client
	clients          cluster.ClientProvider
	source           inventory.LookupSource
	projection       *inventory.Projection
	writer           cluster.Writer
	audit            vm.AuditRecorder
	refresher        vm.IndexRefresher
	store            *store.Store
	log              *slog.Logger
	trustedProxyHops int
}

// AdminPoolsDeps groups the collaborators AdminPools needs. Bundling them
// keeps NewAdminPools's parameter count under go:S107's ceiling and makes
// mis-ordering at call sites impossible.
type AdminPoolsDeps struct {
	Auth       *Auth
	Client     cluster.Client
	Projection *inventory.Projection
	Writer     cluster.Writer
	Audit      vm.AuditRecorder
	Refresher  vm.IndexRefresher
	Store      *store.Store
	Log        *slog.Logger
}

// NewAdminPools creates the pool administration handler, bound to a single
// cluster. The store enables managed-pool tracking: only pools PVMSS
// provisioned may be deleted. Use NewAdminPoolsWithRegistry for multi-cluster
// deployments.
func NewAdminPools(deps AdminPoolsDeps) *AdminPools {
	return &AdminPools{auth: deps.Auth, client: deps.Client, projection: deps.Projection, writer: deps.Writer, audit: deps.Audit, refresher: deps.Refresher, store: deps.Store, log: deps.Log}
}

// AdminPoolsRegistryDeps groups the collaborators NewAdminPoolsWithRegistry
// needs for multi-cluster wiring. Bundling them keeps the parameter count
// under go:S107's ceiling and makes mis-ordering at call sites impossible.
type AdminPoolsRegistryDeps struct {
	Auth       *Auth
	Clients    cluster.ClientProvider
	Source     inventory.LookupSource
	Projection *inventory.Projection
	Writer     cluster.Writer
	Audit      vm.AuditRecorder
	Refresher  vm.IndexRefresher
	Store      *store.Store
	Log        *slog.Logger
}

// NewAdminPoolsWithRegistry creates the handler with per-request client and
// projection resolution, keyed on the ?cluster= query parameter every
// endpoint here already reads - without this, an admin managing pools on a
// non-default cluster would silently operate against the default cluster's
// Proxmox API instead.
func NewAdminPoolsWithRegistry(deps AdminPoolsRegistryDeps) *AdminPools {
	handler := NewAdminPools(AdminPoolsDeps{Auth: deps.Auth, Client: nil, Projection: deps.Projection, Writer: deps.Writer, Audit: deps.Audit, Refresher: deps.Refresher, Store: deps.Store, Log: deps.Log})
	handler.clients = deps.Clients
	handler.source = deps.Source

	return handler
}

// clientFor resolves the cluster.Client for clusterName, falling back to the
// single bound client when clients is nil (legacy single-cluster ctor). When
// clusterName is empty and a multi-cluster registry is bound, the first
// available cluster is used (preserves the omitted-?cluster= behavior the
// admin pools tests rely on).
func (h *AdminPools) clientFor(clusterName string) (cluster.Client, error) {
	if h.clients == nil {
		if h.client == nil {
			return nil, cluster.ErrClusterNotFound
		}

		return h.client, nil
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
// back to the single bound projection when source is nil. When clusterName
// is empty and a multi-cluster registry is bound, the first available
// cluster's projection is used.
func (h *AdminPools) projectionFor(clusterName string) (*inventory.Projection, error) {
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

// resolveClusterName resolves the effective cluster name from the request's
// ?cluster= query parameter, falling back to the first available cluster
// when it is empty. Returns the resolved name (never empty on success).
func (h *AdminPools) resolveClusterName(r *http.Request) (string, error) {
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

// ServeList handles GET /api/v1/admin/pools.
func (h *AdminPools) ServeList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminActor(w, r); !ok {
		return
	}
	clusterName, err := h.resolveClusterName(r)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	projection, err := h.projectionFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	rows, err := pools.ListWithManaged(r.Context(), client, projection, h.store, clusterName, r.URL.Query().Get("search"))
	if err != nil {
		SetErrorMsg(w, "admin pool list failed", err)
		writeAdminError(w, http.StatusBadGateway, "cluster_unreachable", "failed to list pools")
		return
	}
	writeAdminJSON(w, http.StatusOK, poolSummaries(rows))
}

// ServeDetail handles GET /api/v1/admin/pools/{name}.
func (h *AdminPools) ServeDetail(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminActor(w, r); !ok {
		return
	}
	name := r.PathValue("name")
	if name == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_pool_name", "invalid pool name")
		return
	}
	clusterName, err := h.resolveClusterName(r)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	projection, err := h.projectionFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	detail, err := pools.Detail(r.Context(), client, projection, h.store, clusterName, name)
	if errors.Is(err, pools.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "pool \""+name+"\" not found")
		return
	}
	if err != nil {
		SetErrorMsg(w, "admin pool detail failed for "+name, err)
		writeAdminError(w, http.StatusBadGateway, "cluster_unreachable", "failed to load pool")
		return
	}
	quota, err := policy.New(h.store, projection, nil).Quota(r.Context(), clusterName, auth.Identity{Pool: name})
	if err != nil {
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "admin pool quota failed", "component", "httpapi", "pool", name, "error", err)
		quota = policy.Quota{Used: len(detail.Members)}
	}
	writeAdminJSON(w, http.StatusOK, poolDetailDTO{
		Name:      detail.Name,
		Username:  detail.Username,
		Comment:   detail.Comment,
		Cluster:   clusterName,
		Managed:   detail.Managed,
		CreatedAt: detail.CreatedAt,
		Quota:     poolQuotaDTO{Used: quota.Used, Allowed: quota.Allowed},
		VMs:       poolMembers(detail.Members),
		Activity:  h.poolActivity(r, clusterName, name),
	})
}

// poolActivityLimit caps the merged recent-activity feed on the pool detail
// page; the full audit view lives on /admin/settings.
const poolActivityLimit = 10

// poolActivity merges the pool user's own actions (actor "<pool>@pve") with
// admin actions targeting the pool into one most-recent-first feed. Query
// failures degrade to whatever the other query returned - activity is
// informational and must not fail the detail page.
func (h *AdminPools) poolActivity(r *http.Request, clusterName, name string) []auditEntryDTO {
	entries := map[int64]store.AuditEntry{}
	if h.store == nil {
		return []auditEntryDTO{}
	}
	for _, filter := range []store.AuditFilter{
		{Cluster: clusterName, Actor: name + "@pve", Page: 1, PageSize: poolActivityLimit},
		{TargetType: "pool", TargetID: name, Page: 1, PageSize: poolActivityLimit},
	} {
		page, err := h.store.ListAuditLog(r.Context(), filter)
		if err != nil {
			logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "pool activity query failed", "component", "httpapi", "pool", name, "error", err)
			continue
		}
		for _, entry := range page.Items {
			entries[entry.ID] = entry
		}
	}
	sorted := slices.SortedFunc(maps.Values(entries), func(a, b store.AuditEntry) int {
		if order := b.Timestamp.Compare(a.Timestamp); order != 0 {
			return order
		}
		return cmp.Compare(b.ID, a.ID)
	})
	if len(sorted) > poolActivityLimit {
		sorted = sorted[:poolActivityLimit]
	}
	out := make([]auditEntryDTO, len(sorted))
	for i, entry := range sorted {
		out[i] = toAuditEntryDTO(entry)
	}
	return out
}

// ServeCreate handles POST /api/v1/admin/pools.
func (h *AdminPools) ServeCreate(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminActor(w, r)
	if !ok {
		return
	}
	var request createPoolRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}
	clusterName, err := h.resolveClusterName(r)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	creds, err := pools.CreateManaged(r.Context(), actor, client, h.store, clusterName, request.Name, request.Comment)
	if err != nil {
		h.writeCreateError(w, err)
		return
	}

	h.recordAdminAction(r, "admin.pools.create", "pool", creds.PoolName,
		fmt.Sprintf("created managed pool %s on cluster %s", creds.PoolName, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: creds.PoolName, "username": creds.Username, "comment": creds.Comment, "managed": true}})
	writeAdminJSON(w, http.StatusCreated, createPoolResponse{
		Name:     creds.PoolName,
		Username: creds.Username,
		Password: creds.Password,
		Comment:  creds.Comment,
		Managed:  true,
	})
}

// ServeDelete handles DELETE /api/v1/admin/pools/{name}.
func (h *AdminPools) ServeDelete(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminActor(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if name == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_pool_name", "invalid pool name")
		return
	}
	clusterName, err := h.resolveClusterName(r)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	projection, err := h.projectionFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	writer, err := resolveCapability(h.clients, h.writer, clusterName, "Writer")
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	result, err := pools.Delete(r.Context(), pools.CascadeDeps{Actor: actor, Client: client, Projection: projection, ClusterName: clusterName, Writer: writer, Audit: h.audit, Refresher: h.refresher, Managed: h.store}, name)
	if errors.Is(err, pools.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "pool \""+name+"\" not found")
		return
	}
	if errors.Is(err, pools.ErrForbidden) {
		writeAdminError(w, http.StatusForbidden, "forbidden", msgAdminOnly)
		return
	}
	if errors.Is(err, pools.ErrNotManaged) {
		writeAdminError(w, http.StatusConflict, "not_managed", "pool \""+name+"\" is not managed by PVMSS")
		return
	}
	if err != nil {
		SetErrorMsg(w, "admin pool deletion failed for "+name, err)
		writeAdminError(w, http.StatusBadGateway, "deletion_failed", "pool deletion failed")
		return
	}
	h.recordAdminAction(r, "admin.pools.delete", "pool", name,
		fmt.Sprintf("deleted managed pool %s on cluster %s", name, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: name, "status": result.Status, "userDeleted": result.UserDeleted}})
	writeAdminJSON(w, http.StatusOK, deletePoolResponse{Status: result.Status, UserDeleted: result.UserDeleted})
}

// deletePoolResponse is the stable JSON contract for DELETE /api/v1/admin/pools/{name}.
type deletePoolResponse struct {
	Status      string `json:"status"`
	UserDeleted bool   `json:"userDeleted"`
}

type createPoolRequest struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

type createPoolResponse struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Comment  string `json:"comment"`
	Managed  bool   `json:"managed"`
}

// poolDetailDTO is the stable JSON contract for GET /api/v1/admin/pools/{name}.
type poolDetailDTO struct {
	Name      string          `json:"name"`
	Username  string          `json:"username"`
	Comment   string          `json:"comment"`
	Cluster   string          `json:"cluster"`
	Managed   bool            `json:"managed"`
	CreatedAt string          `json:"createdAt,omitempty"`
	Quota     poolQuotaDTO    `json:"quota"`
	VMs       []poolMemberDTO `json:"vms"`
	Activity  []auditEntryDTO `json:"activity"`
}

// poolQuotaDTO is the pool's VM count against the per-user allowance.
type poolQuotaDTO struct {
	Used    int `json:"used"`
	Allowed int `json:"allowed"`
}

// poolMemberDTO is one member VM of the pool, with the fields the detail
// page's table shows.
type poolMemberDTO struct {
	VMID          int      `json:"vmid"`
	Name          string   `json:"name"`
	Node          string   `json:"node"`
	Status        string   `json:"status"`
	UptimeSeconds int64    `json:"uptimeSeconds"`
	CPUCores      int      `json:"cpuCores"`
	MemoryBytes   int64    `json:"memoryBytes"`
	DiskBytes     int64    `json:"diskBytes"`
	IPAddresses   []string `json:"ipAddresses"`
}

func poolMembers(members []cluster.VM) []poolMemberDTO {
	out := make([]poolMemberDTO, len(members))
	for i, machine := range members {
		ips := []string{}
		for _, iface := range machine.NetworkInterfaces {
			ips = append(ips, iface.IPAddresses...)
		}
		out[i] = poolMemberDTO{
			VMID:          machine.VMID,
			Name:          machine.Name,
			Node:          machine.Node,
			Status:        string(machine.Status),
			UptimeSeconds: int64(machine.Uptime.Seconds()),
			CPUCores:      machine.CPUCores,
			MemoryBytes:   machine.MemoryTotal,
			DiskBytes:     machine.DiskTotal,
			IPAddresses:   ips,
		}
	}
	return out
}

type poolSummary struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
	Total   int    `json:"total"`
	Running int    `json:"running"`
	Stopped int    `json:"stopped"`
	Managed bool   `json:"managed"`
}

type poolSummaryList []poolSummary

func poolSummaries(rows []pools.PoolSummary) poolSummaryList {
	result := make(poolSummaryList, len(rows))
	for index, row := range rows {
		result[index] = poolSummary{Name: row.Name, Comment: row.Comment, Total: row.Total, Running: row.Running, Stopped: row.Stopped, Managed: row.Managed}
	}
	return result
}

func (h *AdminPools) adminActor(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
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
func (h *AdminPools) SetTrustedProxyHops(n int) {
	h.trustedProxyHops = n
}

func (h *AdminPools) recordAdminAction(r *http.Request, action, targetType, targetID, summary string, changes []any) {
	actor, _ := h.auth.Principal(r)
	if err := h.store.RecordAdminAction(r.Context(), actor.Username, action, targetType, targetID, detailJSON(summary, changes), clientIP(r, h.trustedProxyHops)); err != nil {
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to record admin action", "component", "httpapi", "action", action, "error", err)
	}
}

func (h *AdminPools) writeCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pools.ErrInvalidName):
		writeAdminError(w, http.StatusBadRequest, "invalid_pool_name", "invalid pool name")
	case errors.Is(err, pools.ErrAlreadyExists):
		writeAdminError(w, http.StatusConflict, "duplicate_pool", err.Error())
	case errors.Is(err, pools.ErrForbidden):
		writeAdminError(w, http.StatusForbidden, "forbidden", msgAdminOnly)
	default:
		if provisioningErr, ok := errors.AsType[*pools.ProvisionError](err); ok {
			SetErrorMsg(w, "admin pool provisioning step "+provisioningErr.Step+" failed", provisioningErr.Err)
		} else {
			SetErrorMsg(w, "admin pool provisioning failed", err)
		}
		writeAdminError(w, http.StatusBadGateway, "provisioning_failed", "pool provisioning failed")
	}
}
