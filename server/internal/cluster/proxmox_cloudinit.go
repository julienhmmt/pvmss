package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// errBuildSnippetUpload wraps multipart-builder failures while constructing a
// snippet upload request. Each call site wraps a distinct underlying error
// (WriteField, CreateFormFile, part.Write, writer.Close); only the literal is
// deduplicated.
const errBuildSnippetUpload = "build snippet upload: %w"

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

func parseCloudInitConfig(cfg proxmoxVMConfig) CloudInitConfig {
	result := CloudInitConfig{IPMode: CloudInitIPModeDHCP, User: cfg.str("ciuser")}

	if keys := cfg.str("sshkeys"); keys != "" {
		decoded, err := url.QueryUnescape(keys)
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

// FindSnippetStorage implements CloudInitReader.
func (p Proxmox) FindSnippetStorage(ctx context.Context, node string) (string, error) {
	return proxmoxFindSnippetStorage(ctx, p.rest(), node)
}

func proxmoxFindSnippetStorage(ctx context.Context, rest proxmoxRESTClient, node string) (string, error) {
	raw, err := rest.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/storage", url.PathEscape(node)), url.Values{"content": {"snippets"}})
	if err != nil {
		return "", err
	}

	var rows []struct {
		Storage string `json:"storage"`
	}
	if err := decodeData(raw, &rows); err != nil {
		return "", fmt.Errorf("decode node storages: %w", err)
	}

	if len(rows) == 0 {
		return "", ErrNotFound
	}

	return rows[0].Storage, nil
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

	_, err = rest.do(ctx, http.MethodPut, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), url.Values{
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
		form.Set("sshkeys", url.QueryEscape(strings.Join(config.SSHKeys, "\n")))
	}

	form.Set("ipconfig0", encodeIPConfig(config))

	if config.DNSServer != "" {
		form.Set("nameserver", config.DNSServer)
	}

	if config.SearchDomain != "" {
		form.Set("searchdomain", config.SearchDomain)
	}

	_, err := p.rest().do(ctx, http.MethodPut, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), form)

	return err
}

// AttachCloudInitSnippet points the VM at an already-uploaded snippet file
// through the vendor-data slot. vendor-data MERGES with the generated
// user-data, so ciuser/sshkeys/ipconfig0 keep applying; a user= slot would
// replace the generated user-data and silently drop the structured config.
// An empty filename detaches the snippet by clearing cicustom.
func (p Proxmox) AttachCloudInitSnippet(ctx context.Context, node, storage, filename string, vmid int) error {
	form := url.Values{}

	if filename == "" {
		form.Set("delete", "cicustom")
	} else {
		form.Set("cicustom", fmt.Sprintf("vendor=%s:snippets/%s", storage, filename))
	}

	_, err := p.rest().do(ctx, http.MethodPut, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), form)

	return err
}

// SetCloudInitPassword applies the cloud-init password via the QEMU guest
// agent so it lands only in /etc/shadow on the guest. It deliberately does NOT
// use the cipassword config key: Proxmox writes that as a crypt hash on the
// cloud-init seed drive (/dev/sr0) and cloud-init caches the same user-data
// under /var/lib/cloud on the root disk — both readable by any tenant root for
// the VM's lifetime. The agent path avoids the seed drive entirely. Requires a
// running guest with qemu-guest-agent enabled; callers surface a clear error
// when the agent is unavailable.
func (p Proxmox) SetCloudInitPassword(ctx context.Context, node string, vmid int, password string) error {
	form := url.Values{}
	form.Set("username", "root")
	form.Set("password", password)

	_, err := p.rest().do(ctx, http.MethodPut, fmt.Sprintf("/nodes/%s/qemu/%d/agent/set-user-password", url.PathEscape(node), vmid), form)
	if err != nil {
		return fmt.Errorf("set user password via guest agent: %w", err)
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
// break out of the append.
const sshKeyAddScript = `#!/bin/sh
set -eu
user="$1"
key="$2"
home=$(getent passwd "$user" | cut -d: -f6)
[ -n "$home" ] || { echo "user $user not found" >&2; exit 3; }
auth="$home/.ssh/authorized_keys"
mkdir -p "$home/.ssh"
chmod 700 "$home/.ssh"
echo "$key" >> "$auth"
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
		return 0, fmt.Errorf("guest agent exec returned no pid")
	}

	return envelope.PID, nil
}

// agentExecStatus is one poll of the guest-agent exec-status endpoint.
type agentExecStatus struct {
	Exited   bool   `json:"exited"`
	ExitCode int    `json:"exitcode"`
	OutData  string `json:"out-data"`
	ErrData  string `json:"err-data"`
}

// waitAgentExec polls agent/exec-status until the guest process exits. A zero
// exit code means success; exit code 3 means the user does not exist on the
// guest. Any other non-zero exit surfaces the guest's stderr.
func (p Proxmox) waitAgentExec(ctx context.Context, node string, vmid, pid int) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for guest agent exec: %w", ctx.Err())
		default:
		}

		raw, err := p.rest().do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec-status?pid=%d", url.PathEscape(node), vmid, pid), nil)
		if err != nil {
			return fmt.Errorf("poll guest agent exec-status: %w", err)
		}

		var status agentExecStatus
		if err := decodeData(raw, &status); err != nil {
			return fmt.Errorf("decode agent exec-status: %w", err)
		}

		if !status.Exited {
			continue
		}

		switch status.ExitCode {
		case 0:
			return nil
		case 3:
			return fmt.Errorf("guest user does not exist: %w", ErrSSHKeyUserUnknown)
		default:
			msg := status.ErrData
			if msg == "" {
				msg = status.OutData
			}

			return fmt.Errorf("guest agent ssh-key add failed (exit %d): %s", status.ExitCode, msg)
		}
	}
}

// PushCloudInitSnippet implements Writer. Proxmox has no dedicated "write a
// snippet" API call — a snippet is just a file under a snippets-capable
// storage's directory, written through the storage's own multipart upload
// endpoint with content type "snippets". vmid is unused: the filename already
// encodes the VM (vm/cloudinit.go's snippetFilenamePrefix), matching the
// interface signature every other implementation shares.
func (p Proxmox) PushCloudInitSnippet(ctx context.Context, node, storage, filename string, _ int, content string) error {
	rest := p.rest()

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("content", "snippets"); err != nil {
		return fmt.Errorf(errBuildSnippetUpload, err)
	}

	part, err := writer.CreateFormFile("filename", filename)
	if err != nil {
		return fmt.Errorf(errBuildSnippetUpload, err)
	}

	if _, err := part.Write([]byte(content)); err != nil {
		return fmt.Errorf(errBuildSnippetUpload, err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf(errBuildSnippetUpload, err)
	}

	path := fmt.Sprintf("/nodes/%s/storage/%s/upload", url.PathEscape(node), url.PathEscape(storage))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rest.base+path, &body)
	if err != nil {
		return fmt.Errorf("build snippet upload request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	rest.authenticate(req)

	resp, err := rest.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read snippet upload response: %w", err)
	}

	_, err = parseProxmoxResponse(http.MethodPost, path, resp.StatusCode, raw)

	return err
}
