package store_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/store"
	"testing"
)

const (
	overridePoolA = "pvmss-a"
	overrideActor = "root"
)

// countOverrides asserts how many overrides ListPoolQuotaOverrides returns
// for cluster ("" = every cluster).
func countOverrides(t *testing.T, st *store.Store, cluster string, want int) {
	t.Helper()

	rows, err := st.ListPoolQuotaOverrides(context.Background(), cluster)
	if err != nil || len(rows) != want {
		t.Fatalf("ListPoolQuotaOverrides(%q) = %d rows, err %v; want %d", cluster, len(rows), err, want)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestPoolQuotaOverride_CRUD(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if _, err := st.PoolQuotaOverride(ctx, testStoreCluster, overridePoolA); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("PoolQuotaOverride absent err = %v, want sql.ErrNoRows", err)
	}

	if err := st.UpsertPoolQuotaOverride(ctx, store.PoolQuotaOverride{Cluster: testStoreCluster, Pool: overridePoolA, MaxVMs: 4, UpdatedBy: overrideActor}); err != nil {
		t.Fatalf("UpsertPoolQuotaOverride insert: %v", err)
	}

	if err := st.UpsertPoolQuotaOverride(ctx, store.PoolQuotaOverride{Cluster: testStoreCluster, Pool: overridePoolA, MaxVMs: 9, UpdatedBy: adminDocID}); err != nil {
		t.Fatalf("UpsertPoolQuotaOverride update: %v", err)
	}

	row, err := st.PoolQuotaOverride(ctx, testStoreCluster, overridePoolA)
	if err != nil || row.MaxVMs != 9 || row.UpdatedBy != adminDocID || row.UpdatedAt == "" {
		t.Fatalf("PoolQuotaOverride = %+v, err %v; want MaxVMs 9 by admin", row, err)
	}

	if err := st.UpsertPoolQuotaOverride(ctx, store.PoolQuotaOverride{Cluster: "other", Pool: "pvmss-b", MaxVMs: -1, UpdatedBy: overrideActor}); err != nil {
		t.Fatalf("UpsertPoolQuotaOverride other cluster: %v", err)
	}

	countOverrides(t, st, testStoreCluster, 1)
	countOverrides(t, st, "", 2)

	if err := st.DeletePoolQuotaOverride(ctx, testStoreCluster, overridePoolA); err != nil {
		t.Fatalf("DeletePoolQuotaOverride: %v", err)
	}

	if err := st.DeletePoolQuotaOverride(ctx, testStoreCluster, overridePoolA); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeletePoolQuotaOverride absent err = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestQuotaRequest_DecideWithOverrideCommitsBothWrites(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.CreateQuotaRequest(ctx, sampleQuotaRequest("frank")); err != nil {
		t.Fatalf("CreateQuotaRequest: %v", err)
	}

	pending, err := st.PendingQuotaRequest(ctx, "frank", testStoreCluster, "pvmss-frank")
	if err != nil {
		t.Fatalf("PendingQuotaRequest: %v", err)
	}

	before, err := st.PolicyRow(ctx, testStoreCluster)
	if err != nil {
		t.Fatalf("PolicyRow: %v", err)
	}

	next := store.PoolQuotaOverride{Cluster: testStoreCluster, Pool: "pvmss-frank", MaxVMs: 7, UpdatedBy: adminDocID}
	if err := st.DecideQuotaRequestWithPolicy(ctx, pending.ID, store.QuotaRequestApproved, adminDocID, nil, next); err != nil {
		t.Fatalf("DecideQuotaRequestWithPolicy: %v", err)
	}

	decided, err := st.QuotaRequestByID(ctx, pending.ID)
	if err != nil || decided.Status != store.QuotaRequestApproved {
		t.Fatalf("request = %+v, err %v; want approved", decided, err)
	}

	override, err := st.PoolQuotaOverride(ctx, testStoreCluster, "pvmss-frank")
	if err != nil || override.MaxVMs != 7 {
		t.Fatalf("override = %+v, err %v; want MaxVMs 7", override, err)
	}

	stored, err := st.PolicyRow(ctx, testStoreCluster)
	if err != nil || stored.MaxVMPerUser != before.MaxVMPerUser {
		t.Fatalf("global MaxVMPerUser = %d, err %v; want untouched %d", stored.MaxVMPerUser, err, before.MaxVMPerUser)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestQuotaRequest_DecideWithOverrideRollsBackOnConflict(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.UpsertPoolQuotaOverride(ctx, store.PoolQuotaOverride{Cluster: testStoreCluster, Pool: "pvmss-grace", MaxVMs: 3, UpdatedBy: overrideActor}); err != nil {
		t.Fatalf("UpsertPoolQuotaOverride: %v", err)
	}

	if err := st.CreateQuotaRequest(ctx, sampleQuotaRequest("grace")); err != nil {
		t.Fatalf("CreateQuotaRequest: %v", err)
	}

	pending, err := st.PendingQuotaRequest(ctx, "grace", testStoreCluster, "pvmss-grace")
	if err != nil {
		t.Fatalf("PendingQuotaRequest: %v", err)
	}

	current, err := st.PoolQuotaOverride(ctx, testStoreCluster, "pvmss-grace")
	if err != nil {
		t.Fatalf("PoolQuotaOverride: %v", err)
	}

	stale := current
	stale.MaxVMs = current.MaxVMs + 100
	next := store.PoolQuotaOverride{Cluster: testStoreCluster, Pool: "pvmss-grace", MaxVMs: 7, UpdatedBy: adminDocID}

	err = st.DecideQuotaRequestWithPolicy(ctx, pending.ID, store.QuotaRequestApproved, adminDocID, &stale, next)
	if !errors.Is(err, store.ErrPolicyConflict) {
		t.Fatalf("error = %v, want ErrPolicyConflict", err)
	}

	still, err := st.QuotaRequestByID(ctx, pending.ID)
	if err != nil || still.Status != store.QuotaRequestPending {
		t.Fatalf("request = %+v, err %v; want still pending after rollback", still, err)
	}

	stored, err := st.PoolQuotaOverride(ctx, testStoreCluster, "pvmss-grace")
	if err != nil || stored.MaxVMs != current.MaxVMs {
		t.Fatalf("override MaxVMs = %d, err %v; want untouched %d", stored.MaxVMs, err, current.MaxVMs)
	}

	// A nil expected asserts that no override exists yet, which is false here.
	err = st.DecideQuotaRequestWithPolicy(ctx, pending.ID, store.QuotaRequestApproved, adminDocID, nil, next)
	if !errors.Is(err, store.ErrPolicyConflict) {
		t.Fatalf("insert-over-existing error = %v, want ErrPolicyConflict", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestQuotaRequest_DecideWithOverrideLosesRaceWithoutWrite(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.CreateQuotaRequest(ctx, sampleQuotaRequest("heidi")); err != nil {
		t.Fatalf("CreateQuotaRequest: %v", err)
	}

	pending, err := st.PendingQuotaRequest(ctx, "heidi", testStoreCluster, "pvmss-heidi")
	if err != nil {
		t.Fatalf("PendingQuotaRequest: %v", err)
	}

	if err := st.DecideQuotaRequest(ctx, pending.ID, store.QuotaRequestDenied, "other"); err != nil {
		t.Fatalf("DecideQuotaRequest: %v", err)
	}

	next := store.PoolQuotaOverride{Cluster: testStoreCluster, Pool: "pvmss-heidi", MaxVMs: 7, UpdatedBy: adminDocID}

	err = st.DecideQuotaRequestWithPolicy(ctx, pending.ID, store.QuotaRequestApproved, adminDocID, nil, next)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("error = %v, want sql.ErrNoRows", err)
	}

	if _, err := st.PoolQuotaOverride(ctx, testStoreCluster, "pvmss-heidi"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("override written despite lost race, err = %v", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestUnregisterManagedPool_RemovesOverride(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.RegisterManagedPool(ctx, testStoreCluster, "pvmss-gone"); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}

	if err := st.UpsertPoolQuotaOverride(ctx, store.PoolQuotaOverride{Cluster: testStoreCluster, Pool: "pvmss-gone", MaxVMs: 5, UpdatedBy: overrideActor}); err != nil {
		t.Fatalf("UpsertPoolQuotaOverride: %v", err)
	}

	if err := st.UnregisterManagedPool(ctx, testStoreCluster, "pvmss-gone"); err != nil {
		t.Fatalf("UnregisterManagedPool: %v", err)
	}

	if _, err := st.PoolQuotaOverride(ctx, testStoreCluster, "pvmss-gone"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("override survived pool unregister, err = %v", err)
	}
}
