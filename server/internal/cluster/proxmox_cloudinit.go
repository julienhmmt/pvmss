package cluster

import (
	"context"
	"net/url"
	"strings"
)

// GetCloudInitConfig implements CloudInitReader by reading the live VM
// config. Password is always left empty: Proxmox does not return cipassword
// on read (write-only), matching the fake's own cloneCloudInitConfig, which
// clears it too.
func (p Proxmox) GetCloudInitConfig(ctx context.Context, node string, vmid int) (CloudInitConfig, error) {
	cfg, err := fetchVMConfig(ctx, p.rest(), node, vmid)
	if err != nil {
		return CloudInitConfig{}, err
	}

	return parseCloudInitConfig(cfg), nil
}

// encodeSSHKeys percent-encodes a newline-joined key list for Proxmox's
// sshkeys form field. Proxmox's own format validator rejects the result of
// url.PathEscape: RFC3986 leaves path-segment sub-delims - notably '@',
// which every "user@host" SSH key comment contains - unescaped, and Proxmox
// requires those escaped too ("sshkeys: invalid format - invalid urlencoded
// string", confirmed live). url.QueryEscape escapes those, matching what
// ProxMate's encodeURIComponent produces, but it also turns space into '+'
// - and Proxmox decodes with Perl's uri_unescape, which does NOT turn '+'
// back into a space, corrupting the key. Escaping first and then rewriting
// only the '+' that QueryEscape used for space back to '%20' gets both
// right: QueryEscape already turned any literal '+' in the input into
// "%2B", so every remaining '+' in its output is unambiguously an encoded
// space. The read side (parseCloudInitConfig) stays PathUnescape - a
// general percent-decoder that handles %40, %20, %2B and everything else
// this produces identically.
func parseCloudInitConfig(cfg proxmoxVMConfig) CloudInitConfig {
	result := CloudInitConfig{IPMode: CloudInitIPModeDHCP, User: cfg.str("ciuser"), Agent: agentEnabled(cfg.str("agent"))}

	if keys := cfg.str("sshkeys"); keys != "" {
		// encodeSSHKeys never emits a literal '+' (every space and literal
		// '+' in the input is percent-encoded), so a plain %XX decoder is
		// the exact inverse - matching Proxmox's own uri_unescape on the
		// read side too, which likewise never turns '+' into a space.
		decoded, err := url.PathUnescape(keys)
		if err != nil {
			decoded = keys
		}

		for key := range strings.SplitSeq(decoded, "\n") {
			if key = strings.TrimSpace(key); key != "" {
				result.SSHKeys = append(result.SSHKeys, key)
			}
		}
	}

	if ipconfig := cfg.str("ipconfig0"); ipconfig != "" {
		parseIPConfig(ipconfig, &result)
	}

	result.DNSServer = cfg.str("nameserver")
	result.SearchDomain = cfg.str("searchdomain")

	return result
}

// agentEnabled parses the VM config's agent= flag. Proxmox's grammar is
// "[1|0][,frozen=[1|0]]" - the first comma token decides, so "0,frozen=1" is
// disabled and "1,frozen=1" is enabled.
func agentEnabled(raw string) bool {
	first, _, _ := strings.Cut(raw, ",")

	// Proxmox accepts both "1,..." and the keyed "enabled=1,..." form.
	return strings.TrimPrefix(first, "enabled=") == "1"
}

// parseIPConfig reads Proxmox's ipconfigN grammar ("ip=dhcp" or
// "ip=<addr>/<cidr>,gw=<gateway>") into result.
func parseIPConfig(raw string, result *CloudInitConfig) {
	for opt := range strings.SplitSeq(raw, ",") {
		key, val, ok := strings.Cut(opt, "=")
		if !ok {
			continue
		}

		switch key {
		case "ip":
			if val == "dhcp" {
				result.IPMode = CloudInitIPModeDHCP
			} else {
				result.IPMode = CloudInitIPModeStatic
				result.IPAddress = val
			}
		case "gw":
			result.Gateway = val
		}
	}
}
