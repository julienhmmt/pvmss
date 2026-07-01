// Package apiv1 — shared input validators.
//
// All regexes used by request validation live here as package-level vars so
// they compile once at startup, not per-request. Validators return typed
// *errors.ValidationError so handlers can:
//
//   - branch with errors.Is/errors.As against the validation sentinel
//   - rely on writeAppError to surface the message as 400 bad-request
package apiv1

import (
	"net"
	"regexp"
	"strings"

	pverrors "pvmss/errors"
)

// Shared regexes (compiled once at startup).
var (
	cloudInitIDUnsafeRegex = regexp.MustCompile(`[^a-z0-9-]`)
	// ciUserRe restricts the cloud-init user to a POSIX-safe login name.
	ciUserRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,32}$`)
	// ciIPTokenRe matches a single Proxmox ipconfigN token (ip=, ip6=, gw=, gw6=, net=, net6=).
	ciIPTokenRe = regexp.MustCompile(`^(ip|ip6|gw|gw6|net|net6)=(.+)$`)
	// sshKeyPrefixRe matches the algorithm prefix of an OpenSSH public key line.
	sshKeyPrefixRe = regexp.MustCompile(`^(ssh-rsa|ssh-ed25519|ssh-dss|ecdsa-sha2-nistp(256|384|521)|sk-ecdsa-sha2-nistp(256|384|521)|sk-ssh-ed25519) `)
	// domainLabelRe matches a single DNS label (RFC 1035, relaxed length).
	domainLabelRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
	// poolNameRegex restricts a pool name to characters safe for Proxmox
	// pool-id concatenation and API paths (letters, digits, hyphens,
	// underscores; 1..50 chars).
	poolNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)
)

// validatePoolName rejects pool names that could inject unexpected characters
// into the Proxmox API path or pool-id concatenation. Returns a
// *ValidationError on failure.
func validatePoolName(name string) error {
	if !poolNameRegex.MatchString(name) {
		return pverrors.ValidationErr(
			"pool",
			name,
			"use only letters, digits, hyphens, underscores (max 50 chars)",
		)
	}
	return nil
}

// validateTagName checks that name matches the tag-name format (letters,
// digits, hyphens, underscores; 1..50 chars). Returns a *ValidationError on
// failure.
func validateTagName(name string) error {
	if !tagNameRegex.MatchString(name) {
		return pverrors.ValidationErr(
			"name",
			name,
			"use only letters, digits, hyphens, underscores (max 50 chars)",
		)
	}
	return nil
}

// validateCIUser checks a cloud-init username. Empty input is rejected; callers
// treat empty as "clear" and bypass this validator.
func validateCIUser(user string) error {
	if !ciUserRe.MatchString(user) {
		return pverrors.ValidationErr("user", user, "use only letters, digits, dots, hyphens, underscores (max 32 chars)")
	}
	return nil
}

// validateCIPassword checks a cloud-init password length and rejects control
// characters. Proxmox hashes the value server-side, so we do not constrain the
// character set beyond printable bytes.
func validateCIPassword(pwd string) error {
	if len(pwd) == 0 || len(pwd) > 256 {
		return pverrors.ValidationErr("password", "", "password must be 1..256 characters")
	}
	for _, ch := range pwd {
		if ch < 0x20 || ch == 0x7f {
			return pverrors.ValidationErr("password", "", "password must not contain control characters")
		}
	}
	return nil
}

// validateCISSHKeys checks a newline-separated list of OpenSSH public keys.
// Empty input is rejected; callers treat empty as "clear".
func validateCISSHKeys(keys string) error {
	const maxKeys = 40
	const maxBytes = 8192
	trimmed := strings.TrimSpace(keys)
	if len(trimmed) == 0 {
		return pverrors.ValidationErr("ssh_keys", "", "ssh_keys must not be empty")
	}
	if len(trimmed) > maxBytes {
		return pverrors.ValidationErr("ssh_keys", "", "ssh_keys too large (max 8 KiB)")
	}
	lines := strings.Split(trimmed, "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		count++
		if count > maxKeys {
			return pverrors.ValidationErr("ssh_keys", "", "too many ssh keys (max 40)")
		}
		if !sshKeyPrefixRe.MatchString(line) {
			return pverrors.ValidationErr("ssh_keys", "", "invalid ssh key line (expected an OpenSSH public key)")
		}
	}
	if count == 0 {
		return pverrors.ValidationErr("ssh_keys", "", "ssh_keys must not be empty")
	}
	return nil
}

// validateCIIPConfig checks a Proxmox ipconfigN string. Tokens are comma-
// separated and must match ip=|ip6=|gw=|gw6=|net=|net6=. ip=/ip6= values may be
// "dhcp"/"auto" or a valid IP/CIDR; gw=/gw6= values must be valid IPs.
func validateCIIPConfig(ipConfig string) error {
	trimmed := strings.TrimSpace(ipConfig)
	if trimmed == "" {
		return pverrors.ValidationErr("ip_config", "", "ip_config must not be empty")
	}
	for _, tok := range strings.Split(trimmed, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		m := ciIPTokenRe.FindStringSubmatch(tok)
		if m == nil {
			return pverrors.ValidationErr("ip_config", tok, "invalid ip_config token (expected ip=, ip6=, gw=, gw6=, net= or net6=)")
		}
		key, val := m[1], m[2]
		switch key {
		case "ip", "ip6":
			if val == "dhcp" || val == "auto" {
				continue
			}
			if _, _, err := net.ParseCIDR(val); err != nil {
				return pverrors.ValidationErr("ip_config", val, "ip= value must be dhcp, auto, or a valid IP/CIDR")
			}
		case "gw", "gw6":
			if net.ParseIP(val) == nil {
				return pverrors.ValidationErr("ip_config", val, "gw= value must be a valid IP address")
			}
		case "net", "net6":
			if _, _, err := net.ParseCIDR(val); err != nil {
				return pverrors.ValidationErr("ip_config", val, "net= value must be a valid network/CIDR")
			}
		}
	}
	return nil
}

// validateCIDNSList checks a comma-separated list of IP addresses (used for
// nameserver). Empty input is rejected; callers treat empty as "clear".
func validateCIDNSList(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return pverrors.ValidationErr("nameserver", "", "nameserver must not be empty")
	}
	for _, part := range strings.Split(trimmed, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if net.ParseIP(part) == nil {
			return pverrors.ValidationErr("nameserver", part, "nameserver entries must be valid IP addresses")
		}
	}
	return nil
}

// validateCISearchdomain checks a comma-separated list of DNS search domains.
// Empty input is rejected; callers treat empty as "clear".
func validateCISearchdomain(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return pverrors.ValidationErr("searchdomain", "", "searchdomain must not be empty")
	}
	for _, part := range strings.Split(trimmed, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(part) > 253 {
			return pverrors.ValidationErr("searchdomain", part, "searchdomain entry too long (max 253 chars)")
		}
		for _, label := range strings.Split(part, ".") {
			if !domainLabelRe.MatchString(label) {
				return pverrors.ValidationErr("searchdomain", part, "searchdomain entries must be valid domain names")
			}
		}
	}
	return nil
}
