package proxmox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"

	"pvmss/logger"
)

// RestyClient is a resty-based Proxmox API client for modern API interactions.
//
// For token-authenticated callers the underlying *resty.Client is a process-wide
// singleton keyed by (baseURL, tokenID, tokenSecret, insecureSkipVerify) so that
// the TCP and TLS pool maintained by the shared http.Transport is reused across
// every handler. The wrapper retains a per-caller timeout that is enforced via
// context.WithTimeout on each request, so different callsites can keep their
// own deadlines without mutating the shared client.
type RestyClient struct {
	client  *resty.Client
	baseURL string
	timeout time.Duration
}

var (
	tokenClientMu sync.Mutex
	tokenClients  = make(map[string]*resty.Client)
)

func tokenClientCacheKey(baseURL, tokenID, tokenSecret string, insecureSkipVerify bool) string {
	sum := sha256.Sum256([]byte(baseURL + "\x00" + tokenID + "\x00" + tokenSecret))
	return fmt.Sprintf("%t|%s", insecureSkipVerify, hex.EncodeToString(sum[:]))
}

// MakeRestyClient returns a *RestyClient backed by the process-wide singleton
// *resty.Client for the given Proxmox API token configuration. The first call
// for a given (baseURL, token, skipVerify) tuple builds the client; subsequent
// calls reuse it. The timeout argument is stored on the wrapper and applied
// per-request via context.WithTimeout — it does not mutate the shared client.
func MakeRestyClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool, timeout time.Duration) (*RestyClient, error) {
	if apiURL == "" || apiTokenID == "" || apiTokenSecret == "" {
		return nil, fmt.Errorf("apiURL, apiTokenID, and apiTokenSecret are required")
	}

	normalizedURL, err := normalizeBaseURL(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Proxmox API URL: %w", err)
	}

	key := tokenClientCacheKey(normalizedURL, apiTokenID, apiTokenSecret, insecureSkipVerify)

	tokenClientMu.Lock()
	defer tokenClientMu.Unlock()

	client, ok := tokenClients[key]
	if !ok {
		client = buildTokenClient(normalizedURL, apiTokenID, apiTokenSecret, insecureSkipVerify)
		tokenClients[key] = client
	}

	return &RestyClient{
		client:  client,
		baseURL: normalizedURL,
		timeout: timeout,
	}, nil
}

func buildTokenClient(normalizedURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool) *resty.Client {
	client := resty.New()
	client.SetTransport(getSharedTransport(insecureSkipVerify))
	client.SetBaseURL(normalizedURL)
	// Client-level timeout is intentionally left at 0; per-request deadlines
	// are enforced via context so that one shared client can serve callers
	// with different deadline requirements.
	client.SetHeader("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", apiTokenID, apiTokenSecret))
	client.SetHeader("Accept", "application/json")
	client.SetHeader("Content-Type", "application/json")

	client.SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	client.OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
		logger.Get().Debug().
			Str("method", req.Method).
			Str("url", req.URL).
			Msg("Resty API request")
		return nil
	})

	client.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
		logger.Get().Debug().
			Str("method", resp.Request.Method).
			Str("url", resp.Request.URL).
			Int("status", resp.StatusCode()).
			Dur("duration", resp.Time()).
			Msg("Resty API response")
		return nil
	})

	return client
}

// MakeRestyClientCookieAuth creates a new resty-based Proxmox API client without API token auth.
// This is used for operations that require cookie-based authentication (PVEAuthCookie + CSRFPreventionToken),
// such as ticket creation, password updates, and VNC proxy.
//
// Each call returns a fresh *resty.Client because authentication cookies are
// per-user and must not be shared. Connection-pool reuse is still achieved via
// the shared http.Transport.
func MakeRestyClientCookieAuth(apiURL string, insecureSkipVerify bool, timeout time.Duration) (*RestyClient, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("apiURL is required")
	}

	normalizedURL, err := normalizeBaseURL(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Proxmox API URL: %w", err)
	}

	client := resty.New()
	client.SetTransport(getSharedTransport(insecureSkipVerify))
	client.SetBaseURL(normalizedURL)
	client.SetTimeout(timeout)

	client.SetHeader("Accept", "application/json")
	client.SetHeader("Content-Type", "application/json")

	client.SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	client.OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
		logger.Get().Debug().
			Str("method", req.Method).
			Str("url", req.URL).
			Msg("Resty API request (cookie auth)")
		return nil
	})

	client.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
		logger.Get().Debug().
			Str("method", resp.Request.Method).
			Str("url", resp.Request.URL).
			Int("status", resp.StatusCode()).
			Dur("duration", resp.Time()).
			Msg("Resty API response (cookie auth)")
		return nil
	})

	return &RestyClient{
		client:  client,
		baseURL: normalizedURL,
		timeout: timeout,
	}, nil
}

// SetCookieAuth sets PVEAuthCookie and CSRFPreventionToken on the underlying resty client
// for cookie-based authentication with the Proxmox API.
func (rc *RestyClient) SetCookieAuth(ticket, csrfToken string) {
	rc.client.SetCookie(&http.Cookie{
		Name:  "PVEAuthCookie",
		Value: ticket,
	})
	rc.client.SetHeader("CSRFPreventionToken", csrfToken)
}

// withRequestTimeout returns a derived context honoring the wrapper's per-caller
// timeout. If the caller already supplied a tighter deadline the original
// context is returned unchanged. Cancel must always be invoked.
func (rc *RestyClient) withRequestTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if rc.timeout <= 0 {
		return ctx, func() {}
	}
	if dl, ok := ctx.Deadline(); ok && time.Until(dl) < rc.timeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, rc.timeout)
}

// Get performs a GET request and unmarshals the response into target
func (rc *RestyClient) Get(ctx context.Context, path string, target any) error {
	ctx, cancel := rc.withRequestTimeout(ctx)
	defer cancel()

	resp, err := rc.client.R().
		SetContext(ctx).
		SetResult(target).
		Get(path)

	if err != nil {
		return translateTransportErr("GET", path, err)
	}

	if resp.IsError() {
		return translateStatusErr("GET", path, resp.StatusCode(), resp.String())
	}

	return nil
}

// Post performs a POST request with form data
func (rc *RestyClient) Post(ctx context.Context, path string, data url.Values, target any) error {
	ctx, cancel := rc.withRequestTimeout(ctx)
	defer cancel()

	resp, err := rc.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormDataFromValues(data).
		SetResult(target).
		Post(path)

	if err != nil {
		return translateTransportErr("POST", path, err)
	}

	if resp.IsError() {
		return translateStatusErr("POST", path, resp.StatusCode(), resp.String())
	}

	return nil
}

// PostEmpty performs a POST request with empty form data
// Used for Proxmox API endpoints that require POST but don't need parameters
// Sends empty url.Values to ensure proper Content-Type header
func (rc *RestyClient) PostEmpty(ctx context.Context, path string, target any) error {
	ctx, cancel := rc.withRequestTimeout(ctx)
	defer cancel()

	resp, err := rc.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetBody("").
		SetResult(target).
		Post(path)

	if err != nil {
		return translateTransportErr("POST", path, err)
	}

	if resp.IsError() {
		respBody := strings.TrimSpace(resp.String())
		if respBody == "" {
			return nil
		}
		return translateStatusErr("POST", path, resp.StatusCode(), resp.String())
	}

	return nil
}

// Put performs a PUT request with form data
func (rc *RestyClient) Put(ctx context.Context, path string, data url.Values, target any) error {
	ctx, cancel := rc.withRequestTimeout(ctx)
	defer cancel()

	resp, err := rc.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormDataFromValues(data).
		SetResult(target).
		Put(path)

	if err != nil {
		return translateTransportErr("PUT", path, err)
	}

	if resp.IsError() {
		return translateStatusErr("PUT", path, resp.StatusCode(), resp.String())
	}

	return nil
}

// Delete performs a DELETE request
func (rc *RestyClient) Delete(ctx context.Context, path string, target any) error {
	ctx, cancel := rc.withRequestTimeout(ctx)
	defer cancel()

	resp, err := rc.client.R().
		SetContext(ctx).
		SetResult(target).
		Delete(path)

	if err != nil {
		return translateTransportErr("DELETE", path, err)
	}

	if resp.IsError() {
		return translateStatusErr("DELETE", path, resp.StatusCode(), resp.String())
	}

	return nil
}

// GetTimeout returns the configured timeout
func (rc *RestyClient) GetTimeout() time.Duration {
	return rc.timeout
}

// GetBaseURL returns the base URL
func (rc *RestyClient) GetBaseURL() string {
	return rc.baseURL
}

// ResetTokenClients clears the cached token clients. Intended for tests so
// that subsequent MakeRestyClient calls rebuild the singleton.
func ResetTokenClients() {
	tokenClientMu.Lock()
	defer tokenClientMu.Unlock()
	tokenClients = make(map[string]*resty.Client)
}
