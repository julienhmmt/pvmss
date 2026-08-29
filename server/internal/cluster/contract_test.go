package cluster_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"reflect"
	"testing"
)

// The shared expectation set (FR-010): every implementation of cluster.Client
// is verified against the same behaviour, so the fake cannot drift from what
// a real cluster is expected to do. Black-box on purpose — only the public
// Client contract is exercised here.
//
// cluster.Proxmox talks to a real Proxmox VE API, so it has no fixed dataset
// to compare against the fake's — its own coverage lives in
// proxmox_test.go and friends, against an httptest-mocked PVE server. This
// file's invariant checks (checkSnapshotNodes and friends) still apply to it
// there.
const fakeImplementationName = "fake"

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_Snapshot(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName: cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			snap, err := impl.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(snap.Nodes) == 0 {
				t.Fatal("expected at least one node")
			}

			checkSnapshotNodes(t, snap.Nodes)
			checkSnapshotVMs(t, snap.VMs)
			checkSnapshotStorages(t, snap.Storages)
		})
	}
}

// checkSnapshotNodes validates every node in a Snapshot against the contract
// invariants (FR-010).
func checkSnapshotNodes(t *testing.T, nodes []cluster.Node) {
	t.Helper()

	for _, n := range nodes {
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
}

// checkSnapshotVMs validates every VM in a Snapshot against the contract
// invariants (FR-010).
func checkSnapshotVMs(t *testing.T, vms []cluster.VM) {
	t.Helper()

	for _, vm := range vms {
		if vm.VMID == 0 {
			t.Error("VM with zero VMID")
		}

		if vm.Node == "" {
			t.Errorf("VM %d (%s) has empty node", vm.VMID, vm.Name)
		}
	}
}

// checkSnapshotStorages validates every storage in a Snapshot against the
// contract invariants (FR-010).
func checkSnapshotStorages(t *testing.T, storages []cluster.Storage) {
	t.Helper()

	for _, s := range storages {
		if s.Name == "" || s.Node == "" {
			t.Errorf("storage %+v has empty name or node", s)
		}

		if s.PluginType == "" || s.Content == "" {
			t.Errorf("storage %q has empty plugin type or content", s.Name)
		}

		if s.Used > s.Total {
			t.Errorf("storage %q: used > total", s.Name)
		}
	}
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_Authenticate(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName: cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			if _, err := impl.Authenticate(context.Background(), cluster.FakeUserAlice, "pvmss-alice"); err != nil {
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
			if _, err := impl.Authenticate(context.Background(), cluster.FakeUserAlice, "wrong-password"); !errors.Is(err, cluster.ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_ChangePassword(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName: cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			runChangePasswordCase(t, impl)
		})
	}
}

// runChangePasswordCase checks ChangePassword against the contract: it
// changes the password and the new one authenticates. Extracted from
// TestContract_ChangePassword to keep its Cognitive Complexity under the
// SonarQube go:S3776 threshold.
func runChangePasswordCase(t *testing.T, impl cluster.Client) {
	t.Helper()

	if err := impl.ChangePassword(context.Background(), cluster.FakeUserAlice, "pvmss-alice", "temporary-new-password"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if err := impl.ChangePassword(context.Background(), cluster.FakeUserAlice, "temporary-new-password", "pvmss-alice"); err != nil {
			t.Fatalf("restore demo password: %v", err)
		}
	})

	if _, err := impl.Authenticate(context.Background(), cluster.FakeUserAlice, "temporary-new-password"); err != nil {
		t.Fatalf("authenticate with new password: %v", err)
	}
}

//nolint:paralleltest // serial: shared fake cloud-init fixture
func TestContract_CloudInitReaderAndWriter(t *testing.T) {
	cluster.ResetFake()

	config := cluster.CloudInitConfig{User: cluster.FakeCloudInitUser, IPMode: cluster.CloudInitIPModeDHCP}
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

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_Snapshot_StableAcrossCalls(t *testing.T) {
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
		fakeImplementationName: cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			runListBridgesCase(t, impl)
		})
	}
}

// runListBridgesCase checks ListBridges returns at least one bridge with a
// non-empty name. Extracted from TestContract_ListBridges to keep its
// Cognitive Complexity under the SonarQube go:S3776 threshold.
func runListBridgesCase(t *testing.T, impl cluster.Client) {
	t.Helper()

	bridges, err := impl.ListBridges(context.Background())
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
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_ListISOs(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName: cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			runListISOsCase(t, impl)
		})
	}
}

// runListISOsCase checks ListISOs returns at least one ISO with non-empty
// storage and file. Extracted from TestContract_ListISOs to keep its
// Cognitive Complexity under the SonarQube go:S3776 threshold.
func runListISOsCase(t *testing.T, impl cluster.Client) {
	t.Helper()

	isos, err := impl.ListISOs(context.Background())
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
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_ListTemplates(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName: cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			runListTemplatesCase(t, impl)
		})
	}
}

// runListTemplatesCase checks ListTemplates returns at least one template
// with a non-empty node and positive VMID. Extracted from
// TestContract_ListTemplates to keep its Cognitive Complexity under the
// SonarQube go:S3776 threshold.
func runListTemplatesCase(t *testing.T, impl cluster.Client) {
	t.Helper()

	templates, err := impl.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(templates) == 0 {
		t.Fatal("expected at least one template")
	}

	for _, tmpl := range templates {
		if tmpl.VMID == 0 || tmpl.Node == "" {
			t.Errorf("template %+v has empty vmid or node", tmpl)
		}
	}
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_GetMetricsHistory(t *testing.T) {
	impls := map[string]cluster.MetricsHistoryReader{
		fakeImplementationName: cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, runMetricsHistoryCase(impl))
	}
}

// runMetricsHistoryCase checks GetMetricsHistory against the contract: every
// timeframe returns at least one sample, memory used never exceeds memory
// max, and CPU never reports outside 0-100%. cluster.Proxmox has no fixed
// dataset to compare against the fake's, so its own coverage lives in
// proxmox_metrics_test.go against an httptest-mocked PVE server (matching
// this file's header comment on Snapshot).
func runMetricsHistoryCase(impl cluster.MetricsHistoryReader) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()

		for _, timeframe := range []cluster.MetricsTimeframe{cluster.MetricsTimeframeHour, cluster.MetricsTimeframeDay, cluster.MetricsTimeframeWeek} {
			samples, err := impl.GetMetricsHistory(context.Background(), cluster.FakeNode01, 101, timeframe)
			if err != nil {
				t.Fatalf("timeframe %q: unexpected error: %v", timeframe, err)
			}

			if len(samples) == 0 {
				t.Fatalf("timeframe %q: expected at least one sample", timeframe)
			}

			checkMetricsSamples(t, timeframe, samples)
		}
	}
}

func checkMetricsSamples(t *testing.T, timeframe cluster.MetricsTimeframe, samples []cluster.MetricsSample) {
	t.Helper()

	for _, s := range samples {
		if s.MemoryUsed > s.MemoryMax {
			t.Errorf("timeframe %q: memoryUsed %d > memoryMax %d", timeframe, s.MemoryUsed, s.MemoryMax)
		}

		if s.CPUPercent < 0 || s.CPUPercent > 100 {
			t.Errorf("timeframe %q: cpuPercent %v out of 0-100 range", timeframe, s.CPUPercent)
		}
	}
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_GetMetricsCurrent(t *testing.T) {
	impls := map[string]cluster.MetricsCurrentReader{
		fakeImplementationName: cluster.Fake{},
	}

	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			t.Helper()

			sample, err := impl.GetMetricsCurrent(context.Background(), cluster.FakeNode01, 101)
			if err != nil {
				t.Fatalf("GetMetricsCurrent: %v", err)
			}

			checkMetricsSamples(t, cluster.MetricsTimeframeHour, []cluster.MetricsSample{sample})
		})
	}
}

//nolint:paralleltest // serial: shared fake identity fixture
func TestContract_DisplayName(t *testing.T) {
	impls := map[string]cluster.Client{
		fakeImplementationName: cluster.Fake{},
	}
	for name, impl := range impls {
		t.Run(name, func(t *testing.T) {
			display, err := impl.DisplayName(context.Background())
			if err != nil {
				t.Fatalf("DisplayName: %v", err)
			}

			if display == "" {
				t.Fatal("DisplayName returned empty string")
			}
		})
	}
}
