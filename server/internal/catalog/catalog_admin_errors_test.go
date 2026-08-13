package catalog_test

import (
	"context"
	"errors"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"testing"
)

// errDiscovery is the sentinel returned by errDiscoveryClient to prove the
// Set*Enabled toggles surface discovery errors verbatim (5xx at the handler)
// rather than mapping them to cluster.ErrNotFound (404).
var errDiscovery = errors.New("discovery unavailable")

// errDiscoveryClient is a cluster.Client whose Snapshot/ListBridges/ListISOs
// always fail with errDiscovery — the path the admin toggles must distinguish
// from "not present". The remaining interface methods are out of scope for the
// catalog toggles and return cluster.ErrNotImplemented.
type errDiscoveryClient struct{}

func (errDiscoveryClient) Snapshot(_ context.Context) (cluster.Snapshot, error) {
	return cluster.Snapshot{}, errDiscovery
}

func (errDiscoveryClient) ListBridges(_ context.Context) ([]cluster.Bridge, error) {
	return nil, errDiscovery
}

func (errDiscoveryClient) ListISOs(_ context.Context) ([]cluster.ISOImage, error) {
	return nil, errDiscovery
}

func (errDiscoveryClient) Authenticate(_ context.Context, _, _ string) (cluster.Identity, error) {
	return cluster.Identity{}, cluster.ErrNotImplemented
}

func (errDiscoveryClient) ChangePassword(_ context.Context, _, _, _ string) error {
	return cluster.ErrNotImplemented
}

func (errDiscoveryClient) ListPools(_ context.Context) ([]cluster.Pool, error) {
	return nil, cluster.ErrNotImplemented
}

func (errDiscoveryClient) EnsurePoolRole(_ context.Context) error {
	return cluster.ErrNotImplemented
}

func (errDiscoveryClient) EnsurePoolUser(_ context.Context, _, _ string) (string, error) {
	return "", cluster.ErrNotImplemented
}

func (errDiscoveryClient) CreatePool(_ context.Context, _, _ string) error {
	return cluster.ErrNotImplemented
}

func (errDiscoveryClient) SetPoolACL(_ context.Context, _, _, _ string) error {
	return cluster.ErrNotImplemented
}

func (errDiscoveryClient) DeletePool(_ context.Context, _ string) error {
	return cluster.ErrNotImplemented
}

func (errDiscoveryClient) DeleteUser(_ context.Context, _ string) error {
	return cluster.ErrNotImplemented
}

// TestSetNodeEnabled_DiscoveryErrorSurfaced — a Snapshot failure is returned
// verbatim so the handler maps it to 5xx, not 404 (the contract documented on
// SetNodeEnabled).
func TestSetNodeEnabled_DiscoveryErrorSurfaced(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)

	err := catalog.SetNodeEnabled(context.Background(), st, errDiscoveryClient{}, "default", "pve-node-01", true)
	if !errors.Is(err, errDiscovery) {
		t.Fatalf("SetNodeEnabled discovery error: got %v, want errDiscovery", err)
	}
}

// TestSetStorageEnabled_DiscoveryErrorSurfaced — same contract as the node
// toggle: a Snapshot failure surfaces verbatim, not as cluster.ErrNotFound.
func TestSetStorageEnabled_DiscoveryErrorSurfaced(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)

	err := catalog.SetStorageEnabled(context.Background(), st, errDiscoveryClient{}, "default", "local", "pve-node-01", true)
	if !errors.Is(err, errDiscovery) {
		t.Fatalf("SetStorageEnabled discovery error: got %v, want errDiscovery", err)
	}
}

// TestSetBridgeEnabled_DiscoveryErrorSurfaced — a ListBridges failure surfaces
// verbatim.
func TestSetBridgeEnabled_DiscoveryErrorSurfaced(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)

	err := catalog.SetBridgeEnabled(context.Background(), st, errDiscoveryClient{}, "default", "vmbr0", true)
	if !errors.Is(err, errDiscovery) {
		t.Fatalf("SetBridgeEnabled discovery error: got %v, want errDiscovery", err)
	}
}

// TestSetISOEnabled_DiscoveryErrorSurfaced — a ListISOs failure surfaces
// verbatim.
func TestSetISOEnabled_DiscoveryErrorSurfaced(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)

	err := catalog.SetISOEnabled(context.Background(), st, errDiscoveryClient{}, "default", "local", "debian-12.iso", true)
	if !errors.Is(err, errDiscovery) {
		t.Fatalf("SetISOEnabled discovery error: got %v, want errDiscovery", err)
	}
}
