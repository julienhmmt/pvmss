package vm_test

import (
	"context"
	"fmt"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

const bulkStatusError = "error"

// testIndexResolver is a simple ClusterIndexResolver backed by a static map of
// cluster name → Index. It stands in for the inventory Registry in unit tests
// — no goroutines, no refresh, just the per-cluster lookup BulkAction needs.
type testIndexResolver struct {
	indexes map[string]*inventory.Index
}

func (r testIndexResolver) IndexFor(clusterName string) (*inventory.Index, error) {
	idx, ok := r.indexes[clusterName]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", clusterName)
	}

	return idx, nil
}

// bulkTestResolver builds a resolver from the current fake snapshot, keyed
// under testClusterName ("default"). Every test that mutates the fake MUST
// defer cluster.ResetFake() so later tests see the full 25-VM dataset.
func bulkTestResolver(t *testing.T) testIndexResolver {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := inventory.BuildIndex(snap)

	return testIndexResolver{indexes: map[string]*inventory.Index{testClusterName: &idx}}
}

func bulkTestStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "bulk-test.db"),
		LogLevel:  "info",
		LogFormat: "json",
		LogOutput: "stdout",
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// TestBulkAction_AllSuccessBatch — T003: a batch where every target is owned
// and status-compatible produces one "ok" entry per target, in order.
//
//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_AllSuccessBatch(t *testing.T) {
	resolver := bulkTestResolver(t)
	// VM 101 and 124 are stopped, owned by alice — start succeeds on both.
	targets := []vm.BulkTarget{
		{Cluster: testClusterName, VMID: 101},
		{Cluster: testClusterName, VMID: 124},
	}

	results := vm.BulkAction(context.Background(), resolver, aliceIdentity(), targets, "start", cluster.Fake{}, noopAudit{}, noopRefresher{})
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}

	for i, r := range results {
		if r.Cluster != testClusterName {
			t.Errorf("result[%d].Cluster = %q, want %q", i, r.Cluster, testClusterName)
		}

		if r.Status != "ok" {
			t.Errorf("result[%d].Status = %q, want ok; message=%q", i, r.Status, r.Message)
		}

		if r.VMID != targets[i].VMID {
			t.Errorf("result[%d].VMID = %d, want %d", i, r.VMID, targets[i].VMID)
		}
	}
}

// TestBulkAction_MixedBatch — T003: a batch mixing owned/status-compatible,
// owned/already-in-target-state, and non-owned targets. Each result entry is
// asserted independently per data-model.md's sequence.
//
//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_MixedBatch(t *testing.T) {
	resolver := bulkTestResolver(t)
	// 101: alice's, stopped → start → ok.
	// 100: alice's, running → start → error "vm already running".
	// 103: bob's, running → alice tries start → error "forbidden".
	targets := []vm.BulkTarget{
		{Cluster: testClusterName, VMID: 101},
		{Cluster: testClusterName, VMID: 100},
		{Cluster: testClusterName, VMID: 103},
	}

	results := vm.BulkAction(context.Background(), resolver, aliceIdentity(), targets, "start", cluster.Fake{}, noopAudit{}, noopRefresher{})
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}

	if results[0].Status != "ok" {
		t.Errorf("result[0] (101 stopped→start) = %q, want ok; msg=%q", results[0].Status, results[0].Message)
	}

	if results[1].Status != bulkStatusError {
		t.Errorf("result[1] (100 running→start) = %q, want error", results[1].Status)
	}

	if results[2].Status != bulkStatusError {
		t.Errorf("result[2] (103 bob's→alice start) = %q, want error", results[2].Status)
	}

	// Non-owned target: zero fake client calls for that VMID.
	if calls := cluster.FakeCallsFor(103); len(calls) != 0 {
		t.Errorf("fake calls for 103 (non-owned) = %d, want 0", len(calls))
	}
}

// TestBulkAction_DuplicateTargetProcessedTwice — T003: a duplicate (cluster,
// vmid) pair is processed twice independently. The first start on a stopped
// VM succeeds; the second start on the now-running VM fails.
//
//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_DuplicateTargetProcessedTwice(t *testing.T) {
	resolver := bulkTestResolver(t)
	targets := []vm.BulkTarget{
		{Cluster: testClusterName, VMID: 101},
		{Cluster: testClusterName, VMID: 101},
	}

	results := vm.BulkAction(context.Background(), resolver, aliceIdentity(), targets, "start", cluster.Fake{}, noopAudit{}, noopRefresher{})
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}

	if results[0].Status != "ok" {
		t.Errorf("result[0] = %q, want ok; msg=%q", results[0].Status, results[0].Message)
	}

	if results[1].Status != bulkStatusError {
		t.Errorf("result[1] = %q, want error (vm already running after first start)", results[1].Status)
	}
}

// TestBulkAction_SingleTargetBatch — T003: a single-target batch produces one
// entry.
//
//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_SingleTargetBatch(t *testing.T) {
	resolver := bulkTestResolver(t)
	targets := []vm.BulkTarget{{Cluster: testClusterName, VMID: 101}}

	results := vm.BulkAction(context.Background(), resolver, aliceIdentity(), targets, "start", cluster.Fake{}, noopAudit{}, noopRefresher{})
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	if results[0].Status != "ok" {
		t.Errorf("result[0] = %q, want ok; msg=%q", results[0].Status, results[0].Message)
	}
}

// TestBulkAction_FullCeilingUnder2s — T003/SC-005: a 100-target batch against
// the fake cluster.Client returns in under 2 seconds.
//
//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_FullCeilingUnder2s(t *testing.T) {
	resolver := bulkTestResolver(t)

	targets := make([]vm.BulkTarget, vm.MaxBulkTargets)
	for i := range targets {
		targets[i] = vm.BulkTarget{Cluster: testClusterName, VMID: 101}
	}

	start := time.Now()
	results := vm.BulkAction(context.Background(), resolver, aliceIdentity(), targets, "start", cluster.Fake{}, noopAudit{}, noopRefresher{})
	elapsed := time.Since(start)

	if len(results) != vm.MaxBulkTargets {
		t.Fatalf("results = %d, want %d", len(results), vm.MaxBulkTargets)
	}

	if elapsed >= 2*time.Second {
		t.Fatalf("100-target batch took %v, want < 2s (SC-005)", elapsed)
	}

	t.Logf("100-target batch completed in %v", elapsed)
}

// TestBulkAction_AuditRowsMatchSuccesses — SC-004: a batch of N targets, M of
// which succeed, produces exactly M new audit_log rows (via store.RecordAction,
// called inside T05's own Action()).
//
//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_AuditRowsMatchSuccesses(t *testing.T) {
	resolver := bulkTestResolver(t)
	st := bulkTestStore(t)
	// 101: stopped → start → ok (1 audit row).
	// 100: running → start → error (0 audit rows — Action returns before RecordAction).
	// 124: stopped → start → ok (1 audit row).
	targets := []vm.BulkTarget{
		{Cluster: testClusterName, VMID: 101},
		{Cluster: testClusterName, VMID: 100},
		{Cluster: testClusterName, VMID: 124},
	}

	results := vm.BulkAction(context.Background(), resolver, aliceIdentity(), targets, "start", cluster.Fake{}, st, noopRefresher{})
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	wantRows := 0

	for _, r := range results {
		if r.Status == "ok" {
			wantRows++
		}
	}

	if len(rows) != wantRows {
		t.Fatalf("audit rows = %d, want %d (one per successful target)", len(rows), wantRows)
	}
}

// TestBulkAction_NonExistentClusterError — a target naming a cluster the
// resolver doesn't know produces an "error" entry; the batch never fails as a
// whole.
//
//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_NonExistentClusterError(t *testing.T) {
	resolver := bulkTestResolver(t)
	targets := []vm.BulkTarget{
		{Cluster: testClusterName, VMID: 101},
		{Cluster: "nonexistent", VMID: 200},
	}

	results := vm.BulkAction(context.Background(), resolver, aliceIdentity(), targets, "start", cluster.Fake{}, noopAudit{}, noopRefresher{})
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}

	if results[0].Status != "ok" {
		t.Errorf("result[0] = %q, want ok", results[0].Status)
	}

	if results[1].Status != bulkStatusError {
		t.Errorf("result[1] = %q, want error", results[1].Status)
	}
}

// Ensure the unused import is referenced (auth is used via aliceIdentity which
// returns auth.Identity, but the import is in create_test.go — this file needs
// its own reference if it constructs identities directly).
var _ auth.Identity
