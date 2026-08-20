// White-box tests for the cluster package's HTTP surface and the only
// executable helper in client.go (NetworkInterface.MarshalJSON).
//
// client.go itself is the contract (interfaces + types); the real Proxmox HTTP
// client lives in websocket_real.go. These tests exercise that HTTP surface
// against an httptest.Server returning Proxmox-shaped JSON fixtures, plus the
// pure URL builder and the constructor — all without a live Proxmox endpoint
// and without changing any production code.
package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Fixture constants shared across the vncproxy tests. Centralised to satisfy
// goconst (min-occurrences 4) and keep the fixtures readable.
const (
	testNodeName  = "node01"
	testVMID      = 101
	testTicket    = "ticket-abc"
	testTokenName = "user@pve!pvmss" //nolint:gosec // test fixture, not a real credential
	testTokenVal  = "secret-token"   //nolint:gosec // test fixture, not a real credential
)

// nopReadWriteCloser wraps a bytes.Buffer so it satisfies io.ReadWriteCloser
// for the RelayConsole dial-error test, where the peer is never used because
// the WebSocket dial fails first.
type nopReadWriteCloser struct {
	*bytes.Buffer
}

func (nopReadWriteCloser) Close() error { return nil }

// newTestVNCClient builds a proxmoxVNCClient whose httpClient routes to the
// given httptest server. The default http.Transport honours http:// test
// server URLs directly (the TLS config is only consulted for https://).
func newTestVNCClient(t *testing.T, baseURL string, timeout time.Duration) proxmoxVNCClient {
	t.Helper()

	return proxmoxVNCClient{
		baseURL:      baseURL,
		apiTokenName: testTokenName,
		apiTokenVal:  testTokenVal,
		httpClient:   &http.Client{Timeout: timeout},
	}
}

// vncproxyHandler returns an http.HandlerFunc that writes the given status and
// body for every request. Used for the success and HTTP-error cases.
func vncproxyHandler(t *testing.T, status int, body string) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		if _, err := io.WriteString(w, body); err != nil {
			t.Fatalf("write fixture body: %v", err)
		}
	}
}

// TestNetworkInterfaceMarshalJSON covers the only executable code in
// client.go: the nil-IPAddresses -> [] normalisation and the populated path.
func TestNetworkInterfaceMarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("nil IPAddresses encode as empty array", func(t *testing.T) {
		t.Parallel()

		got, err := json.Marshal(NetworkInterface{Index: 0, Bridge: "vmbr0"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		want := `{"index":0,"bridge":"vmbr0","model":"","mac":"","vlan":null,"rateMbps":null,"ipAddresses":[]}`

		if string(got) != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	})

	t.Run("populated IPAddresses encode as-is", func(t *testing.T) {
		t.Parallel()

		ni := NetworkInterface{Index: 1, Bridge: "vmbr1", IPAddresses: []string{"10.0.0.5", "fc00::5"}}

		got, err := json.Marshal(ni)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		if !strings.Contains(string(got), `"ipAddresses":["10.0.0.5","fc00::5"]`) {
			t.Fatalf("got %s, expected populated ipAddresses", got)
		}
	})
}

func TestIsVMCapableStorage_ExactImagesTokenAndPBSExclusion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		storage Storage
		want    bool
	}{
		{name: "images only", storage: Storage{PluginType: storagePluginDir, Content: storageContentImages}, want: true},
		{name: "images among capabilities", storage: Storage{PluginType: "lvmthin", Content: "rootdir,images"}, want: true},
		{name: "backup only", storage: Storage{PluginType: "nfs", Content: "backup"}, want: false},
		{name: "substring is not a token", storage: Storage{PluginType: storagePluginDir, Content: "isoimages"}, want: false},
		{name: "PBS rejected despite images", storage: Storage{PluginType: storagePluginPBS, Content: "images,backup"}, want: false},
		{name: "legacy PBS type rejected", storage: Storage{Type: storagePluginPBS, Content: storageContentImages}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := IsVMCapableStorage(test.storage); got != test.want {
				t.Errorf("IsVMCapableStorage(%+v) = %t, want %t", test.storage, got, test.want)
			}
		})
	}
}

// TestProxmoxGetVNCTicket_Success parses the ticket and port from a 200 response.
func TestProxmoxGetVNCTicket_Success(t *testing.T) {
	t.Parallel()

	body := `{"data":{"ticket":"` + testTicket + `","port":"5901"}}`
	srv := httptest.NewServer(vncproxyHandler(t, http.StatusOK, body))
	t.Cleanup(srv.Close)

	c := newTestVNCClient(t, srv.URL, 5*time.Second)

	got, err := proxmoxGetVNCTicket(context.Background(), c, testNodeName, testVMID)
	if err != nil {
		t.Fatalf("proxmoxGetVNCTicket: %v", err)
	}

	if got.Ticket != testTicket {
		t.Errorf("ticket = %q, want %q", got.Ticket, testTicket)
	}

	if got.Port != 5901 {
		t.Errorf("port = %d, want 5901", got.Port)
	}

	if got.Node != testNodeName {
		t.Errorf("node = %q, want %q", got.Node, testNodeName)
	}
}

// TestProxmoxGetVNCTicket_Non200 returns an error mentioning the status code.
func TestProxmoxGetVNCTicket_Non200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(vncproxyHandler(t, http.StatusUnauthorized, `{"errors":[]}`))
	t.Cleanup(srv.Close)

	c := newTestVNCClient(t, srv.URL, 5*time.Second)

	_, err := proxmoxGetVNCTicket(context.Background(), c, testNodeName, testVMID)
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error %q should mention status 401", err)
	}
}

// TestProxmoxGetVNCTicket_InvalidJSON returns a decode error.
func TestProxmoxGetVNCTicket_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(vncproxyHandler(t, http.StatusOK, `{not-json`))
	t.Cleanup(srv.Close)

	c := newTestVNCClient(t, srv.URL, 5*time.Second)

	_, err := proxmoxGetVNCTicket(context.Background(), c, testNodeName, testVMID)
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}

	if !strings.Contains(err.Error(), "decode vncproxy response") {
		t.Fatalf("error %q should mention decode", err)
	}
}

// TestProxmoxGetVNCTicket_NonNumericPort returns an invalid-port error.
func TestProxmoxGetVNCTicket_NonNumericPort(t *testing.T) {
	t.Parallel()

	body := `{"data":{"ticket":"` + testTicket + `","port":"not-a-port"}}`
	srv := httptest.NewServer(vncproxyHandler(t, http.StatusOK, body))
	t.Cleanup(srv.Close)

	c := newTestVNCClient(t, srv.URL, 5*time.Second)

	_, err := proxmoxGetVNCTicket(context.Background(), c, testNodeName, testVMID)
	if err == nil {
		t.Fatal("expected port error, got nil")
	}

	if !strings.Contains(err.Error(), "invalid vncproxy port") {
		t.Fatalf("error %q should mention invalid port", err)
	}
}

// TestProxmoxGetVNCTicket_Timeout returns a request-timeout error.
func TestProxmoxGetVNCTicket_Timeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := newTestVNCClient(t, srv.URL, 1*time.Millisecond)

	_, err := proxmoxGetVNCTicket(context.Background(), c, testNodeName, testVMID)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "vncproxy request") {
		t.Fatalf("error %q should mention vncproxy request", err)
	}
}

// TestProxmoxGetVNCTicket_VerifiesAuthHeader confirms the request carries the
// Proxmox API-token authorization header and the websocket=1 form field, so
// the parsing surface is exercised against a request shape Proxmox expects.
func TestProxmoxGetVNCTicket_VerifiesAuthHeader(t *testing.T) {
	t.Parallel()

	var gotAuth, gotForm string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		gotForm = r.FormValue("websocket")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = io.WriteString(w, `{"data":{"ticket":"`+testTicket+`","port":"5901"}}`)
	}))
	t.Cleanup(srv.Close)

	c := newTestVNCClient(t, srv.URL, 5*time.Second)

	if _, err := proxmoxGetVNCTicket(context.Background(), c, testNodeName, testVMID); err != nil {
		t.Fatalf("proxmoxGetVNCTicket: %v", err)
	}

	wantAuth := "PVEAPIToken=" + testTokenName + "=" + testTokenVal

	if gotAuth != wantAuth {
		t.Errorf("auth = %q, want %q", gotAuth, wantAuth)
	}

	if gotForm != "1" {
		t.Errorf("websocket form field = %q, want %q", gotForm, "1")
	}
}

// TestBuildProxmoxVNCWebSocketURL covers the pure URL builder for the three
// scheme inputs: https -> wss, http -> ws, and a bare host that defaults to
// https/wss.
func TestBuildProxmoxVNCWebSocketURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		baseURL    string
		wantScheme string
	}{
		{"https becomes wss", "https://pve.example:8006", "wss"},
		{"http becomes ws", "http://pve.example:8006", "ws"},
		{"bare host defaults to wss", "pve.example:8006", "wss"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildProxmoxVNCWebSocketURL(tt.baseURL, testNodeName, testVMID, 5901, testTicket)

			if !strings.HasPrefix(got, tt.wantScheme+"://") {
				t.Fatalf("got %q, want %s scheme", got, tt.wantScheme)
			}

			wantPath := "/api2/json/nodes/node01/qemu/101/vncwebsocket"

			if !strings.Contains(got, wantPath) {
				t.Fatalf("got %q, want path %q", got, wantPath)
			}

			if !strings.Contains(got, "port=5901") || !strings.Contains(got, "vncticket="+testTicket) {
				t.Fatalf("got %q, missing port/vncticket query", got)
			}
		})
	}
}

// TestNewProxmoxVNCClient confirms the constructor wires configuration into the
// client fields and produces a usable http.Client with the configured timeout.
func TestNewProxmoxVNCClient(t *testing.T) {
	t.Parallel()

	c := newProxmoxVNCClient("https://pve.example:8006", testTokenName, testTokenVal, false)

	if c.baseURL != "https://pve.example:8006" {
		t.Errorf("baseURL = %q, want the configured value", c.baseURL)
	}

	if c.apiTokenName != testTokenName || c.apiTokenVal != testTokenVal {
		t.Errorf("token = %q/%q, want %q/%q", c.apiTokenName, c.apiTokenVal, testTokenName, testTokenVal)
	}

	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}

	if c.httpClient.Timeout != 15*time.Second {
		t.Errorf("timeout = %v, want 15s", c.httpClient.Timeout)
	}
}

// TestProxmoxGetVNCTicket_EndToEnd covers the public ConsoleRelay.GetVNCTicket
// method through the real constructor path: Proxmox.BaseURL points at an
// httptest server, so newProxmoxVNCClient's http.Client (TLS config ignored for
// plain http) reaches the fixture. No API change required.
func TestProxmoxGetVNCTicket_EndToEnd(t *testing.T) {
	t.Parallel()

	body := `{"data":{"ticket":"` + testTicket + `","port":"5902"}}`
	srv := httptest.NewServer(vncproxyHandler(t, http.StatusOK, body))
	t.Cleanup(srv.Close)

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	got, err := p.GetVNCTicket(context.Background(), "cluster-a", testVMID, testNodeName)
	if err != nil {
		t.Fatalf("GetVNCTicket: %v", err)
	}

	if got.Ticket != testTicket || got.Port != 5902 || got.Node != testNodeName {
		t.Fatalf("got %+v, want ticket=%q port=5902 node=%q", got, testTicket, testNodeName)
	}
}

// TestProxmoxRelayConsole_DialError covers the RelayConsole dial-failure path:
// pointing BaseURL at a plain HTTP server (no WebSocket upgrade) makes
// websocket.Dial fail, so the method returns the dial error without touching
// the peer.
func TestProxmoxRelayConsole_DialError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(vncproxyHandler(t, http.StatusOK, "not a websocket"))
	t.Cleanup(srv.Close)

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}
	peer := nopReadWriteCloser{Buffer: &bytes.Buffer{}}

	err := p.RelayConsole(context.Background(), "cluster-a", testVMID, VNCProxyTicket{
		Ticket: testTicket,
		Port:   5901,
		Node:   testNodeName,
	}, peer)
	if err == nil {
		t.Fatal("expected dial error, got nil")
	}

	if !strings.Contains(err.Error(), "dial proxmox vncwebsocket") {
		t.Fatalf("error %q should mention dial", err)
	}
}

// TestProxmoxRelayConsole_CancelledContext covers the dial-error path when the
// context is already cancelled, giving a second, distinct error route.
func TestProxmoxRelayConsole_CancelledContext(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(vncproxyHandler(t, http.StatusOK, "not a websocket"))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}
	peer := nopReadWriteCloser{Buffer: &bytes.Buffer{}}

	err := p.RelayConsole(ctx, "cluster-a", testVMID, VNCProxyTicket{
		Ticket: testTicket,
		Port:   5901,
		Node:   testNodeName,
	}, peer)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v should wrap context.Canceled", err)
	}
}
