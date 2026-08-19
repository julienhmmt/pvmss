package cluster

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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

	if config.Password != "" {
		form.Set("cipassword", config.Password)
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

func encodeIPConfig(config CloudInitConfig) string {
	if config.IPMode == CloudInitIPModeStatic && config.IPAddress != "" {
		if config.Gateway != "" {
			return fmt.Sprintf("ip=%s,gw=%s", config.IPAddress, config.Gateway)
		}

		return "ip=" + config.IPAddress
	}

	return "ip=dhcp"
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
		return fmt.Errorf("build snippet upload: %w", err)
	}

	part, err := writer.CreateFormFile("filename", filename)
	if err != nil {
		return fmt.Errorf("build snippet upload: %w", err)
	}

	if _, err := part.Write([]byte(content)); err != nil {
		return fmt.Errorf("build snippet upload: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("build snippet upload: %w", err)
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
