package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
)

// TestCreate_OSTypeISOPath covers the ostype field on the ISO source: absent
// means l26, an allowed value reaches the cluster, anything else is rejected
// before any cluster call.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_OSTypeISOPath(t *testing.T) {
	cases := []struct {
		name    string
		ostype  string
		want    string
		wantErr error
	}{
		{name: "absent defaults to l26", ostype: "", want: "l26"},
		{name: "win10 passes through", ostype: "win10", want: "win10"},
		{name: "win11 passes through", ostype: "win11", want: "win11"},
		{name: "unknown value rejected", ostype: "solaris", wantErr: vm.ErrInvalidRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newCreateFixture(t)
			if err := fixture.store.SetISOEnabled(context.Background(), testClusterName, cluster.FakeNode01, cluster.FakeStorageLocal, testISOFile, true); err != nil {
				t.Fatalf("seed iso approval: %v", err)
			}

			req := detailedRequest()
			req.OSType = tc.ostype
			req.ISO = &vm.ISORequest{Storage: cluster.FakeStorageLocal, File: testISOFile}

			result, err := fixture.create(t, aliceIdentity(), req)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}

				if calls := cluster.FakeCalls(); len(calls) != 0 {
					t.Fatalf("rejected request reached the cluster: %+v", calls)
				}

				return
			}

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
