package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"pvmss/logger"
)

// EnsureUser creates a Proxmox user if it does not already exist. This function is idempotent.
func EnsureUser(ctx context.Context, client ClientInterface, username, password, email, comment, realm string, enable bool) error {
	if err := validateClientAndParams(client, param{"username", username}, param{"password", password}); err != nil {
		return err
	}

	if realm == "" {
		realm = "pve"
	}
	uid := normalizeUserID(username, realm)

	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	// Check if the user already exists
	path := fmt.Sprintf("/access/users/%s", url.PathEscape(uid))
	var probe map[string]any
	if err := client.GetJSON(ctx, path, &probe); err == nil {
		logger.Get().Debug().Str("userid", uid).Msg("User already exists; EnsureUser is a no-op.")
		return nil
	}

	// Create user
	form := url.Values{}
	form.Set("userid", uid)
	form.Set("password", password)
	form.Set("enable", boolToForm(enable))
	if email != "" {
		form.Set("email", email)
	}
	if comment != "" {
		form.Set("comment", comment)
	}

	if _, err := client.PostFormWithContext(ctx, "/access/users", form); err != nil {
		if isConflictError(err) {
			logger.Get().Warn().Err(err).Str("userid", uid).Msg("User creation raced; treating as existing.")
			return nil
		}
		return fmt.Errorf("failed to create user %s: %w", uid, err)
	}

	logger.Get().Info().Str("userid", uid).Msg("Created user")
	return nil
}

// EnsurePool creates a Proxmox pool if it is missing. This function is idempotent.
func EnsurePool(ctx context.Context, client ClientInterface, poolID, comment string) error {
	if err := validateClientAndParams(client, param{"poolID", poolID}); err != nil {
		return err
	}

	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	// Check for existence
	checkPath := fmt.Sprintf("/pools/%s", url.PathEscape(poolID))
	var probe map[string]any
	if err := client.GetJSON(ctx, checkPath, &probe); err == nil {
		logger.Get().Debug().Str("pool", poolID).Msg("Pool already exists; EnsurePool is a no-op.")
		return nil
	}

	form := url.Values{}
	form.Set("poolid", poolID)
	if comment != "" {
		form.Set("comment", comment)
	}

	if _, err := client.PostFormWithContext(ctx, "/pools", form); err != nil {
		if isConflictError(err) {
			logger.Get().Warn().Err(err).Str("pool", poolID).Msg("Pool creation raced; treating as existing.")
			return nil
		}
		return fmt.Errorf("failed to create pool %s: %w", poolID, err)
	}

	logger.Get().Info().Str("pool", poolID).Msg("Created pool")
	return nil
}

// EnsurePoolACL grants a role to a user for a pool. This operation is idempotent on the Proxmox API side.
func EnsurePoolACL(ctx context.Context, client ClientInterface, userID, poolID, role string, propagate bool) error {
	if err := validateClientAndParams(client, param{"userID", userID}, param{"poolID", poolID}, param{"role", role}); err != nil {
		return err
	}

	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	form := url.Values{}
	form.Set("path", poolPath(poolID))
	form.Set("users", userID)
	form.Set("roles", role)
	if propagate {
		form.Set("propagate", "1")
	}

	if _, err := client.PutFormWithContext(ctx, "/access/acl", form); err != nil {
		return fmt.Errorf("failed to grant ACL (role: %s, pool: %s, user: %s): %w", role, poolID, userID, err)
	}

	logger.Get().Info().Str("user", userID).Str("pool", poolID).Str("role", role).Bool("propagate", propagate).Msg("Granted pool ACL")
	return nil
}

// EnsureRole creates a custom Proxmox role if it does not already exist. This function is idempotent.
func EnsureRole(ctx context.Context, client ClientInterface, roleID string, privileges []string) error {
	if err := validateClientAndParams(client, param{"roleID", roleID}); err != nil {
		return err
	}
	if len(privileges) == 0 {
		return fmt.Errorf("at least one privilege is required for role %s", roleID)
	}

	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	// Check if role exists
	checkPath := fmt.Sprintf("/access/roles/%s", url.PathEscape(roleID))
	var probe map[string]any
	if err := client.GetJSON(ctx, checkPath, &probe); err == nil {
		logger.Get().Debug().Str("role", roleID).Msg("Role already exists; EnsureRole is a no-op.")
		return nil
	}

	// Create role
	form := url.Values{}
	form.Set("roleid", roleID)
	form.Set("privs", strings.Join(privileges, ","))

	if _, err := client.PostFormWithContext(ctx, "/access/roles", form); err != nil {
		if isConflictError(err) {
			logger.Get().Warn().Err(err).Str("role", roleID).Msg("Role creation raced; treating as existing.")
			return nil
		}
		return fmt.Errorf("failed to create role %s: %w", roleID, err)
	}

	logger.Get().Info().Str("role", roleID).Strs("privileges", privileges).Msg("Created custom role")
	return nil
}

// --- Helpers ---

// normalizeUserID ensures the username has a realm suffix.
func normalizeUserID(username, realm string) string {
	if username == "" {
		return ""
	}
	if strings.Contains(username, "@") {
		return username
	}
	if realm == "" {
		realm = "pve" // Default realm
	}
	return fmt.Sprintf("%s@%s", username, realm)
}

func poolPath(poolID string) string {
	return "/pool/" + poolID
}

// withDefaultTimeout wraps a context with a default timeout if it has no deadline.
func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {} // No-op cancel
	}
	if d <= 0 {
		d = defaultTimeout
	}
	return context.WithTimeout(ctx, d)
}

// isConflictError checks if an error message indicates a resource conflict (HTTP 409).
func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "409") || strings.Contains(msg, "exist")
}

type param struct {
	name  string
	value string
}

// validateClientAndParams checks for a nil client and non-empty string parameters.
func validateClientAndParams(client ClientInterface, params ...param) error {
	if client == nil {
		return fmt.Errorf("proxmox client is nil")
	}
	for _, p := range params {
		if p.value == "" {
			return fmt.Errorf("%s is required", p.name)
		}
	}
	return nil
}

// boolToForm converts a boolean to a form-compatible string ("1" or "0").
func boolToForm(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
