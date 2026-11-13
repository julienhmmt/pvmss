package pools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"regexp"
	"strings"
	"sync"
)

const (
	ssoSlugMax = 32
	// ssoHashLen hex chars of the username hash disambiguate a taken name.
	ssoHashLen = 8
)

// ssoMu serializes EnsureForSSO so two logins cannot pick the same name.
var ssoMu sync.Mutex

var ssoNonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// EnsureForSSO provisions the pool of an SSO user at first login: a managed
// pool named after the user, with the PVMSSUser role granted to the Proxmox
// user the realm already created. No @pve user, no password.
//
// The ACL is only ever granted on a pool this call created: an existing name
// may belong to someone else (two usernames can share a slug), so the user
// gets a hash-suffixed pool, and when that is taken too the call fails.
//
// shortcut: ssoMu makes list-create-grant atomic for one PVMSS process only,
// switch to a fail-if-exists create if PVMSS ever runs several replicas.
func EnsureForSSO(ctx context.Context, client cluster.Client, recorder *store.Store, clusterName, username string) (string, error) {
	ssoMu.Lock()
	defer ssoMu.Unlock()

	existing, err := client.ListPools(ctx)
	if err != nil {
		return "", err
	}

	slug := strings.Trim(ssoNonSlug.ReplaceAllString(strings.ToLower(username), "-"), "-")

	name := PoolPrefix + strings.TrimRight(truncate(slug, ssoSlugMax), "-")
	if slug == "" || containsPool(existing, name) {
		sum := sha256.Sum256([]byte(username))

		name = PoolPrefix + strings.TrimLeft(strings.TrimRight(truncate(slug, ssoSlugMax-ssoHashLen-1), "-")+"-", "-") + hex.EncodeToString(sum[:])[:ssoHashLen]
		if containsPool(existing, name) {
			return "", fmt.Errorf("%w: %q", ErrAlreadyExists, name)
		}
	}

	if err := client.EnsurePoolRole(ctx); err != nil {
		return "", &ProvisionError{Step: "role", Err: err}
	}

	if err := client.CreatePool(ctx, name, "SSO user "+username); err != nil {
		return "", &ProvisionError{Step: "pool", Err: err}
	}

	if err := client.SetPoolACL(ctx, username, name, "PVMSSUser"); err != nil {
		// Drop the empty pool so the next login can retry under the same name.
		if delErr := client.DeletePool(ctx, name); delErr != nil {
			slog.Default().ErrorContext(ctx, "sso pool cleanup failed", "cluster", clusterName, "pool", name, "error", delErr)
		}

		return "", &ProvisionError{Step: "ACL", Err: err}
	}

	if recorder != nil && clusterName != "" {
		if err := recorder.RegisterSSOPool(ctx, clusterName, name, username); err != nil {
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
