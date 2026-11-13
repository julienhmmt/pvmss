package config

import "strconv"

// Field is one row of the redacted Configuration view exposed by
// GET /api/v1/admin/appinfo. Name is the environment variable the admin
// sets (or a derived label like TLSMode when no single variable exists).
// Value is the real configured value for non-secret fields; secret fields
// (Redacted == true) carry an empty Value unless they came from
// Unredacted(). The HTTP layer serializes an empty Value as JSON null for
// redacted fields, never as a masked pattern that could leak length
// information.
type Field struct {
	Name     string
	Value    string
	Redacted bool
}

// Redacted returns every Configuration field as a Field, with every
// secret-shaped field redacted to an empty value and Redacted == true. This
// is a hardcoded, one-line-per-field table - not a reflection-based secret
// scanner (no abstraction for a single caller). When
// adds per-cluster Proxmox tokens, it extends fields() by adding one row
// per new secret field, the same extension shape used on catalog.go.
//
// the secret-shaped fields are:
// AdminPasswordHash (env ADMIN_PASSWORD_HASH) - a bcrypt credential
// SessionSecret (env SESSION_SECRET) - the shared session secret
// ProxmoxAPITokenValue (env PROXMOX_API_TOKEN_VALUE) - a bearer
// credential for the Proxmox service account
//
// ProxmoxAPITokenName is not redacted: it identifies the token (e.g.
// "pvmss@pve"), it is not itself a credential - the value is the secret half.
func (c Configuration) Redacted() []Field {
	fields := c.fields()
	for i := range fields {
		if fields[i].Redacted {
			fields[i].Value = ""
		}
	}

	return fields
}

// Unredacted returns every field with its real value, secrets included -
// the Redacted flag still marks which rows are secrets so the UI can mask
// them by default. Only for the admin-only ?reveal path of appinfo; the
// default response must keep using Redacted so secrets never leave the
// process (SC-006).
func (c Configuration) Unredacted() []Field {
	return c.fields()
}

func (c Configuration) fields() []Field {
	return []Field{
		{Name: "PVMSS_HOST", Value: c.Host},
		{Name: "PVMSS_PORT", Value: strconv.Itoa(c.Port)},
		{Name: "PVMSS_DB_PATH", Value: c.DBPath},
		{Name: "LOG_LEVEL", Value: c.LogLevel},
		{Name: "LOG_FORMAT", Value: c.LogFormat},
		{Name: "LOG_OUTPUT", Value: c.LogOutput},
		{Name: "PVMSS_WEB_DIR", Value: c.WebDir},
		{Name: "PVMSS_CLUSTER_SOURCE", Value: c.ClusterSource},
		{Name: "SESSION_SECRET", Value: c.SessionSecret, Redacted: true},
		{Name: "ADMIN_PASSWORD_HASH", Value: c.AdminPasswordHash, Redacted: true},
		{Name: "TLSMode", Value: c.TLSMode()},
		{Name: "PVMSS_TLS_CERT_FILE", Value: c.TLSCertFile},
		{Name: "PVMSS_TLS_CLIENT_CA_FILE", Value: c.TLSClientCAFile},
		{Name: "PVMSS_COOKIE_SECURE", Value: strconv.FormatBool(c.CookieSecure)},
		{Name: "PROXMOX_URL", Value: c.ProxmoxURL},
		{Name: "PROXMOX_API_TOKEN_NAME", Value: c.ProxmoxAPITokenName},
		{Name: "PROXMOX_API_TOKEN_VALUE", Value: c.ProxmoxAPITokenValue, Redacted: true},
	}
}
