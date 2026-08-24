package policy_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"testing"
)

func TestSetGabarit_RejectsOutOfRangeValues(t *testing.T) {
	t.Parallel()
	service, _ := newPolicyService(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		gabarit policy.Gabarit
	}{
		{
			name:    "maxSockets too high",
			gabarit: policy.Gabarit{MaxSockets: 17, MaxCores: 1, MaxMemoryMB: 1, MaxDiskPerVMGB: 1, MaxNetworkCards: 1, MaxSnapshots: 1},
		},
		{
			name:    "maxCores too high",
			gabarit: policy.Gabarit{MaxSockets: 1, MaxCores: 129, MaxMemoryMB: 1, MaxDiskPerVMGB: 1, MaxNetworkCards: 1, MaxSnapshots: 1},
		},
		{
			name:    "maxMemoryMB too high",
			gabarit: policy.Gabarit{MaxSockets: 1, MaxCores: 1, MaxMemoryMB: 1048577, MaxDiskPerVMGB: 1, MaxNetworkCards: 1, MaxSnapshots: 1},
		},
		{
			name:    "maxDiskPerVMGB too high",
			gabarit: policy.Gabarit{MaxSockets: 1, MaxCores: 1, MaxMemoryMB: 1, MaxDiskPerVMGB: 1048577, MaxNetworkCards: 1, MaxSnapshots: 1},
		},
		{
			name:    "maxNetworkCards too high",
			gabarit: policy.Gabarit{MaxSockets: 1, MaxCores: 1, MaxMemoryMB: 1, MaxDiskPerVMGB: 1, MaxNetworkCards: 33, MaxSnapshots: 1},
		},
		{
			name:    "maxSnapshots too high",
			gabarit: policy.Gabarit{MaxSockets: 1, MaxCores: 1, MaxMemoryMB: 1, MaxDiskPerVMGB: 1, MaxNetworkCards: 1, MaxSnapshots: 1001},
		},
		{
			name:    "negative maxSockets",
			gabarit: policy.Gabarit{MaxSockets: -1, MaxCores: 1, MaxMemoryMB: 1, MaxDiskPerVMGB: 1, MaxNetworkCards: 1, MaxSnapshots: 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := service.SetGabarit(ctx, "default", tc.gabarit); !errors.Is(err, policy.ErrInvalidPolicy) {
				t.Fatalf("SetGabarit error = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}

func TestSetPolicy_RejectsOutOfRangeMaxVmPerUser(t *testing.T) {
	t.Parallel()
	service, _ := newPolicyService(t)
	ctx := context.Background()

	valid := policy.Gabarit{MaxSockets: 2, MaxCores: 4, MaxMemoryMB: 4096, MaxDiskPerVMGB: 100, MaxNetworkCards: 2, MaxSnapshots: 5}

	if err := service.SetPolicy(ctx, "default", valid, 100001); !errors.Is(err, policy.ErrInvalidPolicy) {
		t.Fatalf("SetPolicy error = %v, want ErrInvalidPolicy for maxVmPerUser too high", err)
	}

	if err := service.SetPolicy(ctx, "default", valid, -2); !errors.Is(err, policy.ErrInvalidPolicy) {
		t.Fatalf("SetPolicy error = %v, want ErrInvalidPolicy for maxVmPerUser below -1", err)
	}

	if err := service.SetPolicy(ctx, "default", valid, 100000); err != nil {
		t.Fatalf("SetPolicy with maxVmPerUser at upper limit should pass: %v", err)
	}
}

func TestSetGabarit_UpsertsAllFields(t *testing.T) {
	t.Parallel()
	service, _ := newPolicyService(t)
	ctx := context.Background()

	want := policy.Gabarit{MaxSockets: 2, MaxCores: 6, MaxMemoryMB: 8192, MaxDiskPerVMGB: 80, MaxNetworkCards: 2, MaxSnapshots: 3, AllowCustomYAML: false}
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
	t.Parallel()
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
