package cluster

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// testSnippetNode is an in-process SSH server that behaves like a node with
// the pvmss-snippet helper installed as a forced command: it accepts only
// "pvmss-snippet write|remove <name>" and stores files in dir.
type testSnippetNode struct {
	addr    string
	port    int
	dir     string
	hostKey ssh.PublicKey
}

func startTestSnippetNode(t *testing.T, authorizedKey ssh.PublicKey) testSnippetNode {
	t.Helper()

	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}

	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("host key signer: %v", err)
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) != string(authorizedKey.Marshal()) {
				return nil, errors.New("unknown key")
			}

			return nil, nil //nolint:nilnil // ssh.ServerConfig contract: nil permissions accept
		},
	}
	config.AddHostKey(hostSigner)

	node := testSnippetNode{addr: listener.Addr().String(), dir: t.TempDir(), hostKey: hostSigner.PublicKey()}
	_, portText, _ := net.SplitHostPort(node.addr)
	node.port, _ = strconv.Atoi(portText)

	var wg sync.WaitGroup

	var conns sync.Map

	done := make(chan struct{})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				close(done)

				return
			}

			conns.Store(conn, conn)
			wg.Add(1)

			go func(c net.Conn) {
				defer wg.Done()

				node.serve(c, config)
				conns.Delete(c)
			}(conn)
		}
	}()

	t.Cleanup(func() {
		_ = listener.Close()

		<-done

		conns.Range(func(_, v any) bool {
			conn, _ := v.(net.Conn)
			_ = conn.Close()

			return true
		})
		wg.Wait()
	})

	return node
}

func (n testSnippetNode) serve(conn net.Conn, config *ssh.ServerConfig) {
	defer func() { _ = conn.Close() }()

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}

	defer func() { _ = sshConn.Close() }()

	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "only session")

			continue
		}

		channel, chReqs, err := newChan.Accept()
		if err != nil {
			continue
		}

		go n.session(channel, chReqs)
	}
}

func (n testSnippetNode) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer func() { _ = ch.Close() }()

	for req := range reqs {
		if req.Type != "exec" || len(req.Payload) < 4 {
			_ = req.Reply(false, nil)

			continue
		}

		_ = req.Reply(true, nil)

		cmdLen := binary.BigEndian.Uint32(req.Payload[:4])
		fields := strings.Fields(string(req.Payload[4 : 4+cmdLen]))
		status := n.run(ch, fields)
		_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))

		return
	}
}

// run is the helper: forced-command semantics, name validated, dir owned.
func (n testSnippetNode) run(ch ssh.Channel, fields []string) uint32 {
	if len(fields) != 3 || fields[0] != SnippetHelperCommand || !snippetFilenameRE.MatchString(fields[2]) {
		_, _ = fmt.Fprint(ch.Stderr(), "invalid snippet name")

		return 2
	}

	path := filepath.Join(n.dir, fields[2])

	switch fields[1] {
	case "write":
		data, err := io.ReadAll(ch)
		if err != nil || os.WriteFile(path, data, 0o600) != nil {
			return 1
		}
	case "remove":
		_ = os.Remove(path)
	default:
		return 2
	}

	return 0
}

// knownHostsLine is the pinned host key line for this node.
func (n testSnippetNode) knownHostsLine() string {
	return knownhosts.Line([]string{knownhosts.Normalize(n.addr)}, n.hostKey)
}

// newTestSigner generates PVMSS's key.
func newTestSigner(t *testing.T) ssh.Signer {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer from key: %v", err)
	}

	return signer
}

// publishTestAPI serves /cluster/status (one online node on 127.0.0.1, one
// offline) and the snippets content of storage "shared" on node01 from
// dir. listFiles=false simulates a helper writing into a directory that is
// not the storage's snippets/ dir.
func publishTestAPI(t *testing.T, dir string, listFiles bool) string {
	t.Helper()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/status", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"type":"cluster","name":"c"},{"type":"node","name":"node01","ip":"127.0.0.1","online":1},{"type":"node","name":"node02","ip":"127.0.0.2","online":0}]}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/node01/storage/shared/content", func(w http.ResponseWriter, _ *http.Request) {
			rows := []string{}

			if listFiles {
				entries, _ := os.ReadDir(dir)
				for _, e := range entries {
					rows = append(rows, fmt.Sprintf(`{"volid":"shared:snippets/%s","size":1}`, e.Name()))
				}
			}

			writeJSONFixture(t, w, `{"data":[`+strings.Join(rows, ",")+`]}`)
		})
	})

	return srv.URL
}

func TestProxmox_PublishSnippet_WritesAndVerifiesEveryNode(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	node := startTestSnippetNode(t, signer.PublicKey())

	p := Proxmox{
		BaseURL: publishTestAPI(t, node.dir, true), APITokenName: testTokenName, APITokenValue: testTokenVal,
		SnippetStorage: "shared",
		SSH:            SnippetSSH{User: testSSHUser, Port: node.port, KnownHosts: node.knownHostsLine(), Signer: signer},
	}

	results, err := p.PublishSnippet(context.Background(), "pvmss-tpl-web-abc.yml", "#cloud-config\n")
	if err != nil {
		t.Fatalf("PublishSnippet: %v", err)
	}

	if len(results) != 2 || !results[0].OK || results[0].Node != "node01" {
		t.Fatalf("results = %+v, want node01 OK", results)
	}

	if results[1].OK || !strings.Contains(results[1].Error, "offline") {
		t.Errorf("node02 = %+v, want offline failure", results[1])
	}

	data, err := os.ReadFile(filepath.Join(node.dir, "pvmss-tpl-web-abc.yml"))
	if err != nil || string(data) != "#cloud-config\n" {
		t.Fatalf("published file = %q (%v)", data, err)
	}

	if err := p.RemoveCloudInitSnippet(context.Background(), "shared", "pvmss-tpl-web-abc.yml"); err != nil {
		t.Fatalf("RemoveCloudInitSnippet: %v", err)
	}

	if _, err := os.Stat(filepath.Join(node.dir, "pvmss-tpl-web-abc.yml")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file still present after remove: %v", err)
	}
}

func TestProxmox_PublishSnippet_ReportsFileProxmoxDoesNotList(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	node := startTestSnippetNode(t, signer.PublicKey())

	p := Proxmox{
		BaseURL: publishTestAPI(t, node.dir, false), APITokenName: testTokenName, APITokenValue: testTokenVal,
		SnippetStorage: "shared",
		SSH:            SnippetSSH{User: testSSHUser, Port: node.port, KnownHosts: node.knownHostsLine(), Signer: signer},
	}

	results, err := p.PublishSnippet(context.Background(), "pvmss-baseline-abc.yml", "#cloud-config\n")
	if err != nil {
		t.Fatalf("PublishSnippet: %v", err)
	}

	if results[0].OK || !strings.Contains(results[0].Error, "does not list") {
		t.Fatalf("node01 = %+v, want a visibility failure", results[0])
	}
}

func TestProxmox_PublishSnippet_RefusesUnpinnedOrChangedHostKey(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	node := startTestSnippetNode(t, signer.PublicKey())
	other := startTestSnippetNode(t, signer.PublicKey())

	cases := map[string]struct {
		knownHosts string
		want       string
	}{
		"unpinned": {knownHosts: other.knownHostsLine(), want: "not in the cluster's pinned host keys"},
		"changed": {
			knownHosts: knownhosts.Line([]string{knownhosts.Normalize(node.addr)}, other.hostKey),
			want:       "host key mismatch",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p := Proxmox{
				BaseURL: publishTestAPI(t, node.dir, true), APITokenName: testTokenName, APITokenValue: testTokenVal,
				SnippetStorage: "shared",
				SSH:            SnippetSSH{User: testSSHUser, Port: node.port, KnownHosts: tc.knownHosts, Signer: signer},
			}

			results, err := p.PublishSnippet(context.Background(), "pvmss-baseline-abc.yml", "#cloud-config\n")
			if err != nil {
				t.Fatalf("PublishSnippet: %v", err)
			}

			if results[0].OK || !strings.Contains(results[0].Error, tc.want) {
				t.Fatalf("node01 = %+v, want %q", results[0], tc.want)
			}

			if entries, _ := os.ReadDir(node.dir); len(entries) != 0 {
				t.Errorf("file written to an unverified host: %v", entries)
			}
		})
	}
}

func TestProxmox_PublishSnippet_NotConfigured(t *testing.T) {
	t.Parallel()

	for name, p := range map[string]Proxmox{
		"no storage":   {SSH: SnippetSSH{User: testSSHUser, KnownHosts: testPinnedHost, Signer: newTestSigner(t)}},
		"no user":      {SnippetStorage: "shared", SSH: SnippetSSH{KnownHosts: testPinnedHost, Signer: newTestSigner(t)}},
		"no key":       {SnippetStorage: "shared", SSH: SnippetSSH{User: testSSHUser, KnownHosts: testPinnedHost}},
		"no host keys": {SnippetStorage: "shared", SSH: SnippetSSH{User: testSSHUser, Signer: newTestSigner(t)}},
	} {
		if _, err := p.PublishSnippet(context.Background(), "pvmss-x.yml", ""); !errors.Is(err, ErrSSHNotConfigured) {
			t.Errorf("%s: err = %v, want ErrSSHNotConfigured", name, err)
		}
	}
}

func TestProxmox_PublishSnippet_RejectsUnsafeFilename(t *testing.T) {
	t.Parallel()

	p := Proxmox{SnippetStorage: "shared", SSH: SnippetSSH{User: testSSHUser, KnownHosts: testPinnedHost, Signer: newTestSigner(t)}}

	for _, name := range []string{"../etc/passwd", "pvmss-a.yml;rm -rf /", "evil.yml", "pvmss-a b.yml"} {
		if _, err := p.PublishSnippet(context.Background(), name, ""); err == nil || errors.Is(err, ErrSSHNotConfigured) {
			t.Errorf("%q: err = %v, want refusal", name, err)
		}
	}
}

func TestProxmox_ScanHostKeys_ReturnsPinnableLines(t *testing.T) {
	t.Parallel()

	signer := newTestSigner(t)
	node := startTestSnippetNode(t, signer.PublicKey())

	p := Proxmox{
		BaseURL: publishTestAPI(t, node.dir, true), APITokenName: testTokenName, APITokenValue: testTokenVal,
		SSH: SnippetSSH{Port: node.port},
	}

	scans, err := p.ScanHostKeys(context.Background())
	if err != nil {
		t.Fatalf("ScanHostKeys: %v", err)
	}

	if len(scans) != 2 || scans[0].Line != node.knownHostsLine() {
		t.Fatalf("scans = %+v, want node01 line %q", scans, node.knownHostsLine())
	}

	if scans[1].Error == "" {
		t.Errorf("node02 (unreachable) = %+v, want an error", scans[1])
	}

	if err := ValidateKnownHosts(scans[0].Line); err != nil {
		t.Errorf("scanned line does not validate: %v", err)
	}
}

func TestValidateKnownHosts(t *testing.T) {
	t.Parallel()

	good := knownhosts.Line([]string{"10.0.0.1"}, newTestSigner(t).PublicKey())

	for text, wantErr := range map[string]bool{
		"":                                 false,
		"# comment\n\n" + good:             false,
		good + "\n" + good:                 false,
		"not a key line":                   true,
		"|1|abc=|def= ssh-ed25519 AAAAC3N": true,
	} {
		if err := ValidateKnownHosts(text); (err != nil) != wantErr {
			t.Errorf("ValidateKnownHosts(%q) = %v, wantErr %v", text, err, wantErr)
		}
	}
}

func TestLoadSSHSigner(t *testing.T) {
	t.Parallel()

	if signer, err := LoadSSHSigner(""); err != nil || signer != nil {
		t.Fatalf("empty path = %v/%v, want nil/nil", signer, err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	keyFile := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	signer, err := LoadSSHSigner(keyFile)
	if err != nil || signer == nil {
		t.Fatalf("LoadSSHSigner = %v/%v", signer, err)
	}

	if !strings.HasPrefix(AuthorizedKey(signer), "ssh-ed25519 ") || AuthorizedKey(nil) != "" {
		t.Errorf("AuthorizedKey = %q", AuthorizedKey(signer))
	}

	if _, err := LoadSSHSigner(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("missing key file: want an error")
	}
}
