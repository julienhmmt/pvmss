//nolint:paralleltest,goconst // tests share the fake cluster state and fixed names
package pools_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/pools"
	"testing"
)

func resetFixture(t *testing.T) (cluster.Fake, auth.Identity, context.Context) {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	return cluster.Fake{}, auth.Identity{Username: "admin", IsAdmin: true}, context.Background()
}

// A managed non-SSO pool rotates: the empty password generates one, a given
// one is used verbatim, and both runs repair the user + ACL on the cluster.
func TestResetManagedPassword_GeneratesAndUsesGivenPassword(t *testing.T) {
	client, admin, ctx := resetFixture(t)
	st := openAuditStore(t)

	const pool = "team-reset"
	if err := client.CreatePool(ctx, pool, "team"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}

	if err := st.RegisterManagedPool(ctx, "default", pool); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}

	generated, err := pools.ResetManagedPassword(ctx, admin, client, st, "default", pool, "")
	if err != nil {
		t.Fatalf("reset generated: %v", err)
	}

	if generated.PoolName != pool || generated.Username == "" || generated.Password == "" {
		t.Fatalf("generated creds = %+v", generated)
	}

	given, err := pools.ResetManagedPassword(ctx, admin, client, st, "default", pool, "s3cret-value")
	if err != nil {
		t.Fatalf("reset given: %v", err)
	}

	if given.Password != "s3cret-value" || given.Username != generated.Username {
		t.Fatalf("given creds = %+v, want password verbatim on same user", given)
	}
}

// The guard rails run before any cluster write: non-admin, unknown pool and
// unmanaged pool each refuse without touching Proxmox.
func TestResetManagedPassword_Refusals(t *testing.T) {
	client, admin, ctx := resetFixture(t)
	st := openAuditStore(t)

	const pool = "team-guard"
	if err := client.CreatePool(ctx, pool, "team"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}

	if err := st.RegisterManagedPool(ctx, "default", pool); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}

	user := auth.Identity{Username: "alice@pve", Pool: pool}
	if _, err := pools.ResetManagedPassword(ctx, user, client, st, "default", pool, ""); !errors.Is(err, pools.ErrForbidden) {
		t.Fatalf("non-admin err = %v, want ErrForbidden", err)
	}

	if _, err := pools.ResetManagedPassword(ctx, admin, client, st, "default", "no-such-pool", ""); !errors.Is(err, pools.ErrNotFound) {
		t.Fatalf("missing pool err = %v, want ErrNotFound", err)
	}

	if err := client.CreatePool(ctx, "foreign", "not managed"); err != nil {
		t.Fatalf("CreatePool foreign: %v", err)
	}

	if _, err := pools.ResetManagedPassword(ctx, admin, client, st, "default", "foreign", ""); !errors.Is(err, pools.ErrNotManaged) {
		t.Fatalf("unmanaged err = %v, want ErrNotManaged", err)
	}
}

// A nil registry skips the managed/SSO checks entirely - the reset only
// needs the pool to exist on the cluster.
func TestResetManagedPassword_NilRegistrySkipsManagedCheck(t *testing.T) {
	client, admin, ctx := resetFixture(t)

	const pool = "unregistered"
	if err := client.CreatePool(ctx, pool, "adhoc"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}

	creds, err := pools.ResetManagedPassword(ctx, admin, client, nil, "", pool, "given-pw")
	if err != nil {
		t.Fatalf("reset without registry: %v", err)
	}

	if creds.Password != "given-pw" {
		t.Fatalf("creds = %+v", creds)
	}
}
