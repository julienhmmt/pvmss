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
