package cluster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SnippetHelperCommand is the node-side helper PVMSS drives over SSH
// (installed by tools/pvmss-node-setup.sh, normally as the forced command
// of PVMSS's key). PVMSS only ever sends "write <name>" (content on stdin)
// and "remove <name>": no path, no shell. The helper validates the name and
// owns the target directory (the storage's snippets/ dir).
const SnippetHelperCommand = "pvmss-snippet"

// hostKeyAlgorithms is the order both the scan and the real connection
// negotiate, so the key type scanned is the key type later verified.
var hostKeyAlgorithms = []string{ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256, ssh.KeyAlgoRSASHA512}

// sshDialTimeout bounds one TCP connect + handshake to a node.
const sshDialTimeout = 10 * time.Second

// ErrSSHNotConfigured reports a publishing attempt on a cluster without SSH
// settings or without the global private key.
var ErrSSHNotConfigured = errors.New("SSH publishing is not configured for this cluster (Admin > Clusters: SSH user, host keys; PVMSS_SSH_KEY_FILE)")

// SnippetSSH is one cluster's SSH publishing configuration. User, Port and
// KnownHosts come from the cluster row (Admin > Clusters); Signer is the
// global private key (PVMSS_SSH_KEY_FILE). Host keys are always verified
// against KnownHosts - there is no insecure mode.
type SnippetSSH struct {
	User       string
	Port       int
	KnownHosts string
	Signer     ssh.Signer
}

// LoadSSHSigner reads and parses the global private key. An empty path
// returns (nil, nil): publishing is then unavailable on every cluster.
func LoadSSHSigner(keyFile string) (ssh.Signer, error) {
	if keyFile == "" {
		return nil, nil //nolint:nilnil // no key configured is a valid state, not an error
	}

	keyBytes, err := os.ReadFile(keyFile) //nolint:gosec // admin-configured key path
	if err != nil {
		return nil, fmt.Errorf("read SSH key file %q: %w", keyFile, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse SSH key %q: %w", keyFile, err)
	}

	return signer, nil
}

// AuthorizedKey renders signer's public key as an authorized_keys line
// (without options), for display in Admin > Clusters. Empty when nil.
func AuthorizedKey(signer ssh.Signer) string {
	if signer == nil {
		return ""
	}

	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
}

// Enabled reports whether this cluster can publish over SSH: a user, the
// global signer, and pinned host keys (host keys are always verified, so an
// empty list can never connect). ScanHostKeys deliberately does not use this
// - the admin scans before any key is pinned.
func (s SnippetSSH) Enabled() bool {
	return s.User != "" && s.Signer != nil && strings.TrimSpace(s.KnownHosts) != ""
}

func (s SnippetSSH) port() int {
	if s.Port <= 0 {
		return 22
	}

	return s.Port
}

// ValidateKnownHosts checks every non-empty, non-comment line parses as a
// known_hosts entry. Hashed hostnames are rejected: PVMSS matches node
// addresses literally and the admin must be able to read what they trust.
func ValidateKnownHosts(text string) error {
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "|1|") {
			return fmt.Errorf("line %d: hashed host names are not supported, use plain entries (ssh-keyscan without -H)", i+1)
		}

		if _, _, _, _, _, err := ssh.ParseKnownHosts([]byte(line)); err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
	}

	return nil
}

// hostKeyCallback verifies a node's host key against KnownHosts. Matching
// is on the normalized address (knownhosts.Normalize: "ip" for port 22,
// "[ip]:port" otherwise); "@revoked" and "@cert-authority" markers are not
// honored and make the entry ignored.
func (s SnippetSSH) hostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		known, matched := s.matchPinnedHostKey(wantedHosts(hostname, remote), key)

		switch {
		case matched:
			return nil
		case known:
			return fmt.Errorf("host key mismatch for %s (%s %s): update the cluster's pinned host keys only if the node was reinstalled", hostname, key.Type(), ssh.FingerprintSHA256(key))
		default:
			return fmt.Errorf("host %s is not in the cluster's pinned host keys (%s %s): scan and confirm it in Admin > Clusters", hostname, key.Type(), ssh.FingerprintSHA256(key))
		}
	}
}

// wantedHosts is the set of normalized addresses a node's key may match: the
// requested hostname and, when available, the remote address.
func wantedHosts(hostname string, remote net.Addr) map[string]bool {
	wanted := map[string]bool{knownhosts.Normalize(hostname): true}
	if remote != nil {
		wanted[knownhosts.Normalize(remote.String())] = true
	}

	return wanted
}

// matchPinnedHostKey reports whether key belongs to a wanted host (known)
// and whether it equals that host's pinned key (matched). Marker entries
// (@revoked, @cert-authority) are ignored.
func (s SnippetSSH) matchPinnedHostKey(wanted map[string]bool, key ssh.PublicKey) (known, matched bool) {
	rest := []byte(s.KnownHosts)

	for len(rest) > 0 {
		marker, hosts, pub, _, next, err := ssh.ParseKnownHosts(rest)
		if err != nil {
			break
		}

		rest = next

		if marker != "" {
			continue
		}

		for _, h := range hosts {
			if !wanted[knownhosts.Normalize(h)] {
				continue
			}

			known = true

			if bytes.Equal(pub.Marshal(), key.Marshal()) {
				return true, true
			}
		}
	}

	return known, false
}

// snippetNode is one cluster node PVMSS publishes to.
type snippetNode struct {
	Name   string
	Host   string
	Online bool
}

// snippetNodes lists every node of the cluster with its SSH address from
// /cluster/status. A standalone node without an IP in the status falls back
// to the API URL's host.
func (p Proxmox) snippetNodes(ctx context.Context) ([]snippetNode, error) {
	raw, err := p.rest().do(ctx, "GET", "/cluster/status", nil)
	if err != nil {
		return nil, fmt.Errorf("query cluster status: %w", err)
	}

	var rows []proxmoxClusterStatusRow
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode cluster status: %w", err)
	}

	var nodes []snippetNode

	for _, row := range rows {
		if row.Type != "node" {
			continue
		}

		host := row.IP
		if host == "" {
			host = p.apiHost()
		}

		nodes = append(nodes, snippetNode{Name: row.Name, Host: host, Online: row.Online == 1})
	}

	if len(nodes) == 0 {
		return nil, errors.New("cluster status lists no node")
	}

	return nodes, nil
}

func (p Proxmox) apiHost() string {
	parsed, err := url.Parse(p.BaseURL)
	if err != nil {
		return ""
	}

	return parsed.Hostname()
}

// sshDial opens a short-lived connection to one node, verifying its host
// key. Publishing is rare (admin action), so no pooling.
func (p Proxmox) sshDial(ctx context.Context, host string) (*ssh.Client, error) {
	if !p.SSH.Enabled() {
		return nil, ErrSSHNotConfigured
	}

	addr := net.JoinHostPort(host, strconv.Itoa(p.SSH.port()))
	config := &ssh.ClientConfig{
		User:              p.SSH.User,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(p.SSH.Signer)},
		HostKeyCallback:   p.SSH.hostKeyCallback(),
		HostKeyAlgorithms: hostKeyAlgorithms,
		Timeout:           sshDialTimeout,
	}

	dialer := net.Dialer{Timeout: sshDialTimeout}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh connect %s: %w", addr, err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()

		return nil, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}

	return ssh.NewClient(c, chans, reqs), nil
}

// runHelper runs one helper verb on host. stdin may be nil.
func (p Proxmox) runHelper(ctx context.Context, host, verb, filename string, stdin []byte) error {
	// The node helper enforces the same rule; this check (with the one in
	// PublishSnippet) keeps a caller bug from even sending a bad name.
	if !snippetFilenameRE.MatchString(filename) {
		return fmt.Errorf("refusing unsafe snippet filename %q", filename)
	}

	client, err := p.sshDial(ctx, host)
	if err != nil {
		return err
	}

	defer func() { _ = client.Close() }()

	return runHelperSession(ctx, client, host, verb, filename, stdin)
}

// runHelperSession drives one helper invocation on an open client and maps
// the helper's stderr into the returned error. ctx cancellation closes the
// client so a hung run cannot block shutdown.
func runHelperSession(ctx context.Context, client *ssh.Client, host, verb, filename string, stdin []byte) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session: %w", err)
	}

	defer func() { _ = session.Close() }()

	var stderr bytes.Buffer

	session.Stderr = &stderr

	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}

	done := make(chan error, 1)

	go func() { done <- session.Run(SnippetHelperCommand + " " + verb + " " + filename) }()

	select {
	case <-ctx.Done():
		_ = client.Close()

		return ctx.Err()
	case err := <-done:
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				return fmt.Errorf("%s %s on %s: %s", SnippetHelperCommand, verb, host, msg)
			}

			return fmt.Errorf("%s %s on %s: %w", SnippetHelperCommand, verb, host, err)
		}

		return nil
	}
}

// scanHostKey connects to host only far enough to capture its host key.
func (p Proxmox) scanHostKey(ctx context.Context, host string) (string, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(p.SSH.port()))

	var captured ssh.PublicKey

	errCaptured := errors.New("host key captured")
	config := &ssh.ClientConfig{
		User: "pvmss-scan",
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			captured = key

			return errCaptured
		},
		HostKeyAlgorithms: hostKeyAlgorithms,
		Timeout:           sshDialTimeout,
	}

	dialer := net.Dialer{Timeout: sshDialTimeout}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("connect %s: %w", addr, err)
	}

	defer func() { _ = conn.Close() }()

	_, _, _, err = ssh.NewClientConn(conn, addr, config)
	if captured == nil {
		return "", fmt.Errorf("read host key of %s: %w", addr, err)
	}

	return knownhosts.Line([]string{knownhosts.Normalize(addr)}, captured), nil
}
