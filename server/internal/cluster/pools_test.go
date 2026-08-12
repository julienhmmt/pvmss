package cluster

import (
	"context"
	"slices"
	"testing"
)

func TestFakePoolState_ListAndIdempotentProvisioning(t *testing.T) {
	ResetFake()
	t.Cleanup(ResetFake)

	client := Fake{}

	pools, err := client.ListPools(context.Background())
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}

	if len(pools) != 4 {
		t.Fatalf("got %d fixture pools, want 4", len(pools))
	}

	if err := client.EnsurePoolRole(context.Background()); err != nil {
		t.Fatalf("EnsurePoolRole first: %v", err)
	}

	if err := client.EnsurePoolRole(context.Background()); err != nil {
		t.Fatalf("EnsurePoolRole second: %v", err)
	}

	roleCalls := FakeRoleCalls()
	if len(roleCalls) != 1 {
		t.Fatalf("role calls = %d, want 1", len(roleCalls))
	}

	wantPrivileges := slices.Clone(rolePrivileges)
	if !slices.Equal(roleCalls[0].Privileges, wantPrivileges) {
		t.Fatalf("privileges = %v, want %v", roleCalls[0].Privileges, wantPrivileges)
	}

	username, err := client.EnsurePoolUser(context.Background(), "carol", "password-one")
	if err != nil {
		t.Fatalf("EnsurePoolUser: %v", err)
	}

	if username != "carol@pve" {
		t.Fatalf("username = %q", username)
	}

	username, err = client.EnsurePoolUser(context.Background(), "carol", "password-two")
	if err != nil {
		t.Fatalf("EnsurePoolUser idempotent: %v", err)
	}

	if username != "carol@pve" {
		t.Fatalf("idempotent username = %q", username)
	}

	if err := client.CreatePool(context.Background(), "carol", "PVMSS managed pool for carol"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}

	if err := client.CreatePool(context.Background(), "carol", "changed"); err != nil {
		t.Fatalf("CreatePool idempotent: %v", err)
	}

	if err := client.SetPoolACL(context.Background(), username, "carol", "PVMSSUser"); err != nil {
		t.Fatalf("SetPoolACL: %v", err)
	}

	pools, err = client.ListPools(context.Background())
	if err != nil {
		t.Fatalf("ListPools after create: %v", err)
	}

	if len(pools) != 5 {
		t.Fatalf("got %d pools after create, want 5", len(pools))
	}
}
