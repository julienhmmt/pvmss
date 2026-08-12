package config

import "strconv"

// Field is one row of the redacted Configuration view exposed by
// GET /api/v1/admin/appinfo (T14 data-model.md). Value is the real configured
// value for non-secret fields, and empty for secret fields (Redacted == true).
// The HTTP layer serializes an empty Value as JSON null for redacted fields,
// never as a masked pattern that could leak length information.
type Field struct {
	Name     string
	Value    string
	Redacted bool
}

// Redacted returns every Configuration field as a Field, with every
// secret-shaped field redacted to an empty value and Redacted == true. This
// is a hardcoded, one-line-per-field table — not a reflection-based secret
// scanner (constitution VIII: no abstraction for a single caller). When T15
// adds per-cluster Proxmox tokens, it extends this function by adding one row
// per new secret field, the same extension shape T11 used on catalog.go.
//
// As of T14, the secret-shaped fields are:
//   - AdminPasswordHash (env ADMIN_PASSWORD_HASH, T02) — a bcrypt credential
//   - SessionSecret (env SESSION_SECRET, T02) — the shared session secret
//   - ProxmoxAPITokenValue (env PROXMOX_API_TOKEN_VALUE, T01) — a bearer
//     credential for the Proxmox service account
//
// ProxmoxAPITokenName is not redacted: it identifies the token (e.g.
// "pvmss@pve"), it is not itself a credential — the value is the secret half.
func (c Configuration) Redacted() []Field {
	return []Field{
		{Name: "Host", Value: c.Host},
		{Name: "Port", Value: strconv.Itoa(c.Port)},
		{Name: "DBPath", Value: c.DBPath},
		{Name: "LogLevel", Value: c.LogLevel},
		{Name: "LogFormat", Value: c.LogFormat},
		{Name: "LogOutput", Value: c.LogOutput},
		{Name: "WebDir", Value: c.WebDir},
		{Name: "ClusterSource", Value: c.ClusterSource},
		{Name: "SESSION_SECRET", Value: "", Redacted: true},
		{Name: "ADMIN_PASSWORD_HASH", Value: "", Redacted: true},
		{Name: "CookieSecure", Value: strconv.FormatBool(c.CookieSecure)},
		{Name: "ProxmoxURL", Value: c.ProxmoxURL},
		{Name: "ProxmoxAPITokenName", Value: c.ProxmoxAPITokenName},
		{Name: "PROXMOX_API_TOKEN_VALUE", Value: "", Redacted: true},
	}
}
