package httpapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// tracesPath is the browser OTLP export route. It is also referenced by
	// the tracing filter, which must not trace the proxy's own export.
	tracesPath = "/api/v1/traces"

	// tracesBodyLimit caps the forwarded export. The SPA sends one pageview
	// span (~1 kB); the cap exists because the proxy forwards untrusted bytes.
	tracesBodyLimit = 64 << 10
	// tracesUpstreamTimeout bounds the collector round trip. Export is
	// best-effort: a slow collector must never hang a request goroutine.
	tracesUpstreamTimeout = 5 * time.Second
	// tracesUpstreamDrain bounds how much of a refused upstream response is
	// read so the keepalive connection can be reused.
	tracesUpstreamDrain = 1 << 10
)

// TracesProxy forwards browser OTLP/HTTP trace exports (POST /api/v1/traces)
// to the collector the server is configured for. The body is relayed
// verbatim - never decoded, never logged (logging rules: no request bodies) -
// and the collector credentials from OTEL_EXPORTER_OTLP_HEADERS are attached
// server-side so they stay out of the browser.
type TracesProxy struct {
	endpoint string
	header   http.Header
	client   *http.Client
}

// NewTracesProxy builds the proxy for the resolved collector trace endpoint
// (telemetry.TraceEndpoint) and its configured headers (telemetry.TraceHeaders).
func NewTracesProxy(endpoint string, header http.Header) *TracesProxy {
	return &TracesProxy{
		endpoint: endpoint,
		header:   header,
		client:   &http.Client{Timeout: tracesUpstreamTimeout},
	}
}

// ServeHTTP handles POST /api/v1/traces. Unauthenticated like client-errors -
// a page view must export before login too - and per-IP rate limited by the
// router. A collector refusal or outage is a 502; the client treats export as
// fire-and-forget either way.
func (h *TracesProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, tracesBodyLimit))
	if err != nil {
		writeAuthError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "trace export body too large")
		return
	}

	up, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.endpoint, bytes.NewReader(body))
	if err != nil {
		SetErrorMsg(w, "build trace export forward request", err)
		writeAuthError(w, http.StatusBadGateway, "upstream_error", "trace export failed")

		return
	}

	up.Header = h.header.Clone()
	if up.Header == nil {
		up.Header = http.Header{}
	}

	up.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(up)
	if err != nil {
		SetErrorMsg(w, "forward trace export", err)
		writeAuthError(w, http.StatusBadGateway, "upstream_error", "trace export failed")

		return
	}

	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, tracesUpstreamDrain))

	if resp.StatusCode >= http.StatusMultipleChoices {
		SetErrorMsg(w, "collector refused trace export", fmt.Errorf("upstream status %d", resp.StatusCode))
		writeAuthError(w, http.StatusBadGateway, "upstream_error", "trace export failed")

		return
	}

	w.WriteHeader(http.StatusNoContent)
}
