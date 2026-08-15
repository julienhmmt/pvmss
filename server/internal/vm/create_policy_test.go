package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
)

func TestCreate_PolicyGuardsRejectBeforeAllocation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		prepare func(*policy.Policy, context.Context) error
		wantErr error
	}{
		{
			name: "quota",
			prepare: func(service *policy.Policy, ctx context.Context) error {
				return service.SetQuota(ctx, testClusterName, 0)
			},
			wantErr: policy.ErrQuotaExceeded,
		},
		{
			name: "gabarit disk",
			prepare: func(service *policy.Policy, ctx context.Context) error {
				gabarit, err := service.Gabarit(ctx, testClusterName)
				if err != nil {
					return err
				}

				gabarit.MaxDiskPerVMGB = 10

				return service.SetGabarit(ctx, testClusterName, gabarit)
			},
			wantErr: policy.ErrGabaritExceeded,
		},
		{
			name: "node capacity",
			prepare: func(service *policy.Policy, ctx context.Context) error {
				capacity, err := service.NodeCapacity(ctx, testClusterName, cluster.FakeNode02)
				if err != nil {
					return err
				}

				capacity.MaxVCPUs = capacity.UsedVCPUs

				return service.SetNodeCapacity(ctx, testClusterName, cluster.FakeNode02, capacity)
			},
			wantErr: policy.ErrNodeCapacityExceeded,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			runPolicyCase(t, testCase.name, testCase.prepare, testCase.wantErr)
		})
	}
}

// runPolicyCase builds a fixture, applies the policy preparation, issues a
// Create request, and asserts that the expected policy error is returned
// before any cluster call is made. Extracted from
// TestCreate_PolicyGuardsRejectBeforeAllocation to keep its Cognitive
// Complexity under the SonarQube go:S3776 threshold.
func runPolicyCase(
	t *testing.T,
	name string,
	prepare func(*policy.Policy, context.Context) error,
	wantErr error,
) {
	t.Helper()

	fixture := newCreateFixture(t)

	snapshot, err := fixture.fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)

	service := policy.New(fixture.store, inventory.NewProjectionFromIndex(&index), fixture.fake)
	if err := prepare(service, context.Background()); err != nil {
		t.Fatalf("prepare policy: %v", err)
	}

	req := detailedRequest()

	req.Name = "policy-" + strings.ReplaceAll(name, " ", "-")
	if name == "gabarit disk" {
		req.Disk.SizeGB = 20
	}

	if name == "node capacity" {
		req.Node = cluster.FakeNode02
		req.Disk.Storage = "ceph-data"
		req.CPUCores = 1
		req.MemoryMB = 128
	}

	_, err = vm.Create(context.Background(), aliceIdentity(), req.Cluster, req, vm.CreateDeps{
		Store:    fixture.store,
		Creator:  fixture.fake,
		Pusher:   fixture.fake,
		Audit:    fixture.store,
		Log:      slog.New(slog.DiscardHandler),
		Services: []*policy.Policy{service},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Create error = %v, want %v", err, wantErr)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("policy rejection reached cluster: %+v", calls)
	}
}
