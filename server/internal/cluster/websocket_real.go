// Package cluster — T10 real Proxmox VNC relay (ConsoleRelay implementation).
//
// This is the real cluster.Client's ConsoleRelay: GetVNCTicket dials Proxmox's
// vncproxy endpoint for (node, vmid) to obtain a Proxmox-side VNC ticket and
// port; RelayConsole dials Proxmox's own vncwebsocket endpoint and relays
// frames bidirectionally between the browser WebSocket and Proxmox until either
// side closes. The idea is reused from the legacy's B11 (GetVNCProxyResty,
// buildVNCWebSocketURL, forwardVNCMessages) — the code is not (constitution
// VIII: no copy-paste from v0.3).
//
// The real Proxmox client is not fully wired in v0.4 yet (T01 left Proxmox as a
// stub for every read/write method). This file implements the console surface
// against a minimal REST + WebSocket client so that a reachable Proxmox server
// would actually work, but it is exercised only by integration tests against a
// live endpoint — the tranche's own demo and unit tests run against the fake.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// proxmoxVNCProxyResponse is the JSON envelope Proxmox returns from
// POST /nodes/{node}/qemu/{vmid}/vncproxy. The fields PVMSS uses are Ticket
// (the VNC ticket, carried in the WebSocket URL) and Port (the VNC port,
// also carried in the WebSocket URL).
type proxmoxVNCProxyResponse struct {
	Data struct {
		Ticket string `json:"ticket"`
		Port   string `json:"port"`
	} `json:"data"`
}

// proxmoxVNCClient is the minimal REST surface GetVNCTicket and RelayConsole
// need. Constructed per-call from cluster.Proxmox's own BaseURL/APITokenName/
// APITokenValue fields (set in main.go from PROXMOX_URL/PROXMOX_API_TOKEN_NAME/
// PROXMOX_API_TOKEN_VALUE). Proxmox itself still returns ErrNotImplemented for
// every read/write method beyond ConsoleRelay (T01 stub) — this is only the
// console surface, not the full client.
type proxmoxVNCClient struct {
	baseURL      string
	apiTokenName string
	apiTokenVal  string
	httpClient   *http.Client
}

// GetVNCTicket implements ConsoleRelay for the real Proxmox client. It calls
// Proxmox's vncproxy endpoint for (node, vmid) and returns the Proxmox-side
// ticket and port. The node is always Resolve()'s server-resolved value — the
// caller never supplies one (FR-007).
//
// Proxmox is not reachable in the tranche's own demo or unit tests; this
// method is exercised only by integration tests against a live endpoint.
func (p Proxmox) GetVNCTicket(ctx context.Context, _ string, vmid int, node string) (VNCProxyTicket, error) {
	c := newProxmoxVNCClient(p.BaseURL, p.APITokenName, p.APITokenValue)
	return proxmoxGetVNCTicket(ctx, c, node, vmid)
}

// RelayConsole implements ConsoleRelay for the real Proxmox client. It dials
// Proxmox's own vncwebsocket endpoint and relays frames bidirectionally
// between the browser WebSocket (peer) and Proxmox until either side closes.
//
// Proxmox is not reachable in the tranche's own demo or unit tests; this
// method is exercised only by integration tests against a live endpoint.
func (p Proxmox) RelayConsole(ctx context.Context, _ string, vmid int, proxy VNCProxyTicket, peer io.ReadWriteCloser) error {
	c := newProxmoxVNCClient(p.BaseURL, p.APITokenName, p.APITokenValue)
	return proxmoxRelayConsole(ctx, c, proxy.Node, vmid, proxy, peer)
}

// --- The real flow, called directly from GetVNCTicket and RelayConsole above.
// Kept as free functions taking proxmoxVNCClient rather than methods on
// Proxmox so the "idea reused from B11" stays a small, reviewable unit,
// separate from the ConsoleRelay interface's method shape. ---

// proxmoxGetVNCTicket dials the vncproxy endpoint and returns the ticket+port.
func proxmoxGetVNCTicket(ctx context.Context, c proxmoxVNCClient, node string, vmid int) (VNCProxyTicket, error) {
	endpoint := fmt.Sprintf("%s/api2/json/nodes/%s/qemu/%d/vncproxy", strings.TrimRight(c.baseURL, "/"), url.PathEscape(node), vmid)

	form := url.Values{"websocket": {"1"}}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form))
	if err != nil {
		return VNCProxyTicket{}, fmt.Errorf("build vncproxy request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.apiTokenName, c.apiTokenVal))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return VNCProxyTicket{}, fmt.Errorf("vncproxy request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return VNCProxyTicket{}, fmt.Errorf("vncproxy returned %d", resp.StatusCode)
	}

	var envelope proxmoxVNCProxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return VNCProxyTicket{}, fmt.Errorf("decode vncproxy response: %w", err)
	}

	port, err := strconv.Atoi(envelope.Data.Port)
	if err != nil {
		return VNCProxyTicket{}, fmt.Errorf("invalid vncproxy port %q: %w", envelope.Data.Port, err)
	}

	return VNCProxyTicket{Ticket: envelope.Data.Ticket, Port: port, Node: node}, nil
}

// proxmoxRelayConsole dials Proxmox's vncwebsocket endpoint and copies frames
// both ways between the browser peer and Proxmox until either side closes.
func proxmoxRelayConsole(ctx context.Context, c proxmoxVNCClient, node string, vmid int, proxy VNCProxyTicket, peer io.ReadWriteCloser) error {
	wsURL := buildProxmoxVNCWebSocketURL(c.baseURL, node, vmid, proxy.Port, proxy.Ticket)

	proxmoxConn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{ //nolint:bodyclose // coder/websocket owns the response body lifecycle per its Dial docs
		HTTPHeader: http.Header{
			"Authorization": []string{fmt.Sprintf("PVEAPIToken=%s=%s", c.apiTokenName, c.apiTokenVal)},
		},
		HTTPClient: c.httpClient,
	})
	if err != nil {
		return fmt.Errorf("dial proxmox vncwebsocket: %w", err)
	}
	defer func() { _ = proxmoxConn.CloseNow() }()

	proxmoxNetConn := websocket.NetConn(ctx, proxmoxConn, websocket.MessageBinary)
	defer func() { _ = proxmoxNetConn.Close() }()

	errCh := make(chan error, 2)

	go func() { _, err := io.Copy(proxmoxNetConn, peer); errCh <- err }()
	go func() { _, err := io.Copy(peer, proxmoxNetConn); errCh <- err }()

	err = <-errCh
	_ = proxmoxNetConn.Close()
	_ = peer.Close()

	return err
}

// buildProxmoxVNCWebSocketURL converts a Proxmox HTTP(S) base URL to the
// vncwebsocket URL — the idea behind legacy's buildVNCWebSocketURL, reused.
func buildProxmoxVNCWebSocketURL(baseURL, node string, vmid, port int, vncticket string) string {
	base := strings.TrimSpace(baseURL)
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}

	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}

	parsed.Path = fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/vncwebsocket", url.PathEscape(node), vmid)
	q := parsed.Query()
	q.Set("port", strconv.Itoa(port))
	q.Set("vncticket", vncticket)
	parsed.RawQuery = q.Encode()

	return parsed.String()
}

// newProxmoxVNCClient constructs the minimal REST client from configuration.
// Called once per GetVNCTicket/RelayConsole invocation from cluster.Proxmox's
// own fields — see the comment on proxmoxVNCClient.
func newProxmoxVNCClient(baseURL, tokenName, tokenValue string) proxmoxVNCClient {
	return proxmoxVNCClient{
		baseURL:      baseURL,
		apiTokenName: tokenName,
		apiTokenVal:  tokenValue,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}
