package policy_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"testing"
)

func TestSetGabarit_UpsertsAllFields(t *testing.T) {
	service, _ := newPolicyService(t)
	ctx := context.Background()
	want := policy.Gabarit{MaxSockets: 2, MaxCores: 6, MaxMemoryMB: 8192, MaxDiskPerVMGB: 80, MaxNetworkCards: 2, MaxSnapshots: 3, AllowCustomYaml: false}
	if err := service.SetGabarit(ctx, "default", want); err != nil {
		t.Fatalf("SetGabarit: %v", err)
	}
	got, err := service.Gabarit(ctx, "default")
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}
	if got != want {
		t.Fatalf("gabarit = %+v, want %+v", got, want)
	}
}

func TestSetNodeCapacity_RejectsUsagePhysicalAndAcceptsZero(t *testing.T) {
	service, _ := newPolicyService(t)
	ctx := context.Background()
	current, err := service.NodeCapacity(ctx, "default", cluster.FakeNode02)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}
	below := current
	below.MaxVCPUs = current.UsedVCPUs - 1
	below.MaxVCPUs = max(below.MaxVCPUs, 1)
	if err := service.SetNodeCapacity(ctx, "default", cluster.FakeNode02, below); !errors.Is(err, policy.ErrBelowCurrentUsage) {
		t.Fatalf("below-usage error = %v, want ErrBelowCurrentUsage", err)
	}
	above := current
	above.MaxVCPUs = current.PhysicalVCPUs + 1
	if err := service.SetNodeCapacity(ctx, "default", cluster.FakeNode02, above); !errors.Is(err, policy.ErrAboveNodeCapacity) {
		t.Fatalf("above-physical error = %v, want ErrAboveNodeCapacity", err)
	}
	if err := service.SetNodeCapacity(ctx, "default", cluster.FakeNode02, policy.Capacity{}); err != nil {
		t.Fatalf("zero capacity should be accepted: %v", err)
	}
}
