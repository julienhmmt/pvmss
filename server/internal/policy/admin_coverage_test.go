package policy_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

// newPolicyServiceWithProjectionOnly builds a Policy backed by a store and
// projection but a nil cluster client, exercising the projection branch of
// discoveredNode.
func newPolicyServiceWithProjectionOnly(t *testing.T, projection *inventory.Projection) *policy.Policy {
	t.Helper()

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "policy-proj.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return policy.New(st, projection, nil)
}

// --- SetPolicy ---

func TestSetPolicy_ValidGabaritAndQuota_PersistsAllFields(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	gabarit := policy.Gabarit{
		MaxSockets: 2, MaxCores: 6, MaxMemoryMB: 8192,
		MaxDiskPerVMGB: 80, MaxNetworkCards: 2, MaxSnapshots: 3,
		AllowCustomYAML: false,
	}
	if err := service.SetPolicy(ctx, "default", gabarit, 7); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	got, err := service.Gabarit(ctx, "default")
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}

	if got != gabarit {
		t.Fatalf("gabarit = %+v, want %+v", got, gabarit)
	}

	quota, err := service.Quota(ctx, "default", auth.Identity{Username: testUserAlice, Pool: cluster.FakePoolAlice})
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}

	if quota.Allowed != 7 {
		t.Fatalf("quota.Allowed = %d, want 7", quota.Allowed)
	}
}

func TestSetPolicy_UnlimitedQuota_Persists(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	gabarit := policy.Gabarit{MaxSockets: 1, MaxCores: 2, MaxMemoryMB: 4096}
	if err := service.SetPolicy(ctx, "default", gabarit, -1); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	quota, err := service.Quota(ctx, "default", auth.Identity{Username: testUserAlice, Pool: cluster.FakePoolAlice})
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}

	if quota.Allowed != -1 {
		t.Fatalf("quota.Allowed = %d, want -1", quota.Allowed)
	}
}

func TestSetPolicy_InvalidGabarit_ReturnsErrInvalidPolicy(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		gabarit policy.Gabarit
	}{
		{"negative sockets", policy.Gabarit{MaxSockets: -1}},
		{"negative cores", policy.Gabarit{MaxCores: -1}},
		{"negative memory", policy.Gabarit{MaxMemoryMB: -1}},
		{"negative disk", policy.Gabarit{MaxDiskPerVMGB: -1}},
		{"negative network cards", policy.Gabarit{MaxNetworkCards: -1}},
		{"negative snapshots", policy.Gabarit{MaxSnapshots: -1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := service.SetPolicy(ctx, "default", tc.gabarit, 5)
			if !errors.Is(err, policy.ErrInvalidPolicy) {
				t.Fatalf("error = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}

func TestSetPolicy_InvalidQuota_ReturnsErrInvalidPolicy(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	err := service.SetPolicy(ctx, "default", policy.Gabarit{}, -2)
	if !errors.Is(err, policy.ErrInvalidPolicy) {
		t.Fatalf("error = %v, want ErrInvalidPolicy", err)
	}
}

// --- SetQuota ---

func TestSetQuota_ValidValue_Persists(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	if err := service.SetQuota(ctx, "default", 10); err != nil {
		t.Fatalf("SetQuota: %v", err)
	}

	quota, err := service.Quota(ctx, "default", auth.Identity{Username: testUserAlice, Pool: cluster.FakePoolAlice})
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}

	if quota.Allowed != 10 {
		t.Fatalf("quota.Allowed = %d, want 10", quota.Allowed)
	}
}

func TestSetQuota_Unlimited_Persists(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	if err := service.SetQuota(ctx, "default", -1); err != nil {
		t.Fatalf("SetQuota: %v", err)
	}

	quota, err := service.Quota(ctx, "default", auth.Identity{Username: testUserAlice, Pool: cluster.FakePoolAlice})
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}

	if quota.Allowed != -1 {
		t.Fatalf("quota.Allowed = %d, want -1", quota.Allowed)
	}
}

func TestSetQuota_NegativeValue_ReturnsErrInvalidPolicy(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	err := service.SetQuota(ctx, "default", -2)
	if !errors.Is(err, policy.ErrInvalidPolicy) {
		t.Fatalf("error = %v, want ErrInvalidPolicy", err)
	}
}

// --- SetNodeCapacity ---

func TestSetNodeCapacity_InvalidCapacity_ReturnsErrInvalidPolicy(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	cases := []struct {
		name     string
		capacity policy.Capacity
	}{
		{"negative maxVMs", policy.Capacity{MaxVMs: -1}},
		{"negative maxVCPUs", policy.Capacity{MaxVCPUs: -1}},
		{"negative maxRAMGB", policy.Capacity{MaxRAMGB: -1}},
		{"negative maxDiskGB", policy.Capacity{MaxDiskGB: -1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := service.SetNodeCapacity(ctx, "default", cluster.FakeNode01, tc.capacity)
			if !errors.Is(err, policy.ErrInvalidPolicy) {
				t.Fatalf("error = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}

func TestSetNodeCapacity_UnknownNode_ReturnsErrNotFound(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	err := service.SetNodeCapacity(ctx, "default", "nonexistent-node", policy.Capacity{MaxVMs: 5})
	if !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("error = %v, want cluster.ErrNotFound", err)
	}
}

func TestSetNodeCapacity_BelowVMUsage_ReturnsErrBelowCurrentUsage(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	current, err := service.NodeCapacity(ctx, "default", cluster.FakeNode01)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	below := policy.Capacity{MaxVMs: max(current.UsedVMs-1, 1)}

	err = service.SetNodeCapacity(ctx, "default", cluster.FakeNode01, below)
	if !errors.Is(err, policy.ErrBelowCurrentUsage) {
		t.Fatalf("error = %v, want ErrBelowCurrentUsage", err)
	}
}

func TestSetNodeCapacity_BelowRAMUsage_ReturnsErrBelowCurrentUsage(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	current, err := service.NodeCapacity(ctx, "default", cluster.FakeNode01)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	below := policy.Capacity{MaxRAMGB: max(current.UsedRAMGB-1, 1)}

	err = service.SetNodeCapacity(ctx, "default", cluster.FakeNode01, below)
	if !errors.Is(err, policy.ErrBelowCurrentUsage) {
		t.Fatalf("error = %v, want ErrBelowCurrentUsage", err)
	}
}

func TestSetNodeCapacity_AbovePhysicalRAM_ReturnsErrAboveNodeCapacity(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	current, err := service.NodeCapacity(ctx, "default", cluster.FakeNode01)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	above := policy.Capacity{MaxRAMGB: current.PhysicalRAMGB + 1}

	err = service.SetNodeCapacity(ctx, "default", cluster.FakeNode01, above)
	if !errors.Is(err, policy.ErrAboveNodeCapacity) {
		t.Fatalf("error = %v, want ErrAboveNodeCapacity", err)
	}
}

func TestSetNodeCapacity_ValidCapacity_Persists(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	// FakeNode03 is offline with zero pvmss-tagged VMs, so any non-negative
	// capacity passes the below-usage check. Its physical limits are 16
	// vCPUs and 64 GB RAM, so we stay within them.
	capacity := policy.Capacity{MaxVMs: 10, MaxVCPUs: 16, MaxRAMGB: 64, MaxDiskGB: 500}
	if err := service.SetNodeCapacity(ctx, "default", cluster.FakeNode03, capacity); err != nil {
		t.Fatalf("SetNodeCapacity: %v", err)
	}

	got, err := service.NodeCapacity(ctx, "default", cluster.FakeNode03)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	if got.MaxVMs != 10 || got.MaxVCPUs != 16 || got.MaxRAMGB != 64 || got.MaxDiskGB != 500 {
		t.Fatalf("capacity = %+v, want %+v", got, capacity)
	}
}

func TestSetNodeCapacity_ZeroCapacityAlwaysAccepted(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	for _, node := range []string{cluster.FakeNode01, cluster.FakeNode02, cluster.FakeNode03} {
		t.Run(node, func(t *testing.T) {
			t.Parallel()

			if err := service.SetNodeCapacity(ctx, "default", node, policy.Capacity{}); err != nil {
				t.Fatalf("zero capacity on %q: %v", node, err)
			}
		})
	}
}

// --- discoveredNode via client snapshot error ---

func TestSetNodeCapacity_ClientSnapshotError_ReturnsWrappedError(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyServiceWithClient(t, failingSnapshotClient{})
	ctx := context.Background()

	err := service.SetNodeCapacity(ctx, "default", cluster.FakeNode01, policy.Capacity{MaxVMs: 5})
	if err == nil {
		t.Fatal("expected error from failing snapshot client, got nil")
	}
}

// --- discoveredNode via projection (no client) ---

func TestSetNodeCapacity_ProjectionOnly_FindsNode(t *testing.T) {
	t.Parallel()

	_, projection := newPolicyService(t)
	ctx := context.Background()

	serviceProjOnly := newPolicyServiceWithProjectionOnly(t, projection)

	// FakeNode03 is offline with zero usage, so any capacity within its
	// physical limits (16 vCPUs, 64 GB RAM) passes both checks.
	capacity := policy.Capacity{MaxVMs: 5, MaxVCPUs: 8, MaxRAMGB: 32}
	if err := serviceProjOnly.SetNodeCapacity(ctx, "default", cluster.FakeNode03, capacity); err != nil {
		t.Fatalf("SetNodeCapacity via projection: %v", err)
	}
}

func TestSetNodeCapacity_ProjectionOnly_UnknownNodeReturnsErrNotFound(t *testing.T) {
	t.Parallel()

	_, projection := newPolicyService(t)
	ctx := context.Background()

	serviceProjOnly := newPolicyServiceWithProjectionOnly(t, projection)

	err := serviceProjOnly.SetNodeCapacity(ctx, "default", "nonexistent", policy.Capacity{MaxVMs: 5})
	if !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("error = %v, want cluster.ErrNotFound", err)
	}
}

// --- discoveredNode with nil client and nil projection ---

func TestSetNodeCapacity_NoClientNoProjection_ReturnsErrNotFound(t *testing.T) {
	t.Parallel()

	service := newPolicyServiceNoClientNoProjection(t)
	ctx := context.Background()

	err := service.SetNodeCapacity(ctx, "default", cluster.FakeNode01, policy.Capacity{MaxVMs: 5})
	if !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("error = %v, want cluster.ErrNotFound", err)
	}
}

// --- Error type Error() methods ---

func TestBelowCurrentUsageError_Error_VCPUsDimension(t *testing.T) {
	t.Parallel()

	err := &policy.BelowCurrentUsageError{
		Node: "pve-node-01", Dimension: "vcpus", Requested: 4, Used: 8,
	}

	msg := err.Error()
	if msg == "" {
		t.Fatal("Error() returned empty string")
	}
	// dimensionVCPUs ("vcpus") is rewritten to dimensionVCPU ("vcpu") in the message.
	if !strings.Contains(msg, "vcpu") {
		t.Errorf("Error() = %q, want it to contain 'vcpu' (rewritten from vcpus)", msg)
	}
}

func TestBelowCurrentUsageError_Error_VMsDimension(t *testing.T) {
	t.Parallel()

	err := &policy.BelowCurrentUsageError{
		Node: "pve-node-02", Dimension: "vms", Requested: 2, Used: 5,
	}

	msg := err.Error()
	if msg == "" {
		t.Fatal("Error() returned empty string")
	}

	if !strings.Contains(msg, "vms") {
		t.Errorf("Error() = %q, want it to contain 'vms'", msg)
	}
}

func TestBelowCurrentUsageError_Unwrap(t *testing.T) {
	t.Parallel()

	err := &policy.BelowCurrentUsageError{Node: "n", Dimension: "vms"}
	if !errors.Is(err, policy.ErrBelowCurrentUsage) {
		t.Fatalf("errors.Is(ErrBelowCurrentUsage) = false, want true")
	}
}

func TestAboveNodeCapacityError_Error_VCPUDimension(t *testing.T) {
	t.Parallel()

	err := &policy.AboveNodeCapacityError{
		Node: "pve-node-01", Dimension: "vcpu", Requested: 64, Physical: 32,
	}

	msg := err.Error()
	if msg == "" {
		t.Fatal("Error() returned empty string")
	}
	// vcpu dimension has no unit suffix.
	if strings.Contains(msg, " GB") {
		t.Errorf("Error() = %q, vcpu dimension should not have GB unit", msg)
	}
}

func TestAboveNodeCapacityError_Error_RAMDimension(t *testing.T) {
	t.Parallel()

	err := &policy.AboveNodeCapacityError{
		Node: "pve-node-01", Dimension: "ram", Requested: 128, Physical: 64,
	}

	msg := err.Error()
	if msg == "" {
		t.Fatal("Error() returned empty string")
	}
	// ram dimension includes " GB" unit suffix.
	if !strings.Contains(msg, " GB") {
		t.Errorf("Error() = %q, want it to contain ' GB' unit suffix", msg)
	}
}

func TestAboveNodeCapacityError_Unwrap(t *testing.T) {
	t.Parallel()

	err := &policy.AboveNodeCapacityError{Node: "n", Dimension: "ram"}
	if !errors.Is(err, policy.ErrAboveNodeCapacity) {
		t.Fatalf("errors.Is(ErrAboveNodeCapacity) = false, want true")
	}
}
