package proxmox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"pvmss/constants"
	"pvmss/logger"
)

// TicketResponse represents the response from the /access/ticket endpoint.
type TicketResponse struct {
	Cap                 any    `json:"cap,omitempty"`
	Clustername         string `json:"clustername,omitempty"`
	CSRFPreventionToken string `json:"CSRFPreventionToken"`
	Ticket              string `json:"ticket"`
	Username            string `json:"username"`
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

// HasRole checks if the user has a specific role in their capabilities.
// This function parses the Cap field from TicketResponse to determine user roles.
//
// The Cap field can have different structures depending on Proxmox version:
//
//  1. Legacy format (array of role names):
//     {
//     "/": ["PVEAdmin", "PVEDatastoreUser", ...],
//     "/pool/pool1": ["PVEPoolUser"],
//     ...
//     }
//
//  2. New format (permissions map):
//     {
//     "/": {"PVEAdmin": 1, "PVEDatastoreUser": 1, ...},
//     "nodes": {"Sys.Audit":1, "Sys.Console":1, ...},
//     "storage": {"Datastore.Allocate":1, ...},
//     ...
//     }
//
// Parameters:
//   - cap: The capabilities object from TicketResponse.Cap
//   - role: The role to check for (e.g., "PVEAdmin")
//
// Returns:
//   - true if the user has the role or equivalent admin permissions, false otherwise
func HasRole(cap any, role string) bool {
	if cap == nil {
		return false
	}

	// The cap field is a map[string]any where keys are paths and values are role data
	capMap, ok := cap.(map[string]any)
	if !ok {
		logger.Get().Warn().Interface("cap", cap).Msg("Capabilities field is not a map")
		return false
	}

	// Check for explicit role in legacy format (arrays)
	for path, roles := range capMap {
		// Try legacy format first (array of strings)
		if rolesSlice, ok := roles.([]any); ok {
			for _, r := range rolesSlice {
				if roleStr, ok := r.(string); ok && roleStr == role {
					logger.Get().Debug().
						Str("role", role).
						Str("path", path).
						Interface("all_roles", capMap).
						Msg("User has required role (legacy format)")
					return true
				}
			}
		}

		// Try new format (map of permissions)
		if rolesMap, ok := roles.(map[string]any); ok {
			// Check for explicit role name
			if _, exists := rolesMap[role]; exists {
				logger.Get().Debug().
					Str("role", role).
					Str("path", path).
					Interface("all_roles", capMap).
					Msg("User has required role (new format)")
				return true
			}

			// For PVEAdmin role, also check for admin-level permissions
			if role == "PVEAdmin" {
				// Check if user has broad admin permissions across multiple domains
				adminDomains := []string{"nodes", "storage", "vms", "dc"}
				adminPermissionCount := 0

				for _, domain := range adminDomains {
					if domainPerms, exists := capMap[domain]; exists {
						if domainPermsMap, ok := domainPerms.(map[string]any); ok && len(domainPermsMap) > 0 {
							adminPermissionCount++
						}
					}
				}

				// If user has permissions in most admin domains, consider them admin
				if adminPermissionCount >= 3 { // nodes, storage, vms minimum
					logger.Get().Debug().
						Str("role", role).
						Int("admin_domains", adminPermissionCount).
						Interface("capabilities", capMap).
						Msg("User has admin-level permissions (inferred PVEAdmin)")
					return true
				}
			}
		} else {
			logger.Get().Warn().Str("path", path).Interface("roles", roles).Msg("Roles field is not an array or map")
		}
	}

	logger.Get().Debug().
		Str("role", role).
		Interface("capabilities", capMap).
		Msg("User does not have required role")
	return false
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

// boolToForm converts a boolean to a form-compatible string ("1" or "0").
func boolToForm(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
