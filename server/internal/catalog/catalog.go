// Package catalog exposes the admin-curated set of resources a VM creation
// may reference (AC03 §3.3): approved nodes, storages, bridges, ISOs, and VM
// hardware profiles, scoped per cluster. In T06 the catalog is fixture data
// seeded by store migration version 7 and read-only; T11 adds the admin CRUD.
package catalog

import (
	"context"
	"fmt"
	"pvmss/server/internal/store"
	"slices"
)

// Node is one approved cluster node.
type Node struct {
	Name string
}

// Storage is one approved storage backend on a node.
type Storage struct {
	Name string `json:"storage"`
	Node string `json:"node"`
}

// Bridge is one approved network bridge on a node.
type Bridge struct {
	Name string `json:"bridge"`
	Node string `json:"node"`
}

// ISO is one approved ISO image on an approved storage on a node. Approval is
// keyed by (node, storage, file) — one row per node, consistent with Storage
// and Bridge. An ISO on shared storage has N rows (D1b).
type ISO struct {
	Storage string `json:"storage"`
	Node    string `json:"node"`
	File    string `json:"file"`
}

// Profile is a fixed VM hardware preset (FR-009): when a creation request
// references a profile, these values are authoritative — client-submitted
// hardware fields that accompany a profile are ignored.
type Profile struct {
	ID       string
	Label    string
	Sockets  int
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
}

// Template is one approved Proxmox template (US2/issue-02). The VMID is the
// Proxmox VMID of the template VM; the node determines where the clone lands
// (D2b: cross-node clone is forbidden). CloudInitCapable drives the
// full/linked clone decision. DiskStorage and DiskSizeGB drive the resize
// decision (enlarge after clone, reject reduction before VMID). DiskBus is
// the Proxmox bus family of the template's primary disk (e.g. "scsi") — the
// clone inherits it, and post-clone ResizeDisk must target the correct key.
type Template struct {
	VMID             int
	Node             string
	Name             string
	CloudInitCapable bool
	DiskStorage      string
	DiskSizeGB       int
	DiskBus          string
}

// Resources is the approved-resource catalog for one cluster.
type Resources struct {
	Nodes    []Node
	Storages []Storage
	Bridges  []Bridge
	ISOs     []ISO
	// Tags is the admin-curated tag name allowlist (FR-013): users may only
	// reference these tags on create and hardware updates.
	Tags []string
}

// HasTag reports whether name is an admin-approved tag.
func (r Resources) HasTag(name string) bool {
	return slices.Contains(r.Tags, name)
}

// HasNode reports whether name is an approved node.
func (r Resources) HasNode(name string) bool {
	for _, node := range r.Nodes {
		if node.Name == name {
			return true
		}
	}

	return false
}

// HasStorage reports whether name is an approved storage on node.
func (r Resources) HasStorage(name, node string) bool {
	for _, storage := range r.Storages {
		if storage.Name == name && storage.Node == node {
			return true
		}
	}

	return false
}

// HasBridge reports whether name is an approved bridge on node.
func (r Resources) HasBridge(name, node string) bool {
	for _, bridge := range r.Bridges {
		if bridge.Name == name && bridge.Node == node {
			return true
		}
	}

	return false
}

// HasISO reports whether (storage, file) is an approved ISO on node. A
// shared-storage ISO has one row per node, so each node matches independently.
func (r Resources) HasISO(storage, file, node string) bool {
	for _, iso := range r.ISOs {
		if iso.Storage == storage && iso.File == file && iso.Node == node {
			return true
		}
	}

	return false
}

// ApprovedResources reads the full approved-resource catalog for a cluster
// from the store (T06: fixture rows seeded by migration version 7).
func ApprovedResources(ctx context.Context, st *store.Store, cluster string) (Resources, error) {
	nodes, err := st.CatalogNodes(ctx, cluster)
	if err != nil {
		return Resources{}, err
	}

	storages, err := st.CatalogStorages(ctx, cluster)
	if err != nil {
		return Resources{}, err
	}

	bridges, err := st.CatalogBridges(ctx, cluster)
	if err != nil {
		return Resources{}, err
	}

	isos, err := st.CatalogISOs(ctx, cluster)
	if err != nil {
		return Resources{}, err
	}

	tagRows, err := st.CatalogTags(ctx, cluster)
	if err != nil {
		return Resources{}, err
	}

	resources := Resources{
		Nodes:    make([]Node, 0, len(nodes)),
		Storages: make([]Storage, 0, len(storages)),
		Bridges:  make([]Bridge, 0, len(bridges)),
		ISOs:     make([]ISO, 0, len(isos)),
		Tags:     make([]string, 0, len(tagRows)),
	}
	for _, node := range nodes {
		resources.Nodes = append(resources.Nodes, Node{Name: node.Name})
	}

	for _, storage := range storages {
		resources.Storages = append(resources.Storages, Storage{Name: storage.Name, Node: storage.Node})
	}

	for _, bridge := range bridges {
		resources.Bridges = append(resources.Bridges, Bridge{Name: bridge.Name, Node: bridge.Node})
	}

	for _, iso := range isos {
		resources.ISOs = append(resources.ISOs, ISO{Storage: iso.Storage, Node: iso.Node, File: iso.File})
	}

	for _, tag := range tagRows {
		resources.Tags = append(resources.Tags, tag.Name)
	}

	return resources, nil
}

// Profiles reads the VM hardware profiles for a cluster.
//
//nolint:dupl // structurally similar to Templates by design (row→domain mapping)
func Profiles(ctx context.Context, st *store.Store, cluster string) ([]Profile, error) {
	rows, err := st.CatalogProfiles(ctx, cluster)
	if err != nil {
		return nil, err
	}

	profiles := make([]Profile, 0, len(rows))
	for _, row := range rows {
		profiles = append(profiles, Profile{
			ID:       row.ID,
			Label:    row.Label,
			Sockets:  row.Sockets,
			CPUCores: row.CPUCores,
			MemoryMB: row.MemoryMB,
			DiskGB:   row.DiskGB,
			Bus:      row.Bus,
		})
	}

	return profiles, nil
}

// FindProfile returns the profile with the given id, or an error wrapping
// ErrNotApproved when the id is absent from the catalog (FR-003).
func FindProfile(profiles []Profile, id string) (Profile, error) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, nil
		}
	}

	return Profile{}, fmt.Errorf("profile %q is not approved for this cluster", id)
}

// Templates reads the approved Proxmox templates for a cluster (US2/issue-02).
//
//nolint:dupl // structurally similar to Profiles by design (row→domain mapping)
func Templates(ctx context.Context, st *store.Store, cluster string) ([]Template, error) {
	rows, err := st.CatalogTemplates(ctx, cluster)
	if err != nil {
		return nil, err
	}

	templates := make([]Template, 0, len(rows))
	for _, row := range rows {
		templates = append(templates, Template{
			VMID:             row.VMID,
			Node:             row.Node,
			Name:             row.Name,
			CloudInitCapable: row.CloudInitCapable,
			DiskStorage:      row.DiskStorage,
			DiskSizeGB:       row.DiskSizeGB,
			DiskBus:          row.DiskBus,
		})
	}

	return templates, nil
}

// FindTemplate returns the template with the given VMID, or an error wrapping
// ErrNotApproved when the VMID is absent from the catalog (US2/issue-02).
func FindTemplate(templates []Template, vmid int) (Template, error) {
	for _, tmpl := range templates {
		if tmpl.VMID == vmid {
			return tmpl, nil
		}
	}

	return Template{}, fmt.Errorf("template vmid %d is not approved for this cluster", vmid)
}
