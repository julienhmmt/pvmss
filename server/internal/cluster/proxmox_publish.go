package cluster

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// NodePublishResult is the outcome of publishing one file on one node: the
// helper wrote it over SSH AND the Proxmox API lists it under
// <storage>:snippets/ on that node. Anything short of both is a failure.
type NodePublishResult struct {
	Node  string
	OK    bool
	Error string
}

// HostKeyScan is one node's scanned host key, as a known_hosts line.
type HostKeyScan struct {
	Node  string
	Line  string
	Error string
}

// SnippetPublisher publishes admin cloud-init documents to every node of a
// cluster. It is the only write path for cloud-init files: VM creation and
// VM edits never write, they only point cicustom at a published file.
type SnippetPublisher interface {
	// PublishingEnabled reports whether the cluster has a snippet storage,
	// SSH settings and the global key.
	PublishingEnabled() bool
	// SnippetStorageID is the configured snippet storage ("" when off).
	SnippetStorageID() string
	// PublishSnippet writes filename on every node and verifies each node
	// lists it. The error is reserved for failures that prevent trying at
	// all (not configured, node list unreadable); per-node failures are in
	// the results.
	PublishSnippet(ctx context.Context, filename, content string) ([]NodePublishResult, error)
	// ScanHostKeys captures every node's host key for the admin to confirm.
	ScanHostKeys(ctx context.Context) ([]HostKeyScan, error)
}

// PublishingEnabled implements SnippetPublisher.
func (p Proxmox) PublishingEnabled() bool {
	return p.SnippetStorage != "" && p.SSH.Enabled()
}

// SnippetStorageID implements SnippetPublisher.
func (p Proxmox) SnippetStorageID() string { return p.SnippetStorage }

// PublishSnippet implements SnippetPublisher: one goroutine per node, write
// through the helper, then prove visibility through the API on that node.
func (p Proxmox) PublishSnippet(ctx context.Context, filename, content string) ([]NodePublishResult, error) {
	if !p.PublishingEnabled() {
		return nil, ErrSSHNotConfigured
	}

	// Defence in depth: runHelper re-checks the name before every send, so a
	// bug here still cannot reach the node helper.
	if !snippetFilenameRE.MatchString(filename) {
		return nil, fmt.Errorf("refusing unsafe snippet filename %q", filename)
	}

	nodes, err := p.snippetNodes(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]NodePublishResult, len(nodes))

	var wg sync.WaitGroup

	for i, node := range nodes {
		wg.Go(func() {
			results[i] = p.publishOnNode(ctx, node, filename, []byte(content))
		})
	}

	wg.Wait()

	sort.Slice(results, func(a, b int) bool { return results[a].Node < results[b].Node })

	return results, nil
}

func (p Proxmox) publishOnNode(ctx context.Context, node snippetNode, filename string, content []byte) NodePublishResult {
	result := NodePublishResult{Node: node.Name}

	if !node.Online {
		result.Error = "node is offline"

		return result
	}

	if err := p.runHelper(ctx, node.Host, "write", filename, content); err != nil {
		result.Error = err.Error()

		return result
	}

	visible, err := p.HasSnippet(ctx, node.Name, p.SnippetStorage, filename)
	if err != nil {
		result.Error = fmt.Sprintf("written, but listing %s snippets failed: %v", p.SnippetStorage, err)

		return result
	}

	if !visible {
		result.Error = fmt.Sprintf("written, but Proxmox does not list %s:snippets/%s - the helper's directory is not that storage's snippets/ directory", p.SnippetStorage, filename)

		return result
	}

	result.OK = true

	return result
}

// RemoveCloudInitSnippet implements Writer: removes filename on every online
// node through the helper (legacy per-VM files, on VM delete). A missing
// file is not an error for the helper.
func (p Proxmox) RemoveCloudInitSnippet(ctx context.Context, storage, filename string) error {
	if !p.PublishingEnabled() {
		return ErrSnippetWriteUnavailable
	}

	if storage != p.SnippetStorage {
		return fmt.Errorf("snippet storage %q is not this cluster's snippet storage %q", storage, p.SnippetStorage)
	}

	nodes, err := p.snippetNodes(ctx)
	if err != nil {
		return err
	}

	var errs []error

	for _, node := range nodes {
		if !node.Online {
			continue
		}

		if err := p.runHelper(ctx, node.Host, "remove", filename, nil); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", node.Name, err))
		}
	}

	return errors.Join(errs...)
}

// ScanHostKeys implements SnippetPublisher. It needs the SSH port only (no
// user, no key): the admin scans before saving the rest.
func (p Proxmox) ScanHostKeys(ctx context.Context) ([]HostKeyScan, error) {
	nodes, err := p.snippetNodes(ctx)
	if err != nil {
		return nil, err
	}

	scans := make([]HostKeyScan, len(nodes))

	var wg sync.WaitGroup

	for i, node := range nodes {
		wg.Go(func() {
			scans[i] = HostKeyScan{Node: node.Name}

			line, err := p.scanHostKey(ctx, node.Host)
			if err != nil {
				scans[i].Error = err.Error()

				return
			}

			scans[i].Line = line
		})
	}

	wg.Wait()

	sort.Slice(scans, func(a, b int) bool { return scans[a].Node < scans[b].Node })

	return scans, nil
}
