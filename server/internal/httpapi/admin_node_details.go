//nolint:wsl_v5 // independent node sections stay adjacent to their fallback mapping
package httpapi

import (
	"context"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"slices"
	"sync"
	"time"
)

type nodeDetailDTO struct {
	ClusterKey string                `json:"clusterKey"`
	Cluster    string                `json:"cluster"`
	Name       string                `json:"name"`
	Health     nodeHealthDTO         `json:"health"`
	Network    nodeNetworkSectionDTO `json:"network"`
	Inventory  nodeInventoryDTO      `json:"inventory"`
}

type nodeHealthDTO struct {
	Status string `json:"status"`
	Stale  bool   `json:"stale"`
	cluster.NodeHealth
}

type nodeNetworkSectionDTO struct {
	Available  bool                           `json:"available"`
	Interfaces []cluster.NodeNetworkInterface `json:"interfaces"`
}

type nodePCISectionDTO struct {
	Available bool                `json:"available"`
	Devices   []cluster.PCIDevice `json:"devices"`
}

type nodeInventoryDTO struct {
	RefreshedAt string           `json:"refreshedAt"`
	VMs         []nodeVMDTO      `json:"vms"`
	Storages    []nodeStorageDTO `json:"storages"`
}

type nodeVMDTO struct {
	VMID             int    `json:"vmid"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	CPUCores         int    `json:"cpuCores"`
	MemoryTotalBytes int64  `json:"memoryTotalBytes"`
	Managed          bool   `json:"managed"`
}

type nodeStorageDTO struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Shared     bool   `json:"shared"`
	UsedBytes  int64  `json:"usedBytes"`
	TotalBytes int64  `json:"totalBytes"`
}

type nodeReadResult[T any] struct {
	value T
	err   error
}

type nodeLiveReads struct {
	health  nodeReadResult[cluster.NodeHealth]
	network nodeReadResult[[]cluster.NodeNetworkInterface]
}

// ServeNodeDetails handles GET /api/v1/admin/nodes/{cluster}/{name}.
func (h *AdminCatalog) ServeNodeDetails(w http.ResponseWriter, r *http.Request) {
	clusterKey, client, ok := h.nodeDetailClient(w, r)
	if !ok {
		return
	}
	index, ok := h.nodeDetailIndex(w, clusterKey)
	if !ok {
		return
	}
	node, ok := findInventoryNode(index.Nodes, r.PathValue("name"))
	if !ok {
		writeAdminError(w, http.StatusNotFound, "not_found", nodeNotFoundMsg(r.PathValue("name")))
		return
	}
	reads := readNodeLiveSections(r.Context(), client, node.Name)
	h.logNodeDetailErrors(clusterKey, node.Name, reads)
	writeAdminJSON(w, http.StatusOK, nodeDetailDTO{
		ClusterKey: clusterKey, Cluster: h.clusterDisplayName(r.Context(), clusterKey), Name: node.Name,
		Health: nodeHealthSection(node, reads.health), Network: nodeNetworkSection(reads.network),
		Inventory: nodeInventory(index, node.Name),
	})
}

// ServeNodePCI handles GET /api/v1/admin/nodes/{cluster}/{name}/pci.
func (h *AdminCatalog) ServeNodePCI(w http.ResponseWriter, r *http.Request) {
	clusterKey, client, ok := h.nodeDetailClient(w, r)
	if !ok {
		return
	}
	index, ok := h.nodeDetailIndex(w, clusterKey)
	if !ok {
		return
	}
	node, ok := findInventoryNode(index.Nodes, r.PathValue("name"))
	if !ok {
		writeAdminError(w, http.StatusNotFound, "not_found", nodeNotFoundMsg(r.PathValue("name")))
		return
	}
	var result nodeReadResult[[]cluster.PCIDevice]
	reader, isReader := client.(cluster.NodeDetailsReader)
	if !isReader {
		result.err = cluster.ErrNotImplemented
	} else {
		result.value, result.err = reader.ReadNodePCI(r.Context(), node.Name)
	}
	h.logNodeDetailError("PCI", clusterKey, node.Name, result.err)
	writeAdminJSON(w, http.StatusOK, nodePCISection(result))
}

func (h *AdminCatalog) nodeDetailClient(w http.ResponseWriter, r *http.Request) (string, cluster.Client, bool) {
	clusterKey := r.PathValue("cluster")
	client, err := h.clientFor(clusterKey)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return "", nil, false
	}
	return clusterKey, client, true
}

func (h *AdminCatalog) nodeDetailIndex(w http.ResponseWriter, clusterKey string) (*inventory.Index, bool) {
	var index *inventory.Index
	if h.inventory != nil {
		index = h.inventory.All()[clusterKey]
	}
	if index == nil && h.projection != nil && (h.clients == nil || len(h.clients.List()) == 1) {
		index = h.projection.Load()
	}
	if index == nil {
		writeAdminError(w, http.StatusServiceUnavailable, "inventory_not_ready", msgInventoryNotReady)
		return nil, false
	}
	return index, true
}

func findInventoryNode(nodes []cluster.Node, name string) (cluster.Node, bool) {
	for _, node := range nodes {
		if node.Name == name {
			return node, true
		}
	}
	return cluster.Node{}, false
}

func (h *AdminCatalog) clusterDisplayName(ctx context.Context, clusterKey string) string {
	rows, err := h.store.ListClusters(ctx)
	if err != nil {
		h.log.WarnContext(ctx, "admin node detail cluster lookup failed", "component", "httpapi", "error", err)
		return clusterKey
	}
	for _, row := range rows {
		if row.Name == clusterKey && row.DisplayName != "" {
			return row.DisplayName
		}
	}
	return clusterKey
}

func readNodeLiveSections(ctx context.Context, client cluster.Client, node string) nodeLiveReads {
	reader, ok := client.(cluster.NodeDetailsReader)
	if !ok {
		return nodeLiveReads{
			health:  nodeReadResult[cluster.NodeHealth]{err: cluster.ErrNotImplemented},
			network: nodeReadResult[[]cluster.NodeNetworkInterface]{err: cluster.ErrNotImplemented},
		}
	}
	var result nodeLiveReads
	var wait sync.WaitGroup
	startNodeRead(&wait, func() (cluster.NodeHealth, error) { return reader.ReadNodeHealth(ctx, node) }, &result.health)
	startNodeRead(&wait, func() ([]cluster.NodeNetworkInterface, error) { return reader.ReadNodeNetwork(ctx, node) }, &result.network)
	wait.Wait()
	return result
}

func startNodeRead[T any](wait *sync.WaitGroup, read func() (T, error), result *nodeReadResult[T]) {
	wait.Go(func() {
		result.value, result.err = read()
	})
}

func (h *AdminCatalog) logNodeDetailErrors(clusterKey, node string, reads nodeLiveReads) {
	h.logNodeDetailError("health", clusterKey, node, reads.health.err)
	h.logNodeDetailError("network", clusterKey, node, reads.network.err)
}

func (h *AdminCatalog) logNodeDetailError(section, clusterKey, node string, err error) {
	if err != nil {
		h.log.Warn("admin node detail section unavailable", "component", "httpapi", "section", section, "cluster", clusterKey, "node", node, "error", err)
	}
}

func nodeHealthSection(node cluster.Node, result nodeReadResult[cluster.NodeHealth]) nodeHealthDTO {
	if result.err == nil {
		result.value.LoadAverage = append([]string{}, result.value.LoadAverage...)
		return nodeHealthDTO{Status: string(cluster.NodeOnline), NodeHealth: result.value}
	}
	return nodeHealthDTO{Status: string(node.Status), Stale: true, NodeHealth: cachedNodeHealth(node)}
}

func cachedNodeHealth(node cluster.Node) cluster.NodeHealth {
	return cluster.NodeHealth{
		CPUUsage: node.CPUUsage, CPUCores: node.CPUCores, MemoryTotal: node.MemoryTotal,
		MemoryUsed: node.MemoryUsed, RootFSTotal: node.StorageTotal, RootFSUsed: node.StorageUsed,
		RootFSFree:      max(int64(0), node.StorageTotal-node.StorageUsed),
		RootFSAvailable: max(int64(0), node.StorageTotal-node.StorageUsed), LoadAverage: []string{},
	}
}

func nodeNetworkSection(result nodeReadResult[[]cluster.NodeNetworkInterface]) nodeNetworkSectionDTO {
	return nodeNetworkSectionDTO{Available: result.err == nil, Interfaces: append([]cluster.NodeNetworkInterface{}, result.value...)}
}

func nodePCISection(result nodeReadResult[[]cluster.PCIDevice]) nodePCISectionDTO {
	return nodePCISectionDTO{Available: result.err == nil, Devices: append([]cluster.PCIDevice{}, result.value...)}
}

func nodeInventory(index *inventory.Index, node string) nodeInventoryDTO {
	vms := index.ByNode[node]
	vmDTOs := make([]nodeVMDTO, 0, len(vms))
	for _, vm := range vms {
		vmDTOs = append(vmDTOs, nodeVMDTO{
			VMID: vm.VMID, Name: vm.Name, Status: string(vm.Status), CPUCores: vm.CPUCores,
			MemoryTotalBytes: vm.MemoryTotal, Managed: slices.Contains(vm.Tags, "pvmss"),
		})
	}
	storages := index.StoragesByNode[node]
	storageDTOs := make([]nodeStorageDTO, 0, len(storages))
	for _, storage := range storages {
		storageType := storage.PluginType
		if storageType == "" {
			storageType = storage.Type
		}
		_, shared := sharedStoragePlugins[storageType]
		storageDTOs = append(storageDTOs, nodeStorageDTO{
			Name: storage.Name, Type: storageType, Shared: shared,
			UsedBytes: storage.Used, TotalBytes: storage.Total,
		})
	}
	return nodeInventoryDTO{RefreshedAt: index.RefreshedAt.UTC().Format(time.RFC3339), VMs: vmDTOs, Storages: storageDTOs}
}
