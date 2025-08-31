package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"pvmss/logger"
)

// EnsureUser creates a Proxmox user if it does not already exist.
// - username: without realm (we will append @realm if missing)
// - realm: defaults to "pve" when empty
// - enable: when true, user is created enabled
// This function is idempotent: if the user already exists, it returns nil.
func EnsureUser(ctx context.Context, client ClientInterface, username, password, email, comment, realm string, enable bool) error {
	if client == nil {
		return fmt.Errorf("nil proxmox client")
	}
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}
	if realm == "" {
		realm = "pve"
	}
	uid := normalizeUserID(username, realm)

	// Short timeout wrapper if ctx has no deadline
	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	// Check if the user already exists: GET /access/users/{userid}
	path := fmt.Sprintf("/access/users/%s", url.PathEscape(uid))
	var probe map[string]any
	if err := client.GetJSON(ctx, path, &probe); err == nil {
		// Exists
		logger.Get().Debug().Str("userid", uid).Msg("User already exists; EnsureUser noop")
		return nil
	}

	// Create user: POST /access/users
	form := url.Values{}
	form.Set("userid", uid)
	form.Set("password", password)
	if email != "" {
		form.Set("email", email)
	}
	if comment != "" {
		form.Set("comment", comment)
	}
	if enable {
		form.Set("enable", "1")
	} else {
		form.Set("enable", "0")
	}
	if _, err := client.PostFormWithContext(ctx, "/access/users", form); err != nil {
		// Best-effort idempotency: treat 409/exists as success
		if strings.Contains(strings.ToLower(err.Error()), "409") || strings.Contains(strings.ToLower(err.Error()), "exist") {
			logger.Get().Warn().Err(err).Str("userid", uid).Msg("User create raced; treating as existing")
			return nil
		}
		return fmt.Errorf("failed to create user %s: %w", uid, err)
	}
	logger.Get().Info().Str("userid", uid).Msg("Created user")
	return nil
}

// EnsurePool creates a Proxmox pool if missing. Idempotent when the pool already exists.
func EnsurePool(ctx context.Context, client ClientInterface, poolID, comment string) error {
	if client == nil {
		return fmt.Errorf("nil proxmox client")
	}
	if poolID == "" {
		return fmt.Errorf("poolID is required")
	}

	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	// Check exist: GET /pools/{poolid}
	checkPath := fmt.Sprintf("/pools/%s", url.PathEscape(poolID))
	var probe map[string]any
	if err := client.GetJSON(ctx, checkPath, &probe); err == nil {
		logger.Get().Debug().Str("pool", poolID).Msg("Pool already exists; EnsurePool noop")
		return nil
	}

	form := url.Values{}
	form.Set("poolid", poolID)
	if comment != "" {
		form.Set("comment", comment)
	}
	if _, err := client.PostFormWithContext(ctx, "/pools", form); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "409") || strings.Contains(strings.ToLower(err.Error()), "exist") {
			logger.Get().Warn().Err(err).Str("pool", poolID).Msg("Pool create raced; treating as existing")
			return nil
		}
		return fmt.Errorf("failed to create pool %s: %w", poolID, err)
	}
	logger.Get().Info().Str("pool", poolID).Msg("Created pool")
	return nil
}

// EnsurePoolACL grants a role to a user at the pool path with optional propagation.
// This simply POSTs the ACL assignment and relies on the API to be idempotent for repeated grants.
func EnsurePoolACL(ctx context.Context, client ClientInterface, userID, poolID, role string, propagate bool) error {
	if client == nil {
		return fmt.Errorf("nil proxmox client")
	}
	if userID == "" || poolID == "" || role == "" {
		return fmt.Errorf("userID, poolID and role are required")
	}
	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	form := url.Values{}
	form.Set("path", poolPath(poolID))
	form.Set("users", userID) // comma-separated list supported; we use single
	form.Set("roles", role)
	if propagate {
		form.Set("propagate", "1")
	}
	if _, err := client.PutFormWithContext(ctx, "/access/acl", form); err != nil {
		return fmt.Errorf("failed to grant ACL (%s on %s to %s): %w", role, poolID, userID, err)
	}
	logger.Get().Info().Str("user", userID).Str("pool", poolID).Str("role", role).Bool("propagate", propagate).Msg("Granted pool ACL")
	return nil
}

// Helper: ensure username has realm suffix
func normalizeUserID(username, realm string) string {
	if username == "" {
		return ""
	}
	if strings.Contains(username, "@") {
		return username
	}
	if realm == "" {
		realm = "pve"
	}
	return fmt.Sprintf("%s@%s", username, realm)
}

func poolPath(poolID string) string { return "/pool/" + poolID }

// withDefaultTimeout ensures the context has a deadline; if not, it wraps it with a reasonable timeout.
func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		// Return a no-op cancel when caller already has a deadline
		return ctx, func() {}
	}
	if d <= 0 {
		d = 10 * time.Second
	}
	c, cancel := context.WithTimeout(ctx, d)
	return c, cancel
}
