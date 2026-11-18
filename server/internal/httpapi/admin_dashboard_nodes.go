package httpapi

import (
	"cmp"
	"context"
	"log/slog"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"slices"
)

// Dashboard node states, worst first. An empty state means the catalog could
// not be read, so readiness is unknown.
const (
	nodeStateOffline         = "offline"
	nodeStateIncomplete      = "incomplete"
	nodeStateDisabledWithVMs = "disabled_with_vms"
	nodeStateReady           = "ready"
)

var nodeStateRank = map[string]int{
	nodeStateOffline: 0, nodeStateIncomplete: 1, nodeStateDisabledWithVMs: 2, nodeStateReady: 3,
}

// nodeCatalog is the approved catalog of one cluster, as the dashboard reads
// it to place nodes.
type nodeCatalog struct {
	resources catalog.Resources
	templates []catalog.Template
}

func (c nodeCatalog) enabled(node string) bool {
	return c.resources.HasNode(node)
}

// loadNodeCatalog reads the cluster's catalog from the store. A read failure
// is logged and reported as unknown: the dashboard then falls back to nodes
// hosting PVMSS VMs.
func loadNodeCatalog(ctx context.Context, st *store.Store, log *slog.Logger, clusterName string) (nodeCatalog, bool) {
	resources, err := catalog.ApprovedResources(ctx, st, clusterName)
	if err == nil {
		var templates []catalog.Template

		templates, err = catalog.Templates(ctx, st, clusterName)
		if err == nil {
			return nodeCatalog{resources: resources, templates: templates}, true
		}
	}

	log.WarnContext(ctx, "dashboard node catalog read failed", "component", "httpapi", "cluster", clusterName, "error", err)

	return nodeCatalog{}, false
}

// dashboardNodes lists the nodes PVMSS cares about in one cluster: enabled in
// the catalog, or hosting PVMSS VMs. Enabled nodes the inventory no longer
// reports are listed offline. The result is sorted worst state first, then by
// name; other is how many reported nodes were left out.
//
// shortcut: readiness uses approved ISO/template rows, not live discovery (the
// dashboard makes no Proxmox call); upgrade if inventory starts carrying ISOs.
func dashboardNodes(name, label string, idx *inventory.Index, cat nodeCatalog, catalogKnown bool, caps map[string]store.NodePolicyRow) (nodes []nodeSummaryDTO, other int) {
	seen := make(map[string]bool, len(idx.Nodes))

	for _, node := range idx.Nodes {
		seen[node.Name] = true
		vms := idx.ByNode[node.Name]
		enabled := catalogKnown && cat.enabled(node.Name)

		if !enabled && !hasPVMSSVM(vms) {
			other++
			continue
		}

		summary := nodeSummaryDTO{
			ClusterKey: name, Cluster: label, Name: node.Name, Status: string(node.Status),
			VMCount: len(vms), VMRunningCount: countRunning(vms), PVMSSVMCount: countPVMSS(vms),
			PVMSSRAMGB:     policy.NodePVMSSRAMGB(idx, node.Name),
			PolicyMaxRAMGB: caps[node.Name].MaxRAMGB, PolicyMaxVMs: caps[node.Name].MaxVMs,
			CPUCores: node.CPUCores, CPUUsage: node.CPUUsage,
			MemoryTotalBytes: node.MemoryTotal, MemoryUsedBytes: node.MemoryUsed,
		}
		if catalogKnown {
			summary.State, summary.Missing = nodeState(node.Status == cluster.NodeOffline, enabled, cat, node.Name)
		}

		nodes = append(nodes, summary)
	}

	for _, n := range cat.resources.Nodes {
		if !seen[n.Name] {
			nodes = append(nodes, nodeSummaryDTO{
				ClusterKey: name, Cluster: label, Name: n.Name,
				Status: string(cluster.NodeOffline), State: nodeStateOffline,
			})
		}
	}

	slices.SortStableFunc(nodes, func(a, b nodeSummaryDTO) int {
		return cmp.Or(cmp.Compare(nodeStateRank[a.State], nodeStateRank[b.State]), cmp.Compare(a.Name, b.Name))
	})

	return nodes, other
}

// nodeState places one listed node. Offline wins over everything: a node that
// is down needs attention whether enabled or not. A node not enabled is only
// listed because it still hosts PVMSS VMs.
func nodeState(offline, enabled bool, cat nodeCatalog, node string) (string, []string) {
	switch {
	case offline:
		return nodeStateOffline, nil
	case !enabled:
		return nodeStateDisabledWithVMs, nil
	}

	if missing := catalog.NodeReadiness(cat.resources, cat.templates, node); len(missing) > 0 {
		return nodeStateIncomplete, missing
	}

	return nodeStateReady, nil
}

func countPVMSS(vms []cluster.VM) int {
	n := 0

	for _, vm := range vms {
		if slices.Contains(vm.Tags, catalog.ProtectedTagName) {
			n++
		}
	}

	return n
}

// nodePolicyCaps maps node names to their capacity limits. A read failure is
// logged and yields no limits: the cards still show host usage.
func nodePolicyCaps(ctx context.Context, st *store.Store, log *slog.Logger, clusterName string) map[string]store.NodePolicyRow {
	rows, err := st.NodePolicyRows(ctx, clusterName)
	if err != nil {
		log.WarnContext(ctx, "dashboard node policy read failed", "component", "httpapi", "cluster", clusterName, "error", err)
		return nil
	}

	caps := make(map[string]store.NodePolicyRow, len(rows))
	for _, row := range rows {
		caps[row.Node] = row
	}

	return caps
}

// pvmssStorage reports whether the dashboard lists a datastore: approved in
// the catalog, or holding a disk of a PVMSS VM. A per-node storage must match
// on its own node; a shared one (listed with no node) matches on any node.
func pvmssStorage(s dashboardStorageDTO, cat nodeCatalog, idx *inventory.Index) bool {
	onNode := func(node string) bool { return s.Node == "" || node == s.Node }

	if slices.ContainsFunc(cat.resources.Storages, func(a catalog.Storage) bool { return a.Name == s.Name && onNode(a.Node) }) {
		return true
	}

	for _, vm := range idx.ByVMID {
		if onNode(vm.Node) && slices.Contains(vm.Tags, catalog.ProtectedTagName) &&
			slices.ContainsFunc(vm.Disks, func(d cluster.Disk) bool { return d.Storage == s.Name }) {
			return true
		}
	}

	return false
}
