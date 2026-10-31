package cluster

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseCloudInitConfig(t *testing.T) {
	t.Parallel()

	cfg := proxmoxVMConfig{
		"ciuser":       FakeCloudInitUser,
		"sshkeys":      url.QueryEscape("ssh-ed25519 AAAA demo@laptop\nssh-ed25519 BBBB demo2@laptop"),
		"ipconfig0":    "ip=10.0.0.42/24,gw=10.0.0.1",
		"nameserver":   FakeCloudInitDNS,
		"searchdomain": "example.internal",
	}

	got := parseCloudInitConfig(cfg)

	if got.User != FakeCloudInitUser || got.Password != "" {
		t.Errorf("user/password = %q/%q", got.User, got.Password)
	}

	if len(got.SSHKeys) != 2 {
		t.Fatalf("sshkeys = %v", got.SSHKeys)
	}

	if got.IPMode != CloudInitIPModeStatic || got.IPAddress != "10.0.0.42/24" || got.Gateway != FakeCloudInitDNS {
		t.Errorf("ip = %+v", got)
	}

	if got.DNSServer != FakeCloudInitDNS || got.SearchDomain != "example.internal" {
		t.Errorf("dns/search = %q/%q", got.DNSServer, got.SearchDomain)
	}
}

func TestParseCloudInitConfig_DHCPDefault(t *testing.T) {
	t.Parallel()

	got := parseCloudInitConfig(proxmoxVMConfig{})
	if got.IPMode != CloudInitIPModeDHCP {
		t.Errorf("IPMode = %q, want dhcp default", got.IPMode)
	}
}

func TestProxmox_GetCloudInitConfig(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"ciuser":"debian","ipconfig0":"ip=dhcp"}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	got, err := p.GetCloudInitConfig(context.Background(), testNodeName, testVMID)
	if err != nil {
		t.Fatalf("GetCloudInitConfig: %v", err)
	}

	if got.User != FakeCloudInitUser || got.IPMode != CloudInitIPModeDHCP {
		t.Errorf("got = %+v", got)
	}
}

// TestFindSnippetStorage_ConfiguredMustBeOnNode verifies the new contract
// the configured storage is returned only when the node actually
// lists it as an active snippets provider; unconfigured means the sentinel.
// TestProxmox_FindSnippetStorage_Unconfigured verifies a cluster with no
// write target reports ErrSnippetWriteUnavailable without consulting the
// node - there is no directory PVMSS could write to, so the storage list is
// irrelevant.
// TestProxmox_SetCloudInitConfig_SSHKeysSurviveProxmoxDecode is the
// regression test. Proxmox percent-decodes sshkeys with Perl's uri_unescape,
// which does NOT turn '+' back into a space; url.QueryEscape encodes a space
// AS '+', so every key written that way reached the guest as
// "ssh-ed25519+AAAA...". The wire body must therefore carry %2520 (the form
// encoding of the literal "%20" PathEscape produced), never %2B.
// TestEncodeSSHKeys_OnlyProxmoxSafeCharacters is the live regression test:
// Proxmox's own sshkeys format validator rejected a real key (an email-style
// "user@host" comment, universal in ssh-keygen output) with "HTTP 400:
// sshkeys: invalid format - invalid urlencoded string" - url.PathEscape
// leaves '@' unescaped (RFC3986 allows it raw in a path segment), but
// Proxmox's validator does not accept it raw. Only unreserved characters and
// TestProxmox_SetCloudInitPassword_Agent verifies the password is applied via
// the guest-agent endpoint (writes /etc/shadow), never cipassword,
// and that the username is the caller-resolved ciuser - never a hardcoded root
// (a cloud image's root is locked).
func TestProxmox_SetCloudInitPassword_Agent(t *testing.T) {
	t.Parallel()

	var gotPath, gotUser, gotPassword string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/agent/set-user-password", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotPath = r.URL.Path
			gotUser = r.FormValue("username")
			gotPassword = r.FormValue("password")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.SetCloudInitPassword(context.Background(), testNodeName, testVMID, "debian", "s3cret"); err != nil {
		t.Fatalf("SetCloudInitPassword: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/agent/set-user-password" {
		t.Errorf("path = %q, want agent/set-user-password", gotPath)
	}

	if gotUser != "debian" || gotPassword != "s3cret" {
		t.Errorf("username/password = %q/%q, want debian/s3cret", gotUser, gotPassword)
	}
}

// TestProxmox_SetCloudInitPassword_UserUnknown verifies that the guest agent's
// "user does not exist" rejection maps to the retryable ErrGuestUserUnknown
// sentinel (cloud-init creates the account mid-boot).
func TestProxmox_SetCloudInitPassword_UserUnknown(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/agent/set-user-password", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data":null,"errors":"user 'debian' does not exist"}`))
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.SetCloudInitPassword(context.Background(), testNodeName, testVMID, "debian", "s3cret")
	if !errors.Is(err, ErrGuestUserUnknown) {
		t.Fatalf("err = %v, want ErrGuestUserUnknown", err)
	}
}

// TestProxmox_PingGuestAgent verifies the probe hits the agent ping endpoint.
func TestProxmox_PingGuestAgent(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu/101/agent/ping", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":{}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.PingGuestAgent(context.Background(), testNodeName, testVMID); err != nil {
		t.Fatalf("PingGuestAgent: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/agent/ping" {
		t.Errorf("path = %q, want agent/ping", gotPath)
	}
}

// TestAgentEnabled verifies the agent= grammar: the first comma token decides,
// so "0,frozen=1" is disabled and "1,frozen=1" enabled.
func TestAgentEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "empty is disabled", raw: "", want: false},
		{name: "0 is disabled", raw: "0", want: false},
		{name: "1 is enabled", raw: "1", want: true},
		{name: "1 with frozen is enabled", raw: "1,frozen=1", want: true},
		{name: "0 with frozen is disabled", raw: "0,frozen=1", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := agentEnabled(tc.raw); got != tc.want {
				t.Errorf("agentEnabled(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestProxmox_AddSSHKey_AgentExec verifies the key is passed as a positional
// argv to a fixed script (no shell interpolation), so a crafted key cannot
// break out, and that the guest's exit status is honoured.
//
// withAgentExecTiming temporarily shortens the agent-exec polling constants so
// tests can exercise the ticker/deadline loop in milliseconds instead of real
// seconds. It restores the originals on cleanup. The waitAgentExec tests are
// NOT parallel because they mutate these package-level vars.
//

func withAgentExecTiming(t *testing.T, poll, wait time.Duration) {
	t.Helper()

	prevPoll, prevWait := agentExecPoll, maxAgentExecWait
	agentExecPoll = poll
	maxAgentExecWait = wait

	t.Cleanup(func() {
		agentExecPoll = prevPoll
		maxAgentExecWait = prevWait
	})
}

// newAgentExecStatusServer builds a test Proxmox server whose exec-status
// handler delegates to respond, passing the 1-based request count so callers
// can vary the response per poll. The returned counter lets callers assert the
// request bound.
func newAgentExecStatusServer(t *testing.T, respond func(count int) string) (*httptest.Server, *int32) {
	t.Helper()

	var count int32

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/agent/exec-status", func(w http.ResponseWriter, _ *http.Request) {
			n := atomic.AddInt32(&count, 1)
			writeJSONFixture(t, w, respond(int(n)))
		})
	})

	return srv, &count
}

// TestProxmox_WaitAgentExec_EventualCompletion verifies the loop polls until
// the guest reports completion: the first few polls return "running", then one
// returns exited with exit code 0. The request count equals the number of
// polls actually made (no busy-loop hammering).
//
//nolint:paralleltest // serial: mutates package-level agentExecPoll/maxAgentExecWait
func TestProxmox_WaitAgentExec_EventualCompletion(t *testing.T) {
	withAgentExecTiming(t, 5*time.Millisecond, 200*time.Millisecond)

	srv, countPtr := newAgentExecStatusServer(t, func(n int) string {
		if n < 3 {
			return `{"data":{"exited":false}}`
		}

		return `{"data":{"exited":true,"exitcode":0}}`
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	start := time.Now()

	if err := p.waitAgentExec(context.Background(), testNodeName, testVMID, 4242); err != nil {
		t.Fatalf("waitAgentExec: %v", err)
	}

	elapsed := time.Since(start)

	if got := atomic.LoadInt32(countPtr); got != 3 {
		t.Errorf("request count = %d, want 3 (first two running, third completed)", got)
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("elapsed = %v, want well under the 200ms deadline", elapsed)
	}
}

// TestProxmox_WaitAgentExec_Timeout verifies that when the guest never reports
// completion, the loop gives up after maxAgentExecWait with the named timeout
// error and a request count bounded by maxAgentExecWait/agentExecPoll (not
// hundreds).
//
//nolint:paralleltest // serial: mutates package-level agentExecPoll/maxAgentExecWait
func TestProxmox_WaitAgentExec_Timeout(t *testing.T) {
	withAgentExecTiming(t, 5*time.Millisecond, 60*time.Millisecond)

	srv, countPtr := newAgentExecStatusServer(t, func(_ int) string {
		return `{"data":{"exited":false}}`
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.waitAgentExec(context.Background(), testNodeName, testVMID, 4242)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "guest agent did not report exec completion") {
		t.Errorf("err = %q, want the named timeout error mentioning qemu-guest-agent", err.Error())
	}

	// maxAgentExecWait/agentExecPoll = 12, plus the immediate first poll =>
	// ~13. Allow jitter but reject a busy loop (hundreds).
	if got := atomic.LoadInt32(countPtr); got > 20 {
		t.Errorf("request count = %d, want bounded by ~maxAgentExecWait/agentExecPoll (not hundreds)", got)
	}
}

// TestProxmox_WaitAgentExec_ExitCodes preserves the exit-code mapping: 0 is
// success, 3 is ErrSSHKeyUserUnknown, any other non-zero surfaces the guest
// stderr. These resolve on the immediate first poll so timing is irrelevant.
//
//nolint:paralleltest // serial: mutates package-level agentExecPoll/maxAgentExecWait
func TestProxmox_WaitAgentExec_ExitCodes(t *testing.T) {
	withAgentExecTiming(t, 50*time.Millisecond, time.Second)

	cases := []struct {
		name    string
		exit    int
		wantErr error
		wantMsg string
	}{
		{"exit 0 is success", 0, nil, ""},
		{"exit 3 is user unknown", 3, ErrSSHKeyUserUnknown, ""},
		{"exit 1 surfaces stderr", 1, nil, "guest agent ssh-key add failed (exit 1)"},
	}

	for _, tc := range cases {
		//nolint:paralleltest // serial: parent mutates package-level timing vars
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newAgentExecStatusServer(t, func(_ int) string {
				return `{"data":{"exited":true,"exitcode":` + strconv.Itoa(tc.exit) + `,"err-data":"boom"}}`
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			err := p.waitAgentExec(context.Background(), testNodeName, testVMID, 4242)
			assertAgentExecError(t, err, tc.wantErr, tc.wantMsg)
		})
	}
}

// assertAgentExecError checks the waitAgentExec error against the expected
// sentinel, message substring, or nil. Extracted from
// TestProxmox_WaitAgentExec_ExitCodes to keep its cognitive complexity under
// go:S3776's ceiling.
func assertAgentExecError(t *testing.T, err, wantErr error, wantMsg string) {
	t.Helper()

	switch {
	case wantErr != nil:
		if !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want %v", err, wantErr)
		}
	case wantMsg != "":
		if err == nil || !strings.Contains(err.Error(), wantMsg) {
			t.Fatalf("err = %v, want message containing %q", err, wantMsg)
		}
	default:
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	}
}

// TestProxmox_WaitAgentExec_ContextCancelled verifies that cancelling the
// context mid-loop terminates promptly (within one poll interval) rather than
// waiting for the deadline.
//
//nolint:paralleltest // serial: mutates package-level agentExecPoll/maxAgentExecWait
func TestProxmox_WaitAgentExec_ContextCancelled(t *testing.T) {
	withAgentExecTiming(t, 20*time.Millisecond, 5*time.Second)

	srv, _ := newAgentExecStatusServer(t, func(_ int) string {
		return `{"data":{"exited":false}}`
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := p.waitAgentExec(ctx, testNodeName, testVMID, 4242)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if elapsed > 500*time.Millisecond {
		t.Errorf("elapsed = %v, want prompt cancellation (well under the 5s deadline)", elapsed)
	}
}
