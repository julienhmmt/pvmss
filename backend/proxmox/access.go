package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"pvmss/constants"
	"pvmss/logger"
)

// TicketResponse represents the response from the /access/ticket endpoint.
type TicketResponse struct {
	Username            string `json:"username"`
	Ticket              string `json:"ticket"`
	CSRFPreventionToken string `json:"CSRFPreventionToken"`
	Cap                 any    `json:"cap,omitempty"`
	Clustername         string `json:"clustername,omitempty"`
}

// CreateTicketOptions holds optional parameters for ticket creation.
type CreateTicketOptions struct {
	// Realm for authentication (default: "pam")
	Realm string
	// OTP for two-factor authentication (optional)
	OTP string
	// Path for permission verification (optional)
	Path string
	// Privs for privilege verification (optional)
	Privs string
}

// CreateTicket creates a new authentication ticket using username and password.
// This is the primary authentication method for Proxmox API access.
//
// POST /access/ticket
// Parameters:
//   - username: Username in format "user@realm" or just "user" (realm defaults to "pam")
//   - password: User password
//   - opts: Optional parameters (realm, OTP, path, privs)
//
// Returns:
//   - TicketResponse containing the ticket, CSRF token, and user capabilities
//
// The ticket is valid for 2 hours and must be sent as a cookie (PVEAuthCookie) in subsequent requests.
// The CSRFPreventionToken must be included in the header for any state-changing operations (POST, PUT, DELETE).
func CreateTicket(ctx context.Context, client ClientInterface, username, password string, opts *CreateTicketOptions) (*TicketResponse, error) {
	if err := validateClientAndParams(client, param{"username", username}, param{"password", password}); err != nil {
		return nil, err
	}

	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	// Apply defaults
	if opts == nil {
		opts = &CreateTicketOptions{}
	}
	if opts.Realm == "" {
		opts.Realm = constants.DefaultLoginRealm
	}

	// Ensure username has realm suffix
	if !strings.Contains(username, "@") {
		username = fmt.Sprintf("%s@%s", username, opts.Realm)
	}

	// Build request parameters
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

	// Make the API request
	var respData struct {
		Data TicketResponse `json:"data"`
	}

	if err := client.PostFormAndGetJSON(ctx, "/access/ticket", params, &respData); err != nil {
		logger.Get().Error().Err(err).Str("username", username).Msg("Failed to create authentication ticket")
		return nil, fmt.Errorf("failed to create ticket for %s: %w", username, err)
	}

	// Validate response
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
		Msg("Authentication ticket created successfully")

	return &respData.Data, nil
}

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

// UpdateUserPassword updates the password for an existing Proxmox user.
// This function uses the PUT /access/password endpoint.
//
// Parameters:
//   - username: Username (will be normalized to user@realm format)
//   - password: New password for the user
//   - confirmPassword: Password confirmation (required by Proxmox API)
//   - realm: Authentication realm (defaults to "pve")
//
// Returns:
//   - error if the password update fails
//
// Note: This requires cookie-based authentication (PVEAuthCookie), not API tokens.
func UpdateUserPassword(ctx context.Context, client ClientInterface, username, password, confirmPassword, realm string) error {
	if err := validateClientAndParams(client, param{"username", username}, param{"password", password}); err != nil {
		return err
	}

	if realm == "" {
		realm = "pve"
	}
	uid := normalizeUserID(username, realm)

	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	// Build request parameters
	form := url.Values{}
	form.Set("userid", uid)
	form.Set("password", password)
	// Proxmox requires confirmation-password parameter
	if confirmPassword != "" {
		form.Set("confirmation-password", confirmPassword)
	}

	// Use PUT /access/password to update the password
	if _, err := client.PutFormWithContext(ctx, "/access/password", form); err != nil {
		logger.Get().Error().Err(err).Str("userid", uid).Msg("Failed to update user password")
		return fmt.Errorf("failed to update password for user %s: %w", uid, err)
	}

	logger.Get().Info().Str("userid", uid).Msg("Successfully updated user password")
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
		d = constants.ProxmoxDefaultTimeout
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
