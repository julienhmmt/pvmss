package policy_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"testing"
)

func TestQuota_PoolOverrideReplacesGlobalAllowance(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		override *int
		want     int
	}{
		{name: "no override uses global", override: nil, want: 10},
		{name: "lower override wins", override: new(3), want: 3},
		{name: "higher override wins", override: new(50), want: 50},
		{name: "unlimited override wins", override: new(-1), want: -1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assertQuotaWithOverride(t, testCase.override, testCase.want)
		})
	}
}

func assertQuotaWithOverride(t *testing.T, override *int, want int) {
	t.Helper()

	service, _ := newPolicyService(t)
	ctx := context.Background()
	actor := auth.Identity{Username: testUserAlice, Pool: cluster.FakePoolAlice}

	if err := service.SetPolicy(ctx, "default", policy.Gabarit{}, 10); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	if override != nil {
		if err := service.SetPoolQuotaOverride(ctx, "default", cluster.FakePoolAlice, *override, "admin"); err != nil {
			t.Fatalf("SetPoolQuotaOverride: %v", err)
		}
	}

	quota, err := service.Quota(ctx, "default", actor)
	if err != nil || quota.Allowed != want {
		t.Fatalf("Quota.Allowed = %d, err %v; want %d", quota.Allowed, err, want)
	}

	settings, err := service.Settings(ctx, "default")
	if err != nil || settings.Allowed != 10 {
		t.Fatalf("global Settings.Allowed = %d, err %v; want untouched 10", settings.Allowed, err)
	}
}

func TestSetPoolQuotaOverride_RejectsOutOfRange(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	for _, value := range []int{-2, policy.MaxVMPerUserLimit + 1} {
		if err := service.SetPoolQuotaOverride(ctx, "default", cluster.FakePoolAlice, value, "admin"); !errors.Is(err, policy.ErrInvalidPolicy) {
			t.Fatalf("SetPoolQuotaOverride(%d) err = %v, want ErrInvalidPolicy", value, err)
		}
	}
}
