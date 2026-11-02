//nolint:wsl_v5 // reset sequence is intentionally linear and ordered
package pools

import (
	"context"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// ResetManagedPassword rotates the sign-in credential of a managed pool's
// Proxmox user. An empty password means generate one. The sequence doubles
// as drift repair: a pool user deleted directly in Proxmox is recreated with
// the new password and its ACL is re-granted. The password is returned once
// and never stored.
func ResetManagedPassword(ctx context.Context, actor auth.Identity, client cluster.Client, registry *store.Store, clusterName, name, password string) (GeneratedCredentials, error) {
	if !actor.IsAdmin {
		return GeneratedCredentials{}, ErrForbidden
	}
	if err := ensurePoolExists(ctx, client, name); err != nil {
		return GeneratedCredentials{}, err
	}
	if err := requireManaged(ctx, registry, clusterName, name); err != nil {
		return GeneratedCredentials{}, err
	}
	if password == "" {
		generated, err := generatePassword()
		if err != nil {
			return GeneratedCredentials{}, err
		}
		password = generated
	}
	if err := client.EnsurePoolRole(ctx); err != nil {
		return GeneratedCredentials{}, &ProvisionError{Step: "role", Err: err}
	}
	username, err := client.EnsurePoolUser(ctx, name, password)
	if err != nil {
		return GeneratedCredentials{}, &ProvisionError{Step: "user", Err: err}
	}
	// EnsurePoolUser only creates: an existing user still carries the old
	// password, so the set must always run. On a just-created user it repeats
	// the same secret - idempotent and simpler than a created/exists flag.
	if err := client.SetUserPassword(ctx, username, password); err != nil {
		return GeneratedCredentials{}, &ProvisionError{Step: "password", Err: err}
	}
	if err := client.SetPoolACL(ctx, username, name, "PVMSSUser"); err != nil {
		return GeneratedCredentials{}, &ProvisionError{Step: "ACL", Err: err}
	}
	return GeneratedCredentials{PoolName: name, Username: username, Password: password}, nil
}
