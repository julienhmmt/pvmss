//nolint:wsl_v5 // contract tests group shared implementation expectations
package cluster_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"reflect"
	"testing"
)

// The shared expectation set (FR-010): both implementations of cluster.Client
// are verified against the same behaviour, so the fake cannot drift from what
// a real cluster is expected to do. Black-box on purpose — only the public
// Client contract is exercised here.
const (
	fakeImplementationName    = "fake"
	proxmoxImplementationName = "proxmox"
)

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_Snapshot(t *testing.T) { //nolint:gocyclo // contract test intentionally checks every snapshot invariant
	impls := map[string]cluster.Client{
		fakeImplementationName:    cluster.Fake{},
		proxmoxImplementationName: cluster.Proxmox{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			snap, err := impl.Snapshot(context.Background())

			if name == proxmoxImplementationName {
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

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_Authenticate(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName:    cluster.Fake{},
		proxmoxImplementationName: cluster.Proxmox{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			_, err := impl.Authenticate(context.Background(), "alice@pve", "pvmss-alice")

			if name == proxmoxImplementationName {
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

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_Authenticate_RejectsWrongPassword(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName: cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			if _, err := impl.Authenticate(context.Background(), "alice@pve", "wrong-password"); !errors.Is(err, cluster.ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_ChangePassword(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName:    cluster.Fake{},
		proxmoxImplementationName: cluster.Proxmox{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			err := impl.ChangePassword(context.Background(), "alice@pve", "pvmss-alice", "temporary-new-password")

			if name == proxmoxImplementationName {
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

//nolint:paralleltest // serial: shared fake cloud-init fixture
func TestContract_CloudInitReaderAndWriter(t *testing.T) {
	cluster.ResetFake()

	config := cluster.CloudInitConfig{User: "debian", IPMode: cluster.CloudInitIPModeDHCP}
	if err := (cluster.Fake{}).SetCloudInitConfig(context.Background(), cluster.FakeNode01, 101, config); err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}

	got, err := (cluster.Fake{}).GetCloudInitConfig(context.Background(), cluster.FakeNode01, 101)
	if err != nil {
		t.Fatalf("GetCloudInitConfig: %v", err)
	}

	if got.User != config.User || got.IPMode != config.IPMode {
		t.Fatalf("config = %+v, want %+v", got, config)
	}

	if _, err := (cluster.Fake{}).FindSnippetStorage(context.Background(), cluster.FakeNode01); err != nil {
		t.Fatalf("FindSnippetStorage: %v", err)
	}

	if err := (cluster.Fake{}).PushCloudInitSnippet(context.Background(), cluster.FakeNode01, cluster.FakeSnippetStorage, "pvmss-101.yml", 101, "#cloud-config\n"); err != nil {
		t.Fatalf("PushCloudInitSnippet: %v", err)
	}

	if calls := cluster.FakeCallsFor(101); len(calls) != 3 || calls[0].Action != "ensure_cloudinit_drive" || calls[1].Action != "set_cloudinit_config" || calls[2].Action != "push_cloudinit_snippet" {
		t.Fatalf("calls = %+v, want ensure, set, push", calls)
	}
}

//nolint:paralleltest // serial: shared fake cloud-init fixture
func TestContract_ProxmoxCloudInitStubs(t *testing.T) {
	reader := cluster.Proxmox{}
	if _, err := reader.GetCloudInitConfig(context.Background(), cluster.FakeNode01, 101); !errors.Is(err, cluster.ErrNotImplemented) {
		t.Fatalf("GetCloudInitConfig err = %v, want ErrNotImplemented", err)
	}

	if _, err := reader.FindSnippetStorage(context.Background(), cluster.FakeNode01); !errors.Is(err, cluster.ErrNotImplemented) {
		t.Fatalf("FindSnippetStorage err = %v, want ErrNotImplemented", err)
	}

	writer := cluster.Proxmox{}
	if err := writer.EnsureCloudInitDrive(context.Background(), cluster.FakeNode01, 101); !errors.Is(err, cluster.ErrNotImplemented) {
		t.Fatalf("EnsureCloudInitDrive err = %v, want ErrNotImplemented", err)
	}

	if err := writer.SetCloudInitConfig(context.Background(), cluster.FakeNode01, 101, cluster.CloudInitConfig{}); !errors.Is(err, cluster.ErrNotImplemented) {
		t.Fatalf("SetCloudInitConfig err = %v, want ErrNotImplemented", err)
	}

	if err := writer.PushCloudInitSnippet(context.Background(), cluster.FakeNode01, cluster.FakeSnippetStorage, "pvmss-101.yml", 101, ""); !errors.Is(err, cluster.ErrNotImplemented) {
		t.Fatalf("PushCloudInitSnippet err = %v, want ErrNotImplemented", err)
	}
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_Snapshot_StableAcrossCalls(t *testing.T) {
	// Only implementations that can succeed at all participate — the proxmox
	// stub always errors, so there is nothing stable to compare (FR-003).
	impls := map[string]cluster.Client{
		fakeImplementationName: cluster.Fake{},
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

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_Snapshot_DoesNotMutateAcrossCalls(t *testing.T) {
	// A caller holding a reference to a previous Snapshot must not see it
	// change when a later call returns — the fake returns copies, not
	// references to its package-level literals (data-model.md invariant 3).
	impls := map[string]cluster.Client{
		fakeImplementationName: cluster.Fake{},
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

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_ListBridges(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName:    cluster.Fake{},
		proxmoxImplementationName: cluster.Proxmox{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			bridges, err := impl.ListBridges(context.Background())

			if name == proxmoxImplementationName {
				if !errors.Is(err, cluster.ErrNotImplemented) {
					t.Fatalf("proxmox stub: err = %v, want ErrNotImplemented", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(bridges) == 0 {
				t.Fatal("expected at least one bridge")
			}

			for _, b := range bridges {
				if b.Name == "" {
					t.Error("bridge with empty name")
				}
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_ListISOs(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName:    cluster.Fake{},
		proxmoxImplementationName: cluster.Proxmox{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			isos, err := impl.ListISOs(context.Background())

			if name == proxmoxImplementationName {
				if !errors.Is(err, cluster.ErrNotImplemented) {
					t.Fatalf("proxmox stub: err = %v, want ErrNotImplemented", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(isos) == 0 {
				t.Fatal("expected at least one ISO")
			}

			for _, i := range isos {
				if i.Storage == "" || i.File == "" {
					t.Errorf("ISO %+v has empty storage or file", i)
				}
			}
		})
	}
}
