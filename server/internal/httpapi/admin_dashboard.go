package httpapi

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"slices"
	"strings"
	"time"
)

// Dashboard alert thresholds, in percent. Storage turns critical near full
// because a full datastore stops every VM writing to it.
const (
	nodeUsageAlertPercent       = 90
	storageWarningPercent       = 85
	storageCriticalPercent      = 95
	dashboardRecentChangesLimit = 8
)

const (
	alertCritical = "critical"
	alertWarning  = "warning"
)

// sharedStoragePlugins are Proxmox storage types that every node of a cluster
// sees as one datastore. The dashboard lists them once, not once per node.
var sharedStoragePlugins = map[string]struct{}{
	"rbd": {}, "cephfs": {}, "nfs": {}, "cifs": {}, "glusterfs": {},
	"iscsi": {}, "iscsidirect": {}, "pbs": {},
}

// dashboardInventory is every cluster's current index. Both
// *inventory.Registry and *inventory.Projection satisfy it; a nil index
// means that cluster has not completed a refresh.
type dashboardInventory interface {
	All() map[string]*inventory.Index
}

// SetInventorySource makes the dashboard cover every cluster of src instead
// of the single projection given to NewAdminOps. A cluster whose index is
// missing, or older than staleAfter (when > 0), is reported unreachable.
func (h *AdminOps) SetInventorySource(src dashboardInventory, staleAfter time.Duration) {
	h.inventory = src
	h.staleAfter = staleAfter
}

type nodeSummaryDTO struct {
	ClusterKey       string  `json:"clusterKey"`
	Cluster          string  `json:"cluster"`
	Name             string  `json:"name"`
	Status           string  `json:"status"`
	VMCount          int     `json:"vmCount"`
	VMRunningCount   int     `json:"vmRunningCount"`
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

// dashboardAlertDTO is one thing an administrator should act on. Kind is
// one of cluster_unreachable, node_offline, node_cpu, node_memory,
// storage_full, pool_at_quota; Subject names the node, storage or pool.
// Cluster is the display label; ClusterKey is the registry key routes use.
type dashboardAlertDTO struct {
	Kind       string `json:"kind"`
	Severity   string `json:"severity"`
	Cluster    string `json:"cluster"`
	ClusterKey string `json:"clusterKey,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Percent    int    `json:"percent,omitempty"`
}

// dashboardStorageDTO is one datastore. Node is empty for shared storage.
type dashboardStorageDTO struct {
	Cluster    string `json:"cluster"`
	Name       string `json:"name"`
	Node       string `json:"node,omitempty"`
	Type       string `json:"type"`
	Shared     bool   `json:"shared"`
	UsedBytes  int64  `json:"usedBytes"`
	TotalBytes int64  `json:"totalBytes"`
	Percent    int    `json:"percent"`
}

type dashboardDTO struct {
	Alerts              []dashboardAlertDTO   `json:"alerts"`
	Nodes               []nodeSummaryDTO      `json:"nodes"`
	NodeCount           int                   `json:"nodeCount"`
	VMCount             int                   `json:"vmCount"`
	VMStatusCounts      vmStatusCountsDTO     `json:"vmStatusCounts"`
	PVMSSVMCount        int                   `json:"pvmssVMCount"`
	PVMSSVMStatusCounts vmStatusCountsDTO     `json:"pvmssVMStatusCounts"`
	OtherVMCount        int                   `json:"otherVMCount"`
	Storages            []dashboardStorageDTO `json:"storages"`
	RecentChanges       []auditEntryDTO       `json:"recentChanges"`
	Version             string                `json:"version"`
	RefreshedAt         string                `json:"refreshedAt"`
}

// ServeDashboard handles GET /api/v1/admin/dashboard.
// Everything comes from the in-memory indexes and the store; no
// cluster.Client call is made, satisfying the read/write separation. Only
// nodes hosting at least one PVMSS-managed VM are listed, but alerts cover
// every node. Alerts are computed here so the page needs one request.
func (h *AdminOps) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	indexes := h.inventory.All()
	names := make([]string, 0, len(indexes))

	for name := range indexes {
		names = append(names, name)
	}

	slices.Sort(names)

	dash := dashboardDTO{
		Alerts:        []dashboardAlertDTO{},
		Nodes:         []nodeSummaryDTO{},
		Storages:      []dashboardStorageDTO{},
		RecentChanges: h.recentChanges(r.Context()),
		Version:       h.version,
	}

	var newest time.Time

	labels := h.clusterLabels(r.Context())

	for _, name := range names {
		idx := indexes[name]
		label := cmp.Or(labels[name], name)

		if idx == nil || (h.staleAfter > 0 && time.Since(idx.RefreshedAt) > h.staleAfter) {
			dash.Alerts = append(dash.Alerts, dashboardAlertDTO{Kind: "cluster_unreachable", Severity: alertCritical, Cluster: label, ClusterKey: name})
		}

		if idx == nil {
			continue
		}

		if idx.RefreshedAt.After(newest) {
			newest = idx.RefreshedAt
		}

		h.addClusterToDashboard(r.Context(), &dash, name, label, idx)
	}

	if !anyIndex(indexes) {
		writeAdminError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}

	dash.NodeCount = len(dash.Nodes)
	dash.RefreshedAt = newest.UTC().Format(time.RFC3339)
	slices.SortStableFunc(dash.Alerts, compareAlerts)
	writeAdminJSON(w, http.StatusOK, dash)
}

// clusterLabels maps cluster names to the display names administrators set.
// A read failure is logged and the raw names are shown instead.
func (h *AdminOps) clusterLabels(ctx context.Context) map[string]string {
	rows, err := h.store.ListClusters(ctx)
	if err != nil {
		h.log.Error("dashboard cluster read failed", "component", "httpapi", "error", err)
		return nil
	}

	labels := make(map[string]string, len(rows))
	for _, row := range rows {
		labels[row.Name] = row.DisplayName
	}

	return labels
}

func anyIndex(indexes map[string]*inventory.Index) bool {
	for _, idx := range indexes {
		if idx != nil {
			return true
		}
	}

	return false
}

// addClusterToDashboard folds one cluster into dash. name keys the store
// (policy); label is what the administrator reads.
func (h *AdminOps) addClusterToDashboard(ctx context.Context, dash *dashboardDTO, name, label string, idx *inventory.Index) {
	for _, node := range idx.Nodes {
		dash.Alerts = append(dash.Alerts, nodeAlerts(name, label, node)...)

		vms := idx.ByNode[node.Name]
		if len(vms) == 0 {
			continue
		}

		dash.Nodes = append(dash.Nodes, nodeSummaryDTO{
			ClusterKey:       name,
			Cluster:          label,
			Name:             node.Name,
			Status:           string(node.Status),
			VMCount:          len(vms),
			VMRunningCount:   countRunning(vms),
			CPUCores:         node.CPUCores,
			CPUUsage:         node.CPUUsage,
			MemoryTotalBytes: node.MemoryTotal,
			MemoryUsedBytes:  node.MemoryUsed,
		})
	}

	for _, vm := range idx.ByVMID {
		dash.VMCount++
		countVMStatus(&dash.VMStatusCounts, vm.Status)

		if slices.Contains(vm.Tags, catalog.ProtectedTagName) {
			dash.PVMSSVMCount++
			countVMStatus(&dash.PVMSSVMStatusCounts, vm.Status)
		} else {
			dash.OtherVMCount++
		}
	}

	for _, s := range clusterStorages(label, idx) {
		dash.Storages = append(dash.Storages, s)
		dash.Alerts = appendStorageAlert(dash.Alerts, name, s)
	}

	dash.Alerts = append(dash.Alerts, h.poolQuotaAlerts(ctx, name, label, idx)...)
}

func nodeAlerts(clusterKey, label string, node cluster.Node) []dashboardAlertDTO {
	if node.Status == cluster.NodeOffline {
		return []dashboardAlertDTO{{Kind: "node_offline", Severity: alertCritical, Cluster: label, ClusterKey: clusterKey, Subject: node.Name}}
	}

	var alerts []dashboardAlertDTO

	if cpu := int(node.CPUUsage*100 + 0.5); cpu >= nodeUsageAlertPercent {
		alerts = append(alerts, dashboardAlertDTO{Kind: "node_cpu", Severity: alertWarning, Cluster: label, ClusterKey: clusterKey, Subject: node.Name, Percent: cpu})
	}

	if mem := percentOf(node.MemoryUsed, node.MemoryTotal); mem >= nodeUsageAlertPercent {
		alerts = append(alerts, dashboardAlertDTO{Kind: "node_memory", Severity: alertWarning, Cluster: label, ClusterKey: clusterKey, Subject: node.Name, Percent: mem})
	}

	return alerts
}

// clusterStorages lists a cluster's active datastores, shared ones once.
func clusterStorages(clusterName string, idx *inventory.Index) []dashboardStorageDTO {
	seenShared := map[string]struct{}{}

	var out []dashboardStorageDTO

	for _, node := range idx.Nodes {
		for _, s := range idx.StoragesByNode[node.Name] {
			if s.Total <= 0 {
				continue
			}

			kind := cmp.Or(s.PluginType, s.Type)
			_, shared := sharedStoragePlugins[kind]

			if shared {
				if _, dup := seenShared[s.Name]; dup {
					continue
				}

				seenShared[s.Name] = struct{}{}
			}

			dto := dashboardStorageDTO{
				Cluster: clusterName, Name: s.Name, Node: s.Node, Type: kind, Shared: shared,
				UsedBytes: s.Used, TotalBytes: s.Total, Percent: percentOf(s.Used, s.Total),
			}
			if shared {
				dto.Node = ""
			}

			out = append(out, dto)
		}
	}

	return out
}

func appendStorageAlert(alerts []dashboardAlertDTO, clusterKey string, s dashboardStorageDTO) []dashboardAlertDTO {
	var severity string

	switch {
	case s.Percent >= storageCriticalPercent:
		severity = alertCritical
	case s.Percent >= storageWarningPercent:
		severity = alertWarning
	default:
		return alerts
	}

	return append(alerts, dashboardAlertDTO{Kind: "storage_full", Severity: severity, Cluster: s.Cluster, ClusterKey: clusterKey, Subject: s.Name, Percent: s.Percent})
}

// poolQuotaAlerts reports pools holding as many VMs as the cluster's
// per-user quota allows. A cluster with no stored policy is unlimited.
func (h *AdminOps) poolQuotaAlerts(ctx context.Context, clusterName, label string, idx *inventory.Index) []dashboardAlertDTO {
	row, err := h.store.PolicyRow(ctx, clusterName)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			h.log.Error("dashboard policy read failed", "component", "httpapi", "cluster", clusterName, "error", err)
		}

		return nil
	}

	if row.MaxVMPerUser <= 0 {
		return nil
	}

	var alerts []dashboardAlertDTO

	for pool, vms := range idx.ByPool {
		if pool != "" && len(vms) >= row.MaxVMPerUser {
			alerts = append(alerts, dashboardAlertDTO{
				Kind: "pool_at_quota", Severity: alertWarning, Cluster: label, ClusterKey: clusterName, Subject: pool,
				Percent: percentOf(int64(len(vms)), int64(row.MaxVMPerUser)),
			})
		}
	}

	return alerts
}

// recentChanges returns the latest audit entries. A read failure is logged
// and yields an empty list: the rest of the dashboard is still useful.
func (h *AdminOps) recentChanges(ctx context.Context) []auditEntryDTO {
	page, err := h.store.ListAuditLog(ctx, store.AuditFilter{Page: 1, PageSize: dashboardRecentChangesLimit})
	if err != nil {
		h.log.Error("dashboard audit read failed", "component", "httpapi", "error", err)
		return []auditEntryDTO{}
	}

	out := make([]auditEntryDTO, len(page.Items))
	for i, e := range page.Items {
		out[i] = toAuditEntryDTO(e)
	}

	return out
}

func countRunning(vms []cluster.VM) int {
	n := 0

	for _, vm := range vms {
		if vm.Status == cluster.VMRunning {
			n++
		}
	}

	return n
}

func countVMStatus(counts *vmStatusCountsDTO, status cluster.VMStatus) {
	switch status {
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

func percentOf(used, total int64) int {
	if total <= 0 {
		return 0
	}

	return int((used*100 + total/2) / total)
}

func compareAlerts(a, b dashboardAlertDTO) int {
	return cmp.Or(
		cmp.Compare(severityRank(a.Severity), severityRank(b.Severity)),
		strings.Compare(a.Kind, b.Kind),
		strings.Compare(a.Cluster, b.Cluster),
		strings.Compare(a.Subject, b.Subject),
	)
}

func severityRank(severity string) int {
	if severity == alertCritical {
		return 0
	}

	return 1
}
