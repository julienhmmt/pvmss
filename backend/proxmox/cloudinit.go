package proxmox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"

	"pvmss/logger"
)

// CloudInitConfig represents cloud-init configuration for a VM.
type CloudInitConfig struct {
	CIUser       string `json:"ciuser,omitempty"`
	CIPassword   string `json:"cipassword,omitempty"`
	SSHKeys      string `json:"sshkeys,omitempty"`
	IPConfig0    string `json:"ipconfig0,omitempty"`
	IPConfig1    string `json:"ipconfig1,omitempty"`
	Nameserver   string `json:"nameserver,omitempty"`
	Searchdomain string `json:"searchdomain,omitempty"`
	CICustom     string `json:"cicustom,omitempty"`
	CIType       string `json:"citype,omitempty"`
}

// CloudInitParams represents the parameters to set cloud-init config.
type CloudInitParams struct {
	CIUser       string
	CIPassword   string
	SSHKeys      string
	IPConfig0    string
	IPConfig1    string
	Nameserver   string
	Searchdomain string
	CICustom     string
	CIDrive      string // e.g., "ide2" or "scsi1"
	CIStorage    string // storage for cloud-init drive
}

// CloudInitSFTPConfig defines SSH/SFTP configuration for cloud-init snippet uploads
type CloudInitSFTPConfig struct {
	Enabled        bool   `json:"enabled"`        // Whether SFTP upload is enabled
	Host           string `json:"host"`           // Proxmox node hostname or IP
	Port           int    `json:"port"`           // SSH port (default: 22)
	PrivateKeyPath string `json:"privateKeyPath"` // Path to private SSH key file (fallback when PrivateKey is empty)
	SnippetBaseDir string `json:"snippetBaseDir"` // Base directory for snippets (e.g., /var/lib/vz/snippets)
	Username       string `json:"username"`       // SSH username (PAM account)
	HostKeyPath    string `json:"hostKeyPath"`    // Path to known_hosts file for host key verification (required when Enabled)
	// PrivateKey holds the SSH private key content (plaintext, in memory only).
	// Takes precedence over PrivateKeyPath. json:"-" so it is never serialized
	// into API responses, logs, or settings dumps.
	PrivateKey string `json:"-"`
}

// sshSignerFromConfig builds an ssh.Signer from the configured private key,
// preferring the in-memory key content over the on-disk key file.
func sshSignerFromConfig(config CloudInitSFTPConfig) (ssh.Signer, error) {
	if strings.TrimSpace(config.PrivateKey) != "" {
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse configured private key: %w", err)
		}
		return signer, nil
	}
	if config.PrivateKeyPath == "" {
		return nil, fmt.Errorf("no SSH private key configured (set a key or a key path)")
	}
	keyBytes, err := os.ReadFile(config.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file %s: %w", config.PrivateKeyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return signer, nil
}

// SSHKeyFingerprint parses a private key (PEM content) and returns the SHA256
// fingerprint of its public key, e.g. "SHA256:abc…". Used to show which key is
// configured without ever exposing the key itself.
func SSHKeyFingerprint(privateKey string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	return ssh.FingerprintSHA256(signer.PublicKey()), nil
}

// TestSFTPConnection dials the SFTP server with the given config and writes then
// removes a small probe file under SnippetBaseDir to verify connectivity,
// authentication, and write permission. Returns a descriptive error on failure.
func TestSFTPConnection(ctx context.Context, config CloudInitSFTPConfig) error {
	probe := fmt.Sprintf(".pvmss-sftp-test-%d", time.Now().UnixNano())
	if err := UploadSnippetFileSFTP(ctx, config, probe, "# pvmss sftp connectivity test\n"); err != nil {
		return err
	}
	if err := DeleteSnippetFileSFTP(config, probe); err != nil {
		// Upload worked; cleanup failure is non-fatal but worth surfacing.
		return fmt.Errorf("probe uploaded but cleanup failed: %w", err)
	}
	return nil
}

// GetVMCloudInitDumpResty returns the rendered cloud-init configuration for a
// VM via the Proxmox HTTP API: GET /nodes/{node}/qemu/{vmid}/cloudinit/dump.
// The dumpType selects which document is returned ("user", "network" or
// "meta"). Unlike snippet files, this endpoint is reliably readable over the
// HTTP API, so it is used to present a read-only view of the effective
// cloud-config when SFTP (required for editing snippets) is not configured.
func GetVMCloudInitDumpResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, dumpType string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/cloudinit/dump?type=%s",
		url.PathEscape(node), vmid, url.QueryEscape(dumpType))

	var response struct {
		Data string `json:"data"`
	}

	if err := restyClient.Get(ctx, path, &response); err != nil {
		logger.Get().Warn().Err(err).Str("node", node).Int("vmid", vmid).Str("type", dumpType).
			Msg("Failed to dump VM cloud-init config (resty)")
		return "", fmt.Errorf("failed to dump cloud-init %s for VM %d on node %s: %w", dumpType, vmid, node, err)
	}

	return response.Data, nil
}

// UpdateVMCloudInitConfigResty updates cloud-init configuration for a VM.
// This uses the standard VM config endpoint: PUT /nodes/{node}/qemu/{vmid}/config
func UpdateVMCloudInitConfigResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, params CloudInitParams) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)

	values := make(url.Values)

	if params.CIUser != "" {
		values.Set("ciuser", params.CIUser)
	}
	if params.CIPassword != "" {
		values.Set("cipassword", params.CIPassword)
	}
	if params.SSHKeys != "" {
		// SSH keys need full URL encoding for Proxmox:
		// - spaces must be %20 (not +)
		// - @ must be %40
		// - newlines must be %0A
		// url.QueryEscape encodes @ as %40 but uses + for spaces
		// So we use QueryEscape and replace + with %20
		encoded := strings.ReplaceAll(url.QueryEscape(strings.TrimSpace(params.SSHKeys)), "+", "%20")
		values.Set("sshkeys", encoded)
	}
	if params.IPConfig0 != "" {
		values.Set("ipconfig0", params.IPConfig0)
	}
	if params.IPConfig1 != "" {
		values.Set("ipconfig1", params.IPConfig1)
	}
	if params.Nameserver != "" {
		values.Set("nameserver", params.Nameserver)
	}
	if params.Searchdomain != "" {
		values.Set("searchdomain", params.Searchdomain)
	}
	if params.CICustom != "" {
		values.Set("cicustom", params.CICustom)
	}

	// Set up cloud-init drive if specified
	if params.CIDrive != "" && params.CIStorage != "" {
		values.Set(params.CIDrive, params.CIStorage+":cloudinit")
	}

	if len(values) == 0 {
		return nil // Nothing to update
	}

	var response interface{}
	if err := restyClient.Post(ctx, path, values, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to update VM cloud-init config (resty)")
		return fmt.Errorf("failed to update cloud-init config for VM %d on node %s: %w", vmid, node, err)
	}

	logger.Get().Info().Str("node", node).Int("vmid", vmid).Msg("VM cloud-init config updated successfully (resty)")
	return nil
}

// CloudInitUpdate carries an optional, partial cloud-init field update for an
// existing VM. Pointer fields distinguish "leave unchanged" (nil) from "clear
// the key in Proxmox" (non-nil empty string). Password uses a plain string
// where the empty value means "keep current" — a cloud-init password cannot be
// meaningfully cleared, only replaced.
type CloudInitUpdate struct {
	User         *string
	Password     string // "" → keep current; non-empty → set
	SSHKeys      *string
	IPConfig0    *string
	Nameserver   *string
	Searchdomain *string
}

// SetVMCloudInitConfigResty applies a partial cloud-init config update to an
// existing VM via POST /nodes/{node}/qemu/{vmid}/config. Nil fields are left
// untouched; non-nil empty strings remove the corresponding key in Proxmox
// using the official `delete` parameter. SSH keys are URL-encoded with %20 for
// spaces, %40 for @ and %0A for newlines as Proxmox requires.
func SetVMCloudInitConfigResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, upd CloudInitUpdate) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)

	values := make(url.Values)
	var deleteKeys []string

	if upd.User != nil {
		if *upd.User == "" {
			deleteKeys = append(deleteKeys, "ciuser")
		} else {
			values.Set("ciuser", *upd.User)
		}
	}
	if upd.Password != "" {
		values.Set("cipassword", upd.Password)
	}
	if upd.SSHKeys != nil {
		if *upd.SSHKeys == "" {
			deleteKeys = append(deleteKeys, "sshkeys")
		} else {
			encoded := strings.ReplaceAll(url.QueryEscape(strings.TrimSpace(*upd.SSHKeys)), "+", "%20")
			values.Set("sshkeys", encoded)
		}
	}
	if upd.IPConfig0 != nil {
		if *upd.IPConfig0 == "" {
			deleteKeys = append(deleteKeys, "ipconfig0")
		} else {
			values.Set("ipconfig0", *upd.IPConfig0)
		}
	}
	if upd.Nameserver != nil {
		if *upd.Nameserver == "" {
			deleteKeys = append(deleteKeys, "nameserver")
		} else {
			values.Set("nameserver", *upd.Nameserver)
		}
	}
	if upd.Searchdomain != nil {
		if *upd.Searchdomain == "" {
			deleteKeys = append(deleteKeys, "searchdomain")
		} else {
			values.Set("searchdomain", *upd.Searchdomain)
		}
	}
	if len(deleteKeys) > 0 {
		values.Set("delete", strings.Join(deleteKeys, ","))
	}
	if len(values) == 0 {
		return nil // Nothing to update
	}

	var response interface{}
	if err := restyClient.Post(ctx, path, values, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to set VM cloud-init config (resty)")
		return fmt.Errorf("failed to set cloud-init config for VM %d on node %s: %w", vmid, node, err)
	}

	InvalidateVMCache(node)
	logger.Get().Info().Str("node", node).Int("vmid", vmid).Msg("VM cloud-init config set successfully (resty)")
	return nil
}

// EnsureCloudInitDriveResty ensures a cloud-init drive is attached to the VM.
// It checks if a cloud-init drive exists and creates one if not.
// Default bus is "ide2" which is commonly used for cloud-init.
func EnsureCloudInitDriveResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, storage string) error {
	// First, get current VM config
	config, err := GetVMConfigResty(ctx, restyClient, node, vmid)
	if err != nil {
		return fmt.Errorf("failed to get VM config: %w", err)
	}

	// Check if any cloud-init drive already exists
	cloudInitBuses := []string{"ide2", "ide0", "ide1", "ide3", "scsi0", "scsi1", "sata0", "sata1"}
	for _, bus := range cloudInitBuses {
		if val, ok := config[bus].(string); ok && strings.Contains(val, "cloudinit") {
			logger.Get().Debug().
				Str("node", node).
				Int("vmid", vmid).
				Str("bus", bus).
				Msg("Cloud-init drive already exists")
			return nil
		}
	}

	// No cloud-init drive found, create one on ide2 (most common)
	params := map[string]string{
		"ide2": storage + ":cloudinit",
	}

	if err := UpdateVMConfigResty(ctx, restyClient, node, vmid, params); err != nil {
		return fmt.Errorf("failed to create cloud-init drive: %w", err)
	}

	logger.Get().Info().
		Str("node", node).
		Int("vmid", vmid).
		Str("storage", storage).
		Msg("Created cloud-init drive on ide2")

	return nil
}

// UploadSnippetFileResty uploads a new snippet file to storage.
// POST /nodes/{node}/storage/{storage}/upload
// For snippets, this uses the content upload endpoint with content=snippets
func UploadSnippetFileResty(ctx context.Context, restyClient *RestyClient, node, storage, filename, content string) error {
	path := fmt.Sprintf("/nodes/%s/storage/%s/upload",
		url.PathEscape(node), url.PathEscape(storage))

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add content field first
	if err := writer.WriteField("content", "snippets"); err != nil {
		return fmt.Errorf("failed to write content field: %w", err)
	}

	// Create file part with proper headers
	part, err := writer.CreateFormFile("filename", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	// Write the content
	if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	// Close the writer to set the final boundary
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request with proper multipart content type
	req := restyClient.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", writer.FormDataContentType()).
		SetBody(&buf)

	// Debug: Log the exact multipart body being sent
	logger.Get().Debug().
		Str("contentType", writer.FormDataContentType()).
		Str("body", buf.String()).
		Str("node", node).
		Str("storage", storage).
		Str("filename", filename).
		Msg("Sending multipart request to Proxmox")

	var response interface{}
	resp, err := req.SetResult(&response).Post(path)
	if err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Str("storage", storage).
			Str("filename", filename).
			Msg("Failed to upload snippet file (resty)")
		return fmt.Errorf("failed to upload snippet %s to %s on node %s: %w", filename, storage, node, err)
	}

	if resp.IsError() {
		logger.Get().Error().
			Int("status", resp.StatusCode()).
			Str("response", resp.String()).
			Str("node", node).
			Str("storage", storage).
			Str("filename", filename).
			Msg("Snippet upload returned error status (resty)")
		return fmt.Errorf("failed to upload snippet %s to %s on node %s: status %d", filename, storage, node, resp.StatusCode())
	}

	logger.Get().Info().
		Str("node", node).
		Str("storage", storage).
		Str("filename", filename).
		Msg("Snippet file uploaded successfully (resty)")

	return nil
}

// GetSnippetsStoragesResty returns a list of storages that support snippets content.
func GetSnippetsStoragesResty(ctx context.Context, restyClient *RestyClient) ([]Storage, error) {
	storages, err := GetStoragesResty(ctx, restyClient)
	if err != nil {
		return nil, err
	}

	logger.Get().Debug().Int("total_storages", len(storages)).Msg("Retrieved all storages, filtering for snippets support")

	// Filter to only storages that support snippets
	var snippetStorages []Storage
	for _, s := range storages {
		logger.Get().Debug().
			Str("storage", s.Storage).
			Str("content", s.Content).
			Str("type", s.Type).
			Bool("supports_snippets", strings.Contains(s.Content, "snippets")).
			Msg("Evaluating storage for snippets support")
		if strings.Contains(s.Content, "snippets") {
			snippetStorages = append(snippetStorages, s)
		}
	}

	logger.Get().Info().
		Int("total_storages", len(storages)).
		Int("snippet_storages", len(snippetStorages)).
		Msg("Filtered storages supporting snippets")

	if len(snippetStorages) == 0 {
		logger.Get().Warn().Msg("No storages found that support snippets content - cloud-init template creation will fail")
	}

	return snippetStorages, nil
}

// maxSnippetReadSize caps how many bytes ReadSnippetFileSFTP reads from a
// snippet file. Writes are validated to 128 KB (cloudinit.MaxYAMLSize), but a
// manually-placed or tampered file could be larger; this prevents an unbounded
// read from consuming memory.
const maxSnippetReadSize = 1 << 20 // 1 MB

// validateSnippetFilename rejects filenames that could escape the snippets
// directory via path traversal. A snippet filename must be a bare base name:
// no path separators and no "." / ".." directory references. This is a
// defense-in-depth check at the filesystem trust boundary — callers already
// sanitize via path.Base or int formatting, but the SFTP functions are the
// last line of defense before touching the remote filesystem.
func validateSnippetFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("snippet filename must not be empty")
	}
	if strings.ContainsAny(filename, `/\`) {
		return fmt.Errorf("snippet filename must not contain path separators: %q", filename)
	}
	if filename == "." || filename == ".." {
		return fmt.Errorf("snippet filename must not be a directory reference: %q", filename)
	}
	return nil
}

// buildHostKeyCallback returns an ssh.HostKeyCallback that verifies the
// server's host key against the known_hosts file at hostKeyPath. If
// hostKeyPath is empty, it returns an error — host key verification is
// mandatory and InsecureIgnoreHostKey is never used.
func buildHostKeyCallback(hostKeyPath string) (ssh.HostKeyCallback, error) {
	if strings.TrimSpace(hostKeyPath) == "" {
		return nil, fmt.Errorf("SFTP host_key_path is required — set it to a known_hosts file containing the Proxmox SSH server's host key")
	}
	callback, err := knownhosts.New(hostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load known_hosts file %s: %w", hostKeyPath, err)
	}
	return ssh.HostKeyCallback(callback), nil
}

// createSFTPClient creates an SFTP client connection to the configured host.
// Host key verification is mandatory: the configured HostKeyPath must point to
// a known_hosts file containing the Proxmox SSH server's host key. Without
// verification, an attacker on the network path could impersonate the server
// and read or modify snippet content.
func createSFTPClient(config CloudInitSFTPConfig) (*sftp.Client, *ssh.Client, error) {
	log := logger.Get()

	// Load the signer from the configured key content (preferred) or key file.
	signer, err := sshSignerFromConfig(config)
	if err != nil {
		return nil, nil, err
	}

	hostKeyCallback, err := buildHostKeyCallback(config.HostKeyPath)
	if err != nil {
		return nil, nil, err
	}

	// Create SSH client config with mandatory host key verification.
	sshConfig := &ssh.ClientConfig{
		User: config.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SSH server %s: %w", addr, err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		if closeErr := sshClient.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close SSH client after SFTP client creation error")
		}
		return nil, nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	log.Debug().
		Str("host", config.Host).
		Int("port", config.Port).
		Str("username", config.Username).
		Msg("SFTP connection established")

	return sftpClient, sshClient, nil
}

// UploadSnippetFileSFTP uploads a snippet file using SFTP instead of HTTP API.
// This works around Proxmox API limitations with content=snippets uploads.
func UploadSnippetFileSFTP(ctx context.Context, config CloudInitSFTPConfig, filename, content string) error {
	if !config.Enabled {
		return fmt.Errorf("SFTP upload is disabled in configuration")
	}
	if err := validateSnippetFilename(filename); err != nil {
		return err
	}

	log := logger.Get()
	log.Info().
		Str("host", config.Host).
		Str("username", config.Username).
		Str("filename", filename).
		Msg("Uploading cloud-init snippet via SFTP")

	// Create SFTP connection
	sftpClient, sshClient, err := createSFTPClient(config)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sftpClient.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close SFTP client")
		}
		if closeErr := sshClient.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close SSH client")
		}
	}()

	// Ensure target directory exists. Only attempt MkdirAll when it is missing —
	// the snippets directory usually already exists (created by Proxmox), and
	// trying to create an existing root-owned path yields a misleading
	// "permission denied". When creation is genuinely needed and fails, the
	// SFTP user lacks write access to the parent directory.
	targetDir := config.SnippetBaseDir
	if fi, statErr := sftpClient.Stat(targetDir); statErr == nil {
		if !fi.IsDir() {
			return fmt.Errorf("snippet path %s exists but is not a directory", targetDir)
		}
	} else if err := sftpClient.MkdirAll(targetDir); err != nil {
		return fmt.Errorf("failed to create directory %s: the SFTP user %q lacks permission to create it — create it on the node and grant the user write access: %w", targetDir, config.Username, err)
	}

	// Create target file path
	targetPath := filepath.Join(targetDir, filename)

	// Create file on remote server
	remoteFile, err := sftpClient.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to write %s: the SFTP user %q lacks write permission on %s — grant it write access (chown/ACL): %w", targetPath, config.Username, targetDir, err)
	}
	defer func() {
		if closeErr := remoteFile.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close remote file")
		}
	}()

	// Write content
	if _, err := remoteFile.Write([]byte(content)); err != nil {
		return fmt.Errorf("failed to write content to remote file %s: %w", targetPath, err)
	}

	log.Info().
		Str("host", config.Host).
		Str("path", targetPath).
		Msg("Cloud-init snippet uploaded successfully via SFTP")

	return nil
}

// DeleteSnippetFileSFTP deletes a snippet file via SFTP.
// Used when deleting a VM to clean up associated cloud-init snippets.
func DeleteSnippetFileSFTP(config CloudInitSFTPConfig, filename string) error {
	if !config.Enabled {
		return fmt.Errorf("SFTP is disabled in configuration")
	}
	if err := validateSnippetFilename(filename); err != nil {
		return err
	}

	log := logger.Get()
	log.Info().
		Str("host", config.Host).
		Str("filename", filename).
		Msg("Deleting cloud-init snippet via SFTP")

	// Create SFTP connection
	sftpClient, sshClient, err := createSFTPClient(config)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sftpClient.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close SFTP client")
		}
		if closeErr := sshClient.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close SSH client")
		}
	}()

	// Build target path
	targetPath := filepath.Join(config.SnippetBaseDir, filename)

	// Check if file exists
	if _, err := sftpClient.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("path", targetPath).Msg("Snippet file does not exist, nothing to delete")
			return nil
		}
		return fmt.Errorf("failed to stat snippet file %s: %w", targetPath, err)
	}

	// Delete the file
	if err := sftpClient.Remove(targetPath); err != nil {
		return fmt.Errorf("failed to delete snippet file %s: %w", targetPath, err)
	}

	log.Info().
		Str("host", config.Host).
		Str("path", targetPath).
		Msg("Cloud-init snippet deleted successfully via SFTP")

	return nil
}

// ReadSnippetFileSFTP reads the content of a snippet file via SFTP.
// Used by the cloud-init tab to display the custom cloud-config YAML attached
// to a VM. Returns an empty string and no error when the file does not exist
// (e.g. snippet upload failed during creation) so the caller can present an
// empty editor.
func ReadSnippetFileSFTP(ctx context.Context, config CloudInitSFTPConfig, filename string) (string, error) {
	if !config.Enabled {
		return "", fmt.Errorf("SFTP upload is disabled in configuration")
	}
	if err := validateSnippetFilename(filename); err != nil {
		return "", err
	}

	log := logger.Get()
	log.Debug().
		Str("host", config.Host).
		Str("filename", filename).
		Msg("Reading cloud-init snippet via SFTP")

	sftpClient, sshClient, err := createSFTPClient(config)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := sftpClient.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close SFTP client")
		}
		if closeErr := sshClient.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close SSH client")
		}
	}()

	targetPath := filepath.Join(config.SnippetBaseDir, filename)

	// Missing file is not an error here — the editor starts empty so the user
	// can (re)create the snippet.
	if _, err := sftpClient.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("path", targetPath).Msg("Snippet file does not exist, returning empty content")
			return "", nil
		}
		return "", fmt.Errorf("failed to stat snippet file %s: %w", targetPath, err)
	}

	remoteFile, err := sftpClient.OpenFile(targetPath, os.O_RDONLY)
	if err != nil {
		return "", fmt.Errorf("failed to open remote file %s: %w", targetPath, err)
	}
	defer func() {
		if closeErr := remoteFile.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close remote file")
		}
	}()

	data, err := io.ReadAll(io.LimitReader(remoteFile, maxSnippetReadSize+1))
	if err != nil {
		return "", fmt.Errorf("failed to read remote file %s: %w", targetPath, err)
	}
	if len(data) > maxSnippetReadSize {
		return "", fmt.Errorf("snippet file %s exceeds the %d-byte read limit", targetPath, maxSnippetReadSize)
	}

	log.Debug().
		Str("host", config.Host).
		Str("path", targetPath).
		Int("bytes", len(data)).
		Msg("Cloud-init snippet read successfully via SFTP")

	return string(data), nil
}
