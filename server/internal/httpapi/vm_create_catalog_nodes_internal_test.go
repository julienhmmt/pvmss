package httpapi

import (
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"slices"
	"testing"
)

// Incomplete nodes (catalog.NodeReadiness) are never offered; ready nodes
// the cluster does not report online are offered as unavailable.
func TestCatalogNodeNames_ReadinessAndAvailability(t *testing.T) {
	t.Parallel()

	const (
		big         = "big"
		viaTemplate = "viatemplate"
		offline     = "offline"
		gone        = "gone"
		noStorage   = "nostorage"
		noISO       = "noiso"
	)

	nodes := []string{big, viaTemplate, offline, gone, noStorage, noISO}
	withStorage := []string{big, viaTemplate, offline, gone, noISO}
	withBridge := []string{big, viaTemplate, offline, gone, noISO, noStorage}
	withISO := []string{big, offline, gone, noStorage}

	var resources catalog.Resources
	for _, n := range nodes {
		resources.Nodes = append(resources.Nodes, catalog.Node{Name: n})
	}

	for _, n := range withStorage {
		resources.Storages = append(resources.Storages, catalog.Storage{Name: "local", Node: n})
	}

	for _, n := range withBridge {
		resources.Bridges = append(resources.Bridges, catalog.Bridge{Name: "vmbr0", Node: n})
	}

	for _, n := range withISO {
		resources.ISOs = append(resources.ISOs, catalog.ISO{Storage: "local", Node: n, File: "a.iso"})
	}

	templates := []catalog.Template{{VMID: 9000, Node: viaTemplate}}
	snap := cluster.Snapshot{Nodes: []cluster.Node{
		{Name: big, Status: cluster.NodeOnline},
		{Name: viaTemplate, Status: cluster.NodeOnline},
		{Name: offline, Status: cluster.NodeOffline},
		{Name: noStorage, Status: cluster.NodeOnline},
		{Name: noISO, Status: cluster.NodeOnline},
	}}

	nodes, unavailable := catalogNodeNames(resources, templates, snap)

	if want := []string{big, viaTemplate}; !slices.Equal(nodes, want) {
		t.Errorf("nodes = %v, want %v", nodes, want)
	}

	if want := []string{offline, gone}; !slices.Equal(unavailable, want) {
		t.Errorf("unavailable = %v, want %v", unavailable, want)
	}
}
