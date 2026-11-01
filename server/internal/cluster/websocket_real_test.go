package cluster

import (
	"bytes"
	"context"
	"crypto/des" //nolint:gosec // the test verifies the RFB-mandated DES challenge answer
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// rfbHandshakeObserved carries what the fake Proxmox-side endpoint captured
// during the VNC Authentication handshake, for out-of-band verification by
// the test body.
type rfbHandshakeObserved struct {
	challenge [16]byte // what the server sent
	response  [16]byte // what the client answered
	browser   string   // first post-handshake frame forwarded from the browser peer
}

// proxmoxVNCAuthServer emulates Proxmox's vncwebsocket endpoint in
// websocket=1 mode: it accepts the upgrade, runs the RFB server side of the
// security negotiation offering the types in secTypes, and when VNC
// Authentication is selected sends challenge and records the DES answer.
// Post-handshake it pushes one marker frame toward the browser and records
// the first frame the relay forwards back. Observations are sent on observed
// once the exchange completes; failures close the connection silently (the
// test's ctx timeout is the diagnostic).
func proxmoxVNCAuthServer(t *testing.T, secTypes []byte, challenge [16]byte, observed chan<- rfbHandshakeObserved) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = ws.CloseNow() }()

		conn := websocket.NetConn(r.Context(), ws, websocket.MessageBinary)
		defer func() { _ = conn.Close() }()

		// RFB server speaks first: banner, then the client's version reply.
		if _, err := conn.Write([]byte(rfbClientVersion)); err != nil {
			return
		}

		banner := make([]byte, len(rfbClientVersion))
		if _, err := io.ReadFull(conn, banner); err != nil {
			return
		}

		offer := append([]byte{byte(len(secTypes))}, secTypes...)
		if _, err := conn.Write(offer); err != nil {
			return
		}

		var chosen [1]byte
		if _, err := io.ReadFull(conn, chosen[:]); err != nil {
			return
		}

		obs := rfbHandshakeObserved{challenge: challenge}

		if chosen[0] == rfbSecTypeVNCAuth {
			if _, err := conn.Write(challenge[:]); err != nil {
				return
			}

			if _, err := io.ReadFull(conn, obs.response[:]); err != nil {
				return
			}
		}

		if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
			return
		}

		if _, err := conn.Write([]byte("proxmox-data")); err != nil {
			return
		}

		buf := make([]byte, 32)

		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		obs.browser = string(buf[:n])
		observed <- obs
	}))
}

// TestProxmoxRelayConsole_VNCAuthHandshake exercises the real relay end to
// end - exported RelayConsole through proxmoxRelayConsole,
// completeProxmoxVNCAuth and the RFB helpers - against a fake Proxmox
// endpoint that demands VNC Authentication. The relay must answer the DES
// challenge with the ticket as password, present the browser a
// security-type-None facade, and then pipe bytes both ways. The ticket never
// reaches the browser.
func TestProxmoxRelayConsole_VNCAuthHandshake(t *testing.T) {
	t.Parallel()

	observed := make(chan rfbHandshakeObserved, 1)
	challenge := [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

	srv := proxmoxVNCAuthServer(t, []byte{rfbSecTypeNone, rfbSecTypeVNCAuth}, challenge, observed)
	t.Cleanup(srv.Close)

	browser, relaySide := net.Pipe()
	t.Cleanup(func() { _ = browser.Close() })

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	relayDone := make(chan error, 1)
	go func() {
		relayDone <- p.RelayConsole(ctx, "cluster-a", testVMID,
			VNCProxyTicket{Ticket: testTicket, Port: 5900, Node: testNodeName}, relaySide)
	}()

	if err := browser.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("browser deadline: %v", err)
	}

	// Facade: the browser sees a plain security-type-None RFB handshake.
	banner := make([]byte, len(rfbClientVersion))
	if _, err := io.ReadFull(browser, banner); err != nil {
		t.Fatalf("read banner: %v", err)
	}

	if string(banner) != rfbClientVersion {
		t.Fatalf("banner = %q, want %q", banner, rfbClientVersion)
	}

	if _, err := browser.Write([]byte(rfbClientVersion)); err != nil {
		t.Fatalf("write version reply: %v", err)
	}

	offer := make([]byte, 2)
	if _, err := io.ReadFull(browser, offer); err != nil {
		t.Fatalf("read security offer: %v", err)
	}

	if want := []byte{1, rfbSecTypeNone}; !bytes.Equal(offer, want) {
		t.Fatalf("security offer = %v, want %v", offer, want)
	}

	if _, err := browser.Write([]byte{rfbSecTypeNone}); err != nil {
		t.Fatalf("select none: %v", err)
	}

	result := make([]byte, 4)
	if _, err := io.ReadFull(browser, result); err != nil {
		t.Fatalf("read security result: %v", err)
	}

	if !bytes.Equal(result, []byte{0, 0, 0, 0}) {
		t.Fatalf("security result = %v, want all-zero OK", result)
	}

	// The relay then pipes bytes both ways.
	marker := make([]byte, len("proxmox-data"))
	if _, err := io.ReadFull(browser, marker); err != nil {
		t.Fatalf("read proxmox marker: %v", err)
	}

	if string(marker) != "proxmox-data" {
		t.Fatalf("marker = %q, want %q", marker, "proxmox-data")
	}

	if _, err := browser.Write([]byte("browser-data")); err != nil {
		t.Fatalf("write browser marker: %v", err)
	}

	// Server side: the DES answer must decrypt back to the challenge under a
	// key derived from the ticket - proof the real handshake ran with the
	// real password.
	select {
	case obs := <-observed:
		block, err := des.NewCipher(vncDESKey(testTicket))
		if err != nil {
			t.Fatalf("des cipher: %v", err)
		}

		got := make([]byte, 16)
		block.Decrypt(got[0:8], obs.response[0:8])
		block.Decrypt(got[8:16], obs.response[8:16])

		if !bytes.Equal(got, obs.challenge[:]) {
			t.Errorf("challenge response decrypts to %x, want challenge %x", got, obs.challenge)
		}

		if obs.browser != "browser-data" {
			t.Errorf("browser->proxmox frame = %q, want %q", obs.browser, "browser-data")
		}
	case <-ctx.Done():
		t.Fatal("proxmox-side fake never completed the handshake")
	}

	if err := browser.Close(); err != nil {
		t.Fatalf("close browser: %v", err)
	}

	select {
	case err := <-relayDone:
		t.Logf("relay returned: %v", err)
	case <-ctx.Done():
		t.Fatal("relay did not return after peer close")
	}
}

// TestProxmoxRelayConsole_UnsupportedSecurityTypes - when Proxmox offers only
// security types PVMSS cannot speak, the relay must fail before touching the
// browser peer rather than guessing at an auth scheme.
func TestProxmoxRelayConsole_UnsupportedSecurityTypes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = ws.CloseNow() }()

		conn := websocket.NetConn(r.Context(), ws, websocket.MessageBinary)
		defer func() { _ = conn.Close() }()

		if _, err := conn.Write([]byte(rfbClientVersion)); err != nil {
			return
		}

		banner := make([]byte, len(rfbClientVersion))
		if _, err := io.ReadFull(conn, banner); err != nil {
			return
		}
		// Offer only types the client does not implement.
		_, _ = conn.Write([]byte{2, 3, 9})
	}))
	t.Cleanup(srv.Close)

	browser, relaySide := net.Pipe()
	defer func() { _ = browser.Close() }()

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.RelayConsole(context.Background(), "cluster-a", testVMID,
		VNCProxyTicket{Ticket: testTicket, Port: 5900, Node: testNodeName}, relaySide)
	if err == nil || !strings.Contains(err.Error(), "no supported security type") {
		t.Fatalf("RelayConsole error = %v, want 'no supported security type'", err)
	}
}

// TestProxmoxGetTermProxy_EndToEnd mirrors TestProxmoxGetVNCTicket_EndToEnd
// for the termproxy endpoint: the real REST path parses ticket and port out
// of the data envelope.
func TestProxmoxGetTermProxy_EndToEnd(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"ticket":"`+testTicket+`","port":"5903"}}`)
	}))
	t.Cleanup(srv.Close)

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	got, err := p.GetTermProxy(context.Background(), "cluster-a", testVMID, testNodeName)
	if err != nil {
		t.Fatalf("GetTermProxy: %v", err)
	}

	if got.Ticket != testTicket || got.Port != 5903 || got.Node != testNodeName {
		t.Fatalf("got %+v, want ticket=%q port=5903 node=%q", got, testTicket, testNodeName)
	}
}

func TestReverseByte(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want byte
	}{
		{0x00, 0x00},
		{0xFF, 0xFF},
		{0x01, 0x80}, // 00000001 -> 10000000
		{0x80, 0x01}, // 10000000 -> 00000001
		{0xA5, 0xA5}, // 10100101 is a palindrome
		{0x0F, 0xF0},
	}

	for _, c := range cases {
		if got := reverseByte(c.in); got != c.want {
			t.Errorf("reverseByte(%#02x) = %#02x, want %#02x", c.in, got, c.want)
		}
	}
}

func TestVNCDESKey(t *testing.T) {
	t.Parallel()

	// A password of exactly 8 non-zero bytes: every byte gets bit-reversed,
	// nothing gets padded.
	key := vncDESKey("password")

	want := []byte{
		reverseByte('p'), reverseByte('a'), reverseByte('s'), reverseByte('s'),
		reverseByte('w'), reverseByte('o'), reverseByte('r'), reverseByte('d'),
	}
	if !bytes.Equal(key, want) {
		t.Errorf("vncDESKey(%q) = %x, want %x", "password", key, want)
	}

	// Shorter than 8 bytes: only the significant bytes are reversed, the
	// rest stays zero-padded (VNC's password truncation rule).
	short := vncDESKey("ab")

	wantShort := []byte{reverseByte('a'), reverseByte('b'), 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(short, wantShort) {
		t.Errorf("vncDESKey(%q) = %x, want %x", "ab", short, wantShort)
	}

	// Longer than 8 bytes: only the first 8 are significant (matches
	// Proxmox's own truncation of the VNC ticket-as-password).
	long := vncDESKey("abcdefghij")

	wantLong := []byte{
		reverseByte('a'), reverseByte('b'), reverseByte('c'), reverseByte('d'),
		reverseByte('e'), reverseByte('f'), reverseByte('g'), reverseByte('h'),
	}
	if !bytes.Equal(long, wantLong) {
		t.Errorf("vncDESKey(%q) = %x, want %x", "abcdefghij", long, wantLong)
	}

	if got := len(vncDESKey("")); got != 8 {
		t.Errorf("vncDESKey(\"\") length = %d, want 8", got)
	}
}
