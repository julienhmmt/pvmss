//nolint:paralleltest // tests share the fake cluster state
package pools_test

import (
	"context"
	"pvmss/server/internal/cluster"
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
