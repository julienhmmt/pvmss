package store_test

import (
	"context"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

//nolint:paralleltest // migration fixtures are intentionally serial
func TestRecordClusterTest_KeepsNewestHundredNewestFirst(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	createHistoryCluster(t, st)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range store.MaxClusterTestHistory + 5 {
		duration := int64(i)

		test := store.ClusterTest{Cluster: historyCluster, Status: "ok", Actor: "history-admin", DurationMs: &duration, TestedAt: base.Add(time.Duration(i) * time.Minute)}
		if err := st.RecordClusterTest(ctx, test); err != nil {
			t.Fatalf("RecordClusterTest %d: %v", i, err)
		}
	}

	all, err := st.ListClusterTests(ctx, historyCluster, 1000)
	if err != nil {
		t.Fatalf("ListClusterTests: %v", err)
	}

	if len(all) != store.MaxClusterTestHistory {
		t.Fatalf("history len = %d, want %d", len(all), store.MaxClusterTestHistory)
	}

	newest := all[0]
	if *newest.DurationMs != store.MaxClusterTestHistory+4 || newest.Actor != "history-admin" || *all[len(all)-1].DurationMs != 5 {
		t.Fatalf("newest = %+v, oldest duration = %d, want newest 104 and oldest 5", newest, *all[len(all)-1].DurationMs)
	}

	five, err := st.ListClusterTests(ctx, historyCluster, 5)
	if err != nil || len(five) != 5 {
		t.Fatalf("limit 5 = %d rows, err %v", len(five), err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestRecordClusterTest_FollowsRename(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	createHistoryCluster(t, st)

	if err := st.SetClusterTestResult(ctx, historyCluster, "unreachable", "", "refused", time.Now()); err != nil {
		t.Fatalf("SetClusterTestResult: %v", err)
	}

	if err := st.RenameCluster(ctx, historyCluster, "renamed"); err != nil {
		t.Fatalf("RenameCluster: %v", err)
	}

	tests, err := st.ListClusterTests(ctx, "renamed", 5)
	if err != nil || len(tests) != 1 || tests[0].Message != "refused" || tests[0].DurationMs != nil {
		t.Fatalf("renamed history = %+v, err %v", tests, err)
	}
}

const historyCluster = "history-test"

func createHistoryCluster(t *testing.T, st *store.Store) {
	t.Helper()

	row := store.ClusterRow{Name: historyCluster, URL: testRestoreClusterURL, TokenID: testClusterTokenID, TokenSecret: testClusterTokenSecret}
	if err := st.CreateCluster(context.Background(), row); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
}
