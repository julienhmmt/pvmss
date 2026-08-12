package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateNetwork_ValidatesBridgeAndCardCount(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []cluster.NetworkInterface
		actor      auth.Identity
		wantErr    error
	}{
		{name: "approved bridge", interfaces: []cluster.NetworkInterface{{Index: 0, Bridge: testBridgeVMbr1, Model: testModelVirtio}}},
		{name: "unapproved bridge", interfaces: []cluster.NetworkInterface{{Index: 0, Bridge: "vmbr9", Model: testModelVirtio}}, wantErr: vm.ErrBridgeNotApproved},
		{name: "invalid model", interfaces: []cluster.NetworkInterface{{Index: 0, Bridge: testBridgeVMbr1, Model: "unknown"}}, wantErr: vm.ErrInvalidNetworkModel},
		{name: testNonOwnerCase, interfaces: []cluster.NetworkInterface{{Index: 0, Bridge: testBridgeVMbr1, Model: testModelVirtio}}, actor: bobIdentity(), wantErr: vm.ErrForbidden},
	}
	for _, test := range tests { //nolint:paralleltest // serial: shared fake cluster dataset
		t.Run(test.name, func(t *testing.T) {
			cluster.ResetFake()

			actor := test.actor
			if actor.Username == "" {
				actor = aliceIdentity()
			}

			deps := networkDependencies(diskTestIndex(t, 101, cluster.VMRunning), actor, 101)

			result, err := vm.UpdateNetwork(context.Background(), deps, test.interfaces)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("err = %v, want %v", err, test.wantErr)
				}

				if calls := cluster.FakeCallsFor(101); len(calls) != 0 {
					t.Fatalf("fake calls = %+v, want none", calls)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != 1 || result[0].Bridge != testBridgeVMbr1 || result[0].MAC == "" {
				t.Fatalf("result = %+v, want updated bridge with server MAC", result)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateNetwork_RejectsTooManyCards(t *testing.T) {
	cluster.ResetFake()

	deps := networkDependencies(diskTestIndex(t, 101, cluster.VMRunning), aliceIdentity(), 101)

	interfaces := make([]cluster.NetworkInterface, policy.DefaultGabarit().MaxNetworkCards+1)
	for index := range interfaces {
		interfaces[index] = cluster.NetworkInterface{Index: index, Bridge: testBridgeVMbr0, Model: testModelVirtio}
	}

	_, err := vm.UpdateNetwork(context.Background(), deps, interfaces)
	if !errors.Is(err, vm.ErrNetworkCardsExceedLimit) {
		t.Fatalf("err = %v, want ErrNetworkCardsExceedLimit", err)
	}

	if calls := cluster.FakeCallsFor(101); len(calls) != 0 {
		t.Fatalf("fake calls = %+v, want none", calls)
	}
}

// TestUpdateNetwork_RemovesInterface covers removing one interface from a
// multi-interface VM: attach a second NIC, then submit a list that drops it.
//
//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateNetwork_RemovesInterface(t *testing.T) {
	cluster.ResetFake()

	actor := aliceIdentity()
	deps := networkDependencies(diskTestIndex(t, 101, cluster.VMRunning), actor, 101)

	twoNICs := []cluster.NetworkInterface{
		{Index: 0, Bridge: testBridgeVMbr0, Model: testModelVirtio},
		{Index: 1, Bridge: testBridgeVMbr1, Model: testModelVirtio},
	}
	if _, err := vm.UpdateNetwork(context.Background(), deps, twoNICs); err != nil {
		t.Fatalf("attach second NIC: %v", err)
	}

	deps = networkDependencies(diskTestIndex(t, 101, cluster.VMRunning), actor, 101)

	result, err := vm.UpdateNetwork(context.Background(), deps, []cluster.NetworkInterface{
		{Index: 0, Bridge: testBridgeVMbr0, Model: testModelVirtio},
	})
	if err != nil {
		t.Fatalf("remove NIC: %v", err)
	}

	if len(result) != 1 || result[0].Index != 0 {
		t.Fatalf("result = %+v, want only index 0 remaining", result)
	}
}

func networkDependencies(index *inventory.Index, actor auth.Identity, vmid int) vm.NetworkDependencies {
	return vm.NetworkDependencies{
		Index: index, Actor: actor, ClusterName: testClusterName, VMID: vmid, Writer: cluster.Fake{},
		Resources: catalog.Resources{Bridges: []string{testBridgeVMbr0, testBridgeVMbr1}}, Gabarit: policy.DefaultGabarit(), Audit: noopAudit{}, Refresher: noopRefresher{},
	}
}
