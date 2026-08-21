//nolint:paralleltest,wsl_v5,goconst // tests use shared fake fixtures and fixed credentials
package pools_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"testing"
)

func TestCreate_RoleProvisionedOnce(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	admin := auth.Identity{Username: "admin", IsAdmin: true}
	client := cluster.Fake{}

	first, err := pools.Create(context.Background(), admin, client, "carol", "S0meLongPW!")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	second, err := pools.Create(context.Background(), admin, client, "dave", "S0meLongPW!")
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if first.Name != "carol" || second.Name != "dave" {
		t.Fatalf("created pools = %+v, %+v", first, second)
	}
	roleCalls := cluster.FakeRoleCalls()
	if len(roleCalls) != 1 {
		t.Fatalf("role call log length = %d, want 1", len(roleCalls))
	}
	if len(roleCalls[0].Privileges) != 16 || roleCalls[0].Privileges[0] != "VM.Allocate" || roleCalls[0].Privileges[15] != "SDN.Use" {
		t.Fatalf("role privileges = %v, want fixed 16-entry list", roleCalls[0].Privileges)
	}
	calls := cluster.FakeCalls()
	wantActions := []string{"ensure_role", "ensure_user", "create_pool", "set_acl", "ensure_user", "create_pool", "set_acl"}
	if len(calls) != len(wantActions) {
		t.Fatalf("calls = %+v, want %d calls", calls, len(wantActions))
	}
	for index, want := range wantActions {
		if calls[index].Action != want {
			t.Fatalf("call %d = %q, want %q", index, calls[index].Action, want)
		}
	}
}

func TestCreate_ValidationAndDuplicateBeforeProvisioning(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "", password: "S0meLongPW!", wantErr: pools.ErrInvalidName},
		{name: "-leading", password: "S0meLongPW!", wantErr: pools.ErrInvalidName},
		{name: "trailing-", password: "S0meLongPW!", wantErr: pools.ErrInvalidName},
		{name: "UPPER", password: "S0meLongPW!", wantErr: pools.ErrInvalidName},
		{name: "has_underscore", password: "S0meLongPW!", wantErr: pools.ErrInvalidName},
		{name: "valid", password: "short", wantErr: pools.ErrWeakPassword},
	}
	for _, tc := range cases {
		t.Run(tc.name+tc.password, func(t *testing.T) {
			cluster.ResetFake()
			_, err := pools.Create(context.Background(), auth.Identity{Username: "admin", IsAdmin: true}, cluster.Fake{}, tc.name, tc.password)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if calls := cluster.FakeCalls(); len(calls) != 0 {
				t.Fatalf("provision calls = %+v, want none", calls)
			}
		})
	}

	cluster.ResetFake()
	client := cluster.Fake{}
	admin := auth.Identity{Username: "admin", IsAdmin: true}
	if _, err := pools.Create(context.Background(), admin, client, "carol", "S0meLongPW!"); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	cluster.ResetFake()
	if _, err := pools.Create(context.Background(), admin, client, "pool-alice", "S0meLongPW!"); !errors.Is(err, pools.ErrAlreadyExists) {
		t.Fatalf("duplicate error = %v, want ErrAlreadyExists", err)
	}
	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("duplicate provision calls = %+v, want none", calls)
	}
}

func TestCreate_NonAdminIsRejectedBeforeClusterCalls(t *testing.T) {
	cluster.ResetFake()
	_, err := pools.Create(context.Background(), auth.Identity{Username: "alice"}, cluster.Fake{}, "carol", "S0meLongPW!")
	if !errors.Is(err, pools.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("calls = %+v, want none", calls)
	}
}

// TestCreate_WithRecorderRegistersManagedPool verifies that a successful
// CreateWithRecorder records the pool as managed in the store.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestCreate_WithRecorderRegistersManagedPool(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	admin := auth.Identity{Username: "admin", IsAdmin: true}
	client := cluster.Fake{}
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "pools-managed.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := pools.CreateWithRecorder(context.Background(), pools.CreateParams{Actor: admin, Client: client, Recorder: st, ClusterName: "default", Name: "team-y", Password: "S0meLongPW!"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	managed, err := st.IsPoolManaged(context.Background(), "default", "team-y")
	if err != nil {
		t.Fatalf("IsPoolManaged: %v", err)
	}
	if !managed {
		t.Fatal("pool not recorded as managed after CreateWithRecorder")
	}
}

// TestCreateManaged_GeneratesPasswordAndPrefixesPool verifies that CreateManaged
// generates a password, prefixes the pool name with pvmss-, and registers the
// prefixed name as managed.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestCreateManaged_GeneratesPasswordAndPrefixesPool(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	admin := auth.Identity{Username: "admin", IsAdmin: true}
	client := cluster.Fake{}
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "pools-managed-gen.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	creds, err := pools.CreateManaged(context.Background(), admin, client, st, "default", "alice", "Alice pool")
	if err != nil {
		t.Fatalf("CreateManaged: %v", err)
	}
	if creds.PoolName != "pvmss-alice" {
		t.Errorf("PoolName = %q, want pvmss-alice", creds.PoolName)
	}
	if creds.Username != "pvmss-alice" {
		t.Errorf("Username = %q, want pvmss-alice", creds.Username)
	}
	if len(creds.Password) < 8 {
		t.Errorf("Password length = %d, want >= 8", len(creds.Password))
	}
	if creds.Comment != "Alice pool" {
		t.Errorf("Comment = %q, want Alice pool", creds.Comment)
	}
	managed, err := st.IsPoolManaged(context.Background(), "default", "pvmss-alice")
	if err != nil {
		t.Fatalf("IsPoolManaged: %v", err)
	}
	if !managed {
		t.Fatal("pvmss-alice not recorded as managed")
	}
}

// TestCreateManaged_RejectsNonAdmin verifies that non-admins cannot create
// managed pools.
func TestCreateManaged_RejectsNonAdmin(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	_, err := pools.CreateManaged(context.Background(), auth.Identity{IsAdmin: false}, cluster.Fake{}, nil, "default", "alice", "")
	if !errors.Is(err, pools.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

// TestCreateManaged_RejectsInvalidName verifies name validation still applies.
func TestCreateManaged_RejectsInvalidName(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	_, err := pools.CreateManaged(context.Background(), auth.Identity{IsAdmin: true}, cluster.Fake{}, nil, "default", "UPPER", "")
	if !errors.Is(err, pools.ErrInvalidName) {
		t.Fatalf("err = %v, want ErrInvalidName", err)
	}
}
