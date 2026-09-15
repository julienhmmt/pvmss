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
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

// startTestSSHServer starts a minimal SSH server that accepts public-key
// auth and runs exec commands locally. Returns the address and a cleanup
// function. The server runs on localhost so tests never touch the network.
// The cleanup waits for all connection goroutines to exit so no lingering
// goroutines stress the race detector in other parallel tests.
func startTestSSHServer(t *testing.T, authorizedKey ssh.PublicKey) (string, func()) {
	t.Helper()

	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatalf("host key signer: %v", err)
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) != string(authorizedKey.Marshal()) {
				return nil, errors.New("unknown key")
			}

			return nil, nil
		},
	}
	config.AddHostKey(signer)

	var wg sync.WaitGroup

	var conns sync.Map // track active connections for cleanup

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

				handleSSHConn(c, config)
				conns.Delete(c)
			}(conn)
		}
	}()

	cleanup := func() {
		_ = listener.Close()

		<-done
		// Close any lingering connections so handleSSHConn goroutines exit.
		conns.Range(func(_, v any) bool {
			conn, _ := v.(net.Conn)
			_ = conn.Close()

			return true
		})
		wg.Wait()
	}

	return listener.Addr().String(), cleanup
}

// handleSSHConn handles one SSH connection, accepting session channels and
// executing exec requests via the local shell. This is a test-only server:
// it trusts the commands PVMSS constructs (all regex-validated filenames).
func handleSSHConn(conn net.Conn, config *ssh.ServerConfig) {
	defer func() { _ = conn.Close() }()

	sshConn, chans, _, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}

	defer func() { _ = sshConn.Close() }()

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "only session")
			continue
		}

		channel, reqs, err := newChan.Accept()
		if err != nil {
			continue
		}

		go func(ch ssh.Channel, reqs <-chan *ssh.Request) {
			defer func() { _ = ch.Close() }()

			for req := range reqs {
				if req.Type != "exec" {
					_ = req.Reply(false, nil)
					continue
				}

				_ = req.Reply(true, nil)
				// exec payload: 4-byte big-endian length + command string.
				if len(req.Payload) < 4 {
					continue
				}

				cmdLen := binary.BigEndian.Uint32(req.Payload[:4])
				if int(cmdLen) > len(req.Payload)-4 {
					continue
				}

				cmdStr := string(req.Payload[4 : 4+cmdLen])
				//nolint:gosec // test-only server: cmdStr comes from PVMSS's regex-validated filenames
				cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", cmdStr)
				cmd.Stdout = ch
				cmd.Stderr = ch.Stderr()
				// Only pipe stdin for commands that read it (cat >). For
				// commands like rm/cat/echo, setting cmd.Stdin = ch would
				// start an internal copy goroutine that blocks on ch.Read()
				// forever (the client sends no stdin), deadlocking cmd.Run().
				if strings.Contains(cmdStr, "cat >") {
					cmd.Stdin = ch
				}

				exitCode := uint32(0)

				if err := cmd.Run(); err != nil {
					if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
						exitCode = uint32(exitErr.ExitCode()) //nolint:gosec // G115: exit codes are 0-255
					} else {
						exitCode = 1
					}
				}

				_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{exitCode}))

				break // one exec per session; closing the channel unblocks the client's Wait
			}
		}(channel, reqs)
	}
}

// writeTestKey generates an ed25519 key, writes it to a temp file in PKCS8
// PEM format, and returns the path and the ssh.Signer.
func writeTestKey(t *testing.T) (string, ssh.Signer) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer from key: %v", err)
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal PKCS8: %v", err)
	}

	pemData := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	keyFile := filepath.Join(t.TempDir(), "test_key")
	if err := os.WriteFile(keyFile, pemData, 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	return keyFile, signer
}

//nolint:paralleltest // lightweight; no parallel to reduce race-scheduler pressure
func TestNewSnippetSSH_DisabledWhenUserEmpty(t *testing.T) {
	sshCfg, err := NewSnippetSSH("", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sshCfg.Enabled() {
		t.Fatal("SSH should be disabled when user is empty")
	}
}

//nolint:paralleltest // lightweight; no parallel to reduce race-scheduler pressure
func TestNewSnippetSSH_RequiresKeyFile(t *testing.T) {
	_, err := NewSnippetSSH("root", "", 22)
	if err == nil || !strings.Contains(err.Error(), "PVMSS_SSH_KEY_FILE is required") {
		t.Fatalf("expected key file error, got %v", err)
	}
}

//nolint:paralleltest // lightweight; no parallel to reduce race-scheduler pressure
func TestNewSnippetSSH_ParsesKeyFile(t *testing.T) {
	keyFile, _ := writeTestKey(t)

	sshCfg, err := NewSnippetSSH("root", keyFile, 2222)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sshCfg.Enabled() {
		t.Fatal("SSH should be enabled")
	}

	if sshCfg.User != "root" {
		t.Fatalf("user = %q, want root", sshCfg.User)
	}

	if sshCfg.Port != 2222 {
		t.Fatalf("port = %d, want 2222", sshCfg.Port)
	}
}

//nolint:paralleltest // lightweight; no parallel to reduce race-scheduler pressure
func TestNewSnippetSSH_DefaultPort(t *testing.T) {
	keyFile, _ := writeTestKey(t)

	sshCfg, err := NewSnippetSSH("root", keyFile, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sshCfg.Port != 22 {
		t.Fatalf("default port = %d, want 22", sshCfg.Port)
	}
}

//nolint:paralleltest // starts a TCP SSH server; parallel goroutines stress the race scheduler
func TestProxmoxSSH_WriteReadRemoveSnippet(t *testing.T) {
	_, clientSigner := writeTestKey(t)

	addr, cleanup := startTestSSHServer(t, clientSigner.PublicKey())
	defer cleanup()

	snippetDir := t.TempDir()
	_, portStr, _ := net.SplitHostPort(addr)

	p := Proxmox{
		BaseURL:        "https://" + addr + "/api2/json",
		SnippetDir:     snippetDir,
		SnippetStorage: "test-storage",
		SSH: SnippetSSH{
			User:   "root",
			Signer: clientSigner,
			Port:   atoiOrFatal(t, portStr),
		},
	}

	content := "#cloud-config\nruncmd: [echo hello]\n"
	filename := "pvmss-100.yml"

	// Write via SSH
	if err := p.PushCloudInitSnippet(context.Background(), "", "test-storage", filename, 100, content); err != nil {
		t.Fatalf("PushCloudInitSnippet: %v", err)
	}

	// Verify file exists on disk (test SSH server runs locally).
	//nolint:gosec // G304: test-controlled path
	written, err := os.ReadFile(filepath.Join(snippetDir, filename))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}

	if string(written) != content {
		t.Fatalf("content = %q, want %q", string(written), content)
	}

	// Read back via SSH
	got, err := p.ReadSnippet(context.Background(), "", "test-storage", filename)
	if err != nil {
		t.Fatalf("ReadSnippet: %v", err)
	}

	if got != content {
		t.Fatalf("ReadSnippet content = %q, want %q", got, content)
	}

	// Remove via SSH
	if err := p.RemoveCloudInitSnippet(context.Background(), "test-storage", filename); err != nil {
		t.Fatalf("RemoveCloudInitSnippet: %v", err)
	}

	// Verify file is gone.
	if _, err := os.Stat(filepath.Join(snippetDir, filename)); !os.IsNotExist(err) {
		t.Fatalf("file should be removed, stat err = %v", err)
	}

	// Remove again - missing file is not an error.
	if err := p.RemoveCloudInitSnippet(context.Background(), "test-storage", filename); err != nil {
		t.Fatalf("RemoveCloudInitSnippet (missing): %v", err)
	}
}

//nolint:paralleltest // starts a TCP SSH server; parallel goroutines stress the race scheduler
func TestProxmoxSSH_ReadSnippetNotFound(t *testing.T) {
	_, clientSigner := writeTestKey(t)

	addr, cleanup := startTestSSHServer(t, clientSigner.PublicKey())
	defer cleanup()

	snippetDir := t.TempDir()
	_, portStr, _ := net.SplitHostPort(addr)

	p := Proxmox{
		BaseURL:        "https://" + addr + "/api2/json",
		SnippetDir:     snippetDir,
		SnippetStorage: "test-storage",
		SSH: SnippetSSH{
			User:   "root",
			Signer: clientSigner,
			Port:   atoiOrFatal(t, portStr),
		},
	}

	_, err := p.ReadSnippet(context.Background(), "", "test-storage", "pvmss-999.yml")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadSnippet missing file: err = %v, want ErrNotFound", err)
	}
}

//nolint:paralleltest // no parallel to reduce race-scheduler pressure
func TestProxmoxSSH_LocalFallbackWhenDisabled(t *testing.T) {
	// When SSH is not enabled, the existing local filesystem path is used.
	// This verifies the branching: SSH disabled = local write.
	snippetDir := t.TempDir()
	p := Proxmox{
		BaseURL:        "https://pve.example.com:8006/api2/json",
		SnippetDir:     snippetDir,
		SnippetStorage: "local",
		SSH:            SnippetSSH{}, // disabled
	}

	content := "#cloud-config\npackages: [vim]\n"
	if err := p.PushCloudInitSnippet(context.Background(), "", "local", "pvmss-200.yml", 200, content); err != nil {
		t.Fatalf("PushCloudInitSnippet (local): %v", err)
	}

	//nolint:gosec // G304: test-controlled path
	got, err := os.ReadFile(filepath.Join(snippetDir, "pvmss-200.yml"))
	if err != nil {
		t.Fatalf("read local file: %v", err)
	}

	if string(got) != content {
		t.Fatalf("local content = %q, want %q", string(got), content)
	}
}

func atoiOrFatal(t *testing.T, s string) int {
	t.Helper()

	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		t.Fatalf("parse port %q: %v", s, err)
	}

	return n
}
