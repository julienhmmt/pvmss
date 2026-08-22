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
// uses. Errors carries per-field validation messages on a 4xx response.
type proxmoxEnvelope struct {
	Data   json.RawMessage   `json:"data"`
	Errors map[string]string `json:"errors,omitempty"`
}

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
	// ticket/csrf, when set, authenticate as a specific end user via a PVE
	// ticket instead of the service API token. Used only by Authenticate's
	// own follow-up calls and by ChangePassword, both of which must act with
	// that user's own privileges (self-service password change; permission
	// introspection scoped to that user) rather than the service account's.
	ticket string
	csrf   string
}

func (p Proxmox) rest() proxmoxRESTClient {
	return newProxmoxREST(p.BaseURL, p.APITokenName, p.APITokenValue, p.TLSInsecureSkipVerify)
}

func newProxmoxREST(baseURL, tokenName, tokenValue string, insecureSkipVerify bool) proxmoxRESTClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,   // minimum TLS 1.2 enforced
			InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // operator-configured per cluster; defaults to false
		},
	}

	return proxmoxRESTClient{
		base:       apiBase(baseURL),
		tokenName:  tokenName,
		tokenValue: tokenValue,
		http:       &http.Client{Timeout: proxmoxTimeout, Transport: transport},
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

// do executes one authenticated call and returns the decoded "data" payload.
// GET requests encode form as a query string; every other method sends it as
// an application/x-www-form-urlencoded body, matching Proxmox's own API.
func (c proxmoxRESTClient) do(ctx context.Context, method, path string, form url.Values) (json.RawMessage, error) {
	req, err := c.buildRequest(ctx, method, path, form)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read proxmox response: %w", err)
	}

	return parseProxmoxResponse(method, path, resp.StatusCode, raw)
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
func parseProxmoxResponse(method, path string, status int, raw []byte) (json.RawMessage, error) {
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if status >= http.StatusBadRequest {
		return nil, fmt.Errorf("proxmox %s %s: HTTP %d: %s", method, path, status, proxmoxErrorMessage(raw))
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
// response body, falling back to the raw (trimmed) body when it isn't the
// expected {"data":null,"errors":{...}} shape.
func proxmoxErrorMessage(raw []byte) string {
	var envelope proxmoxEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Errors) == 0 {
		return strings.TrimSpace(string(raw))
	}

	parts := make([]string, 0, len(envelope.Errors))
	for field, message := range envelope.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", field, message))
	}

	return strings.Join(parts, "; ")
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
