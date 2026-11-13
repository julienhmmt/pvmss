package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
)

// seedISOApproval marks the test ISO as approved on the fake cluster.
func seedISOApproval(t *testing.T, fixture createFixture) {
	t.Helper()

	if err := fixture.store.SetISOEnabled(context.Background(), testClusterName, cluster.FakeNode01, cluster.FakeStorageLocal, testISOFile, true); err != nil {
		t.Fatalf("seed iso approval: %v", err)
	}
}

// isoRequest builds the detailed create request carrying the test ISO and the
// given ostype value.
func isoRequest(ostype string) vm.CreateRequest {
	req := detailedRequest()
	req.OSType = ostype
	req.ISO = &vm.ISORequest{Storage: cluster.FakeStorageLocal, File: testISOFile}

	return req
}

// TestCreate_OSTypeISOPath_PassesThrough - an absent ostype defaults to l26
// and every allowed value reaches the cluster verbatim.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_OSTypeISOPath_PassesThrough(t *testing.T) {
	cases := []struct {
		name   string
		ostype string
		want   string
	}{
		{name: "absent defaults to l26", ostype: "", want: "l26"},
		{name: "win10 passes through", ostype: "win10", want: "win10"},
		{name: "win11 passes through", ostype: "win11", want: "win11"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newCreateFixture(t)
			seedISOApproval(t, fixture)

			result, err := fixture.create(t, aliceIdentity(), isoRequest(tc.ostype))
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			snap, err := fixture.fake.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("Snapshot: %v", err)
			}

			for _, v := range snap.VMs {
				if v.VMID == result.VMID && v.OSType != tc.want {
					t.Errorf("ostype = %q, want %q", v.OSType, tc.want)
				}
			}
		})
	}
}

// TestCreate_OSTypeISOPath_RejectsUnknown - an ostype outside the allowed set
// is rejected before any cluster call.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_OSTypeISOPath_RejectsUnknown(t *testing.T) {
	fixture := newCreateFixture(t)
	seedISOApproval(t, fixture)

	if _, err := fixture.create(t, aliceIdentity(), isoRequest("solaris")); !errors.Is(err, vm.ErrInvalidRequest) {
		t.Fatalf("error = %v, want %v", err, vm.ErrInvalidRequest)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
	}
}
