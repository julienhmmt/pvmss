package policy_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"testing"
)

func TestSetPolicy_RejectsOutOfRangeGabaritValues(t *testing.T) {
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

			if err := service.SetPolicy(ctx, "default", tc.gabarit, -1); !errors.Is(err, policy.ErrInvalidPolicy) {
				t.Fatalf("SetPolicy error = %v, want ErrInvalidPolicy", err)
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

func TestUpdatePolicy_AppliesMutationAndReportsBeforeAfter(t *testing.T) {
	t.Parallel()
	service, _ := newPolicyService(t)
	ctx := context.Background()

	update, err := service.UpdatePolicy(ctx, "default", func(current policy.Settings) policy.Settings {
		next := current
		next.Gabarit.MaxSockets = 3
		next.Allowed = 7

		return next
	})
	if err != nil {
		t.Fatalf("UpdatePolicy: %v", err)
	}

	if !update.Changed {
		t.Fatal("Changed = false, want true")
	}

	if update.Before.Gabarit.MaxSockets != 4 || update.Before.Allowed != -1 {
		t.Fatalf("Before = %+v, want seeded defaults", update.Before)
	}

	if update.After.Gabarit.MaxSockets != 3 || update.After.Allowed != 7 {
		t.Fatalf("After = %+v, want mutated values", update.After)
	}

	stored, err := service.Settings(ctx, "default")
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}

	if stored != update.After {
		t.Fatalf("stored = %+v, want %+v", stored, update.After)
	}
}

func TestUpdatePolicy_NoChangeSkipsWrite(t *testing.T) {
	t.Parallel()
	service, _ := newPolicyService(t)
	ctx := context.Background()

	update, err := service.UpdatePolicy(ctx, "default", func(current policy.Settings) policy.Settings {
		return current
	})
	if err != nil {
		t.Fatalf("UpdatePolicy: %v", err)
	}

	if update.Changed {
		t.Fatal("Changed = true for an identity mutation, want false")
	}
}

func TestUpdatePolicy_InvalidMutation_ReturnsErrInvalidPolicy(t *testing.T) {
	t.Parallel()
	service, _ := newPolicyService(t)
	ctx := context.Background()

	_, err := service.UpdatePolicy(ctx, "default", func(current policy.Settings) policy.Settings {
		next := current
		next.Allowed = -2

		return next
	})
	if !errors.Is(err, policy.ErrInvalidPolicy) {
		t.Fatalf("error = %v, want ErrInvalidPolicy", err)
	}
}

func TestUpdatePolicy_ConcurrentWriteRetriesWithoutLosingIt(t *testing.T) {
	t.Parallel()
	service, _ := newPolicyService(t)
	ctx := context.Background()

	calls := 0

	update, err := service.UpdatePolicy(ctx, "default", func(current policy.Settings) policy.Settings {
		calls++
		if calls == 1 {
			// Simulate a competing write landing between this UpdatePolicy's
			// read and its compare-and-swap: the first CAS must fail and the
			// retry must preserve the competing change.
			if err := service.SetPolicy(ctx, "default", policy.DefaultGabarit(), 42); err != nil {
				t.Fatalf("seed concurrent write: %v", err)
			}
		}

		next := current
		next.Gabarit.MaxSockets = 3

		return next
	})
	if err != nil {
		t.Fatalf("UpdatePolicy: %v", err)
	}

	if calls != 2 {
		t.Fatalf("mutate calls = %d, want 2 (one conflict, one retry)", calls)
	}

	if !update.Changed || update.After.Gabarit.MaxSockets != 3 {
		t.Fatalf("update = %+v, want a changed update with MaxSockets 3", update)
	}

	if update.After.Allowed != 42 {
		t.Fatalf("Allowed = %d, want 42 - the concurrent write was lost", update.After.Allowed)
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
