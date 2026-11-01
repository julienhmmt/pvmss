package store_test

import (
	"context"
	"errors"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

//nolint:paralleltest // serial: shared database fixture
func TestVMLease_SetGetDeleteRoundTrip(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	expires := time.Date(2030, 5, 1, 12, 0, 0, 0, time.UTC)

	if _, err := st.GetVMLease(ctx, testStoreCluster, 101); !errors.Is(err, store.ErrLeaseNotFound) {
		t.Fatalf("GetVMLease before set err = %v, want ErrLeaseNotFound", err)
	}

	if err := st.SetVMLease(ctx, store.VMLease{Cluster: testStoreCluster, VMID: 101, ExpiresAt: expires}); err != nil {
		t.Fatalf("SetVMLease: %v", err)
	}

	got, err := st.GetVMLease(ctx, testStoreCluster, 101)
	if err != nil {
		t.Fatalf("GetVMLease: %v", err)
	}

	if !got.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, expires)
	}

	next := expires.AddDate(0, 1, 0)
	if err := st.SetVMLease(ctx, store.VMLease{Cluster: testStoreCluster, VMID: 101, ExpiresAt: next}); err != nil {
		t.Fatalf("SetVMLease replace: %v", err)
	}

	if got, _ := st.GetVMLease(ctx, testStoreCluster, 101); !got.ExpiresAt.Equal(next) {
		t.Errorf("replaced ExpiresAt = %v, want %v", got.ExpiresAt, next)
	}

	if err := st.DeleteVMLease(ctx, testStoreCluster, 101); err != nil {
		t.Fatalf("DeleteVMLease: %v", err)
	}

	if err := st.DeleteVMLease(ctx, testStoreCluster, 101); err != nil {
		t.Fatalf("DeleteVMLease absent row: %v", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestVMLease_ListExpiredIsStrictlyBeforeNow(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	now := time.Date(2030, 1, 10, 0, 0, 0, 0, time.UTC)

	for vmid, expires := range map[int]time.Time{
		1: now.Add(-time.Hour),
		2: now,
		3: now.Add(time.Hour),
	} {
		if err := st.SetVMLease(ctx, store.VMLease{Cluster: testStoreCluster, VMID: vmid, ExpiresAt: expires}); err != nil {
			t.Fatalf("SetVMLease(%d): %v", vmid, err)
		}
	}

	expired, err := st.ListExpiredVMLeases(ctx, testStoreCluster, now)
	if err != nil {
		t.Fatalf("ListExpiredVMLeases: %v", err)
	}

	if len(expired) != 1 || expired[0].VMID != 1 {
		t.Errorf("expired = %+v, want only VM 1 (expiry == now is not expired)", expired)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestPolicyRow_LeaseDaysRoundTrip(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	row := samplePolicyRow(testStoreCluster)
	row.LeaseDays = 30

	if err := st.UpsertPolicyRow(ctx, row); err != nil {
		t.Fatalf("UpsertPolicyRow: %v", err)
	}

	got, err := st.PolicyRow(ctx, testStoreCluster)
	if err != nil {
		t.Fatalf("PolicyRow: %v", err)
	}

	if got.LeaseDays != 30 {
		t.Errorf("LeaseDays = %d, want 30", got.LeaseDays)
	}
}
