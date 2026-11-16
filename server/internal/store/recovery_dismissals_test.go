//nolint:wsl_v5 // table-style arrange/assert blocks stay compact
package store_test

import (
	"context"
	"pvmss/server/internal/store"
	"testing"
)

const (
	dismissPoolOld   = "pvmss-old"
	dismissPoolOlder = "pvmss-older"
	dismissPoolB     = "pvmss-b"
	dismissTagName   = "oldtag"
	dismissVMID      = "902"
	dismissActor     = "admin"
)

//nolint:paralleltest // serial: shared database fixture
func TestRecoveryDismissals_RoundTripAndKinds(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.DismissRecoveryItems(ctx, testStoreCluster, store.RecoveryDismissalPool, []string{dismissPoolOld, dismissPoolOlder}, dismissActor); err != nil {
		t.Fatalf("DismissRecoveryItems pools: %v", err)
	}
	if err := st.DismissRecoveryItems(ctx, testStoreCluster, store.RecoveryDismissalTag, []string{dismissTagName}, dismissActor); err != nil {
		t.Fatalf("DismissRecoveryItems tags: %v", err)
	}
	if err := st.DismissRecoveryItems(ctx, testStoreCluster, store.RecoveryDismissalVM, []string{dismissVMID}, dismissActor); err != nil {
		t.Fatalf("DismissRecoveryItems vms: %v", err)
	}

	dismissed, err := st.RecoveryDismissals(ctx, testStoreCluster)
	if err != nil {
		t.Fatalf("RecoveryDismissals: %v", err)
	}
	if len(dismissed) != 4 {
		t.Fatalf("dismissals = %+v, want 4 rows", dismissed)
	}

	for _, want := range []store.RecoveryDismissal{
		{Kind: store.RecoveryDismissalPool, Item: dismissPoolOld},
		{Kind: store.RecoveryDismissalPool, Item: dismissPoolOlder},
		{Kind: store.RecoveryDismissalTag, Item: dismissTagName},
		{Kind: store.RecoveryDismissalVM, Item: dismissVMID},
	} {
		if _, ok := dismissed[want]; !ok {
			t.Fatalf("dismissal %+v missing from %+v", want, dismissed)
		}
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestDismissRecoveryItems_Idempotent(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.DismissRecoveryItems(ctx, testStoreCluster, store.RecoveryDismissalPool, []string{dismissPoolOld}, dismissActor); err != nil {
		t.Fatalf("DismissRecoveryItems: %v", err)
	}
	if err := st.DismissRecoveryItems(ctx, testStoreCluster, store.RecoveryDismissalPool, []string{dismissPoolOld}, "admin2"); err != nil {
		t.Fatalf("DismissRecoveryItems again: %v", err)
	}

	dismissed, err := st.RecoveryDismissals(ctx, testStoreCluster)
	if err != nil {
		t.Fatalf("RecoveryDismissals: %v", err)
	}
	if len(dismissed) != 1 {
		t.Fatalf("dismissals = %+v, want a single row after re-hide", dismissed)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRecoveryDismissals_ClusterIsolation(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.DismissRecoveryItems(ctx, testStoreCluster, store.RecoveryDismissalPool, []string{dismissPoolOld}, dismissActor); err != nil {
		t.Fatalf("DismissRecoveryItems: %v", err)
	}

	other, err := st.RecoveryDismissals(ctx, "other-cluster")
	if err != nil {
		t.Fatalf("RecoveryDismissals other: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("other cluster dismissals = %+v, want empty", other)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRestoreRecoveryItems_RemovesAndToleratesMissing(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.DismissRecoveryItems(ctx, testStoreCluster, store.RecoveryDismissalPool, []string{overridePoolA, dismissPoolB}, dismissActor); err != nil {
		t.Fatalf("DismissRecoveryItems: %v", err)
	}
	// "pvmss-c" was never dismissed: restoring it must be a no-op, not an error.
	if err := st.RestoreRecoveryItems(ctx, testStoreCluster, store.RecoveryDismissalPool, []string{overridePoolA, "pvmss-c"}); err != nil {
		t.Fatalf("RestoreRecoveryItems: %v", err)
	}

	dismissed, err := st.RecoveryDismissals(ctx, testStoreCluster)
	if err != nil {
		t.Fatalf("RecoveryDismissals: %v", err)
	}
	if len(dismissed) != 1 {
		t.Fatalf("dismissals = %+v, want only pvmss-b left", dismissed)
	}
	if _, ok := dismissed[store.RecoveryDismissal{Kind: store.RecoveryDismissalPool, Item: dismissPoolB}]; !ok {
		t.Fatalf("pvmss-b dismissal missing from %+v", dismissed)
	}
}
