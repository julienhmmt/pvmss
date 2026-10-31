package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
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

	return first == "1"
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

// SetCloudInitPassword applies the cloud-init password for user via the QEMU
// guest agent so it lands only in /etc/shadow on the guest. It deliberately
// does NOT use the cipassword config key: Proxmox writes that as a crypt hash
// on the cloud-init seed drive (/dev/sr0) and cloud-init caches the same
// user-data under /var/lib/cloud on the root disk - both readable by any
// tenant root for the VM's lifetime. The agent path avoids the seed drive
// entirely. user is the VM's own ciuser - a cloud image's account is
// debian/ubuntu and root is locked, so a hardcoded "root" would write the
// password onto an account nobody can log into. Requires a
// running guest with qemu-guest-agent enabled; callers surface a clear error
// when the agent is unavailable.
func (p Proxmox) SetCloudInitPassword(ctx context.Context, node string, vmid int, user, password string) error {
	form := url.Values{}
	form.Set("username", user)
	form.Set("password", password)

	_, err := p.rest().do(ctx, http.MethodPut, fmt.Sprintf("/nodes/%s/qemu/%d/agent/set-user-password", url.PathEscape(node), vmid), form)
	if err != nil {
		if isGuestUserUnknown(err) {
			return fmt.Errorf("%w: %w", ErrGuestUserUnknown, err)
		}

		return fmt.Errorf("set user password via guest agent: %w", err)
	}

	return nil
}

// guestUserUnknownMarkers are the substrings Proxmox's guest-agent layer
// reports when the target account does not exist on the guest. cloud-init
// creates the account mid-boot, so this error means "too early", not "wrong
// user" - the caller retries within its bounded window.
var guestUserUnknownMarkers = []string{"does not exist", "no such user"}

// isGuestUserUnknown reports whether err is the guest agent's user-not-found
// rejection. The wording varies across Proxmox/QGA versions, so both known
// phrasings are matched case-insensitively; anything else is surfaced as-is.
func isGuestUserUnknown(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return slices.ContainsFunc(guestUserUnknownMarkers, func(marker string) bool {
		return strings.Contains(msg, marker)
	})
}

// agentPingTimeout bounds one guest-agent ping. An agent configured but not
// started pends until timeout - a short per-attempt bound keeps each probe
// cheap; the caller polls instead of retrying.
const agentPingTimeout = 3 * time.Second

// PingGuestAgent implements Writer: one POST to the guest-agent ping endpoint
// with a short timeout and no retry (POSTs are never retried anyway). Any
// error - unreachable, VM stopped, agent not up - means "not ready yet"; the
// caller decides whether to keep polling.
func (p Proxmox) PingGuestAgent(ctx context.Context, node string, vmid int) error {
	ctx, cancel := context.WithTimeout(ctx, agentPingTimeout)
	defer cancel()

	_, err := p.rest().withNoRetry().do(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/agent/ping", url.PathEscape(node), vmid), nil)
	if err != nil {
		return fmt.Errorf("guest agent ping: %w", err)
	}

	return nil
}

// sshKeyAddScript is the fixed guest-side script the SSH key is appended
// through. The username and key are passed as positional argv (see
// AddSSHKey), not interpolated into this string, so a crafted key cannot
// break out of the append. The append is idempotent: a
// missing trailing newline is repaired before appending - otherwise the new
// key glues onto the last existing line and invalidates both - and an exact
// whole-line duplicate is a no-op, so a retry after a network error does not
// duplicate. The newline guard is an explicit if because its condition chain
// legitimately returns non-zero when false, which set -e would treat as
// fatal.
const sshKeyAddScript = `#!/bin/sh
set -eu
user="$1"
key="$2"
home=$(getent passwd "$user" | cut -d: -f6)
[ -n "$home" ] || { echo "user $user not found" >&2; exit 3; }
auth="$home/.ssh/authorized_keys"
mkdir -p "$home/.ssh"
chmod 700 "$home/.ssh"
touch "$auth"
if [ -s "$auth" ] && [ -n "$(tail -c1 "$auth")" ]; then
	echo >> "$auth"
fi
grep -qxF "$key" "$auth" || printf '%s\n' "$key" >> "$auth"
chmod 600 "$auth"
owner=$(getent passwd "$user" | cut -d: -f3,4)
chown "$owner" "$home/.ssh" "$auth"
`

// AddSSHKey injects a single public key into the running guest's
// authorized_keys through the QEMU guest agent. The guest agent executes the
// fixed script (sshKeyAddScript) with the username and key as positional
// arguments - no shell interpolation of the key - so a multi-line or
// malicious value cannot smuggle extra commands. The call is async on the
// guest: Proxmox returns a pid from agent/exec that we then poll via
// agent/exec-status until it exits.
func (p Proxmox) AddSSHKey(ctx context.Context, node string, vmid int, user, key string) error {
	form := url.Values{}
	form["command"] = append(form["command"], "/bin/sh")
	form["command"] = append(form["command"], "-c")
	form["command"] = append(form["command"], sshKeyAddScript)
	form["command"] = append(form["command"], user)
	form["command"] = append(form["command"], key)

	raw, err := p.rest().do(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec", url.PathEscape(node), vmid), form)
	if err != nil {
		return fmt.Errorf("guest agent exec ssh-key add: %w", err)
	}

	pid, err := decodeAgentExecPID(raw)
	if err != nil {
		return err
	}

	return p.waitAgentExec(ctx, node, vmid, pid)
}

// decodeAgentExecPID extracts the pid from a guest-agent exec response
// ({"data":{"pid":N}}).
func decodeAgentExecPID(raw json.RawMessage) (int, error) {
	var envelope struct {
		PID int `json:"pid"`
	}

	if err := decodeData(raw, &envelope); err != nil {
		return 0, fmt.Errorf("decode agent exec pid: %w", err)
	}

	if envelope.PID == 0 {
		return 0, errors.New("guest agent exec returned no pid")
	}

	return envelope.PID, nil
}

// agentExecPoll is the interval between exec-status reads. maxAgentExecWait
// bounds the total wait. Both are vars so tests can shorten them, mirroring
// maxForceStopWait in vm/actions.go. Without a pause the loop hammers
// /agent/exec-status as fast as the network allows (hundreds of calls for a
// two-second guest script), and reconstructing p.rest() per poll opened a fresh
// TLS connection every iteration.
var (
	agentExecPoll    = 500 * time.Millisecond
	maxAgentExecWait = 15 * time.Second
)

// agentExecStatus is one poll of the guest-agent exec-status endpoint.
type agentExecStatus struct {
	Exited   bool   `json:"exited"`
	ExitCode int    `json:"exitcode"`
	OutData  string `json:"out-data"`
	ErrData  string `json:"err-data"`
}

// waitAgentExec polls agent/exec-status until the guest process exits, bounded
// by maxAgentExecWait. A zero exit code means success; exit code 3 means the
// user does not exist on the guest. Any other non-zero exit surfaces the
// guest's stderr. The rest client is built once (outside the loop) so every
// poll reuses one connection; the ticker/deadline shape mirrors deleteWithRetry
// (vm/actions.go). On timeout it returns a symptom-named error pointing at the
// missing qemu-guest-agent rather than an opaque context.DeadlineExceeded.
func (p Proxmox) waitAgentExec(ctx context.Context, node string, vmid, pid int) error {
	rest := p.rest().withNoRetry()
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec-status?pid=%d", url.PathEscape(node), vmid, pid)

	deadline := time.NewTimer(maxAgentExecWait)
	defer deadline.Stop()

	// First poll runs immediately - the guest process may have already exited
	// by the time we get here, and this keeps the common fast path tick-free.
	if exited, err := pollAgentExecStatus(ctx, rest, path); err != nil || exited {
		return err
	}

	ticker := time.NewTicker(agentExecPoll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for guest agent exec: %w", ctx.Err())
		case <-deadline.C:
			return errors.New("guest agent did not report exec completion within 15s (is qemu-guest-agent running?)")
		case <-ticker.C:
			exited, err := pollAgentExecStatus(ctx, rest, path)
			if err != nil {
				return err
			}

			if exited {
				return nil
			}
		}
	}
}

// pollAgentExecStatus performs one exec-status read and maps the exit code to
// its error semantics: nil on success or still-running, ErrSSHKeyUserUnknown
// on exit code 3, the guest's stderr on any other non-zero exit. The returned
// bool is true once the guest process has exited (caller should stop polling).
func pollAgentExecStatus(ctx context.Context, rest proxmoxRESTClient, path string) (bool, error) {
	raw, err := rest.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return false, fmt.Errorf("poll guest agent exec-status: %w", err)
	}

	var status agentExecStatus
	if err := decodeData(raw, &status); err != nil {
		return false, fmt.Errorf("decode agent exec-status: %w", err)
	}

	if !status.Exited {
		return false, nil
	}

	switch status.ExitCode {
	case 0:
		return true, nil
	case 3:
		return true, fmt.Errorf("guest user does not exist: %w", ErrSSHKeyUserUnknown)
	default:
		msg := status.ErrData
		if msg == "" {
			msg = status.OutData
		}

		return true, fmt.Errorf("guest agent ssh-key add failed (exit %d): %s", status.ExitCode, msg)
	}
}
