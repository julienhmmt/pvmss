package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// vmConfigEndpointFmt is the Proxmox REST path for a VM's config endpoint,
// formatted with the URL-escaped node name and the numeric VMID.
const vmConfigEndpointFmt = "/nodes/%s/qemu/%d/config"

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
// url.PathEscape: RFC3986 leaves path-segment sub-delims — notably '@',
// which every "user@host" SSH key comment contains — unescaped, and Proxmox
// requires those escaped too ("sshkeys: invalid format - invalid urlencoded
// string", confirmed live). url.QueryEscape escapes those, matching what
// ProxMate's encodeURIComponent produces, but it also turns space into '+'
// — and Proxmox decodes with Perl's uri_unescape, which does NOT turn '+'
// back into a space, corrupting the key. Escaping first and then rewriting
// only the '+' that QueryEscape used for space back to '%20' gets both
// right: QueryEscape already turned any literal '+' in the input into
// "%2B", so every remaining '+' in its output is unambiguously an encoded
// space. The read side (parseCloudInitConfig) stays PathUnescape — a
// general percent-decoder that handles %40, %20, %2B and everything else
// this produces identically.
func encodeSSHKeys(keys []string) string {
	return strings.ReplaceAll(url.QueryEscape(strings.Join(keys, "\n")), "+", "%20")
}

func parseCloudInitConfig(cfg proxmoxVMConfig) CloudInitConfig {
	result := CloudInitConfig{IPMode: CloudInitIPModeDHCP, User: cfg.str("ciuser"), Agent: agentEnabled(cfg.str("agent"))}

	if keys := cfg.str("sshkeys"); keys != "" {
		// encodeSSHKeys never emits a literal '+' (every space and literal
		// '+' in the input is percent-encoded), so a plain %XX decoder is
		// the exact inverse — matching Proxmox's own uri_unescape on the
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
// "[1|0][,frozen=[1|0]]" — the first comma token decides, so "0,frozen=1" is
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

// FindSnippetStorage implements CloudInitReader. PVMSS can only write to the
// one snippet directory the administrator configured for the cluster
// (spec D1), so this returns p.SnippetStorage — but only after proving the
// node lists it as an active snippets provider: a mistyped id or a storage
// without the snippets content flag must not produce a cicustom pointing at
// nothing. With no write target configured it reports
// ErrSnippetWriteUnavailable.
func (p Proxmox) FindSnippetStorage(ctx context.Context, node string) (string, error) {
	if !p.SnippetWriteAvailable() {
		return "", ErrSnippetWriteUnavailable
	}

	rows, err := listSnippetStorages(ctx, p.rest(), node)
	if err != nil {
		return "", err
	}

	for _, row := range rows {
		if row.Storage == p.SnippetStorage && row.Active == 1 {
			return p.SnippetStorage, nil
		}
	}

	return "", fmt.Errorf("%w: storage %q is not snippets-enabled or not active on node %s", ErrNotFound, p.SnippetStorage, node)
}

// snippetStorageRow is one row of GET /nodes/{node}/storage?content=snippets.
type snippetStorageRow struct {
	Storage string `json:"storage"`
	Active  int    `json:"active"`
	Shared  int    `json:"shared"`
}

func listSnippetStorages(ctx context.Context, rest proxmoxRESTClient, node string) ([]snippetStorageRow, error) {
	raw, err := rest.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/storage", url.PathEscape(node)), url.Values{"content": {"snippets"}})
	if err != nil {
		return nil, err
	}

	var rows []snippetStorageRow
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode node storages: %w", err)
	}

	return rows, nil
}

// proxmoxFindSnippetStorage picks any active snippet-capable storage on the
// node — the fallback for the cloud-init DRIVE placement (ide3), which only
// needs a storage the VM's node can see. It is unrelated to the configured
// snippet write target, which FindSnippetStorage owns.
func proxmoxFindSnippetStorage(ctx context.Context, rest proxmoxRESTClient, node string) (string, error) {
	rows, err := listSnippetStorages(ctx, rest, node)
	if err != nil {
		return "", err
	}

	// Prefer a shared storage over a node-local one: a snippet on local
	// storage is invisible to the other nodes, so a later migration would
	// leave the VM pointing at a file it cannot read (ticket 04). Inactive
	// storages are skipped outright. Proxmox reports the flags as 1/0.
	best := ""

	for _, row := range rows {
		if row.Active != 1 {
			continue
		}

		if row.Shared == 1 {
			return row.Storage, nil
		}

		if best == "" {
			best = row.Storage
		}
	}

	if best == "" {
		return "", ErrNotFound
	}

	return best, nil
}

// EnsureCloudInitDrive implements Writer: idempotently ensures the fixed
// cloudInitDiskKey slot (proxmox_config.go) holds a cloud-init drive. The
// drive's storage is the VM's own first data disk's storage when one exists
// (always valid, since the VM already has a disk there), falling back to a
// snippet-capable storage on node otherwise.
func (p Proxmox) EnsureCloudInitDrive(ctx context.Context, node string, vmid int) error {
	rest := p.rest()

	cfg, err := fetchVMConfig(ctx, rest, node, vmid)
	if err != nil {
		return err
	}

	if value, ok := cfg[cloudInitDiskKey].(string); ok && strings.Contains(value, "cloudinit") {
		return nil
	}

	storage, err := cloudInitDriveStorage(ctx, rest, node, cfg)
	if err != nil {
		return err
	}

	_, err = rest.do(ctx, http.MethodPut, fmt.Sprintf(vmConfigEndpointFmt, url.PathEscape(node), vmid), url.Values{
		cloudInitDiskKey: {storage + ":cloudinit"},
	})

	return err
}

func cloudInitDriveStorage(ctx context.Context, rest proxmoxRESTClient, node string, cfg proxmoxVMConfig) (string, error) {
	disks, _ := parseDisks(cfg)
	if len(disks) > 0 {
		return disks[0].Storage, nil
	}

	return proxmoxFindSnippetStorage(ctx, rest, node)
}

// SetCloudInitConfig implements Writer. It ensures the cloud-init drive
// exists first — Proxmox silently ignores ciuser/sshkeys/ipconfig0/... params
// without one — matching the fake's own EnsureCloudInitDrive-first contract.
func (p Proxmox) SetCloudInitConfig(ctx context.Context, node string, vmid int, config CloudInitConfig) error {
	if err := p.EnsureCloudInitDrive(ctx, node, vmid); err != nil {
		return err
	}

	form := url.Values{}
	if config.User != "" {
		form.Set("ciuser", config.User)
	}

	if len(config.SSHKeys) > 0 {
		form.Set("sshkeys", encodeSSHKeys(config.SSHKeys))
	}

	form.Set("ipconfig0", encodeIPConfig(config))

	if config.DNSServer != "" {
		form.Set("nameserver", config.DNSServer)
	}

	if config.SearchDomain != "" {
		form.Set("searchdomain", config.SearchDomain)
	}

	_, err := p.rest().do(ctx, http.MethodPut, fmt.Sprintf(vmConfigEndpointFmt, url.PathEscape(node), vmid), form)

	return err
}

// HasSnippet implements Writer by listing storage's snippets content and
// checking for filename — the visibility proof after PushCloudInitSnippet:
// the write went through the mount, this confirms Proxmox sees it.
func (p Proxmox) HasSnippet(ctx context.Context, node, storage, filename string) (bool, error) {
	found, err := proxmoxListContent(ctx, p.rest(), node, storage, "snippets")
	if err != nil {
		return false, err
	}

	for _, f := range found {
		if f.File == filename {
			return true, nil
		}
	}

	return false, nil
}

// AttachCloudInitSnippet points the VM at an already-uploaded snippet file
// through the vendor-data slot. vendor-data MERGES with the generated
// user-data, so ciuser/sshkeys/ipconfig0 keep applying; a user= slot would
// replace the generated user-data and silently drop the structured config.
// An empty filename detaches the snippet by clearing cicustom.
//
// Like SetCloudInitConfig, the attach path ensures the cloud-init drive first:
// without one in the fixed ide3 slot, Proxmox silently ignores cicustom — no
// seed ISO is generated and the snippet never reaches the guest. Detaching
// needs no drive, so the ensure runs only when a filename is given; the
// contract matches the fake's own ordering (ticket 03).
func (p Proxmox) AttachCloudInitSnippet(ctx context.Context, node, storage, filename string, vmid int) error {
	if filename != "" {
		if err := p.EnsureCloudInitDrive(ctx, node, vmid); err != nil {
			return err
		}
	}

	form := url.Values{}

	if filename == "" {
		form.Set(actionDelete, "cicustom")
	} else {
		form.Set("cicustom", fmt.Sprintf("vendor=%s:snippets/%s", storage, filename))
	}

	_, err := p.rest().do(ctx, http.MethodPut, fmt.Sprintf(vmConfigEndpointFmt, url.PathEscape(node), vmid), form)

	return err
}

// SetCloudInitPassword applies the cloud-init password for user via the QEMU
// guest agent so it lands only in /etc/shadow on the guest. It deliberately
// does NOT use the cipassword config key: Proxmox writes that as a crypt hash
// on the cloud-init seed drive (/dev/sr0) and cloud-init caches the same
// user-data under /var/lib/cloud on the root disk — both readable by any
// tenant root for the VM's lifetime. The agent path avoids the seed drive
// entirely. user is the VM's own ciuser — a cloud image's account is
// debian/ubuntu and root is locked, so a hardcoded "root" would write the
// password onto an account nobody can log into (ticket 02). Requires a
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
// user" — the caller retries within its bounded window (ticket 05).
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
// started pends until timeout — a short per-attempt bound keeps each probe
// cheap; the caller polls instead of retrying.
const agentPingTimeout = 3 * time.Second

// PingGuestAgent implements Writer: one POST to the guest-agent ping endpoint
// with a short timeout and no retry (POSTs are never retried anyway). Any
// error — unreachable, VM stopped, agent not up — means "not ready yet"; the
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

func encodeIPConfig(config CloudInitConfig) string {
	if config.IPMode == CloudInitIPModeStatic && config.IPAddress != "" {
		if config.Gateway != "" {
			return fmt.Sprintf("ip=%s,gw=%s", config.IPAddress, config.Gateway)
		}

		return "ip=" + config.IPAddress
	}

	return "ip=dhcp"
}

// sshKeyAddScript is the fixed guest-side script the SSH key is appended
// through. The username and key are passed as positional argv (see
// AddSSHKey), not interpolated into this string, so a crafted key cannot
// break out of the append. The append is idempotent (ticket 07): a
// missing trailing newline is repaired before appending — otherwise the new
// key glues onto the last existing line and invalidates both — and an exact
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
// arguments — no shell interpolation of the key — so a multi-line or
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

	// First poll runs immediately — the guest process may have already exited
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

// snippetFilenameRE is the only shape PVMSS ever writes: a pvmss- prefix,
// a safe body, a yaml extension. Callers build names from VMIDs, but the
// writer re-checks so a bug elsewhere cannot escape the snippet directory.
var snippetFilenameRE = regexp.MustCompile(`^pvmss-[A-Za-z0-9._-]+\.ya?ml$`)

// SnippetWriteAvailable implements Writer.
func (p Proxmox) SnippetWriteAvailable() bool {
	return p.SnippetDir != "" && p.SnippetStorage != ""
}

// PushCloudInitSnippet implements Writer by writing content into the
// cluster's configured snippet directory. There is no Proxmox API for this
// (the upload endpoint's content enum is iso/vztmpl/import); the directory
// is the storage's own snippets/ dir, bind-mounted into the PVMSS process
// (spec D1). temp-file + rename is atomic, so Proxmox never reads a
// half-written file, and a retry simply overwrites. node and vmid are
// unused: the filename already carries the VM.
func (p Proxmox) PushCloudInitSnippet(_ context.Context, _, storage, filename string, _ int, content string) error {
	if !p.SnippetWriteAvailable() {
		return ErrSnippetWriteUnavailable
	}

	if storage != p.SnippetStorage {
		return fmt.Errorf("snippet storage %q is not this cluster's configured snippet storage %q", storage, p.SnippetStorage)
	}

	if !snippetFilenameRE.MatchString(filename) || filepath.Base(filename) != filename {
		return fmt.Errorf("refusing to write snippet with unsafe filename %q", filename)
	}

	return writeFileAtomic(p.SnippetDir, filename, content)
}

// writeFileAtomic writes content to dir/filename via a temp file and rename,
// mode 0644 (cloud-init on the Proxmox node reads it as a non-root user).
// dir must already exist: it is the administrator-mounted snippets/ share —
// creating it silently would mask a missing mount and drop the document into
// the container's local filesystem where Proxmox can never see it.
func writeFileAtomic(dir, filename, content string) (err error) {
	tmp, err := os.CreateTemp(dir, ".pvmss-*.tmp")
	if err != nil {
		return fmt.Errorf("create snippet temp file: %w", err)
	}

	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err = tmp.WriteString(content); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("write snippet: %w", err)
	}

	if err = tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("chmod snippet: %w", err)
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close snippet: %w", err)
	}

	if err = os.Rename(tmp.Name(), filepath.Join(dir, filename)); err != nil {
		return fmt.Errorf("publish snippet: %w", err)
	}

	return nil
}
