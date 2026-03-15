package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"pvmss/constants"
	"pvmss/logger"
)

// CreateTicketResty creates a new authentication ticket using the Resty client.
// This requires a cookie-auth RestyClient (created via MakeRestyClientCookieAuth)
// since ticket creation does not use API token authentication.
//
// POST /access/ticket
func CreateTicketResty(ctx context.Context, restyClient *RestyClient, username, password string, opts *CreateTicketOptions) (*TicketResponse, error) {
	if restyClient == nil {
		return nil, fmt.Errorf("restyClient is nil")
	}
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	if opts == nil {
		opts = &CreateTicketOptions{}
	}
	if opts.Realm == "" {
		opts.Realm = constants.DefaultLoginRealm
	}

	if !strings.Contains(username, "@") {
		username = fmt.Sprintf("%s@%s", username, opts.Realm)
	}

	params := url.Values{}
	params.Set("username", username)
	params.Set("password", password)
	if opts.OTP != "" {
		params.Set("otp", opts.OTP)
	}
	if opts.Path != "" {
		params.Set("path", opts.Path)
	}
	if opts.Privs != "" {
		params.Set("privs", opts.Privs)
	}

	var respData struct {
		Data TicketResponse `json:"data"`
	}

	if err := restyClient.Post(ctx, "/access/ticket", params, &respData); err != nil {
		logger.Get().Error().Err(err).Str("username", username).Msg("Failed to create authentication ticket (resty)")
		return nil, fmt.Errorf("failed to create ticket for %s: %w", username, err)
	}

	if respData.Data.Ticket == "" {
		return nil, fmt.Errorf("ticket creation succeeded but response missing ticket field")
	}
	if respData.Data.CSRFPreventionToken == "" {
		logger.Get().Warn().Str("username", username).Msg("Ticket created but CSRFPreventionToken is empty")
	}

	logger.Get().Info().
		Str("username", respData.Data.Username).
		Bool("has_csrf_token", respData.Data.CSRFPreventionToken != "").
		Str("clustername", respData.Data.Clustername).
		Msg("Authentication ticket created successfully (resty)")

	return &respData.Data, nil
}

// EnsureUserResty creates a Proxmox user if it does not already exist using the Resty client.
// This function is idempotent.
//
// GET /access/users/{uid} to check existence, POST /access/users to create.
func EnsureUserResty(ctx context.Context, restyClient *RestyClient, username, password, email, comment, realm string, enable bool) error {
	if restyClient == nil {
		return fmt.Errorf("restyClient is nil")
	}
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}

	if realm == "" {
		realm = "pve"
	}
	uid := normalizeUserID(username, realm)

	// Check if the user already exists
	path := fmt.Sprintf("/access/users/%s", url.PathEscape(uid))
	var probe map[string]any
	if err := restyClient.Get(ctx, path, &probe); err == nil {
		logger.Get().Debug().Str("userid", uid).Msg("User already exists; EnsureUserResty is a no-op.")
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

	if err := restyClient.Post(ctx, "/access/users", form, nil); err != nil {
		if isConflictError(err) {
			logger.Get().Warn().Err(err).Str("userid", uid).Msg("User creation raced; treating as existing.")
			return nil
		}
		return fmt.Errorf("failed to create user %s: %w", uid, err)
	}

	logger.Get().Info().Str("userid", uid).Msg("Created user (resty)")
	return nil
}

// UpdateUserPasswordResty updates the password for an existing Proxmox user using the Resty client.
// This requires cookie-based authentication (PVEAuthCookie + CSRFPreventionToken).
//
// PUT /access/password
func UpdateUserPasswordResty(ctx context.Context, restyClient *RestyClient, username, password, confirmPassword, realm string) error {
	if restyClient == nil {
		return fmt.Errorf("restyClient is nil")
	}
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}

	if realm == "" {
		realm = "pve"
	}
	uid := normalizeUserID(username, realm)

	form := url.Values{}
	form.Set("userid", uid)
	form.Set("password", password)
	if confirmPassword != "" {
		form.Set("confirmation-password", confirmPassword)
	}

	if err := restyClient.Put(ctx, "/access/password", form, nil); err != nil {
		logger.Get().Error().Err(err).Str("userid", uid).Msg("Failed to update user password (resty)")
		return fmt.Errorf("failed to update password for user %s: %w", uid, err)
	}

	logger.Get().Info().Str("userid", uid).Msg("Successfully updated user password (resty)")
	return nil
}

// EnsurePoolResty creates a Proxmox pool if it is missing using the Resty client.
// This function is idempotent.
//
// GET /pools/{poolid} to check existence, POST /pools to create.
func EnsurePoolResty(ctx context.Context, restyClient *RestyClient, poolID, comment string) error {
	if restyClient == nil {
		return fmt.Errorf("restyClient is nil")
	}
	if poolID == "" {
		return fmt.Errorf("poolID is required")
	}

	// Check for existence
	checkPath := fmt.Sprintf("/pools/%s", url.PathEscape(poolID))
	var probe map[string]any
	if err := restyClient.Get(ctx, checkPath, &probe); err == nil {
		logger.Get().Debug().Str("pool", poolID).Msg("Pool already exists; EnsurePoolResty is a no-op.")
		return nil
	}

	form := url.Values{}
	form.Set("poolid", poolID)
	if comment != "" {
		form.Set("comment", comment)
	}

	if err := restyClient.Post(ctx, "/pools", form, nil); err != nil {
		if isConflictError(err) {
			logger.Get().Warn().Err(err).Str("pool", poolID).Msg("Pool creation raced; treating as existing.")
			return nil
		}
		logger.Get().Error().Err(err).Str("pool", poolID).Str("comment", comment).Msg("Proxmox API error during pool creation (resty)")
		return fmt.Errorf("failed to create pool %s: %w", poolID, err)
	}

	logger.Get().Info().Str("pool", poolID).Msg("Created pool (resty)")
	return nil
}

// EnsurePoolACLResty grants a role to a user for a pool using the Resty client.
// This operation is idempotent on the Proxmox API side.
//
// PUT /access/acl
func EnsurePoolACLResty(ctx context.Context, restyClient *RestyClient, userID, poolID, role string, propagate bool) error {
	if restyClient == nil {
		return fmt.Errorf("restyClient is nil")
	}
	if userID == "" {
		return fmt.Errorf("userID is required")
	}
	if poolID == "" {
		return fmt.Errorf("poolID is required")
	}
	if role == "" {
		return fmt.Errorf("role is required")
	}

	form := url.Values{}
	form.Set("path", poolPath(poolID))
	form.Set("users", userID)
	form.Set("roles", role)
	if propagate {
		form.Set("propagate", "1")
	}

	if err := restyClient.Put(ctx, "/access/acl", form, nil); err != nil {
		return fmt.Errorf("failed to grant ACL (role: %s, pool: %s, user: %s): %w", role, poolID, userID, err)
	}

	logger.Get().Info().Str("user", userID).Str("pool", poolID).Str("role", role).Bool("propagate", propagate).Msg("Granted pool ACL (resty)")
	return nil
}

// EnsureRoleResty creates a custom Proxmox role if it does not already exist using the Resty client.
// This function is idempotent.
//
// GET /access/roles/{roleid} to check existence, POST /access/roles to create.
func EnsureRoleResty(ctx context.Context, restyClient *RestyClient, roleID string, privileges []string) error {
	if restyClient == nil {
		return fmt.Errorf("restyClient is nil")
	}
	if roleID == "" {
		return fmt.Errorf("roleID is required")
	}
	if len(privileges) == 0 {
		return fmt.Errorf("at least one privilege is required for role %s", roleID)
	}

	// Check if role exists
	checkPath := fmt.Sprintf("/access/roles/%s", url.PathEscape(roleID))
	var probe map[string]any
	if err := restyClient.Get(ctx, checkPath, &probe); err == nil {
		logger.Get().Debug().Str("role", roleID).Msg("Role already exists; EnsureRoleResty is a no-op.")
		return nil
	}

	// Create role
	form := url.Values{}
	form.Set("roleid", roleID)
	form.Set("privs", strings.Join(privileges, ","))

	if err := restyClient.Post(ctx, "/access/roles", form, nil); err != nil {
		if isConflictError(err) {
			logger.Get().Warn().Err(err).Str("role", roleID).Msg("Role creation raced; treating as existing.")
			return nil
		}
		logger.Get().Error().Err(err).Str("role", roleID).Strs("privileges", privileges).Msg("Proxmox API error during role creation (resty)")
		return fmt.Errorf("failed to create role %s: %w", roleID, err)
	}

	logger.Get().Info().Str("role", roleID).Strs("privileges", privileges).Msg("Created custom role (resty)")
	return nil
}
