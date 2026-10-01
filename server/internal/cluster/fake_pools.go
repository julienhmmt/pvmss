//nolint:wsl_v5 // fake state methods keep mutation and call recording adjacent
package cluster

import (
	"context"
	"slices"
	"time"
)

// ListPools implements Client and returns a defensive copy of the live pool table.
func (fake Fake) ListPools(_ context.Context) ([]Pool, error) {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	return slices.Clone(state.pools), nil
}

// EnsurePoolRole creates the shared PVMSSUser role once and never rewrites it.
func (fake Fake) EnsurePoolRole(_ context.Context) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	if state.roleState == nil {
		state.roleState = make(map[string][]string)
	}
	if _, exists := state.roleState[poolRoleName]; exists {
		return nil
	}

	privileges := slices.Clone(rolePrivileges)
	state.roleState[poolRoleName] = privileges
	state.roleCallLog = append(state.roleCallLog, RoleCall{Privileges: slices.Clone(privileges), At: time.Now().UTC()})
	state.record(FakeCall{Action: "ensure_role", Name: poolRoleName})

	return nil
}

// EnsurePoolUser creates the pool login once and returns its PVE username.
func (fake Fake) EnsurePoolUser(_ context.Context, pool, password string) (string, error) {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return "", ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	username := pool + "@pve"
	state.identMu.Lock()
	if _, exists := state.identities[username]; !exists {
		state.identities[username] = fakeIdentity{password: password, pool: pool}
	}
	state.identMu.Unlock()
	state.record(FakeCall{Action: "ensure_user", Name: username})

	return username, nil
}

// CreatePool inserts a pool only when its name is absent.
func (fake Fake) CreatePool(_ context.Context, poolID, comment string) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	for _, pool := range state.pools {
		if pool.Name == poolID {
			return nil
		}
	}

	state.pools = append(state.pools, Pool{Name: poolID, Comment: comment})
	state.record(FakeCall{Action: "create_pool", Name: poolID})

	return nil
}

// SetPoolACL records a pool-to-role binding.
func (fake Fake) SetPoolACL(_ context.Context, username, poolID, role string) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	state.acls = append(state.acls, ACLEntry{Username: username, PoolID: poolID, Role: role})
	state.record(FakeCall{Action: "set_acl", Name: username})

	return nil
}

// DeletePool removes a pool and its ACL entries. It is idempotent for cleanup.
func (fake Fake) DeletePool(_ context.Context, poolID string) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	index := slices.IndexFunc(state.pools, func(pool Pool) bool { return pool.Name == poolID })
	if index < 0 {
		return ErrNotFound
	}
	state.pools = slices.Delete(state.pools, index, index+1)
	state.acls = slices.DeleteFunc(state.acls, func(acl ACLEntry) bool { return acl.PoolID == poolID })
	state.record(FakeCall{Action: "delete_pool", Name: poolID})

	return nil
}

// DeleteUser removes a PVE identity. Tests can force a best-effort failure.
func (fake Fake) DeleteUser(_ context.Context, username string) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	if state.errDeleteUser != nil {
		return state.errDeleteUser
	}
	state.identMu.Lock()
	delete(state.identities, username)
	state.identMu.Unlock()
	state.record(FakeCall{Action: "delete_user", Name: username})

	return nil
}

// FakeRoleCalls returns a defensive copy of the default fake's role creation log.
func FakeRoleCalls() []RoleCall {
	return defaultState().roleCalls()
}

// SetFakeDeleteUserError configures a deterministic user deletion failure on
// the default fake used by zero-value Fake{}.
func SetFakeDeleteUserError(err error) {
	state := defaultState()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()
	state.errDeleteUser = err
}
