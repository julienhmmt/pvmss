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

// ISO is one approved ISO image on an approved storage.
type ISO struct {
	Storage string `json:"storage"`
	File    string `json:"file"`
}

// Profile is a fixed VM hardware preset (FR-009): when a creation request
// references a profile, these values are authoritative — client-submitted
// hardware fields that accompany a profile are ignored.
type Profile struct {
	ID       string
	Label    string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
}

// Resources is the approved-resource catalog for one cluster.
type Resources struct {
	Nodes    []Node
	Storages []Storage
	Bridges  []string
	ISOs     []ISO
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

// HasBridge reports whether name is an approved bridge.
func (r Resources) HasBridge(name string) bool {
	return slices.Contains(r.Bridges, name)
}

// HasISO reports whether (storage, file) is an approved ISO.
func (r Resources) HasISO(storage, file string) bool {
	for _, iso := range r.ISOs {
		if iso.Storage == storage && iso.File == file {
			return true
		}
	}

	return false
}

// ListStorages returns the approved storages for a cluster.
func ListStorages(ctx context.Context, st *store.Store, cluster string) ([]Storage, error) {
	rows, err := st.CatalogStorages(ctx, cluster)
	if err != nil {
		return nil, err
	}

	storages := make([]Storage, 0, len(rows))
	for _, row := range rows {
		storages = append(storages, Storage{Name: row.Name, Node: row.Node})
	}

	return storages, nil
}

// ListBridges returns the approved bridge names for a cluster.
func ListBridges(ctx context.Context, st *store.Store, cluster string) ([]string, error) {
	rows, err := st.CatalogBridges(ctx, cluster)
	if err != nil {
		return nil, err
	}

	bridges := make([]string, 0, len(rows))
	for _, row := range rows {
		bridges = append(bridges, row.Name)
	}

	return bridges, nil
}

// ListISOs returns the approved ISO images for a cluster.
func ListISOs(ctx context.Context, st *store.Store, cluster string) ([]ISO, error) {
	rows, err := st.CatalogISOs(ctx, cluster)
	if err != nil {
		return nil, err
	}

	isos := make([]ISO, 0, len(rows))
	for _, row := range rows {
		isos = append(isos, ISO{Storage: row.Storage, File: row.File})
	}

	return isos, nil
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

	resources := Resources{
		Nodes:    make([]Node, 0, len(nodes)),
		Storages: make([]Storage, 0, len(storages)),
		Bridges:  make([]string, 0, len(bridges)),
		ISOs:     make([]ISO, 0, len(isos)),
	}
	for _, node := range nodes {
		resources.Nodes = append(resources.Nodes, Node{Name: node.Name})
	}

	for _, storage := range storages {
		resources.Storages = append(resources.Storages, Storage{Name: storage.Name, Node: storage.Node})
	}

	for _, bridge := range bridges {
		resources.Bridges = append(resources.Bridges, bridge.Name)
	}

	for _, iso := range isos {
		resources.ISOs = append(resources.ISOs, ISO{Storage: iso.Storage, File: iso.File})
	}

	return resources, nil
}

// Profiles reads the VM hardware profiles for a cluster.
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
