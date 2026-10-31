package policy_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"testing"
)

// TestMultiCluster_ProjectionPerCluster is the regression for the audit
// finding: policy used to read the alphabetically-first cluster's projection
// for every cluster, so a secondary cluster's quota and name checks counted
// the default cluster's VMs.
func TestMultiCluster_ProjectionPerCluster(t *testing.T) {
	t.Parallel()

	const (
		secondary = "secondary"
		pool      = "pvmss-p"
	)

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "policy.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	poolVMs := func(count int) []cluster.VM {
		vms := make([]cluster.VM, 0, count)
		for i := range count {
			vms = append(vms, cluster.VM{
				VMID: 100 + i,
				Name: fmt.Sprintf("vm-%d", i),
				Node: "node-a",
				Pool: pool,
				Tags: []string{"pvmss"},
			})
		}

		return vms
	}

	defaultVMs := poolVMs(7)
	defaultVMs[0].Name = "taken"
	defaultIndex := inventory.BuildIndexForCluster("default", cluster.Snapshot{VMs: defaultVMs})
	secondaryIndex := inventory.BuildIndexForCluster(secondary, cluster.Snapshot{VMs: poolVMs(5)})

	service := policy.New(st, nil, nil)
	service.SetProjectionSource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{
		"default": &defaultIndex,
		secondary: &secondaryIndex,
	}))

	ctx := context.Background()
	actor := auth.Identity{Username: testUserAlice, Pool: pool}

	if err := service.SetPolicy(ctx, secondary, policy.Gabarit{}, 3); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	quota, err := service.Quota(ctx, secondary, actor)
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}

	if quota.Used != 5 {
		t.Fatalf("secondary quota used = %d, want 5 (default holds 7)", quota.Used)
	}

	_, err = service.ReserveCreate(ctx, secondary, actor, "fresh")

	var quotaErr *policy.QuotaExceededError
	if !errors.As(err, &quotaErr) {
		t.Fatalf("ReserveCreate error = %v, want QuotaExceededError", err)
	}

	if quotaErr.Used != 5 || quotaErr.Allowed != 3 {
		t.Fatalf("quota error = %d of %d, want 5 of 3", quotaErr.Used, quotaErr.Allowed)
	}

	if err := service.SetPolicy(ctx, secondary, policy.Gabarit{}, -1); err != nil {
		t.Fatalf("SetPolicy unlimited: %v", err)
	}

	release, err := service.ReserveCreate(ctx, secondary, actor, "taken")
	if err != nil {
		t.Fatalf("ReserveCreate a name only taken on default: %v", err)
	}

	release()
}
