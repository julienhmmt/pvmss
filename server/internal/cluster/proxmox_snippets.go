package cluster

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// NodeSnippetStatus is whether one node's Proxmox API lists a cloud-init
// file under <storage>:snippets/. PVMSS never writes the file: the admin
// pastes the command shown in Admin > Cloud-init on the nodes they choose.
type NodeSnippetStatus struct {
	Node    string
	Present bool
	Error   string
}

// SnippetChecker reports, node by node, whether an admin cloud-init file is
// on the cluster's snippet storage. Read-only: the Proxmox API cannot write
// snippets (upload and download-url accept iso, vztmpl and import only).
type SnippetChecker interface {
	// SnippetStorageID is the configured snippet storage ("" when the
	// cloud-init documents feature is off on this cluster).
	SnippetStorageID() string
	// CheckSnippet lists filename on every node. The error is reserved for
	// failures that prevent checking at all (feature off, node list
	// unreadable); per-node failures are in the results.
	CheckSnippet(ctx context.Context, filename string) ([]NodeSnippetStatus, error)
}

// SnippetStorageID implements SnippetChecker.
func (p Proxmox) SnippetStorageID() string { return p.SnippetStorage }

// CheckSnippet implements SnippetChecker: one API listing per online node.
func (p Proxmox) CheckSnippet(ctx context.Context, filename string) ([]NodeSnippetStatus, error) {
	if p.SnippetStorage == "" {
		return nil, ErrSnippetWriteUnavailable
	}

	nodes, err := p.clusterNodes(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]NodeSnippetStatus, len(nodes))

	var wg sync.WaitGroup

	for i, node := range nodes {
		wg.Go(func() {
			results[i] = p.checkOnNode(ctx, node, filename)
		})
	}

	wg.Wait()

	sort.Slice(results, func(a, b int) bool { return results[a].Node < results[b].Node })

	return results, nil
}

func (p Proxmox) checkOnNode(ctx context.Context, node clusterNode, filename string) NodeSnippetStatus {
	result := NodeSnippetStatus{Node: node.Name}

	if !node.Online {
		result.Error = "node is offline"

		return result
	}

	present, err := p.HasSnippet(ctx, node.Name, p.SnippetStorage, filename)
	if err != nil {
		result.Error = fmt.Sprintf("listing %s snippets failed: %v", p.SnippetStorage, err)

		return result
	}

	result.Present = present

	return result
}

// clusterNode is one node of the cluster from /cluster/status.
type clusterNode struct {
	Name   string
	Online bool
}

// clusterNodes lists every node of the cluster with its online flag.
func (p Proxmox) clusterNodes(ctx context.Context) ([]clusterNode, error) {
	raw, err := p.rest().do(ctx, "GET", "/cluster/status", nil)
	if err != nil {
		return nil, fmt.Errorf("query cluster status: %w", err)
	}

	var rows []proxmoxClusterStatusRow
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode cluster status: %w", err)
	}

	var nodes []clusterNode

	for _, row := range rows {
		if row.Type == "node" {
			nodes = append(nodes, clusterNode{Name: row.Name, Online: row.Online == 1})
		}
	}

	if len(nodes) == 0 {
		return nil, errors.New("cluster status lists no node")
	}

	return nodes, nil
}
