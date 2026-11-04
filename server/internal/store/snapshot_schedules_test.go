package store_test

import (
	"context"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

func newScheduleStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "schedules.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

//nolint:wsl_v5 // lifecycle tests keep fixture setup and assertions adjacent
func TestSnapshotScheduleFailureClearedByRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newScheduleStore(t)
	if err := st.UpsertSnapshotSchedule(ctx, testStoreCluster, 100, "tenants", "daily", 3); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	at := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	if err := st.MarkSnapshotScheduleFailure(ctx, testStoreCluster, 100, "cap_reached", at); err != nil {
		t.Fatalf("mark failure: %v", err)
	}
	got, _, err := st.GetSnapshotSchedule(ctx, testStoreCluster, 100)
	if err != nil || got.LastFailure != "cap_reached" || !got.LastFailureAt.Equal(at) || !got.LastRunAt.IsZero() {
		t.Fatalf("after failure: %+v err=%v", got, err)
	}

	if err := st.MarkSnapshotScheduleRun(ctx, testStoreCluster, 100, at.Add(time.Hour)); err != nil {
		t.Fatalf("mark run: %v", err)
	}
	got, _, err = st.GetSnapshotSchedule(ctx, testStoreCluster, 100)
	if err != nil || got.LastFailure != "" || !got.LastFailureAt.IsZero() {
		t.Fatalf("after run: %+v err=%v", got, err)
	}
}

//nolint:wsl_v5 // lifecycle tests keep fixture setup and assertions adjacent
func TestSnapshotScheduleLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newScheduleStore(t)

	if _, found, err := st.GetSnapshotSchedule(ctx, testStoreCluster, 100); err != nil || found {
		t.Fatalf("empty table: found=%v err=%v", found, err)
	}

	if err := st.UpsertSnapshotSchedule(ctx, testStoreCluster, 100, "tenants", "daily", 3); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	run := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	if err := st.MarkSnapshotScheduleRun(ctx, testStoreCluster, 100, run); err != nil {
		t.Fatalf("mark run: %v", err)
	}

	// Changing the policy keeps the last run, so an edit does not re-fire.
	if err := st.UpsertSnapshotSchedule(ctx, testStoreCluster, 100, "tenants", "weekly", 5); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}

	got, found, err := st.GetSnapshotSchedule(ctx, testStoreCluster, 100)
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if got.Pool != "tenants" || got.Frequency != "weekly" || got.Retain != 5 || !got.LastRunAt.Equal(run) {
		t.Fatalf("schedule = %+v", got)
	}
}

//nolint:wsl_v5 // lifecycle tests keep fixture setup and assertions adjacent
func TestSnapshotScheduleListAndDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newScheduleStore(t)

	if err := st.UpsertSnapshotSchedule(ctx, testStoreCluster, 100, "tenants", "daily", 3); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	all, err := st.ListSnapshotSchedules(ctx)
	if err != nil || len(all) != 1 || !all[0].LastRunAt.IsZero() {
		t.Fatalf("list: %v err=%v", all, err)
	}

	if err := st.DeleteSnapshotSchedule(ctx, testStoreCluster, 100); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := st.DeleteSnapshotSchedule(ctx, testStoreCluster, 100); err != nil {
		t.Fatalf("delete of absent row must not error: %v", err)
	}
}
