package store_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/store"
	"testing"
)

func sampleQuotaRequest(requester string) store.QuotaRequest {
	return store.QuotaRequest{
		Requester: requester, Cluster: testStoreCluster, Pool: "pvmss-" + requester,
		CurrentQuota: 2, RequestedQuota: 5, Reason: "need more",
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestQuotaRequest_OnePendingPerRequesterPool(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.CreateQuotaRequest(ctx, sampleQuotaRequest("alice")); err != nil {
		t.Fatalf("first CreateQuotaRequest: %v", err)
	}

	if err := st.CreateQuotaRequest(ctx, sampleQuotaRequest("alice")); !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("second pending error = %v, want ErrDuplicate", err)
	}

	pending, err := st.PendingQuotaRequest(ctx, "alice", "default", "pvmss-alice")
	if err != nil {
		t.Fatalf("PendingQuotaRequest: %v", err)
	}

	if pending.Status != store.QuotaRequestPending || pending.RequestedQuota != 5 || pending.Reason != "need more" {
		t.Fatalf("pending row = %+v", pending)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestQuotaRequest_DecideOnlyOnce(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.CreateQuotaRequest(ctx, sampleQuotaRequest("bob")); err != nil {
		t.Fatalf("CreateQuotaRequest: %v", err)
	}

	pending, err := st.PendingQuotaRequest(ctx, "bob", "default", "pvmss-bob")
	if err != nil {
		t.Fatalf("PendingQuotaRequest: %v", err)
	}

	if err := st.DecideQuotaRequest(ctx, pending.ID, store.QuotaRequestApproved, "admin"); err != nil {
		t.Fatalf("DecideQuotaRequest: %v", err)
	}

	if err := st.DecideQuotaRequest(ctx, pending.ID, store.QuotaRequestDenied, "other"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second decide error = %v, want sql.ErrNoRows", err)
	}

	decided, err := st.QuotaRequestByID(ctx, pending.ID)
	if err != nil {
		t.Fatalf("QuotaRequestByID: %v", err)
	}

	if decided.Status != store.QuotaRequestApproved || decided.DecidedBy != adminDocID || decided.DecidedAt == "" {
		t.Fatalf("decided row = %+v", decided)
	}

	// A new request is allowed once the previous one is decided.
	if err := st.CreateQuotaRequest(ctx, sampleQuotaRequest("bob")); err != nil {
		t.Fatalf("CreateQuotaRequest after decision: %v", err)
	}

	latest, err := st.LatestQuotaRequest(ctx, "bob", "default", "pvmss-bob")
	if err != nil || latest.Status != store.QuotaRequestPending {
		t.Fatalf("latest = %+v, err %v; want the new pending row", latest, err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestQuotaRequest_ListByStatusAndMissing(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.CreateQuotaRequest(ctx, sampleQuotaRequest("carol")); err != nil {
		t.Fatalf("CreateQuotaRequest: %v", err)
	}

	pending, err := st.ListQuotaRequests(ctx, store.QuotaRequestPending)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending list = %d rows, err %v; want 1", len(pending), err)
	}

	denied, err := st.ListQuotaRequests(ctx, store.QuotaRequestDenied)
	if err != nil || len(denied) != 0 {
		t.Fatalf("denied list = %d rows, err %v; want 0", len(denied), err)
	}

	all, err := st.ListQuotaRequests(ctx, "")
	if err != nil || len(all) != 1 {
		t.Fatalf("all list = %d rows, err %v; want 1", len(all), err)
	}

	if _, err := st.QuotaRequestByID(ctx, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing id error = %v, want sql.ErrNoRows", err)
	}

	if _, err := st.PendingQuotaRequest(ctx, "nobody", "default", "pvmss-nobody"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("no pending error = %v, want sql.ErrNoRows", err)
	}
}
