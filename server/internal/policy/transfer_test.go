package policy_test

import (
	"context"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"slices"
	"testing"
)

const transferCluster = "default"

func TestImportPolicy_PerClusterOutcomes(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()
	wanted := policy.Settings{
		Gabarit: policy.Gabarit{MaxSockets: 2, MaxCores: 8, MaxMemoryMB: 16384, MaxDiskPerVMGB: 200, MaxNetworkCards: 2, MaxSnapshots: 4},
		Allowed: 7,
	}

	out, err := service.ImportPolicy(ctx, []string{transferCluster, "other"}, wanted)
	if err != nil {
		t.Fatalf("ImportPolicy: %v", err)
	}

	if got, want := importResults(out), []string{"default:updated", "other:updated"}; !slices.Equal(got, want) {
		t.Fatalf("outcomes = %v, want %v", got, want)
	}

	for _, clusterName := range []string{transferCluster, "other"} {
		stored, err := service.Settings(ctx, clusterName)
		if err != nil {
			t.Fatalf("Settings(%s): %v", clusterName, err)
		}

		if stored != wanted {
			t.Errorf("stored policy on %s = %+v, want %+v", clusterName, stored, wanted)
		}
	}

	out, err = service.ImportPolicy(ctx, []string{transferCluster}, wanted)
	if err != nil || out[0].Result != policy.ImportUnchanged {
		t.Fatalf("second import = %v, %v; want unchanged", importResults(out), err)
	}

	broken := policy.Settings{Gabarit: policy.Gabarit{MaxSockets: -1}, Allowed: 7}

	out, err = service.ImportPolicy(ctx, []string{transferCluster, "other"}, broken)
	if err != nil {
		t.Fatalf("ImportPolicy invalid: %v", err)
	}

	if got, want := importResults(out), []string{"default:invalid", "other:invalid"}; !slices.Equal(got, want) {
		t.Fatalf("invalid outcomes = %v, want %v", got, want)
	}

	for _, outcome := range out {
		if outcome.Error == "" {
			t.Errorf("invalid outcome for %s carries no message", outcome.Cluster)
		}
	}

	if stored, _ := service.Settings(ctx, transferCluster); stored != wanted {
		t.Errorf("invalid import changed the stored policy: %+v", stored)
	}
}

func TestImportNodeCapacities_Outcomes(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()
	// pve-node-03 hosts one small pvmss VM, so caps just above its usage pass
	// both the below-usage and the physical checks.
	capped := policy.Capacity{MaxVMs: 3, MaxVCPUs: 4, MaxRAMGB: 6, MaxDiskGB: 2048}

	items := []policy.NodeCapacityImport{
		{Node: cluster.FakeNode03, Capacity: capped},
		{Node: "ghost-node", Capacity: policy.Capacity{MaxVMs: 1}},
		{Node: "ghost-node-zero", Capacity: policy.Capacity{}},
		{Node: cluster.FakeNode02, Capacity: policy.Capacity{MaxVCPUs: 9999}},
	}

	out, err := service.ImportNodeCapacities(ctx, transferCluster, items, false)
	if err != nil {
		t.Fatalf("ImportNodeCapacities: %v", err)
	}

	want := []string{
		cluster.FakeNode03 + ":updated",
		"ghost-node:" + policy.ImportNotFound,
		"ghost-node-zero:" + policy.ImportNotFound,
		cluster.FakeNode02 + ":" + policy.ImportInvalid,
	}
	if got := importResults(out); !slices.Equal(got, want) {
		t.Fatalf("outcomes = %v, want %v", got, want)
	}

	stored, err := service.NodeCapacity(ctx, transferCluster, cluster.FakeNode03)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	if stored.MaxVMs != capped.MaxVMs || stored.MaxRAMGB != capped.MaxRAMGB {
		t.Errorf("stored caps = %+v, want %+v", stored, capped)
	}

	out, err = service.ImportNodeCapacities(ctx, transferCluster, items[:1], false)
	if err != nil || out[0].Result != policy.ImportSkipped {
		t.Fatalf("import over caps = %v, %v; want skipped", importResults(out), err)
	}

	out, err = service.ImportNodeCapacities(ctx, transferCluster, items[:1], true)
	if err != nil || out[0].Result != policy.ImportUnchanged {
		t.Fatalf("overwrite with same caps = %v, %v; want unchanged", importResults(out), err)
	}

	raised := []policy.NodeCapacityImport{{Node: cluster.FakeNode03, Capacity: policy.Capacity{MaxVMs: 5, MaxVCPUs: 4, MaxRAMGB: 6, MaxDiskGB: 2048}}}

	out, err = service.ImportNodeCapacities(ctx, transferCluster, raised, true)
	if err != nil || out[0].Result != policy.ImportUpdated {
		t.Fatalf("overwrite with new caps = %v, %v; want updated", importResults(out), err)
	}

	if stored, _ := service.NodeCapacity(ctx, transferCluster, cluster.FakeNode03); stored.MaxVMs != 5 {
		t.Errorf("maxVms after overwrite = %d, want 5", stored.MaxVMs)
	}
}

func importResults(outcomes []policy.ImportOutcome) []string {
	results := make([]string, len(outcomes))
	for i, outcome := range outcomes {
		subject := outcome.Cluster
		if outcome.Node != "" {
			subject = outcome.Node
		}

		results[i] = subject + ":" + outcome.Result
	}

	return results
}
