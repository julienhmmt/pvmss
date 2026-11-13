package pools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"regexp"
	"strings"
)

const (
	ssoSlugMax = 32
	// ssoHashLen hex chars of the username hash disambiguate a taken name.
	ssoHashLen = 8
)

var ssoNonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// EnsureForSSO provisions the pool of an SSO user at first login: a managed
// pool named after the user, with the PVMSSUser role granted to the Proxmox
// user the realm already created. No @pve user, no password.
//
// A name that already exists may belong to someone else (two usernames can
// share a slug), so it is never granted: the user gets a hash-suffixed pool.
func EnsureForSSO(ctx context.Context, client cluster.Client, recorder *store.Store, clusterName, username string) (string, error) {
	existing, err := client.ListPools(ctx)
	if err != nil {
		return "", err
	}

	slug := strings.Trim(ssoNonSlug.ReplaceAllString(strings.ToLower(username), "-"), "-")
	name := PoolPrefix + strings.TrimRight(truncate(slug, ssoSlugMax), "-")
	create := true

	if slug == "" || containsPool(existing, name) {
		sum := sha256.Sum256([]byte(username))
		name = PoolPrefix + strings.TrimLeft(strings.TrimRight(truncate(slug, ssoSlugMax-ssoHashLen-1), "-")+"-", "-") + hex.EncodeToString(sum[:])[:ssoHashLen]
		// Our own hash name already there: an earlier run stopped before the ACL.
		create = !containsPool(existing, name)
	}

	if err := client.EnsurePoolRole(ctx); err != nil {
		return "", &ProvisionError{Step: "role", Err: err}
	}

	if create {
		if err := client.CreatePool(ctx, name, "SSO user "+username); err != nil {
			return "", &ProvisionError{Step: "pool", Err: err}
		}
	}

	if err := client.SetPoolACL(ctx, username, name, "PVMSSUser"); err != nil {
		return "", &ProvisionError{Step: "ACL", Err: err}
	}

	if recorder != nil && clusterName != "" {
		if err := recorder.RegisterManagedPool(ctx, clusterName, name); err != nil {
			slog.Default().ErrorContext(ctx, "managed pool registration failed", "cluster", clusterName, "pool", name, "error", err)
		}
	}

	return name, nil
}

func truncate(s string, limit int) string {
	if len(s) > limit {
		return s[:limit]
	}

	return s
}
