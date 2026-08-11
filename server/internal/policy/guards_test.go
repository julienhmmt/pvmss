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
	service, projection := newPolicyService(t)
	ctx := context.Background()
	quota, err := service.Quota(ctx, "default", auth.Identity{Username: "alice", Pool: cluster.FakePoolAlice})
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
	if err := service.CheckQuota(ctx, "default", auth.Identity{Username: "alice", Pool: cluster.FakePoolAlice}); !errors.Is(err, policy.ErrQuotaExceeded) {
		t.Fatalf("quota check error = %v, want ErrQuotaExceeded", err)
	}
	if len(projection.Load().ByPool[cluster.FakePoolAlice]) == 0 {
		t.Fatal("fixture must provide an owned VM")
	}
}

func TestCheckGabarit_ReportsFirstOffendingField(t *testing.T) {
	service, _ := newPolicyService(t)
	ctx := context.Background()
	cases := []struct {
		name  string
		value policy.Gabarit
		check func() error
	}{
		{name: "sockets", value: policy.Gabarit{MaxSockets: 1}, check: func() error { return service.CheckGabarit(ctx, "default", 2, 1, 128, 1, 1) }},
		{name: "cores", value: policy.Gabarit{MaxCores: 1}, check: func() error { return service.CheckGabarit(ctx, "default", 1, 2, 128, 1, 1) }},
		{name: "memory", value: policy.Gabarit{MaxMemoryMB: 128}, check: func() error { return service.CheckGabarit(ctx, "default", 1, 1, 256, 1, 1) }},
		{name: "disk", value: policy.Gabarit{MaxDiskPerVMGB: 1}, check: func() error { return service.CheckGabarit(ctx, "default", 1, 1, 128, 2, 1) }},
		{name: "network", value: policy.Gabarit{MaxNetworkCards: 1}, check: func() error { return service.CheckGabarit(ctx, "default", 1, 1, 128, 1, 2) }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := service.SetGabarit(ctx, "default", testCase.value); err != nil {
				t.Fatalf("SetGabarit: %v", err)
			}
			if err := testCase.check(); !errors.Is(err, policy.ErrGabaritExceeded) {
				t.Fatalf("CheckGabarit error = %v, want ErrGabaritExceeded", err)
			}
		})
	}
}

func TestCheckNodeCapacity_ExcludesVMAndRejectsAdditionalUsage(t *testing.T) {
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
	if err := service.CheckNodeCapacity(ctx, "default", machine.Node, machine.Sockets, machine.Cores, int(machine.MemoryTotal/(1024*1024)), machine.VMID); err != nil {
		t.Fatalf("same VM footprint should fit after exclusion: %v", err)
	}
	if err := service.CheckNodeCapacity(ctx, "default", machine.Node, 1, 1, 1, 0); !errors.Is(err, policy.ErrNodeCapacityExceeded) {
		t.Fatalf("additional capacity error = %v, want ErrNodeCapacityExceeded", err)
	}
}
