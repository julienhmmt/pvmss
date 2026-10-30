package config_test

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

const syslogTestAddr = "siem.example.com:6514"

// syslogTestCert issues a self-signed 127.0.0.1 certificate that is also its
// own trust anchor, and writes the certificate PEM as the CA file.
func syslogTestCert(t *testing.T) (tls.Certificate, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "pvmss-syslog-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("key pair: %v", err)
	}

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	crtPath := filepath.Join(dir, "client.pem")
	keyPath := filepath.Join(dir, "client-key.pem")

	for path, data := range map[string][]byte{caPath: certPEM, crtPath: certPEM, keyPath: keyPEM} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	return cert, caPath
}

// syslogReceiver is a local TLS listener that splits octet-counted frames and
// reports each message on lines.
type syslogReceiver struct {
	addr  string
	lines chan string
}

func startSyslogReceiver(t *testing.T, cert tls.Certificate) *syslogReceiver {
	t.Helper()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() { _ = ln.Close() })

	rcv := &syslogReceiver{addr: ln.Addr().String(), lines: make(chan string, 64)}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			go readFrames(conn, rcv.lines)
		}
	}()

	return rcv
}

// readFrames reads "<len> <msg>" frames until the connection closes.
func readFrames(conn net.Conn, out chan<- string) {
	defer func() { _ = conn.Close() }()

	br := bufio.NewReader(conn)

	for {
		lenStr, err := br.ReadString(' ')
		if err != nil {
			return
		}

		n, err := strconv.Atoi(strings.TrimSpace(lenStr))
		if err != nil {
			return
		}

		buf := make([]byte, n)
		if _, err := io.ReadFull(br, buf); err != nil {
			return
		}

		out <- string(buf)
	}
}

func recvLine(t *testing.T, ch <-chan string) string {
	t.Helper()

	select {
	case line := <-ch:
		return line
	case <-time.After(5 * time.Second):
		t.Fatal("no syslog frame received within 5s")
	}

	return ""
}

// newSyslogLogger builds a NewLogger wired to rcv, writing stdout to a temp file.
func newSyslogLogger(t *testing.T, rcv *syslogReceiver, caPath, level string) (*slog.Logger, *slog.LevelVar, string) {
	t.Helper()

	out := filepath.Join(t.TempDir(), "stdout.log")

	logger, lv, closer, err := config.NewLogger(config.Configuration{
		LogLevel:        level,
		LogFormat:       testLogFormat,
		LogOutput:       out,
		SyslogAddr:      rcv.addr,
		SyslogTLSCAFile: caPath,
	})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	t.Cleanup(func() { _ = closer.Close() })

	return logger, lv, out
}

func TestLoad_SyslogSettings(t *testing.T) {
	cert, caPath := syslogTestCert(t)
	dir := t.TempDir()
	crt := filepath.Join(dir, "c.pem")
	key := filepath.Join(dir, "k.pem")

	if err := os.WriteFile(crt, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(key, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	_ = cert

	cases := []struct {
		name, addr, ca, cert, key, wantErr string
	}{
		{name: "off"},
		{name: "valid host port", addr: syslogTestAddr},
		{name: "host alone rejected", addr: "siem.example.com", wantErr: "host:port"},
		{name: "port zero rejected", addr: "siem.example.com:0", wantErr: "port must be 1-65535"},
		{name: "port too large rejected", addr: "siem.example.com:70000", wantErr: "port must be 1-65535"},
		{name: "unreadable CA rejected", addr: syslogTestAddr, ca: caPath + ".nope", wantErr: "PVMSS_SYSLOG_TLS_CA_FILE"},
		{name: "CA not PEM rejected", addr: syslogTestAddr, ca: key, wantErr: "no PEM certificate"},
		{name: "cert without key rejected", addr: syslogTestAddr, cert: crt, wantErr: "must be set together"},
		{name: "TLS settings without addr rejected", ca: caPath, wantErr: "require PVMSS_SYSLOG_ADDR"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setMetricsBaseEnv(t)
			t.Setenv("PVMSS_SYSLOG_ADDR", c.addr)
			t.Setenv("PVMSS_SYSLOG_TLS_CA_FILE", c.ca)
			t.Setenv("PVMSS_SYSLOG_TLS_CERT_FILE", c.cert)
			t.Setenv("PVMSS_SYSLOG_TLS_KEY_FILE", c.key)

			cfg, err := config.Load()
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want %q", err, c.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			if cfg.SyslogAddr != c.addr {
				t.Fatalf("SyslogAddr = %q, want %q", cfg.SyslogAddr, c.addr)
			}
		})
	}
}

func TestSyslogHost_HostOnly(t *testing.T) {
	t.Parallel()

	cfg := config.Configuration{SyslogAddr: syslogTestAddr}
	if got := cfg.SyslogHost(); got != "siem.example.com" {
		t.Fatalf("SyslogHost = %q, want host only", got)
	}

	if got := (config.Configuration{}).SyslogHost(); got != "" {
		t.Fatalf("SyslogHost with sink off = %q, want empty", got)
	}
}

//nolint:paralleltest // serial: shared logger fixtures
func TestNewLogger_SyslogOffIsUnwrapped(t *testing.T) {
	_, _, closer, err := config.NewLogger(config.Configuration{
		LogLevel: "info", LogFormat: testLogFormat, LogOutput: filepath.Join(t.TempDir(), "x.log"),
	})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	if _, wrapped := closer.(interface{ Close() error }); !wrapped {
		t.Fatal("closer missing")
	}

	_ = closer.Close()
}

//nolint:paralleltest // serial: shared logger fixtures
func TestSyslog_IntegrationDeliversInOrder(t *testing.T) {
	cert, caPath := syslogTestCert(t)
	rcv := startSyslogReceiver(t, cert)
	logger, _, _ := newSyslogLogger(t, rcv, caPath, "info")

	logger.Info("first record")
	logger.Warn("second record")
	logger.Error("third record")

	for i, want := range []string{"first record", "second record", "third record"} {
		line := recvLine(t, rcv.lines)
		if !strings.Contains(line, want) {
			t.Fatalf("frame %d = %q, want it to contain %q", i, line, want)
		}

		if !syslogLineRE.MatchString(line) {
			t.Fatalf("frame %d is not RFC 5424: %q", i, line)
		}
	}
}

var syslogLineRE = regexp.MustCompile(`^<\d+>1 \S+ \S+ pvmss \d+ \S+ (-|\[pvmss@32473.*\]) .+$`)

//nolint:paralleltest // serial: shared logger fixtures
func TestSyslog_RedactsSecrets(t *testing.T) {
	cert, caPath := syslogTestCert(t)
	rcv := startSyslogReceiver(t, cert)
	logger, _, _ := newSyslogLogger(t, rcv, caPath, "info")

	logger.Info("login", "password", "s3cr3t-pass", "token", "tok-XYZ-123", "user", "alice")

	line := recvLine(t, rcv.lines)
	if strings.Contains(line, "s3cr3t-pass") || strings.Contains(line, "tok-XYZ-123") {
		t.Fatalf("secret reached syslog: %q", line)
	}

	if !strings.Contains(line, `user="alice"`) {
		t.Fatalf("non-secret attr missing: %q", line)
	}
}

//nolint:paralleltest // serial: shared logger fixtures
func TestSyslog_WithAttrsAndGroupsReachSyslog(t *testing.T) {
	cert, caPath := syslogTestCert(t)
	rcv := startSyslogReceiver(t, cert)
	logger, _, _ := newSyslogLogger(t, rcv, caPath, "info")

	logger.With("component", "auth").With("event", "audit").
		WithGroup("req").Info("hello", "route", "/x")

	line := recvLine(t, rcv.lines)
	for _, sub := range []string{`component="auth"`, `req.route="/x"`, " audit [pvmss@32473 "} {
		if !strings.Contains(line, sub) {
			t.Fatalf("line = %q, want it to contain %q", line, sub)
		}
	}
}

//nolint:paralleltest // serial: shared logger fixtures
func TestSyslog_LevelSwitchAppliesToSyslog(t *testing.T) {
	cert, caPath := syslogTestCert(t)
	rcv := startSyslogReceiver(t, cert)
	logger, lv, out := newSyslogLogger(t, rcv, caPath, "warn")

	logger.Info("quiet-info")
	logger.Warn("loud-warn")

	if line := recvLine(t, rcv.lines); !strings.Contains(line, "loud-warn") {
		t.Fatalf("first frame = %q, info must not pass at warn", line)
	}

	lv.Set(slog.LevelDebug)
	logger.Debug("now-debug")

	if line := recvLine(t, rcv.lines); !strings.Contains(line, "now-debug") {
		t.Fatalf("frame after switch = %q, want now-debug", line)
	}

	//nolint:gosec // test-owned temp file
	if data, err := os.ReadFile(out); err != nil || strings.Contains(string(data), "quiet-info") {
		t.Fatalf("stdout holds info record at warn level (err=%v)", err)
	}
}

//nolint:paralleltest // serial: shared logger fixtures
func TestSyslog_UnreachableReceiverNeverBlocks(t *testing.T) {
	// Reserve a port, then close it so every dial is refused.
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr := ln.Addr().String()
	_ = ln.Close()

	out := filepath.Join(t.TempDir(), "stdout.log")

	logger, _, closer, err := config.NewLogger(config.Configuration{
		LogLevel: "info", LogFormat: testLogFormat, LogOutput: out, SyslogAddr: addr,
	})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	t.Cleanup(func() { _ = closer.Close() })

	start := time.Now()

	for range 5000 {
		logger.Info("flood", "n", 1)
	}

	if elapsed := time.Since(start); elapsed/5000 > time.Millisecond {
		t.Fatalf("average Info took %v, want < 1ms", elapsed/5000)
	}

	_ = closer.Close()

	//nolint:gosec // test-owned temp file
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "syslog sink dropping records") {
		t.Fatal("drop warning missing from stdout")
	}
}
