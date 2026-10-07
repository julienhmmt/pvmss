package httpapi

import (
	"context"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

// The projection lags /cluster/resources by seconds after a write; a write
// response must show what was just written, not the stale read.
func TestWrittenValuesOverlayStaleEntity(t *testing.T) {
	t.Parallel()

	stale := vm.Entity{Name: "old", Description: "d", Sockets: 1, Cores: 1, CPUCores: 1, MemoryTotal: 1 << 30}

	renamed := withPatch(stale, patchRequest{Name: "new"})
	if renamed.Name != "new" || renamed.Description != "d" {
		t.Errorf("patch overlay = %q/%q, want new/d", renamed.Name, renamed.Description)
	}

	two, mem := 2, 1536

	resized := withHardware(stale, hardwareRequest{Cores: &two, MemoryMB: &mem})
	if resized.Cores != 2 || resized.CPUCores != 2 || resized.MemoryTotal != 1536<<20 || resized.Sockets != 1 {
		t.Errorf("hardware overlay = %+v", resized)
	}

	if stale.Name != "old" || stale.Cores != 1 {
		t.Error("overlay mutated its input")
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestPatchVM_EmptyDescriptionClearsIt(t *testing.T) {
	cluster.ResetFake()

	ctx := context.Background()
	_ = (cluster.Fake{}).Patch(ctx, cluster.FakeNode01, 101, "", "old text")

	snapshot, _ := (cluster.Fake{}).Snapshot(ctx)
	index := inventory.BuildIndex(snapshot)
	actor := auth.Identity{Username: "alice@pve", Pool: "pool-alice"}
	empty := ""

	err := patchVM(ctx, vm.WriteDeps{Index: &index, Actor: actor, ClusterName: "default", VMID: 101, Writer: cluster.Fake{}, Audit: nopAudit{}, Refresher: nopRefresh{}}, patchRequest{Description: &empty})
	if err != nil {
		t.Fatalf("patchVM: %v", err)
	}

	snapshot, _ = (cluster.Fake{}).Snapshot(ctx)
	for _, v := range snapshot.VMs {
		if v.VMID == 101 && v.Description != "" {
			t.Errorf("description = %q, want cleared", v.Description)
		}
	}
}

type nopAudit struct{}

func (nopAudit) RecordAction(context.Context, string, string, int, string) error { return nil }

type nopRefresh struct{}

func (nopRefresh) Refresh(context.Context) (time.Time, error) { return time.Time{}, nil }
