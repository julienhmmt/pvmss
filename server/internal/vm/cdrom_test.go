package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestSetCDROM_StatesAndApproval(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		isoVolID  string
		wantState string
		wantErr   error
		wantCalls int
	}{
		{name: "mount approved ISO", action: "mount", isoVolID: "local:iso/debian-12-generic-amd64.iso", wantState: cluster.CDROMMounted, wantCalls: 1},
		{name: "disconnect keeps drive", action: "disconnect", wantState: cluster.CDROMEmpty, wantCalls: 1},
		{name: "remove drive", action: "remove", wantState: cluster.CDROMAbsent, wantCalls: 1},
		{name: "reject unapproved ISO", action: "mount", isoVolID: "local:forged.iso", wantErr: vm.ErrISOVolumeNotApproved},
		{name: "reject unknown action", action: "eject", wantErr: vm.ErrInvalidCDROMAction},
		{name: testNonOwnerCase, action: "mount", isoVolID: "local:iso/debian-12-generic-amd64.iso", wantErr: vm.ErrForbidden},
	}
	for _, test := range tests { //nolint:paralleltest // serial: shared fake cluster dataset
		t.Run(test.name, func(t *testing.T) {
			cluster.ResetFake()

			actor := aliceIdentity()
			if test.name == testNonOwnerCase {
				actor = bobIdentity()
			}

			deps := cdromDependencies(diskTestIndex(t, 101, cluster.VMRunning), actor, 101)

			state, err := vm.SetCDROM(context.Background(), deps, test.action, test.isoVolID)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("err = %v, want %v", err, test.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if state.State != test.wantState {
					t.Fatalf("state = %q, want %q", state.State, test.wantState)
				}
			}

			if calls := cluster.FakeCallsFor(101); len(calls) != test.wantCalls {
				t.Fatalf("fake calls = %+v, want %d", calls, test.wantCalls)
			}
		})
	}
}

func cdromDependencies(index *inventory.Index, actor auth.Identity, vmid int) vm.CDROMDependencies {
	return vm.CDROMDependencies{
		Index: index, Actor: actor, ClusterName: testClusterName, VMID: vmid,
		Writer: cluster.Fake{}, Resources: catalog.Resources{ISOs: []catalog.ISO{{Storage: "local", File: "debian-12-generic-amd64.iso"}}},
		Audit: noopAudit{}, Refresher: noopRefresher{},
	}
}
