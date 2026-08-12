//nolint:wsl_v5 // refresh isolation scenarios keep setup and assertions together
package inventory_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_RefreshKeepsClustersIndependent(t *testing.T) {
	clusters, err := cluster.NewRegistry("fake", []store.ClusterRow{
		{Name: "default", URL: "https://default.invalid", TokenID: "id", TokenSecret: "secret"},
		{Name: "secondary", URL: "https://secondary.invalid", TokenID: "id", TokenSecret: "secret"},
		{Name: "offline-demo", URL: "https://offline.invalid", TokenID: "id", TokenSecret: "secret"},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	registry := inventory.NewRegistry(clusters, time.Hour, slog.Default())

	if _, err := registry.Refresh(context.Background(), "default"); err != nil {
		t.Fatalf("Refresh(default): %v", err)
	}
	defaultIndex, err := registry.Index("default")
	if err != nil || defaultIndex == nil {
		t.Fatalf("Index(default) = %v, %v", defaultIndex, err)
	}
	if _, err := registry.Refresh(context.Background(), "offline-demo"); !errors.Is(err, cluster.ErrUnreachable) {
		t.Fatalf("Refresh(offline-demo) error = %v, want ErrUnreachable", err)
	}
	stillDefault, err := registry.Index("default")
	if err != nil || stillDefault != defaultIndex {
		t.Fatalf("default index changed after offline refresh: %p -> %p", defaultIndex, stillDefault)
	}

	all := registry.All()
	if len(all) != 3 {
		t.Fatalf("All() length = %d, want 3 active clusters", len(all))
	}
	if all["default"] != defaultIndex {
		t.Fatal("All() returned a different default index")
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_UnknownCluster(t *testing.T) {
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{})
	if _, err := registry.Index("missing"); !errors.Is(err, inventory.ErrClusterNotFound) {
		t.Fatalf("Index(missing) error = %v, want ErrClusterNotFound", err)
	}
}
