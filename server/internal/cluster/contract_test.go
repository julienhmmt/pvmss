package cluster_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"pvmss/server/internal/cluster"
)

// The shared expectation set (FR-010): both implementations of cluster.Client
// are verified against the same behaviour, so the fake cannot drift from what
// a real cluster is expected to do. Black-box on purpose — only the public
// Client contract is exercised here.

func TestContract_Snapshot(t *testing.T) {
	impls := map[string]cluster.Client{
		"fake":    cluster.Fake{},
		"proxmox": cluster.Proxmox{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			snap, err := impl.Snapshot(context.Background())

			if name == "proxmox" {
				if !errors.Is(err, cluster.ErrNotImplemented) {
					t.Fatalf("proxmox stub: err = %v, want ErrNotImplemented", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(snap.Nodes) == 0 {
				t.Fatal("expected at least one node")
			}
			for _, n := range snap.Nodes {
				if n.Name == "" {
					t.Error("node with empty name")
				}
				if n.MemoryUsed > n.MemoryTotal {
					t.Errorf("node %q: memoryUsed > memoryTotal", n.Name)
				}
				if n.StorageUsed > n.StorageTotal {
					t.Errorf("node %q: storageUsed > storageTotal", n.Name)
				}
			}
			for _, vm := range snap.VMs {
				if vm.VMID == 0 {
					t.Error("VM with zero VMID")
				}
				if vm.Node == "" {
					t.Errorf("VM %d (%s) has empty node", vm.VMID, vm.Name)
				}
			}
			for _, s := range snap.Storages {
				if s.Name == "" || s.Node == "" {
					t.Errorf("storage %+v has empty name or node", s)
				}
				if s.Used > s.Total {
					t.Errorf("storage %q: used > total", s.Name)
				}
			}
		})
	}
}

func TestContract_Authenticate(t *testing.T) {
	impls := map[string]cluster.Client{
		"fake":    cluster.Fake{},
		"proxmox": cluster.Proxmox{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			_, err := impl.Authenticate(context.Background(), "alice@pve", "pvmss-alice")

			if name == "proxmox" {
				if !errors.Is(err, cluster.ErrNotImplemented) {
					t.Fatalf("proxmox stub: err = %v, want ErrNotImplemented", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestContract_Authenticate_RejectsWrongPassword(t *testing.T) {
	impls := map[string]cluster.Client{
		"fake": cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			if _, err := impl.Authenticate(context.Background(), "alice@pve", "wrong-password"); !errors.Is(err, cluster.ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestContract_ChangePassword(t *testing.T) {
	impls := map[string]cluster.Client{
		"fake":    cluster.Fake{},
		"proxmox": cluster.Proxmox{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			err := impl.ChangePassword(context.Background(), "alice@pve", "pvmss-alice", "temporary-new-password")

			if name == "proxmox" {
				if !errors.Is(err, cluster.ErrNotImplemented) {
					t.Fatalf("proxmox stub: err = %v, want ErrNotImplemented", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			t.Cleanup(func() {
				if err := impl.ChangePassword(context.Background(), "alice@pve", "temporary-new-password", "pvmss-alice"); err != nil {
					t.Fatalf("restore demo password: %v", err)
				}
			})
			if _, err := impl.Authenticate(context.Background(), "alice@pve", "temporary-new-password"); err != nil {
				t.Fatalf("authenticate with new password: %v", err)
			}
		})
	}
}

func TestContract_Snapshot_StableAcrossCalls(t *testing.T) {
	// Only implementations that can succeed at all participate — the proxmox
	// stub always errors, so there is nothing stable to compare (FR-003).
	impls := map[string]cluster.Client{
		"fake": cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			first, err := impl.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			second, err := impl.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("repeated Snapshot calls diverged: %+v vs %+v", first, second)
			}
		})
	}
}

func TestContract_Snapshot_DoesNotMutateAcrossCalls(t *testing.T) {
	// A caller holding a reference to a previous Snapshot must not see it
	// change when a later call returns — the fake returns copies, not
	// references to its package-level literals (data-model.md invariant 3).
	impls := map[string]cluster.Client{
		"fake": cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			first, err := impl.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			firstNodesBefore := append([]cluster.Node(nil), first.Nodes...)
			if len(first.Nodes) > 0 {
				first.Nodes[0].Name = "mutated-by-caller"
			}

			second, err := impl.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if second.Nodes[0].Name != firstNodesBefore[0].Name {
				t.Fatalf("caller mutation of first snapshot leaked into second: %q vs %q",
					second.Nodes[0].Name, firstNodesBefore[0].Name)
			}
		})
	}
}
