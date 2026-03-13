package proxmox

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"pvmss/logger"
)

// RestyClient is a resty-based Proxmox API client for modern API interactions.
type RestyClient struct {
	client  *resty.Client
	baseURL string
	timeout time.Duration
}

// MakeRestyClient creates a new resty-based Proxmox API client
func MakeRestyClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool, timeout time.Duration) (*RestyClient, error) {
	if apiURL == "" || apiTokenID == "" || apiTokenSecret == "" {
		return nil, fmt.Errorf("apiURL, apiTokenID, and apiTokenSecret are required")
	}

	// Normalize base URL
	normalizedURL, err := normalizeBaseURL(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Proxmox API URL: %w", err)
	}

	// Create resty client
	client := resty.New()

	// Set base URL
	client.SetBaseURL(normalizedURL)

	// Set timeout
	client.SetTimeout(timeout)

	// Configure TLS
	if insecureSkipVerify {
		// Used only for development/testing with self-signed certificates
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}) // #nosec G402 - Controlled by PROXMOX_VERIFY_SSL; allowed for dev/test
	}

	// Set authentication header for API token
	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", apiTokenID, apiTokenSecret)
	client.SetHeader("Authorization", authHeader)

	// Set common headers
	client.SetHeader("Accept", "application/json")
	client.SetHeader("Content-Type", "application/json")

	// Enable retry
	client.SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	// Log requests in debug mode
	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		logger.Get().Debug().
			Str("method", req.Method).
			Str("url", req.URL).
			Msg("Resty API request")
		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		logger.Get().Debug().
			Str("method", resp.Request.Method).
			Str("url", resp.Request.URL).
			Int("status", resp.StatusCode()).
			Dur("duration", resp.Time()).
			Msg("Resty API response")
		return nil
	})

	return &RestyClient{
		client:  client,
		baseURL: normalizedURL,
		timeout: timeout,
	}, nil
}

// MakeRestyClientCookieAuth creates a new resty-based Proxmox API client without API token auth.
// This is used for operations that require cookie-based authentication (PVEAuthCookie + CSRFPreventionToken),
// such as ticket creation, password updates, and VNC proxy.
func MakeRestyClientCookieAuth(apiURL string, insecureSkipVerify bool, timeout time.Duration) (*RestyClient, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("apiURL is required")
	}

	normalizedURL, err := normalizeBaseURL(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Proxmox API URL: %w", err)
	}

	client := resty.New()
	client.SetBaseURL(normalizedURL)
	client.SetTimeout(timeout)

	if insecureSkipVerify {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}) // #nosec G402 - Controlled by PROXMOX_VERIFY_SSL; allowed for dev/test
	}

	client.SetHeader("Accept", "application/json")
	client.SetHeader("Content-Type", "application/json")

	client.SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		logger.Get().Debug().
			Str("method", req.Method).
			Str("url", req.URL).
			Msg("Resty API request (cookie auth)")
		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
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

// Get performs a GET request and unmarshals the response into target
func (rc *RestyClient) Get(ctx context.Context, path string, target interface{}) error {
	resp, err := rc.client.R().
		SetContext(ctx).
		SetResult(target).
		Get(path)

	if err != nil {
		return fmt.Errorf("GET request failed for %s: %w", path, err)
	}

	if resp.IsError() {
		return fmt.Errorf("GET request returned error status %d for %s: %s", resp.StatusCode(), path, resp.String())
	}

	return nil
}

// Post performs a POST request with form data
func (rc *RestyClient) Post(ctx context.Context, path string, data url.Values, target interface{}) error {
	resp, err := rc.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormDataFromValues(data).
		SetResult(target).
		Post(path)

	if err != nil {
		return fmt.Errorf("POST request failed for %s: %w", path, err)
	}

	if resp.IsError() {
		return fmt.Errorf("POST request returned error status %d for %s: %s", resp.StatusCode(), path, resp.String())
	}

	return nil
}

// PostEmpty performs a POST request with empty form data
// Used for Proxmox API endpoints that require POST but don't need parameters
// Sends empty url.Values to ensure proper Content-Type header
func (rc *RestyClient) PostEmpty(ctx context.Context, path string, target interface{}) error {
	resp, err := rc.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetBody("").
		SetResult(target).
		Post(path)

	if err != nil {
		return fmt.Errorf("POST request failed for %s: %w", path, err)
	}

	if resp.IsError() {
		// Check if response body is empty or just whitespace
		respBody := strings.TrimSpace(resp.String())
		if respBody == "" {
			// Empty response is acceptable for some Proxmox endpoints
			return nil
		}
		return fmt.Errorf("POST request returned error status %d for %s: %s", resp.StatusCode(), path, resp.String())
	}

	return nil
}

// Put performs a PUT request with form data
func (rc *RestyClient) Put(ctx context.Context, path string, data url.Values, target interface{}) error {
	resp, err := rc.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormDataFromValues(data).
		SetResult(target).
		Put(path)

	if err != nil {
		return fmt.Errorf("PUT request failed for %s: %w", path, err)
	}

	if resp.IsError() {
		return fmt.Errorf("PUT request returned error status %d for %s: %s", resp.StatusCode(), path, resp.String())
	}

	return nil
}

// Delete performs a DELETE request
func (rc *RestyClient) Delete(ctx context.Context, path string, target interface{}) error {
	resp, err := rc.client.R().
		SetContext(ctx).
		SetResult(target).
		Delete(path)

	if err != nil {
		return fmt.Errorf("DELETE request failed for %s: %w", path, err)
	}

	if resp.IsError() {
		return fmt.Errorf("DELETE request returned error status %d for %s: %s", resp.StatusCode(), path, resp.String())
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
