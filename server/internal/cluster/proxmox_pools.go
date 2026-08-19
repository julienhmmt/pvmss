package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ListPools implements Client.
func (p Proxmox) ListPools(ctx context.Context) ([]Pool, error) {
	return proxmoxListPools(ctx, p.rest())
}

func proxmoxListPools(ctx context.Context, rest proxmoxRESTClient) ([]Pool, error) {
	raw, err := rest.do(ctx, http.MethodGet, "/pools", nil)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		PoolID  string `json:"poolid"`
		Comment string `json:"comment"`
	}
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode pools: %w", err)
	}

	pools := make([]Pool, 0, len(rows))
	for _, row := range rows {
		pools = append(pools, Pool{Name: row.PoolID, Comment: row.Comment})
	}

	return pools, nil
}

// EnsurePoolRole implements Client: creates the shared PVMSSUser role once
// (rolePrivileges/poolRoleName, defined in fake.go and reused verbatim here
// so the real and fake clusters grant identical privileges) and never
// rewrites it on a later call.
func (p Proxmox) EnsurePoolRole(ctx context.Context) error {
	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, "/access/roles", nil)
	if err != nil {
		return err
	}

	var rows []struct {
		RoleID string `json:"roleid"`
	}
	if err := decodeData(raw, &rows); err != nil {
		return fmt.Errorf("decode roles: %w", err)
	}

	for _, row := range rows {
		if row.RoleID == poolRoleName {
			return nil
		}
	}

	_, err = rest.do(ctx, http.MethodPost, "/access/roles", url.Values{
		"roleid": {poolRoleName},
		"privs":  {strings.Join(rolePrivileges, ",")},
	})

	return err
}

// EnsurePoolUser implements Client: creates the pool's PVE login once,
// following the "<pool>@pve" convention every other pool operation assumes
// (SetPoolACL's caller, pools/delete.go's deletePoolUser, proxmoxOwnedPool).
func (p Proxmox) EnsurePoolUser(ctx context.Context, pool, password string) (string, error) {
	rest := p.rest()
	username := pool + "@pve"

	raw, err := rest.do(ctx, http.MethodGet, "/access/users", nil)
	if err != nil {
		return "", err
	}

	var rows []struct {
		UserID string `json:"userid"`
	}
	if err := decodeData(raw, &rows); err != nil {
		return "", fmt.Errorf("decode users: %w", err)
	}

	for _, row := range rows {
		if row.UserID == username {
			return username, nil
		}
	}

	_, err = rest.do(ctx, http.MethodPost, "/access/users", url.Values{
		"userid":   {username},
		"password": {password},
		"comment":  {"PVMSS self-service user for pool " + pool},
	})
	if err != nil {
		return "", err
	}

	return username, nil
}

// CreatePool implements Client, idempotently — a pool that already exists is
// left untouched (matching the fake's own contract).
func (p Proxmox) CreatePool(ctx context.Context, poolID, comment string) error {
	rest := p.rest()

	pools, err := proxmoxListPools(ctx, rest)
	if err != nil {
		return err
	}

	for _, pool := range pools {
		if pool.Name == poolID {
			return nil
		}
	}

	form := url.Values{"poolid": {poolID}}
	if comment != "" {
		form.Set("comment", comment)
	}

	_, err = rest.do(ctx, http.MethodPost, "/pools", form)

	return err
}

// SetPoolACL implements Client, granting role to username at the pool's ACL
// path with propagation so it applies to every VM later added to the pool.
func (p Proxmox) SetPoolACL(ctx context.Context, username, poolID, role string) error {
	_, err := p.rest().do(ctx, http.MethodPut, "/access/acl", url.Values{
		"path":      {"/pool/" + poolID},
		"users":     {username},
		"roles":     {role},
		"propagate": {"1"},
	})

	return err
}

// DeletePool implements Client. ErrNotFound on an unknown pool, matching the
// fake's contract — Proxmox's own DELETE does not 404 cleanly on a missing
// pool, so existence is checked first. The pool's ACL entry is cleared too:
// harmless to leave (Proxmox tolerates an ACL pointing at a removed path) but
// worth clearing so entries do not accumulate indefinitely.
func (p Proxmox) DeletePool(ctx context.Context, poolID string) error {
	rest := p.rest()

	pools, err := proxmoxListPools(ctx, rest)
	if err != nil {
		return err
	}

	found := false

	for _, pool := range pools {
		if pool.Name == poolID {
			found = true

			break
		}
	}

	if !found {
		return ErrNotFound
	}

	if _, err := rest.do(ctx, http.MethodDelete, "/pools/"+url.PathEscape(poolID), nil); err != nil {
		return err
	}

	_, _ = rest.do(ctx, http.MethodPut, "/access/acl", url.Values{"path": {"/pool/" + poolID}, "delete": {"1"}})

	return nil
}

// DeleteUser implements Client. Best-effort by contract (pools/delete.go's
// deletePoolUser logs and continues on failure rather than treating it as
// fatal), so no ErrNotFound translation is needed here.
func (p Proxmox) DeleteUser(ctx context.Context, username string) error {
	_, err := p.rest().do(ctx, http.MethodDelete, "/access/users/"+url.PathEscape(username), nil)

	return err
}
