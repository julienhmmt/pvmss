package cluster

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestProxmoxRelaySerial_AuthenticatesFirst - termproxy drops a connection
// whose first frame is not "<user>:<ticket>\n"; the relay must send it before
// piping any browser bytes, then relay both ways.
func TestProxmoxRelaySerial_AuthenticatesFirst(t *testing.T) {
	t.Parallel()

	firstFrame := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()

		_, msg, err := conn.Read(r.Context())
		if err != nil {
			return
		}

		firstFrame <- string(msg)

		_ = conn.Write(r.Context(), websocket.MessageText, []byte("OK"))
	}))
	t.Cleanup(srv.Close)

	browser, relaySide := net.Pipe()
	c := proxmoxVNCClient{baseURL: srv.URL, apiTokenName: testTokenName, apiTokenVal: testTokenVal, httpClient: srv.Client()}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = proxmoxRelaySerial(ctx, c, testNodeName, testVMID,
			TermProxyTicket{Ticket: "PVEVNC:abc", Port: 5900}, relaySide)
	}()

	select {
	case got := <-firstFrame:
		if got != testTokenName+":PVEVNC:abc\n" {
			t.Fatalf("first frame = %q, want auth line", got)
		}
	case <-ctx.Done():
		t.Fatal("no frame reached proxmox")
	}

	buf := make([]byte, 2)
	_ = browser.SetReadDeadline(time.Now().Add(3 * time.Second))

	if _, err := browser.Read(buf); err != nil || string(buf) != "OK" {
		t.Fatalf("browser read = %q, %v; want OK", buf, err)
	}
}
