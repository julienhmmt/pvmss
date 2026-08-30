package cluster

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// proxmoxTimeout bounds every REST call the real client makes. A lab node's
// cold TLS handshake alone has been observed to take ~6s; 20s comfortably
// covers that plus request/response time without hanging a handler forever.
const proxmoxTimeout = 20 * time.Second

// proxmoxAuthHeaderFmt is the Proxmox API-token Authorization header value,
// formatted with the token name and token value.
const proxmoxAuthHeaderFmt = "PVEAPIToken=%s=%s"

// proxmoxEnvelope is the {"data": ...} wrapper every Proxmox API response
// uses. Errors carries per-field validation messages on a 4xx response;
// Message carries the top-level message some error responses use instead.
type proxmoxEnvelope struct {
	Data    json.RawMessage   `json:"data"`
	Errors  map[string]string `json:"errors,omitempty"`
	Message string            `json:"message,omitempty"`
}

// RejectionError wraps a 4xx/5xx Proxmox response: the HTTP status and
// Proxmox's own message (extracted by proxmoxErrorMessage) are kept so the
// HTTP layer can decide what to surface — the message describes the VM's
// storage or state, never cluster internals or credentials, but a 401/403
// body can name the token and must not be rendered. Errors.Is(err,
// ErrClusterRejected) matches it.
type RejectionError struct {
	Status  int
	Message string
	Method  string
	Path    string
}

func (e *RejectionError) Error() string {
	return fmt.Sprintf("proxmox %s %s: HTTP %d: %s", e.Method, e.Path, e.Status, e.Message)
}

// Unwrap exposes ErrClusterRejected so errors.Is works across the package
// boundary without string matching.
func (e *RejectionError) Unwrap() error { return ErrClusterRejected }

// proxmoxRESTClient is the shared low-level REST client every Proxmox method
// builds from the struct's own fields (or, for the two calls that must act as
// a specific end user rather than the service account, from a ticket). Cheap
// to construct — no connection happens until the first request — matching the
// existing proxmoxVNCClient pattern in websocket_real.go.
type proxmoxRESTClient struct {
	base       string // "scheme://host:port/api2/json", no trailing slash
	tokenName  string
	tokenValue string
	http       *http.Client
	// noRetry short-circuits the GET retry loop. Set by withNoRetry for short
	// probes (guest-agent exec-status) that legitimately hang until timeout —
	// retrying them only multiplies the wait and slows every list load.
	noRetry bool
	// ticket/csrf, when set, authenticate as a specific end user via a PVE
	// ticket instead of the service API token. Used only by Authenticate's
	// own follow-up calls and by ChangePassword, both of which must act with
	// that user's own privileges (self-service password change; permission
	// introspection scoped to that user) rather than the service account's.
	ticket string
	csrf   string
}

// rest builds a proxmoxRESTClient from the Proxmox struct's own fields,
// reusing the cached *http.Client so the Transport's keep-alive pool is
// shared across calls (ticket 07). Pointer receiver so the lazy init in
// ensureClient can mutate p.httpClient; every caller passes an addressable
// copy (the value-receiver Client methods), so this is safe.
func (p *Proxmox) rest() proxmoxRESTClient {
	p.ensureClient()

	return newProxmoxREST(p.BaseURL, p.APITokenName, p.APITokenValue, p.httpClient)
}

// ensureClient lazily initializes p.httpClient when nil (zero-value or
// test-constructed Proxmox) so rest() never panics. In production the field
// is set at construction in registry.go and this is a no-op.
func (p *Proxmox) ensureClient() {
	if p.httpClient != nil {
		return
	}

	p.httpClient = newProxmoxHTTPClient(p.TLSInsecureSkipVerify)
}

// newProxmoxHTTPClient builds an *http.Client with a tuned Transport: the Go
// default MaxIdleConnsPerHost is 2, far too low for the per-VM hydration
// burst (one /status/current per VM). Pooling eliminates the per-call TLS
// handshake cost that made the observed 20s timeout more likely.
func newProxmoxHTTPClient(insecureSkipVerify bool) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,   // minimum TLS 1.2 enforced
			InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // operator-configured per cluster; defaults to false
		},
	}

	return &http.Client{Timeout: proxmoxTimeout, Transport: transport}
}

func newProxmoxREST(baseURL, tokenName, tokenValue string, client *http.Client) proxmoxRESTClient {
	return proxmoxRESTClient{
		base:       apiBase(baseURL),
		tokenName:  tokenName,
		tokenValue: tokenValue,
		http:       client,
	}
}

// apiBase normalizes a cluster's configured URL to
// "scheme://host:port/api2/json", tolerating either form an operator might
// enter — with or without the "/api2/json" suffix already present — so the
// whole client (and the console relay in websocket_real.go) agree on one
// convention regardless of which way the "Add Cluster" form was filled in.
func apiBase(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.HasSuffix(trimmed, "/api2/json") {
		return trimmed
	}

	return trimmed + "/api2/json"
}

// withTicket returns a copy of c authenticated as a specific end user rather
// than the service account.
func (c proxmoxRESTClient) withTicket(ticket, csrf string) proxmoxRESTClient {
	c.ticket = ticket
	c.csrf = csrf

	return c
}

// withNoRetry returns a copy of c with the retry loop disabled. Used by
// short probes (guest-agent exec-status) that legitimately hang until
// timeout — retrying them only multiplies the wait.
func (c proxmoxRESTClient) withNoRetry() proxmoxRESTClient {
	c.noRetry = true

	return c
}

// retryMaxAttempts is the total number of attempts (1 initial + 2 retries)
// for a GET that keeps hitting transient failures, matching ProxMate's
// PROXMOX_RETRIES=2 budget.
const retryMaxAttempts = 3

// retryBaseBackoff and retryMaxBackoff bound the exponential backoff between
// retry attempts: 250ms, then 500ms, capped at 2s (ProxMate's
// Math.min(2_000, 250 * 2 ** (attempt - 1))).
const (
	retryBaseBackoff = 250 * time.Millisecond
	retryMaxBackoff  = 2 * time.Second
)

// do executes an authenticated call and returns the decoded "data" payload.
// GET requests are retried on transient failures (transport errors, HTTP
// status >= 500, or 429) with bounded exponential backoff, up to 2
// additional attempts. POST/PUT/DELETE are never retried — a create can't
// double-provision. Context cancellation during backoff returns promptly.
// The noRetry flag short-circuits the loop for short probes.
func (c proxmoxRESTClient) do(ctx context.Context, method, path string, form url.Values) (json.RawMessage, error) {
	if c.noRetry || method != http.MethodGet {
		raw, _, err := c.doOnce(ctx, method, path, form)
		return raw, err
	}

	for attempt := 1; ; attempt++ {
		raw, status, err := c.doOnce(ctx, method, path, form)
		if !isRetryableStatus(status, err) || attempt >= retryMaxAttempts {
			return raw, err
		}

		backoff := retryBackoff(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// retryBackoff returns the bounded exponential backoff for a given attempt
// number (1-based): 250ms, 500ms, 1s, 2s, 2s, and so on.
func retryBackoff(attempt int) time.Duration {
	backoff := retryBaseBackoff << (attempt - 1)
	if backoff > retryMaxBackoff || backoff < 0 {
		return retryMaxBackoff
	}

	return backoff
}

// isRetryableStatus reports whether a failed attempt should be retried: a
// transport error (status 0, no response received), HTTP >= 500, or 429. A
// nil error (success) or any other 4xx is not retryable — 404 is a
// legitimate ErrNotFound, and 400/401/403 are caller errors that won't fix
// themselves.
func isRetryableStatus(status int, err error) bool {
	if err == nil {
		return false
	}

	if status == 0 {
		return true
	}

	return status >= 500 || status == http.StatusTooManyRequests
}

// doOnce executes a single authenticated call and returns the decoded "data"
// payload, the HTTP status code (0 for transport errors with no response),
// and any error. GET requests encode form as a query string; every other
// method sends it as an application/x-www-form-urlencoded body, matching
// Proxmox's own API.
func (c proxmoxRESTClient) doOnce(ctx context.Context, method, path string, form url.Values) (json.RawMessage, int, error) {
	req, err := c.buildRequest(ctx, method, path, form)
	if err != nil {
		return nil, 0, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read proxmox response: %w", err)
	}

	data, perr := parseProxmoxResponse(method, path, resp.StatusCode, raw)

	return data, resp.StatusCode, perr
}

func (c proxmoxRESTClient) buildRequest(ctx context.Context, method, path string, form url.Values) (*http.Request, error) {
	target := c.base + path

	var body io.Reader

	var contentType string

	switch {
	case method == http.MethodGet || method == http.MethodDelete:
		// Go's own http.Request.ParseForm only reads a body for POST/PUT/PATCH
		// (net/http docs) — a DELETE body is silently ignored by many servers,
		// Proxmox's own API included. Query string works uniformly everywhere.
		if len(form) > 0 {
			target += "?" + form.Encode()
		}
	case form != nil:
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("build proxmox request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	c.authenticate(req)

	return req, nil
}

// authenticate attaches either the caller's own ticket (end-user self-service
// calls) or the service API token (everything else) to req. Ticket auth needs
// the CSRF prevention header on state-changing methods; token auth is exempt
// from CSRF by design, which is why the service account uses it.
func (c proxmoxRESTClient) authenticate(req *http.Request) {
	if c.ticket != "" {
		req.AddCookie(&http.Cookie{Name: "PVEAuthCookie", Value: c.ticket}) //nolint:gosec // request cookie: Secure/HttpOnly/SameSite are response-only directives, not applicable here

		if req.Method != http.MethodGet {
			req.Header.Set("CSRFPreventionToken", c.csrf)
		}

		return
	}

	req.Header.Set("Authorization", fmt.Sprintf(proxmoxAuthHeaderFmt, c.tokenName, c.tokenValue))
}

// parseProxmoxResponse maps a raw HTTP response to the cluster package's
// sentinel errors where one applies, or the decoded "data" payload otherwise.
// 404 stays ErrNotFound (the "resource absent" contract callers branch on);
// every other 4xx/5xx becomes a RejectionError carrying Proxmox's own
// message, so the HTTP layer can surface it instead of a generic 500.
func parseProxmoxResponse(method, path string, status int, raw []byte) (json.RawMessage, error) {
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if status >= http.StatusBadRequest {
		return nil, &RejectionError{Status: status, Message: proxmoxErrorMessage(raw), Method: method, Path: path}
	}

	if len(raw) == 0 {
		return nil, nil
	}

	var envelope proxmoxEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode proxmox response: %w", err)
	}

	return envelope.Data, nil
}

// proxmoxErrorMessage extracts a human-readable message from a Proxmox error
// response body. Proxmox uses two shapes: per-field {"data":null,"errors":
// {...}} validation errors, and a top-level {"data":null,"message":"..."}
// message. Both are read, falling back to the raw (trimmed) body otherwise.
func proxmoxErrorMessage(raw []byte) string {
	var envelope proxmoxEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return strings.TrimSpace(string(raw))
	}

	if len(envelope.Errors) > 0 {
		parts := make([]string, 0, len(envelope.Errors))
		for field, message := range envelope.Errors {
			parts = append(parts, fmt.Sprintf("%s: %s", field, message))
		}

		return strings.Join(parts, "; ")
	}

	if envelope.Message != "" {
		return envelope.Message
	}

	return strings.TrimSpace(string(raw))
}

// decodeData unmarshals a decoded "data" payload into out. A nil/empty
// payload leaves out untouched rather than erroring — some Proxmox endpoints
// return no data on success.
func decodeData[T any](raw json.RawMessage, out *T) error {
	if len(raw) == 0 {
		return nil
	}

	return json.Unmarshal(raw, out)
}

// decodeJSONBody decodes an HTTP response body directly into out — used only
// by proxmoxTicketAuth, which builds its own request outside of
// proxmoxRESTClient.do (see that function's doc comment for why).
func decodeJSONBody(resp *http.Response, out any) error {
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode proxmox response: %w", err)
	}

	return nil
}
