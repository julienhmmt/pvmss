package cluster

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestProxmox_TLSVerifyFailureIsNotRetried - an untrusted certificate fails
// the same way every time; retrying only delays the error.
func TestProxmox_TLSVerifyFailureIsNotRetried(t *testing.T) {
	t.Parallel()

	var handshakes atomic.Int32

	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			handshakes.Add(1)
		}
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	_, err := p.ListPools(context.Background())
	if !errors.Is(err, ErrTLSVerify) {
		t.Fatalf("err = %v, want ErrTLSVerify", err)
	}

	if n := handshakes.Load(); n != 1 {
		t.Errorf("connections = %d, want 1 (no retry on a certificate error)", n)
	}
}
