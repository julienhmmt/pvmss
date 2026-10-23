package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A VM write that outlives the server timeouts (lock retry, bulk loop) must
// keep its request context alive and still deliver its response, instead of
// an aborted action and a dropped connection. The server mirrors production:
// a read timeout shorter than the write timeout, both shorter than the work.
func TestWithLongWriteDeadline_ResponseOutlivesServerTimeouts(t *testing.T) {
	t.Parallel()

	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)

		select {
		case <-time.After(300 * time.Millisecond):
			_, _ = w.Write([]byte("done"))
		case <-r.Context().Done():
			_, _ = w.Write([]byte("canceled"))
		}
	})

	server := httptest.NewUnstartedServer(withLongWriteDeadline(slow))
	server.Config.ReadTimeout = 50 * time.Millisecond
	server.Config.WriteTimeout = 100 * time.Millisecond
	server.Start()
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v (response cut by the server write timeout)", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "done" {
		t.Fatalf("body = %q, err = %v; want \"done\"", body, err)
	}
}
