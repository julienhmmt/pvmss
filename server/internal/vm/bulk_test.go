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
	"sync"
	"testing"
	"time"
)

const (
	bulkStatusError = "error"
	// clusterA/clusterB are the two-cluster names used in the per-cluster
	// refresh tests. Kept as consts so the repeated literals don't trip goconst.
	clusterA = "cluster-a"
	clusterB = "cluster-b"
)

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

	results := vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:  resolver,
		Actor:     aliceIdentity(),
		Writer:    cluster.Fake{},
		Audit:     noopAudit{},
		Refresher: noopRefresher{},
	}, targets, "start")
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

	results := vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:  resolver,
		Actor:     aliceIdentity(),
		Writer:    cluster.Fake{},
		Audit:     noopAudit{},
		Refresher: noopRefresher{},
	}, targets, "start")
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

	results := vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:  resolver,
		Actor:     aliceIdentity(),
		Writer:    cluster.Fake{},
		Audit:     noopAudit{},
		Refresher: noopRefresher{},
	}, targets, "start")
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

	results := vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:  resolver,
		Actor:     aliceIdentity(),
		Writer:    cluster.Fake{},
		Audit:     noopAudit{},
		Refresher: noopRefresher{},
	}, targets, "start")
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
	results := vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:  resolver,
		Actor:     aliceIdentity(),
		Writer:    cluster.Fake{},
		Audit:     noopAudit{},
		Refresher: noopRefresher{},
	}, targets, "start")
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

	results := vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:  resolver,
		Actor:     aliceIdentity(),
		Writer:    cluster.Fake{},
		Audit:     st,
		Refresher: noopRefresher{},
	}, targets, "start")
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

	results := vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:  resolver,
		Actor:     aliceIdentity(),
		Writer:    cluster.Fake{},
		Audit:     noopAudit{},
		Refresher: noopRefresher{},
	}, targets, "start")
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

// =============================================================================
// Bulk refresh behavior (ticket 09)
// =============================================================================

// countingRefresher is an IndexRefresher that counts Refresh calls.
// Thread-safe via the embedded mutex.
type countingRefresher struct {
	mu        sync.Mutex
	calls     int
	byCluster map[string]int
}

func (r *countingRefresher) Refresh(_ context.Context) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls++

	return time.Now(), nil
}

func (r *countingRefresher) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.calls
}

// countingRefresherResolver is a ClusterRefresherResolver that returns a
// countingRefresher per cluster, tracking calls per cluster.
type countingRefresherResolver struct {
	mu         sync.Mutex
	refreshers map[string]*countingRefresher
}

func newCountingRefresherResolver() *countingRefresherResolver {
	return &countingRefresherResolver{refreshers: make(map[string]*countingRefresher)}
}

func (r *countingRefresherResolver) RefresherFor(clusterName string) (vm.IndexRefresher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ref, ok := r.refreshers[clusterName]
	if !ok {
		ref = &countingRefresher{byCluster: make(map[string]int)}
		r.refreshers[clusterName] = ref
	}

	return ref, nil
}

func (r *countingRefresherResolver) callsFor(clusterName string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ref, ok := r.refreshers[clusterName]; ok {
		return ref.callCount()
	}

	return 0
}

//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_SingleCluster_OneRefresh(t *testing.T) {
	resolver := bulkTestResolver(t)
	refresher := &countingRefresher{}
	refresherResolver := newCountingRefresherResolver()

	// 10 targets on the same cluster.
	targets := make([]vm.BulkTarget, 10)
	for i := range targets {
		targets[i] = vm.BulkTarget{Cluster: testClusterName, VMID: 101 + i}
	}

	_ = vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:          resolver,
		RefresherResolver: refresherResolver,
		Actor:             aliceIdentity(),
		Writer:            cluster.Fake{},
		Audit:             noopAudit{},
		Refresher:         refresher,
	}, targets, "start")

	// With RefresherResolver, the per-cluster refresher is used once.
	if got := refresherResolver.callsFor(testClusterName); got != 1 {
		t.Errorf("refresh calls for %q = %d, want 1", testClusterName, got)
	}

	// The fallback refresher should NOT be used when a resolver is set.
	if refresher.callCount() != 0 {
		t.Errorf("fallback refresher calls = %d, want 0", refresher.callCount())
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_TwoClusters_OneRefreshPerCluster(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := inventory.BuildIndex(snap)

	resolver := testIndexResolver{indexes: map[string]*inventory.Index{
		clusterA: &idx,
		clusterB: &idx,
	}}
	refresherResolver := newCountingRefresherResolver()

	// 5 targets on cluster-a, 5 on cluster-b.
	targets := []vm.BulkTarget{
		{Cluster: clusterA, VMID: 101},
		{Cluster: clusterA, VMID: 102},
		{Cluster: clusterA, VMID: 103},
		{Cluster: clusterA, VMID: 104},
		{Cluster: clusterA, VMID: 105},
		{Cluster: clusterB, VMID: 101},
		{Cluster: clusterB, VMID: 102},
		{Cluster: clusterB, VMID: 103},
		{Cluster: clusterB, VMID: 104},
		{Cluster: clusterB, VMID: 105},
	}

	_ = vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:          resolver,
		RefresherResolver: refresherResolver,
		Actor:             aliceIdentity(),
		Writer:            cluster.Fake{},
		Audit:             noopAudit{},
	}, targets, "start")

	if got := refresherResolver.callsFor(clusterA); got != 1 {
		t.Errorf("refresh calls for cluster-a = %d, want 1", got)
	}

	if got := refresherResolver.callsFor(clusterB); got != 1 {
		t.Errorf("refresh calls for cluster-b = %d, want 1", got)
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_FailedTarget_StillRefreshes(t *testing.T) {
	resolver := bulkTestResolver(t)
	refresherResolver := newCountingRefresherResolver()

	// VM 100 is running → start fails. VM 101 is stopped → start succeeds.
	// The refresh should still happen after the loop.
	targets := []vm.BulkTarget{
		{Cluster: testClusterName, VMID: 100},
		{Cluster: testClusterName, VMID: 101},
	}

	results := vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:          resolver,
		RefresherResolver: refresherResolver,
		Actor:             aliceIdentity(),
		Writer:            cluster.Fake{},
		Audit:             noopAudit{},
	}, targets, "start")

	// One target failed, one succeeded.
	if results[0].Status != bulkStatusError {
		t.Errorf("result[0] = %q, want error", results[0].Status)
	}

	if results[1].Status != "ok" {
		t.Errorf("result[1] = %q, want ok", results[1].Status)
	}

	// Refresh still happened once for the cluster.
	if got := refresherResolver.callsFor(testClusterName); got != 1 {
		t.Errorf("refresh calls = %d, want 1 (even with a failed target)", got)
	}
}

// Ensure the unused import is referenced (auth is used via aliceIdentity which
// returns auth.Identity, but the import is in create_test.go — this file needs
// its own reference if it constructs identities directly).
var _ auth.Identity
