package policy_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"testing"
)

func TestCheckQuota_UsesCurrentPoolAndAdminBypass(t *testing.T) {
	t.Parallel()
	service, projection := newPolicyService(t)
	ctx := context.Background()

	quota, err := service.Quota(ctx, "default", auth.Identity{Username: testUserAlice, Pool: cluster.FakePoolAlice})
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}

	if err := service.SetGabarit(ctx, "default", policy.Gabarit{}); err != nil {
		t.Fatalf("SetGabarit: %v", err)
	}

	if err := service.SetQuota(ctx, "default", quota.Used); err != nil {
		t.Fatalf("SetQuota: %v", err)
	}

	if err := service.CheckQuota(ctx, "default", auth.Identity{Username: "admin", IsAdmin: true}); err != nil {
		t.Fatalf("admin quota check: %v", err)
	}

	if err := service.CheckQuota(ctx, "default", auth.Identity{Username: testUserAlice, Pool: cluster.FakePoolAlice}); !errors.Is(err, policy.ErrQuotaExceeded) {
		t.Fatalf("quota check error = %v, want ErrQuotaExceeded", err)
	}

	if len(projection.Load().ByPool[cluster.FakePoolAlice]) == 0 {
		t.Fatal("fixture must provide an owned VM")
	}
}

func TestCheckGabarit_ReportsFirstOffendingField(t *testing.T) {
	t.Parallel()
	service, _ := newPolicyService(t)
	ctx := context.Background()

	cases := []struct {
		name                                   string
		value                                  policy.Gabarit
		sockets, cores, memoryMB, diskGB, nics int
	}{
		{name: "sockets", value: policy.Gabarit{MaxSockets: 1}, sockets: 2, cores: 1, memoryMB: 128, diskGB: 1, nics: 1},
		{name: "cores", value: policy.Gabarit{MaxCores: 1}, sockets: 1, cores: 2, memoryMB: 128, diskGB: 1, nics: 1},
		{name: "memory", value: policy.Gabarit{MaxMemoryMB: 128}, sockets: 1, cores: 1, memoryMB: 256, diskGB: 1, nics: 1},
		{name: "disk", value: policy.Gabarit{MaxDiskPerVMGB: 1}, sockets: 1, cores: 1, memoryMB: 128, diskGB: 2, nics: 1},
		{name: "network", value: policy.Gabarit{MaxNetworkCards: 1}, sockets: 1, cores: 1, memoryMB: 128, diskGB: 1, nics: 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// One cluster name per case: the parallel subtests share the
			// store, and a zero field is now uncapped - a clobbered write can
			// no longer fail closed by accident.
			clusterName := "ghost-" + testCase.name
			if err := service.SetGabarit(ctx, clusterName, testCase.value); err != nil {
				t.Fatalf("SetGabarit: %v", err)
			}

			if err := service.CheckGabarit(ctx, clusterName, testCase.sockets, testCase.cores, testCase.memoryMB, testCase.diskGB, testCase.nics); !errors.Is(err, policy.ErrGabaritExceeded) {
				t.Fatalf("CheckGabarit error = %v, want ErrGabaritExceeded", err)
			}
		})
	}
}

func TestCheckGabarit_ZeroFieldsAreUncapped(t *testing.T) {
	t.Parallel()
	service, _ := newPolicyService(t)
	ctx := context.Background()

	if err := service.SetGabarit(ctx, "ghost-uncapped", policy.Gabarit{}); err != nil {
		t.Fatalf("SetGabarit: %v", err)
	}

	if err := service.CheckGabarit(ctx, "ghost-uncapped", 16, 64, 1<<20, 4096, 8); err != nil {
		t.Fatalf("CheckGabarit with all-zero gabarit = %v, want nil", err)
	}
}

func TestCheckNodeCapacity_ExcludesVMAndRejectsAdditionalUsage(t *testing.T) {
	t.Parallel()
	service, projection := newPolicyService(t)
	ctx := context.Background()

	machine, ok := projection.Load().ByNode[cluster.FakeNode02][0], true
	if !ok {
		t.Fatal("fixture must provide a VM on the first node")
	}

	current, err := service.NodeCapacity(ctx, "default", machine.Node)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	current.MaxVCPUs = current.UsedVCPUs

	current.MaxRAMGB = current.UsedRAMGB
	if err := service.SetNodeCapacity(ctx, "default", machine.Node, current); err != nil {
		t.Fatalf("SetNodeCapacity: %v", err)
	}

	if err := service.CheckNodeCapacity(ctx, "default", machine.Node, policy.CapacityDelta{Sockets: machine.Sockets, Cores: machine.Cores, MemoryMB: int(machine.MemoryTotal / (1024 * 1024)), ExcludeVMID: machine.VMID}); err != nil {
		t.Fatalf("same VM footprint should fit after exclusion: %v", err)
	}

	if err := service.CheckNodeCapacity(ctx, "default", machine.Node, policy.CapacityDelta{Sockets: 1, Cores: 1, MemoryMB: 1}); !errors.Is(err, policy.ErrNodeCapacityExceeded) {
		t.Fatalf("additional capacity error = %v, want ErrNodeCapacityExceeded", err)
	}
}
