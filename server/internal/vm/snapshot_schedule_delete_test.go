//nolint:wsl_v5 // delete test keeps fixture setup and assertions adjacent
package vm_test

import (
	"context"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"testing"
)

// deleteWriter satisfies cluster.Writer for Delete; only Delete is exercised.
type deleteWriter struct {
	cluster.Writer
}

func (deleteWriter) Delete(context.Context, string, int) error { return nil }

// TestDeleteVMRemovesSnapshotSchedule proves a deleted VM's schedule does not
// survive to be inherited by a recycled VMID.
func TestDeleteVMRemovesSnapshotSchedule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "delete.db"),
		LogLevel:  "info",
		LogFormat: "json",
		LogOutput: "stdout",
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := st.UpsertSnapshotSchedule(ctx, "default", 100, "", vm.ScheduleDaily, 2); err != nil {
		t.Fatalf("seed schedule: %v", err)
	}

	index := &inventory.Index{ByVMID: map[int]cluster.VM{
		100: {VMID: 100, Node: "pve1", Cluster: "default", Status: cluster.VMStopped, Tags: []string{testPvmssTag}},
	}}
	err = vm.Delete(ctx, vm.WriteDeps{
		Index:       index,
		Actor:       auth.Identity{Username: "admin", IsAdmin: true},
		ClusterName: "default",
		VMID:        100,
		Writer:      deleteWriter{},
		Audit:       &snapshotAudit{},
		Refresher:   noopRefresher{},
		Store:       st,
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, found, err := st.GetSnapshotSchedule(ctx, "default", 100); err != nil || found {
		t.Fatalf("schedule survived VM delete: found=%v err=%v", found, err)
	}
}
