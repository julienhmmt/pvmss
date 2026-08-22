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
	"bytes"
	"context"
	"crypto/des" //nolint:gosec // required by the RFB spec's VNC Authentication type, not chosen for strength
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// RFB protocol constants (RFC 6143) used only for the handshake this file
// intercepts — version negotiation and the two security types Proxmox's
// vncwebsocket endpoint can offer. Everything past SecurityResult (ClientInit
// onward) is opaque framebuffer protocol PVMSS never needs to parse.
const (
	rfbClientVersion  = "RFB 003.008\n"
	rfbSecTypeNone    = 1
	rfbSecTypeVNCAuth = 2
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

// proxmoxTermProxyResponse is the JSON envelope Proxmox returns from
// POST /nodes/{node}/qemu/{vmid}/termproxy. It mirrors vncproxy's shape:
// data.{ticket, port, user}. PVMSS uses Ticket and Port (carried in the
// serial vncwebsocket URL); User is unused.
type proxmoxTermProxyResponse struct {
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
	c := newProxmoxVNCClient(p.BaseURL, p.APITokenName, p.APITokenValue, p.TLSInsecureSkipVerify)
	return proxmoxGetVNCTicket(ctx, c, node, vmid)
}

// RelayConsole implements ConsoleRelay for the real Proxmox client. It dials
// Proxmox's own vncwebsocket endpoint and relays frames bidirectionally
// between the browser WebSocket (peer) and Proxmox until either side closes.
//
// Proxmox is not reachable in the tranche's own demo or unit tests; this
// method is exercised only by integration tests against a live endpoint.
func (p Proxmox) RelayConsole(ctx context.Context, _ string, vmid int, proxy VNCProxyTicket, peer io.ReadWriteCloser) error {
	c := newProxmoxVNCClient(p.BaseURL, p.APITokenName, p.APITokenValue, p.TLSInsecureSkipVerify)
	return proxmoxRelayConsole(ctx, c, proxy.Node, vmid, proxy, peer)
}

// GetTermProxy implements TerminalRelay for the real Proxmox client. It calls
// Proxmox's termproxy endpoint for (node, vmid) and returns the Proxmox-side
// ticket and port. The node is always Resolve()'s server-resolved value — the
// caller never supplies one (FR-007). Same auth-header pattern as
// proxmoxGetVNCTicket.
//
// Proxmox is not reachable in the tranche's own demo or unit tests; this
// method is exercised only by integration tests against a live endpoint.
func (p Proxmox) GetTermProxy(ctx context.Context, _ string, vmid int, node string) (TermProxyTicket, error) {
	c := newProxmoxVNCClient(p.BaseURL, p.APITokenName, p.APITokenValue, p.TLSInsecureSkipVerify)
	return proxmoxGetTermProxy(ctx, c, node, vmid)
}

// RelaySerial implements TerminalRelay for the real Proxmox client. It dials
// Proxmox's vncwebsocket endpoint (the SAME endpoint VNC uses — Proxmox
// multiplexes serial tunnels through it via the termproxy ticket) and relays
// raw bytes bidirectionally between the browser WebSocket (peer) and Proxmox
// until either side closes. There is NO RFB handshake and NO DES auth for
// serial — the vncwebsocket endpoint carries the serial tunnel as an
// already-framed byte stream; PVMSS is a dumb byte pipe and the browser-side
// xterm.js layer owns the "type:payload" framing.
//
// Proxmox is not reachable in the tranche's own demo or unit tests; this
// method is exercised only by integration tests against a live endpoint.
func (p Proxmox) RelaySerial(ctx context.Context, _ string, vmid int, proxy TermProxyTicket, peer io.ReadWriteCloser) error {
	c := newProxmoxVNCClient(p.BaseURL, p.APITokenName, p.APITokenValue, p.TLSInsecureSkipVerify)
	return proxmoxRelaySerial(ctx, c, proxy.Node, vmid, proxy, peer)
}

// --- The real flow, called directly from GetVNCTicket and RelayConsole above.
// Kept as free functions taking proxmoxVNCClient rather than methods on
// Proxmox so the "idea reused from B11" stays a small, reviewable unit,
// separate from the ConsoleRelay interface's method shape. ---

// proxmoxGetVNCTicket dials the vncproxy endpoint and returns the ticket+port.
func proxmoxGetVNCTicket(ctx context.Context, c proxmoxVNCClient, node string, vmid int) (VNCProxyTicket, error) {
	endpoint := fmt.Sprintf("%s/nodes/%s/qemu/%d/vncproxy", apiBase(c.baseURL), url.PathEscape(node), vmid)

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

	// A long-lived WebSocket must never dial through an *http.Client with
	// Timeout set — that timer bounds the whole connection lifetime, not
	// just the handshake, and silently kills the relay ~Timeout after open
	// (coder/websocket's own dial docs warn against this). Reuse the same
	// transport (TLS config) but with no Timeout; ctx is what bounds this dial.
	dialClient := &http.Client{Transport: c.httpClient.Transport}

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	proxmoxConn, _, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{ //nolint:bodyclose // coder/websocket owns the response body lifecycle per its Dial docs
		HTTPHeader: http.Header{
			"Authorization": []string{fmt.Sprintf("PVEAPIToken=%s=%s", c.apiTokenName, c.apiTokenVal)},
		},
		HTTPClient: dialClient,
	})
	if err != nil {
		return fmt.Errorf("dial proxmox vncwebsocket: %w", err)
	}
	defer func() { _ = proxmoxConn.CloseNow() }()

	proxmoxNetConn := websocket.NetConn(ctx, proxmoxConn, websocket.MessageBinary)
	defer func() { _ = proxmoxNetConn.Close() }()

	// Proxmox's websocket=1 vncproxy mode always demands RFB "VNC
	// Authentication" (security type 2) using the ticket itself as the DES
	// password (RFC 6143 §7.2.2) — the URL vncticket only authorizes the
	// WebSocket upgrade, not the RFB session riding on top of it. PVMSS
	// deliberately never sends that ticket to the browser (opaque token
	// only), so we complete this handshake ourselves here, then present the
	// browser a "security type: None" facade — it never sees Proxmox's auth
	// requirement, or the ticket, at all.
	if err := completeProxmoxVNCAuth(proxmoxNetConn, proxy.Ticket); err != nil {
		return fmt.Errorf("proxmox vnc authentication: %w", err)
	}
	if err := presentNoAuthToPeer(peer); err != nil {
		return fmt.Errorf("present no-auth handshake to browser: %w", err)
	}

	errCh := make(chan error, 2)

	go func() { _, err := io.Copy(proxmoxNetConn, peer); errCh <- err }()
	go func() { _, err := io.Copy(peer, proxmoxNetConn); errCh <- err }()

	err = <-errCh
	_ = proxmoxNetConn.Close()
	_ = peer.Close()

	return err
}

// proxmoxGetTermProxy dials the termproxy endpoint and returns the ticket+port.
// It mirrors proxmoxGetVNCTicket but POSTs to .../termproxy and decodes a
// proxmoxTermProxyResponse. The termproxy endpoint does not take a
// "websocket=1" form field (vncproxy does); it returns the port directly.
func proxmoxGetTermProxy(ctx context.Context, c proxmoxVNCClient, node string, vmid int) (TermProxyTicket, error) {
	endpoint := fmt.Sprintf("%s/nodes/%s/qemu/%d/termproxy", apiBase(c.baseURL), url.PathEscape(node), vmid)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(""))
	if err != nil {
		return TermProxyTicket{}, fmt.Errorf("build termproxy request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.apiTokenName, c.apiTokenVal))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TermProxyTicket{}, fmt.Errorf("termproxy request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return TermProxyTicket{}, fmt.Errorf("termproxy returned %d", resp.StatusCode)
	}

	var envelope proxmoxTermProxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return TermProxyTicket{}, fmt.Errorf("decode termproxy response: %w", err)
	}

	port, err := strconv.Atoi(envelope.Data.Port)
	if err != nil {
		return TermProxyTicket{}, fmt.Errorf("invalid termproxy port %q: %w", envelope.Data.Port, err)
	}

	return TermProxyTicket{Ticket: envelope.Data.Ticket, Port: port, Node: node}, nil
}

// proxmoxRelaySerial dials Proxmox's vncwebsocket endpoint (the same endpoint
// VNC uses — Proxmox multiplexes serial tunnels through it via the termproxy
// ticket) and copies raw bytes both ways between the browser peer and Proxmox
// until either side closes. There is NO RFB handshake and NO DES auth for
// serial — the vncwebsocket endpoint carries the serial tunnel as an
// already-framed byte stream; PVMSS is a dumb byte pipe and the browser-side
// xterm.js layer owns the "type:payload" framing.
func proxmoxRelaySerial(ctx context.Context, c proxmoxVNCClient, node string, vmid int, proxy TermProxyTicket, peer io.ReadWriteCloser) error {
	wsURL := buildProxmoxVNCWebSocketURL(c.baseURL, node, vmid, proxy.Port, proxy.Ticket)

	// Same long-lived-WebSocket dial constraint as proxmoxRelayConsole: never
	// dial through an *http.Client with Timeout set — that timer bounds the
	// whole connection lifetime, not just the handshake. Reuse the transport
	// (TLS config) but with no Timeout; ctx bounds this dial.
	dialClient := &http.Client{Transport: c.httpClient.Transport}

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	proxmoxConn, _, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{ //nolint:bodyclose // coder/websocket owns the response body lifecycle per its Dial docs
		HTTPHeader: http.Header{
			"Authorization": []string{fmt.Sprintf("PVEAPIToken=%s=%s", c.apiTokenName, c.apiTokenVal)},
		},
		HTTPClient: dialClient,
	})
	if err != nil {
		return fmt.Errorf("dial proxmox vncwebsocket (serial): %w", err)
	}
	defer func() { _ = proxmoxConn.CloseNow() }()

	proxmoxNetConn := websocket.NetConn(ctx, proxmoxConn, websocket.MessageBinary)
	defer func() { _ = proxmoxNetConn.Close() }()

	// Plain bidirectional byte pipe — no RFB handshake, no DES auth. The
	// browser-side xterm.js layer encodes keystrokes as "0:len:data" and
	// decodes "0:len:output"; PVMSS never inspects or terminates the framing.
	errCh := make(chan error, 2)

	go func() { _, err := io.Copy(proxmoxNetConn, peer); errCh <- err }()
	go func() { _, err := io.Copy(peer, proxmoxNetConn); errCh <- err }()

	err = <-errCh
	_ = proxmoxNetConn.Close()
	_ = peer.Close()

	return err
}

// completeProxmoxVNCAuth performs the RFB version + security handshake with
// Proxmox on PVMSS's own behalf, leaving proxmoxNetConn positioned right
// after SecurityResult (ready for ClientInit/ServerInit — pure byte relay
// from there on).
func completeProxmoxVNCAuth(conn io.ReadWriter, ticket string) error {
	if err := rfbClientVersionHandshake(conn); err != nil {
		return fmt.Errorf("version handshake: %w", err)
	}

	secType, err := rfbChooseSecurityType(conn)
	if err != nil {
		return fmt.Errorf("security type negotiation: %w", err)
	}

	if _, err := conn.Write([]byte{secType}); err != nil {
		return fmt.Errorf("select security type %d: %w", secType, err)
	}

	if secType == rfbSecTypeVNCAuth {
		if err := rfbAnswerVNCAuthChallenge(conn, ticket); err != nil {
			return fmt.Errorf("vnc-auth challenge: %w", err)
		}
	}

	return rfbReadSecurityResult(conn)
}

// presentNoAuthToPeer plays the RFB *server* role toward the browser: sends
// the version banner, reads the browser's reply, offers exactly one security
// type (None), reads the browser's (mandatory, even for one option)
// selection echo, and sends an OK SecurityResult. The browser proceeds
// straight to ClientInit believing no authentication was ever required.
func presentNoAuthToPeer(peer io.ReadWriter) error {
	if err := rfbServerVersionHandshake(peer); err != nil {
		return fmt.Errorf("version handshake: %w", err)
	}

	if _, err := peer.Write([]byte{1, rfbSecTypeNone}); err != nil {
		return fmt.Errorf("offer security type none: %w", err)
	}

	var chosen [1]byte
	if _, err := io.ReadFull(peer, chosen[:]); err != nil {
		return fmt.Errorf("read browser security type selection: %w", err)
	}

	if chosen[0] != rfbSecTypeNone {
		return fmt.Errorf("browser selected unexpected security type %d", chosen[0])
	}

	if _, err := peer.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("write security result: %w", err)
	}

	return nil
}

// rfbClientVersionHandshake plays the RFB *client* role: the peer (Proxmox)
// speaks first, so this reads its version banner before replying.
func rfbClientVersionHandshake(conn io.ReadWriter) error {
	banner := make([]byte, 12)
	if _, err := io.ReadFull(conn, banner); err != nil {
		return fmt.Errorf("read version banner: %w", err)
	}

	if !bytes.HasPrefix(banner, []byte("RFB ")) {
		return fmt.Errorf("unexpected version banner %q", banner)
	}

	if _, err := conn.Write([]byte(rfbClientVersion)); err != nil {
		return fmt.Errorf("write version reply: %w", err)
	}

	return nil
}

// rfbServerVersionHandshake plays the RFB *server* role: PVMSS speaks first
// to the browser, so this writes the version banner before reading the
// browser's reply. Getting this order backwards deadlocks both sides —
// each waiting to read a banner the other is also waiting to read first.
func rfbServerVersionHandshake(conn io.ReadWriter) error {
	if _, err := conn.Write([]byte(rfbClientVersion)); err != nil {
		return fmt.Errorf("write version banner: %w", err)
	}

	reply := make([]byte, 12)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("read version reply: %w", err)
	}

	if !bytes.HasPrefix(reply, []byte("RFB ")) {
		return fmt.Errorf("unexpected version reply %q", reply)
	}

	return nil
}

// rfbChooseSecurityType reads Proxmox's offered security-type list and picks
// VNC Authentication if offered (Proxmox's websocket=1 mode always offers
// it), falling back to None if that's all Proxmox offers.
func rfbChooseSecurityType(conn io.ReadWriter) (byte, error) {
	var count [1]byte
	if _, err := io.ReadFull(conn, count[:]); err != nil {
		return 0, fmt.Errorf("read security type count: %w", err)
	}

	if count[0] == 0 {
		reason, _ := rfbReadReasonString(conn)
		return 0, fmt.Errorf("server rejected connection: %s", reason)
	}

	types := make([]byte, count[0])
	if _, err := io.ReadFull(conn, types); err != nil {
		return 0, fmt.Errorf("read security types: %w", err)
	}

	if slices.Contains(types, byte(rfbSecTypeVNCAuth)) {
		return rfbSecTypeVNCAuth, nil
	}

	if slices.Contains(types, byte(rfbSecTypeNone)) {
		return rfbSecTypeNone, nil
	}

	return 0, fmt.Errorf("no supported security type in %v", types)
}

// rfbAnswerVNCAuthChallenge reads Proxmox's 16-byte DES challenge and
// answers it using the ticket string as the VNC password (RFC 6143 §7.2.2:
// the password is DES-encrypted, in two independent 8-byte ECB blocks, using
// a key derived from the password's first 8 bytes with each byte's bits
// reversed — a quirk of the original VNC protocol, not modern DES usage).
func rfbAnswerVNCAuthChallenge(conn io.ReadWriter, password string) error {
	challenge := make([]byte, 16)
	if _, err := io.ReadFull(conn, challenge); err != nil {
		return fmt.Errorf("read challenge: %w", err)
	}

	block, err := des.NewCipher(vncDESKey(password)) //nolint:gosec // RFB spec mandates DES for this legacy security type
	if err != nil {
		return fmt.Errorf("build des cipher: %w", err)
	}

	response := make([]byte, 16)
	block.Encrypt(response[0:8], challenge[0:8])
	block.Encrypt(response[8:16], challenge[8:16])

	if _, err := conn.Write(response); err != nil {
		return fmt.Errorf("write challenge response: %w", err)
	}

	return nil
}

// rfbReadSecurityResult reads the 4-byte SecurityResult and, on failure,
// the RFB-3.8-style reason string that follows it.
func rfbReadSecurityResult(conn io.ReadWriter) error {
	var result [4]byte
	if _, err := io.ReadFull(conn, result[:]); err != nil {
		return fmt.Errorf("read security result: %w", err)
	}

	if binary.BigEndian.Uint32(result[:]) != 0 {
		reason, _ := rfbReadReasonString(conn)
		return fmt.Errorf("authentication failed: %s", reason)
	}

	return nil
}

// rfbReadReasonString reads an RFB-3.8-style [u32 length][bytes] failure
// reason. Errors are ignored by callers — this only enriches an already-
// failing error path, never the sole failure signal.
func rfbReadReasonString(conn io.ReadWriter) (string, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return "", err
	}

	n := binary.BigEndian.Uint32(lenBuf[:])
	const maxReasonLen = 4096 // guard against a malicious/broken length prefix
	if n > maxReasonLen {
		n = maxReasonLen
	}

	reason := make([]byte, n)
	if _, err := io.ReadFull(conn, reason); err != nil {
		return "", err
	}

	return string(reason), nil
}

// vncDESKey derives the 8-byte DES key from a VNC password: the first 8
// bytes (null-padded if shorter), each with its bits reversed — VNC's
// historical quirk (RFC 6143 §7.2.2), not a general DES convention.
func vncDESKey(password string) []byte {
	key := make([]byte, 8)
	pw := []byte(password)

	for i := range key {
		if i < len(pw) {
			key[i] = reverseByte(pw[i])
		}
	}

	return key
}

func reverseByte(b byte) byte {
	var r byte
	for range 8 {
		r = r<<1 | b&1
		b >>= 1
	}

	return r
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
// The TLS config enforces a minimum of TLS 1.2 and propagates the cluster's
// certificate verification policy (InsecureSkipVerify is only true when the
// operator explicitly disabled verification for that cluster).
func newProxmoxVNCClient(baseURL, tokenName, tokenValue string, insecureSkipVerify bool) proxmoxVNCClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,   // minimum TLS 1.2 enforced
			InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // operator-configured per cluster; defaults to false
			// Force HTTP/1.1: Go's transport auto-negotiates h2 via ALPN once
			// NextProtos is left empty, and Proxmox's api daemon doesn't speak
			// the RFC 8441 extended-CONNECT upgrade h2 would require for the
			// WebSocket handshake — it just hangs forever instead of erroring.
			NextProtos: []string{"http/1.1"},
		},
	}

	return proxmoxVNCClient{
		baseURL:      baseURL,
		apiTokenName: tokenName,
		apiTokenVal:  tokenValue,
		httpClient:   &http.Client{Timeout: 15 * time.Second, Transport: transport},
	}
}
