package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"pvmss/server/internal/logctx"
	"slices"
	"strings"
)

// proxmoxFormPassword is the PVE form field carrying a user password.
const proxmoxFormPassword = "password"

// Authenticate implements Client by exchanging username/password for a PVE
// ticket (proving the credentials are correct - ErrNotFound on a rejection,
// matching the fake's contract), then using that ticket, as that user, to
// determine admin status (Permissions.Modify at "/", granted by the
// PVMSS_Admin role per proxmox-permissions.md) and - for non-admins - the
// personal pool PVMSS provisioned for them (EnsurePoolUser's own
// "<pool>@pve" convention, mirrored here in reverse).
func (p Proxmox) Authenticate(ctx context.Context, username, password string) (Identity, error) {
	rest := p.rest()

	ticket, csrf, err := proxmoxTicketAuth(ctx, rest, username, password)
	if err != nil {
		return Identity{}, err
	}

	userRest := rest.withTicket(ticket, csrf)

	isAdmin, err := proxmoxHasPermission(ctx, userRest, "/", "Permissions.Modify")
	if err != nil {
		return Identity{}, err
	}

	var pool string

	if !isAdmin {
		pool, err = proxmoxOwnedPool(ctx, rest, username)
		if err != nil {
			return Identity{}, err
		}
	}

	return Identity{Username: username, Pool: pool, IsAdmin: isAdmin}, nil
}

// proxmoxTicketAuth exchanges credentials for a PVE ticket + CSRF prevention
// token via POST /access/ticket. Deliberately does not reuse
// proxmoxRESTClient.do: this call must not carry any prior authentication
// (it IS the credential check), and a rejected login is a 401 that must map
// to ErrNotFound (wrong credentials), not the generic wrapped-error path.
func proxmoxTicketAuth(ctx context.Context, rest proxmoxRESTClient, username, password string) (ticket, csrf string, err error) {
	form := url.Values{"username": {username}, proxmoxFormPassword: {password}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rest.base+"/access/ticket", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("build ticket request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := rest.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", ErrNotFound
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", "", fmt.Errorf("proxmox ticket auth: HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Data struct {
			Ticket              string `json:"ticket"`
			CSRFPreventionToken string `json:"CSRFPreventionToken"`
		} `json:"data"`
	}
	if err := decodeJSONBody(resp, &envelope); err != nil {
		return "", "", err
	}

	if envelope.Data.Ticket == "" {
		return "", "", ErrNotFound
	}

	return envelope.Data.Ticket, envelope.Data.CSRFPreventionToken, nil
}

// proxmoxHasPermission checks one privilege at one path for the ticket's own
// user via GET /access/permissions?path=... - any authenticated user may
// query their own effective permissions, no elevated privilege required.
// PVE always nests the response as path -> {privilege: propagate-bool}, even
// for a single requested path (pve-access-control's AccessControl.pm
//
//	`permissions` method: `$res = {$path => $perms}`) - never a flat
//
// privilege map.
func proxmoxHasPermission(ctx context.Context, rest proxmoxRESTClient, path, privilege string) (bool, error) {
	raw, err := rest.do(ctx, http.MethodGet, "/access/permissions", url.Values{"path": {path}})
	if err != nil {
		return false, err
	}

	var perms map[string]map[string]int
	if err := decodeData(raw, &perms); err != nil {
		return false, fmt.Errorf("decode permissions: %w", err)
	}

	return perms[path][privilege] == 1, nil
}

// proxmoxOwnedPool derives the caller's personal pool from PVMSS's own
// provisioning convention (pools/provision.go: EnsurePoolUser creates
// "<pool>@pve"), verified against the live pool list with the service
// account's Pool.Audit privilege - an end user's own PVMSSUser role does not
// carry that privilege (fake.go's rolePrivileges), so this always uses rest,
// not the user's ticket.
func proxmoxOwnedPool(ctx context.Context, rest proxmoxRESTClient, username string) (string, error) {
	candidate, ok := strings.CutSuffix(username, "@pve")
	if !ok {
		// Password sign-in keeps working without a pool when the lookup fails.
		pool, err := proxmoxACLPool(ctx, rest, username)
		if err != nil {
			logctx.From(ctx).WarnContext(ctx, "pool acl lookup failed", "error", err)
		}

		return pool, nil
	}

	pools, err := proxmoxListPools(ctx, rest)
	if err != nil {
		return "", err
	}

	for _, pool := range pools {
		if pool.Name == candidate {
			return candidate, nil
		}
	}

	return "", nil
}

// managedPoolPrefix mirrors pools.PoolPrefix (importing pools here would cycle).
const managedPoolPrefix = "pvmss-"

// proxmoxACLPool binds a non-@pve identity (an OIDC or LDAP realm user) to the
// PVMSS pool its own user ACL grants, the exact shape SetPoolACL writes:
// type user, poolRoleName, path /pool/pvmss-*. Inherited and group grants are
// ignored on purpose, so a broad role never anchors a user to someone else's
// pool. Several matches keep the first by name. No match is "", nil; a failed
// lookup is an error so a caller never mistakes it for "has no pool".
func proxmoxACLPool(ctx context.Context, rest proxmoxRESTClient, username string) (string, error) {
	raw, err := rest.do(ctx, http.MethodGet, "/access/acl", nil)
	if err != nil {
		return "", fmt.Errorf("pool acl lookup: %w", err)
	}

	var rows []struct {
		Path   string `json:"path"`
		UGID   string `json:"ugid"`
		Type   string `json:"type"`
		RoleID string `json:"roleid"`
	}
	if err := decodeData(raw, &rows); err != nil {
		return "", fmt.Errorf("pool acl decode: %w", err)
	}

	var matches []string

	for _, row := range rows {
		pool, ok := strings.CutPrefix(row.Path, "/pool/")
		if ok && row.Type == "user" && row.UGID == username && row.RoleID == poolRoleName && strings.HasPrefix(pool, managedPoolPrefix) {
			matches = append(matches, pool)
		}
	}

	if len(matches) == 0 {
		return "", nil
	}

	slices.Sort(matches)

	if len(matches) > 1 {
		logctx.From(ctx).WarnContext(ctx, "pool multiple matches", "pools", matches)
	}

	return matches[0], nil
}

// ChangePassword implements Client by re-authenticating as username with
// oldPassword (ErrNotFound if that fails, matching Authenticate's contract)
// and then setting the new password with that same ticket - a genuine
// self-service change, requiring no elevated privilege.
func (p Proxmox) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	rest := p.rest()

	ticket, csrf, err := proxmoxTicketAuth(ctx, rest, username, oldPassword)
	if err != nil {
		return err
	}

	userRest := rest.withTicket(ticket, csrf)

	_, err = userRest.do(ctx, http.MethodPut, "/access/password", url.Values{
		"userid":            {username},
		proxmoxFormPassword: {newPassword},
	})

	return err
}

// isNodeUnavailable reports whether err is Proxmox's inter-node connection
// failure (HTTP 595): the API node could not reach the target node's pveproxy,
// in practice because the node is offline. Per-node enumerations skip such
// nodes instead of failing the whole listing.
func isNodeUnavailable(err error) bool {
	var rejection *RejectionError

	return errors.As(err, &rejection) && rejection.Status == 595
}
