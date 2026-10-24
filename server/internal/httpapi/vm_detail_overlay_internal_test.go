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
