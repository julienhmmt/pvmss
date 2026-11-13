//nolint:paralleltest,goconst // tests share the fake cluster state and fixed names
package pools_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/pools"
	"strings"
	"testing"
)

func TestEnsureForSSO_CreatesPoolAndGrantsUser(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	client := cluster.Fake{}

	pool, err := pools.EnsureForSSO(context.Background(), client, nil, "default", "Jane.Doe@zitadel.localhost@zitadel")
	if err != nil {
		t.Fatalf("EnsureForSSO: %v", err)
	}

	if pool != "pvmss-jane-doe-zitadel-localhost-zitad" {
		t.Fatalf("pool = %q", pool)
	}

	var actions []string
	for _, call := range cluster.FakeCalls() {
		actions = append(actions, call.Action+":"+call.Name)
	}

	want := "ensure_role:PVMSSUser create_pool:" + pool + " set_acl:Jane.Doe@zitadel.localhost@zitadel"
	if got := strings.Join(actions, " "); got != want {
		t.Fatalf("calls = %q, want %q (no @pve user, ACL for the SSO user)", got, want)
	}
}

// A taken name may be someone else's pool: never grant on it.
func TestEnsureForSSO_TakenNameGetsDistinctPool(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	client := cluster.Fake{}

	first, err := pools.EnsureForSSO(context.Background(), client, nil, "default", "a.b@zitadel")
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	second, err := pools.EnsureForSSO(context.Background(), client, nil, "default", "a-b@zitadel")
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if first != "pvmss-a-b-zitadel" || second == first || !strings.HasPrefix(second, "pvmss-a-b-zitadel-") {
		t.Fatalf("pools = %q, %q, want distinct", first, second)
	}
}

// Both candidate names taken: fail rather than grant on an existing pool.
func TestEnsureForSSO_NeverGrantsOnExistingPool(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	client := cluster.Fake{}
	ctx := context.Background()

	if _, err := pools.EnsureForSSO(ctx, client, nil, "default", "a.b@zitadel"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	second, err := pools.EnsureForSSO(ctx, client, nil, "default", "a-b@zitadel")
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	cluster.ClearFakeCalls()

	// Same user again while its hash pool exists (e.g. its ACL was removed).
	if _, err := pools.EnsureForSSO(ctx, client, nil, "default", "a-b@zitadel"); !errors.Is(err, pools.ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists for taken %q", err, second)
	}

	for _, call := range cluster.FakeCalls() {
		if call.Action == "set_acl" {
			t.Fatalf("granted on an existing pool: %+v", call)
		}
	}
}

// An SSO pool has no @pve user: the detail names the realm user and a
// password reset is refused before any cluster write.
func TestSSOPool_DetailNamesUserAndResetRefused(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	client := cluster.Fake{}
	ctx := context.Background()
	st := openAuditStore(t)

	pool, err := pools.EnsureForSSO(ctx, client, st, "default", "jane@zitadel")
	if err != nil {
		t.Fatalf("EnsureForSSO: %v", err)
	}

	projection := inventory.NewProjectionFromIndex(&inventory.Index{ByPool: map[string][]cluster.VM{}})

	detail, err := pools.Detail(ctx, client, projection, st, "default", pool)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}

	if !detail.Managed || !detail.SSO || detail.Username != "jane@zitadel" {
		t.Fatalf("detail = %+v, want managed SSO pool of jane@zitadel", detail)
	}

	cluster.ClearFakeCalls()

	admin := auth.Identity{Username: "admin", IsAdmin: true}
	if _, err := pools.ResetManagedPassword(ctx, admin, client, st, "default", pool, ""); !errors.Is(err, pools.ErrSSOPool) {
		t.Fatalf("reset err = %v, want ErrSSOPool", err)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("reset touched the cluster: %+v", calls)
	}
}
