//nolint:wsl_v5 // registry scenarios keep setup and assertions together
package cluster_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"testing"
)

const registryTestSecret = "registry-test-secret"

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_AddAndRemoveIsolated(t *testing.T) {
	rows := []store.ClusterRow{
		{Name: "default", URL: "https://default.invalid", TokenID: "id", TokenSecret: registryTestSecret},
	}
	registry, err := cluster.NewRegistry("fake", rows)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	before, err := registry.Client("default")
	if err != nil {
		t.Fatalf("Client(default): %v", err)
	}
	if err := registry.Add(context.Background(), store.ClusterRow{Name: "secondary", URL: "https://secondary.invalid", TokenID: "id", TokenSecret: registryTestSecret}); err != nil {
		t.Fatalf("Add(secondary): %v", err)
	}
	after, err := registry.Client("default")
	if err != nil {
		t.Fatalf("Client(default) after Add: %v", err)
	}
	if before != after {
		t.Fatal("adding a cluster replaced the existing client")
	}
	if _, err := registry.Client("secondary"); err != nil {
		t.Fatalf("Client(secondary): %v", err)
	}

	registry.Remove("secondary")
	if _, err := registry.Client("secondary"); !errors.Is(err, cluster.ErrClusterNotFound) {
		t.Fatalf("removed client error = %v, want ErrClusterNotFound", err)
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_FakeClustersDoNotShareMutations(t *testing.T) {
	registry, err := cluster.NewRegistry("fake", []store.ClusterRow{
		{Name: "default", URL: "https://default.invalid", TokenID: "id", TokenSecret: registryTestSecret},
		{Name: "secondary", URL: "https://secondary.invalid", TokenID: "id", TokenSecret: registryTestSecret},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defaultClient, err := registry.Client("default")
	if err != nil {
		t.Fatalf("Client(default): %v", err)
	}
	secondaryClient, err := registry.Client("secondary")
	if err != nil {
		t.Fatalf("Client(secondary): %v", err)
	}
	writer, ok := defaultClient.(cluster.Writer)
	if !ok {
		t.Fatal("default fake does not implement Writer")
	}
	before, err := defaultClient.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("default snapshot: %v", err)
	}
	// Pick a stopped VM — the fake's Delete rejects a running VM with
	// ErrVMRunning (mirroring real Proxmox); this test exercises registry
	// isolation, not the force-stop path.
	target := firstStoppedVM(before.VMs)
	if err := writer.Delete(context.Background(), target.Node, target.VMID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	afterDefault, err := defaultClient.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("default snapshot after delete: %v", err)
	}
	afterSecondary, err := secondaryClient.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("secondary snapshot after delete: %v", err)
	}
	if len(afterDefault.VMs) != len(before.VMs)-1 {
		t.Fatalf("default VM count = %d, want %d", len(afterDefault.VMs), len(before.VMs)-1)
	}
	if len(afterSecondary.VMs) == len(afterDefault.VMs) {
		t.Fatal("secondary shared default's mutated VM list")
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_MalformedRowDoesNotBlockHealthyRows(t *testing.T) {
	registry, err := cluster.NewRegistry("proxmox", []store.ClusterRow{
		{Name: "healthy", URL: "https://healthy.invalid/api2/json", TokenID: "id", TokenSecret: registryTestSecret},
		{Name: "broken", URL: "not a URL", TokenID: "id", TokenSecret: registryTestSecret},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if _, err := registry.Client("healthy"); err != nil {
		t.Fatalf("healthy client error = %v", err)
	}
	if _, err := registry.Client("broken"); !errors.Is(err, cluster.ErrClusterNotFound) {
		t.Fatalf("broken client error = %v, want ErrClusterNotFound", err)
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_UnknownAndRemovedNames(t *testing.T) {
	registry, err := cluster.NewRegistry("fake", []store.ClusterRow{{
		Name: "default", URL: "https://default.invalid", TokenID: "id", TokenSecret: registryTestSecret,
	}})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	for _, name := range []string{"missing", ""} {
		if _, err := registry.Client(name); !errors.Is(err, cluster.ErrClusterNotFound) {
			t.Errorf("Client(%q) error = %v, want ErrClusterNotFound", name, err)
		}
	}
}
