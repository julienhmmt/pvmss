package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	px "github.com/Telmate/proxmox-api-go/proxmox"
	"pvmss/logger"
)

const (
	defaultTimeout    = 10 * time.Second
	defaultCacheTTL   = 2 * time.Minute
	defaultLoginRealm = "pam"
)

// Client is a custom wrapper around the Proxmox API client, enhancing it with timeouts, caching, and simplified authentication.
type Client struct {
	*px.Client
	HttpClient *http.Client
	ApiUrl     string
	AuthToken  string
	Timeout    time.Duration
	cache      map[string]*CacheEntry
	cacheTTL   time.Duration
	mux        sync.RWMutex

	PVEAuthCookie       string
	CSRFPreventionToken string
}

// ClientOption defines a function for applying configuration options to the Client.
type ClientOption func(*Client)

// CacheEntry represents a cached API response.
type CacheEntry struct {
	Data      []byte
	Timestamp time.Time
}

// NewClient creates a new Proxmox API client using an API token.
func NewClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool, opts ...ClientOption) (*Client, error) {
	if apiURL == "" || apiTokenID == "" || apiTokenSecret == "" {
		return nil, fmt.Errorf("apiURL, apiTokenID, and apiTokenSecret are required")
	}

	client, err := newBaseClient(apiURL, insecureSkipVerify, opts...)
	if err != nil {
		return nil, err
	}

	client.Client.SetAPIToken(apiTokenID, apiTokenSecret)
	client.AuthToken = fmt.Sprintf("%s=%s", apiTokenID, apiTokenSecret)

	return client, nil
}

// NewClientCookieAuth constructs a client for cookie-based authentication.
func NewClientCookieAuth(apiURL string, insecureSkipVerify bool, opts ...ClientOption) (*Client, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("apiURL is required")
	}
	return newBaseClient(apiURL, insecureSkipVerify, opts...)
}

// newBaseClient is an internal constructor that sets up a client with a shared HTTP transport.
func newBaseClient(apiURL string, insecureSkipVerify bool, opts ...ClientOption) (*Client, error) {
	normalizedURL, err := normalizeBaseURL(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Proxmox API URL: %w", err)
	}

	httpClient := newHTTPClient(insecureSkipVerify, defaultTimeout)

	pxClient, err := px.NewClient(normalizedURL, httpClient, "", nil, "", 300)
	if err != nil {
		return nil, fmt.Errorf("failed to create base Proxmox client: %w", err)
	}

	client := &Client{
		Client:     pxClient,
		HttpClient: httpClient,
		ApiUrl:     normalizedURL,
		Timeout:    defaultTimeout,
		cache:      make(map[string]*CacheEntry),
		cacheTTL:   defaultCacheTTL,
	}

	for _, opt := range opts {
		opt(client)
	}

	// Update the HTTP client's timeout if a custom one was provided via options.
	if client.Timeout != defaultTimeout {
		client.HttpClient.Timeout = client.Timeout
	}

	return client, nil
}

// Login authenticates using a username and password to obtain a session cookie and CSRF token.
func (c *Client) Login(ctx context.Context, username, password, realm string) error {
	if c == nil {
		return fmt.Errorf("client is nil")
	}
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	if !strings.Contains(username, "@") {
		if realm == "" {
			realm = defaultLoginRealm
		}
		username = fmt.Sprintf("%s@%s", username, realm)
	}

	params := url.Values{}
	params.Set("username", username)
	params.Set("password", password)

	var respData struct {
		Data struct {
			Ticket              string `json:"ticket"`
			CSRFPreventionToken string `json:"CSRFPreventionToken"`
		}
	}

	err := c.doJSONRequest(ctx, http.MethodPost, "/access/ticket", params, &respData)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	if respData.Data.Ticket == "" {
		return fmt.Errorf("login response missing ticket")
	}

	c.PVEAuthCookie = respData.Data.Ticket
	c.CSRFPreventionToken = respData.Data.CSRFPreventionToken
	return nil
}

// Get performs a GET request, using a default context with the client's timeout.
func (c *Client) Get(path string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	return c.GetWithContext(ctx, path)
}

// GetWithContext performs a GET request and unmarshals the response into a map.
func (c *Client) GetWithContext(ctx context.Context, path string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.GetJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetJSON performs a GET request and unmarshals the response into a target interface.
func (c *Client) GetJSON(ctx context.Context, path string, target interface{}) error {
	rawData, err := c.GetRawWithContext(ctx, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(rawData, target); err != nil {
		return fmt.Errorf("failed to decode API response for %s: %w", path, err)
	}
	return nil
}

// GetRawWithContext performs a GET request, handling caching and returning the raw response body.
func (c *Client) GetRawWithContext(ctx context.Context, path string) ([]byte, error) {
	if c.cache != nil {
		c.mux.RLock()
		if entry, ok := c.cache[path]; ok && time.Since(entry.Timestamp) < c.cacheTTL {
			c.mux.RUnlock()
			logger.Get().Debug().Str("path", path).Msg("Using cached response")
			return entry.Data, nil
		}
		c.mux.RUnlock()
	}

	logger.Get().Debug().Str("path", path).Msg("Fetching from Proxmox API")
	var m map[string]any
	if err := c.Client.GetJsonRetryable(ctx, path, &m, 3); err != nil {
		return nil, fmt.Errorf("proxmox GET request failed for %s: %w", path, err)
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Telmate response: %w", err)
	}

	if c.cache != nil {
		c.mux.Lock()
		c.cache[path] = &CacheEntry{Data: b, Timestamp: time.Now()}
		c.mux.Unlock()
	}

	return b, nil
}

// PostFormWithContext performs a POST request with form-encoded data.
func (c *Client) PostFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error) {
	return c.doFormRequest(ctx, http.MethodPost, path, data)
}

// PutFormWithContext performs a PUT request with form-encoded data.
func (c *Client) PutFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error) {
	return c.doFormRequest(ctx, http.MethodPut, path, data)
}

// DeleteWithContext performs a DELETE request with optional form-encoded data.
func (c *Client) DeleteWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error) {
	return c.doFormRequest(ctx, http.MethodDelete, path, data)
}

// PostFormAndGetJSON sends a POST request with form data and unmarshals the JSON response into the provided struct.
func (c *Client) PostFormAndGetJSON(ctx context.Context, path string, data url.Values, v interface{}) error {
	return c.doJSONRequest(ctx, http.MethodPost, path, data, v)
}

// doFormRequest is a generic helper for form-based POST, PUT, and DELETE requests.
func (c *Client) doFormRequest(ctx context.Context, method, path string, data url.Values) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doJSONRequest(ctx, method, path, data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// doJSONRequest handles the logic for making a request and decoding the JSON response.
func (c *Client) doJSONRequest(ctx context.Context, method, path string, data url.Values, target interface{}) error {
	fullURL := c.ApiUrl + path
	var body io.Reader
	if data != nil {
		body = strings.NewReader(data.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %s: %s", resp.Status, string(respBody))
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// GetVNCProxy requests a VNC ticket for a specific VM.
func (c *Client) GetVNCProxy(ctx context.Context, node string, vmID int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/vncproxy", node, vmID)
	params := url.Values{}
	params.Set("websocket", "1")

	var result map[string]interface{}
	if err := c.doJSONRequest(ctx, http.MethodPost, path, params, &result); err != nil {
		return nil, fmt.Errorf("vncproxy request failed: %w", err)
	}
	return result, nil
}

// InvalidateCache removes a specific entry from the client's cache.
func (c *Client) InvalidateCache(path string) {
	if c.cache != nil {
		c.mux.Lock()
		delete(c.cache, path)
		c.mux.Unlock()
		logger.Get().Debug().Str("path", path).Msg("Cache entry invalidated")
	}
}

// --- Getters ---

func (c *Client) GetApiUrl() string              { return c.ApiUrl }
func (c *Client) GetTimeout() time.Duration      { return c.Timeout }
func (c *Client) GetPVEAuthCookie() string       { return c.PVEAuthCookie }
func (c *Client) GetCSRFPreventionToken() string { return c.CSRFPreventionToken }
func (c *Client) SetTimeout(timeout time.Duration) {
	if c.HttpClient != nil {
		c.HttpClient.Timeout = timeout
	}
	c.Timeout = timeout
}

// --- Options ---

// WithTimeout returns a ClientOption to set a custom request timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		if timeout > 0 {
			c.Timeout = timeout
		}
	}
}

// WithCache returns a ClientOption to set a custom cache TTL.
func WithCache(ttl time.Duration) ClientOption {
	return func(c *Client) {
		if ttl > 0 {
			c.cacheTTL = ttl
		}
	}
}

// --- Helpers ---

// setAuthHeaders adds the appropriate authentication headers to a request.
func (c *Client) setAuthHeaders(req *http.Request) {
	if c.PVEAuthCookie != "" {
		req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", c.PVEAuthCookie))
		if c.CSRFPreventionToken != "" {
			req.Header.Set("CSRFPreventionToken", c.CSRFPreventionToken)
		}
	} else if c.AuthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))
	}
}

// newHTTPClient creates a new http.Client with optimized transport settings.
func newHTTPClient(insecureSkipVerify bool, timeout time.Duration) *http.Client {
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

// normalizeBaseURL ensures the Proxmox API URL is correctly formatted.
func normalizeBaseURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}

	if u.Path == "" || u.Path == "/" {
		u.Path = "/api2/json"
	} else if !strings.HasSuffix(u.Path, "/api2/json") {
		u.Path = strings.TrimSuffix(u.Path, "/") + "/api2/json"
	}

	return u.String(), nil
}
