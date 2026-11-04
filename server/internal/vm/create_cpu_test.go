package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
)

// cpuFlagsCreator wraps the fake Creator: it can fail the host-flags read and
// records the CPU model the create dispatched.
type cpuFlagsCreator struct {
	cluster.Fake
	flagsErr error
	gotCPU   string
}

func (c *cpuFlagsCreator) HostCPUFlags(ctx context.Context) ([][]string, error) {
	if c.flagsErr != nil {
		return nil, c.flagsErr
	}

	return c.Fake.HostCPUFlags(ctx)
}

func (c *cpuFlagsCreator) CreateVM(ctx context.Context, spec cluster.VMSpec) (string, error) {
	c.gotCPU = spec.CPUType

	return c.Fake.CreateVM(ctx, spec)
}

// TestCreate_CPUTypeFromHostFlags covers the create path's CPU resolution:
// flags that cannot be read fall back to v2-AES without failing the create,
// and readable flags pick the model the nodes support.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_CPUTypeFromHostFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagsErr error
		want     string
	}{
		{"unreadable flags fall back and the create proceeds", errors.New("403 forbidden"), cluster.CPUTypeV2AES},
		{"readable flags pick the richest common model", nil, cluster.CPUTypeV3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newCreateFixture(t)
			creator := &cpuFlagsCreator{Fake: fixture.fake, flagsErr: tt.flagsErr}

			result, err := vm.Create(context.Background(), aliceIdentity(), testClusterName, vm.CreateRequest{
				Cluster:   testClusterName,
				Name:      "cpu-vm",
				ProfileID: "small",
			}, vm.CreateDeps{
				Store:     fixture.store,
				Creator:   creator,
				Writer:    fixture.fake,
				FreeSpace: fixture.fake,
				Audit:     fixture.store,
				Log:       slog.New(slog.DiscardHandler),
				Policy:    policy.New(fixture.store, nil),
			})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			if result.UPID == "" {
				t.Error("Create returned an empty UPID")
			}

			if creator.gotCPU != tt.want {
				t.Errorf("dispatched cpu = %q, want %q", creator.gotCPU, tt.want)
			}
		})
	}
}
