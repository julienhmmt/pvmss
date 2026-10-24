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
			runNetworkCase(t, test.interfaces, test.actor, test.wantErr)
		})
	}
}

// runNetworkCase runs a single UpdateNetwork case: builds the deps, invokes
// the update, and asserts the expected error or the resulting interface list.
// Extracted from TestUpdateNetwork_ValidatesBridgeAndCardCount to keep its
// Cognitive Complexity under the SonarQube go:S3776 threshold.
func runNetworkCase(t *testing.T, interfaces []cluster.NetworkInterface, actor auth.Identity, wantErr error) {
	t.Helper()

	cluster.ResetFake()

	if actor.Username == "" {
		actor = aliceIdentity()
	}

	deps := networkDependencies(diskTestIndex(t, 101, cluster.VMRunning), actor, 101)

	result, err := vm.UpdateNetwork(context.Background(), deps, interfaces)
	if wantErr != nil {
		if !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want %v", err, wantErr)
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

// TestUpdateNetwork_PreservesServerOwnedNICOptions covers a PUT that only
// carries bridge/model/vlan/rate: the firewall flag and MTU are not request
// fields, so they must be carried over from the live NIC, as the MAC is.
//
//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateNetwork_PreservesServerOwnedNICOptions(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	seeded := []cluster.NetworkInterface{{
		Index: 0, Bridge: testBridgeVMbr0, Model: testModelVirtio,
		Firewall: true, MTU: 9000,
	}}
	if err := (cluster.Fake{}).UpdateNetwork(context.Background(), cluster.FakeNode01, 101, seeded); err != nil {
		t.Fatalf("seed NIC: %v", err)
	}

	index := diskTestIndex(t, 101, cluster.VMRunning)

	entity, err := vm.Resolve(index, aliceIdentity(), testClusterName, 101)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	existingMAC := entity.NetworkInterfaces[0].MAC
	deps := networkDependencies(index, aliceIdentity(), 101)

	rate := 50

	result, err := vm.UpdateNetwork(context.Background(), deps, []cluster.NetworkInterface{{
		Index: 0, Bridge: testBridgeVMbr1, Model: testModelVirtio, RateMbps: &rate,
	}})
	if err != nil {
		t.Fatalf("UpdateNetwork: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("result = %+v, want 1 interface", result)
	}

	got := result[0]
	if !got.Firewall {
		t.Errorf("Firewall = false, want true (preserved from live NIC)")
	}

	if got.MTU != 9000 {
		t.Errorf("MTU = %d, want 9000 (preserved from live NIC)", got.MTU)
	}

	if got.MAC != existingMAC {
		t.Errorf("MAC = %q, want %q (preserved)", got.MAC, existingMAC)
	}
}

func networkDependencies(index *inventory.Index, actor auth.Identity, vmid int) vm.NetworkDependencies {
	return vm.NetworkDependencies{
		Index: index, Actor: actor, ClusterName: testClusterName, VMID: vmid, Writer: cluster.Fake{},
		Resources: catalog.Resources{Bridges: []catalog.Bridge{{Name: testBridgeVMbr0, Node: cluster.FakeNode01}, {Name: testBridgeVMbr1, Node: cluster.FakeNode01}}}, Gabarit: policy.DefaultGabarit(), Audit: noopAudit{}, Refresher: noopRefresher{},
	}
}

// TestUpdateNetwork_UserCannotMoveVLANOrRate: VLAN and rate limit isolate
// tenants, so a non-admin PUT keeps the NIC on the imposed isolation VLAN
// (or its current VLAN) and its current rate; an admin may change both.
//
//nolint:paralleltest // serial: shared fake cluster dataset
func TestUpdateNetwork_UserCannotMoveVLANOrRate(t *testing.T) {
	const (
		currentVLAN   = 100
		currentRate   = 10
		isolationVLAN = 42
		requestedVLAN = 7
	)

	cases := []struct {
		name      string
		actor     auth.Identity
		isolation int
		wantVLAN  int
		wantRate  *int
	}{
		{name: "user with isolation VLAN", actor: aliceIdentity(), isolation: isolationVLAN, wantVLAN: isolationVLAN, wantRate: new(int)},
		{name: "user without isolation VLAN", actor: aliceIdentity(), wantVLAN: currentVLAN, wantRate: new(int)},
		{name: "admin", actor: auth.Identity{Username: testAdminUser, IsAdmin: true}, isolation: isolationVLAN, wantVLAN: requestedVLAN},
	}
	for _, tc := range cases { //nolint:paralleltest // serial: shared fake cluster dataset
		t.Run(tc.name, func(t *testing.T) {
			cluster.ResetFake()
			t.Cleanup(cluster.ResetFake)

			vlan, rate := currentVLAN, currentRate

			seeded := []cluster.NetworkInterface{{Index: 0, Bridge: testBridgeVMbr0, Model: testModelVirtio, VLAN: &vlan, RateMbps: &rate}}
			if err := (cluster.Fake{}).UpdateNetwork(context.Background(), cluster.FakeNode01, 101, seeded); err != nil {
				t.Fatalf("seed NIC: %v", err)
			}

			deps := networkDependencies(diskTestIndex(t, 101, cluster.VMRunning), tc.actor, 101)
			deps.Gabarit.IsolationVLANTag = tc.isolation

			requested := requestedVLAN

			result, err := vm.UpdateNetwork(context.Background(), deps, []cluster.NetworkInterface{{
				Index: 0, Bridge: testBridgeVMbr0, Model: testModelVirtio, VLAN: &requested,
			}})
			if err != nil {
				t.Fatalf("UpdateNetwork: %v", err)
			}

			got := result[0]
			if got.VLAN == nil || *got.VLAN != tc.wantVLAN {
				t.Errorf("VLAN = %v, want %d", got.VLAN, tc.wantVLAN)
			}

			if tc.wantRate == nil {
				if got.RateMbps != nil {
					t.Errorf("RateMbps = %d, want nil (admin cleared it)", *got.RateMbps)
				}
			} else if got.RateMbps == nil || *got.RateMbps != currentRate {
				t.Errorf("RateMbps = %v, want %d (kept from live NIC)", got.RateMbps, currentRate)
			}
		})
	}
}
