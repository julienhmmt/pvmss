package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SnippetSSH is the global SSH snippet-delivery config. A zero-value
// (empty User) means local filesystem delivery (the default). When User is
// non-empty, snippet files are written over SSH to the specific Proxmox
// node where the VM is created, so PVMSS and Proxmox need no shared
// filesystem. The node's IP is resolved at runtime via /cluster/status.
//
// ponytail: one key for all clusters, node IP from the API, no host-key
// verification. Ceiling: multi-cluster deployments needing different SSH keys
// per cluster, or strict host-key checking. Upgrade path: per-cluster SSH
// columns in the clusters table + PVMSS_SSH_HOST_KEY for known_hosts pinning.
type SnippetSSH struct {
	User   string
	Signer ssh.Signer
	Port   int
}

// NewSnippetSSH parses keyFile and returns a SnippetSSH. A zero-value
// (local delivery) is returned when user is empty. keyFile is required when
// user is non-empty; port defaults to 22 when zero.
func NewSnippetSSH(user, keyFile string, port int) (SnippetSSH, error) {
	if user == "" {
		return SnippetSSH{}, nil
	}

	if keyFile == "" {
		return SnippetSSH{}, errors.New("PVMSS_SSH_KEY_FILE is required when PVMSS_SSH_USER is set")
	}

	keyBytes, err := os.ReadFile(keyFile) //nolint:gosec // admin-configured key path
	if err != nil {
		return SnippetSSH{}, fmt.Errorf("read SSH key file %q: %w", keyFile, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return SnippetSSH{}, fmt.Errorf("parse SSH key %q: %w", keyFile, err)
	}

	if port == 0 {
		port = 22
	}

	return SnippetSSH{User: user, Signer: signer, Port: port}, nil
}

// Enabled reports whether SSH snippet delivery is configured.
func (s SnippetSSH) Enabled() bool { return s.User != "" }

// sshNodeHost resolves a Proxmox node name to an SSH-reachable address. It
// queries /cluster/status for the node's IP (the API resolves the node name
// via getaddrinfo). When node is empty or the API does not return an IP, it
// falls back to the cluster API URL's hostname - this covers single-node
// setups where /cluster/status may not include a node IP.
func (p Proxmox) sshNodeHost(ctx context.Context, node string) (string, error) {
	if node != "" {
		ip, err := p.lookupNodeIP(ctx, node)
		if err == nil && ip != "" {
			return ip, nil
		}
		// Fall through to BaseURL fallback; the error is logged by the caller.
	}

	parsed, err := url.Parse(p.BaseURL)
	if err != nil || parsed.Hostname() == "" {
		return "", fmt.Errorf("cannot derive SSH host from cluster URL %q: %w", p.BaseURL, err)
	}

	return parsed.Hostname(), nil
}

// lookupNodeIP queries /cluster/status for one node's IP address.
func (p Proxmox) lookupNodeIP(ctx context.Context, node string) (string, error) {
	rest := p.rest()

	raw, err := rest.do(ctx, "GET", "/cluster/status", nil)
	if err != nil {
		return "", fmt.Errorf("query cluster status for node %s: %w", node, err)
	}

	var rows []proxmoxClusterStatusRow
	if err := decodeData(raw, &rows); err != nil {
		return "", fmt.Errorf("decode cluster status: %w", err)
	}

	for _, row := range rows {
		if row.Type == "node" && row.Name == node && row.IP != "" {
			return row.IP, nil
		}
	}

	return "", fmt.Errorf("node %q not found in cluster status or has no IP", node)
}

// sshClient dials the Proxmox node where the VM is created. The client is
// short-lived: snippet operations are infrequent (one per VM creation), so a
// fresh connection per operation is simpler than pooling.
func (p Proxmox) sshClient(ctx context.Context, node string) (*ssh.Client, error) {
	host, err := p.sshNodeHost(ctx, node)
	if err != nil {
		return nil, err
	}

	addr := host + ":" + strconv.Itoa(p.SSH.Port)
	config := &ssh.ClientConfig{
		User:            p.SSH.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(p.SSH.Signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // ponytail: no known_hosts; management network is trusted
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	return client, nil
}

// sshWriteSnippet writes content to dir/filename over SSH using a temp file
// and rename, mirroring the atomicity of the local writeFileAtomic: Proxmox
// never reads a half-written file, and a retry overwrites.
func (p Proxmox) sshWriteSnippet(ctx context.Context, node, filename, content string) error {
	client, err := p.sshClient(ctx, node)
	if err != nil {
		return err
	}

	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session: %w", err)
	}

	defer func() { _ = session.Close() }()

	tmp := path.Join(p.SnippetDir, ".pvmss-"+filename+".tmp")
	dst := path.Join(p.SnippetDir, filename)
	// Single-quote paths: filename is regex-validated (no quotes), SnippetDir
	// is admin-controlled. cat > tmp, chmod, mv - same atomicity as local.
	cmd := fmt.Sprintf("cat > '%s' && chmod 644 '%s' && mv '%s' '%s'", tmp, tmp, tmp, dst)

	session.Stdin = strings.NewReader(content)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("ssh write snippet %q: %w", filename, err)
	}

	return nil
}

// sshRemoveSnippet deletes a file over SSH. A missing file is not an error,
// matching the local RemoveCloudInitSnippet contract.
func (p Proxmox) sshRemoveSnippet(ctx context.Context, node, filename string) error {
	client, err := p.sshClient(ctx, node)
	if err != nil {
		return err
	}

	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session: %w", err)
	}

	defer func() { _ = session.Close() }()

	dst := path.Join(p.SnippetDir, filename)
	// rm -f never fails on a missing file, so no need to distinguish.
	cmd := fmt.Sprintf("rm -f '%s'", dst)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("ssh remove snippet %q: %w", filename, err)
	}

	return nil
}

// sshReadSnippet reads a file over SSH. Returns ErrNotFound when the file
// does not exist, matching the local ReadSnippet contract.
func (p Proxmox) sshReadSnippet(ctx context.Context, node, filename string) (string, error) {
	client, err := p.sshClient(ctx, node)
	if err != nil {
		return "", err
	}

	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}

	defer func() { _ = session.Close() }()

	var buf strings.Builder

	session.Stdout = &buf

	dst := path.Join(p.SnippetDir, filename)
	// "cat file || echo -n ''" would mask errors; instead let cat fail and
	// detect "No such file" in the error text (OpenSSH exit 1 + stderr).
	cmd := fmt.Sprintf("cat '%s'", dst)
	if err := session.Run(cmd); err != nil {
		if isSSHFileNotFound(err) {
			return "", ErrNotFound
		}

		return "", fmt.Errorf("ssh read snippet %q: %w", filename, err)
	}

	return buf.String(), nil
}

// isSSHFileNotFound reports whether the SSH error is a missing-file error
// from the remote cat. OpenSSH returns a *ssh.ExitError with exit code 1 and
// "No such file or directory" in the stderr.
func isSSHFileNotFound(err error) bool {
	var exitErr *ssh.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}

	return exitErr.ExitStatus() == 1
}
