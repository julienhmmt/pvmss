package cluster

import (
	"context"
	"slices"
)

// ListBridges implements Client. Returns the fake bridge dataset - a superset
// of what the catalog approves (vmbr0, vmbr1) so the admin demo has vmbr2 to discover
// and approve (fixture table).
func (fake Fake) ListBridges(_ context.Context) ([]Bridge, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeBridges), nil
}

// ListISOs implements Client. Returns the fake ISO dataset - a superset of
// what the catalog approves (debian-12, ubuntu-24, both on local) so the admin demo
// has rocky-9 to discover and approve (fixture table).
func (fake Fake) ListISOs(_ context.Context) ([]ISOImage, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeISOs), nil
}

// ListCloudImages implements Client. Returns the fake cloud-image dataset -
// a superset of what the catalog seed approved (ubuntu-24.04 cloudimg on
// local/node-01) so the admin demo has debian-12-generic-cloudimg to
// discover and approve, rocky-9 as the unapproved target.
func (fake Fake) ListCloudImages(_ context.Context) ([]CloudImage, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeCloudImages), nil
}

// ListTemplates implements Client. Returns the fake template dataset - two
// template VMs the admin demo can discover and approve.
func (fake Fake) ListTemplates(_ context.Context) ([]TemplateVM, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeTemplates), nil
}

// TemplateByVMID implements Client: a single-template lookup over the fake
// dataset. Unknown VMIDs are ErrNotFound; fake templates are always readable.
func (fake Fake) TemplateByVMID(_ context.Context, vmid int) (TemplateVM, error) {
	if fake.unavailable() {
		return TemplateVM{}, ErrUnreachable
	}

	for _, tmpl := range fakeTemplates {
		if tmpl.VMID == vmid {
			return tmpl, nil
		}
	}

	return TemplateVM{}, ErrNotFound
}

// StorageFreeSpace returns the available bytes on a storage backend on a node.
// The fake computes avail = Total - Used from the static
// storage dataset. Returns ErrNotFound for an unknown (node, storage) pair.
func (fake Fake) StorageFreeSpace(_ context.Context, node, storage string) (int64, error) {
	if fake.unavailable() {
		return 0, ErrUnreachable
	}

	for _, s := range fakeStorages {
		if s.Node == node && s.Name == storage {
			return s.Total - s.Used, nil
		}
	}

	return 0, ErrNotFound
}
