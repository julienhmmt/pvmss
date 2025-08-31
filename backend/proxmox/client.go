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

// Client is a custom wrapper around the standard Proxmox API client.
// It enhances the base client with features like request timeouts, response caching,
// and a simplified authentication mechanism using API tokens.
type Client struct {
	*px.Client
	HttpClient *http.Client
	ApiUrl     string
	AuthToken  string
	Timeout    time.Duration
	cache      map[string]*CacheEntry
	cacheTTL   time.Duration
	mux        sync.RWMutex

	// Optional cookie-based auth for console/noVNC flows
	PVEAuthCookie       string
	CSRFPreventionToken string
}

// NewClientCookieAuth constructs a Proxmox API client without requiring an API token.
// Use Login() to authenticate with username/password and obtain a PVEAuthCookie.
func NewClientCookieAuth(apiURL string, insecureSkipVerify bool, opts ...ClientOption) (*Client, error) {
	if apiURL == "" {
		logger.Get().Error().Msg("Missing required Proxmox API URL")
		return nil, fmt.Errorf("apiURL is required")
	}

	// Normalize and validate base API URL
	normalizedURL, err := normalizeBaseURL(apiURL)
	if err != nil {
		logger.Get().Error().Err(err).Str("apiURL", apiURL).Msg("Invalid PROXMOX_URL; must include host and will be normalized to end with /api2/json")
		return nil, err
	}
	apiURL = normalizedURL

	// Set up TLS configuration with connection pooling
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       60 * time.Second,
		DisableCompression:    false,
		MaxConnsPerHost:       32,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	httpClient := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	// Create the underlying Proxmox client
	pxClient, err := px.NewClient(apiURL, httpClient, "", nil, "", 300)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to create Proxmox client (cookie auth)")
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	client := &Client{
		Client:     pxClient,
		HttpClient: httpClient,
		ApiUrl:     apiURL,
		Timeout:    10 * time.Second,
		cache:      make(map[string]*CacheEntry),
		cacheTTL:   2 * time.Minute,
	}

	// Apply any provided options
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Login authenticates using username/password and stores the PVEAuthCookie and CSRFPreventionToken.
// If realm is empty and not already present in the username, defaults to "pam".
func (c *Client) Login(ctx context.Context, username, password, realm string) error {
	if c == nil {
		return fmt.Errorf("nil client")
	}
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required for login")
	}
	if !strings.Contains(username, "@") {
		if realm == "" {
			realm = "pam"
		}
		username = fmt.Sprintf("%s@%s", username, realm)
	}

	endpoint := c.ApiUrl + "/access/ticket"
	params := url.Values{}
	params.Set("username", username)
	params.Set("password", password)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status %s: %s", resp.Status, string(b))
	}

	var decoded map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}
	data, ok := decoded["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected login response format: missing data")
	}
	ticket, _ := data["ticket"].(string)
	csrf, _ := data["CSRFPreventionToken"].(string)
	if ticket == "" {
		return fmt.Errorf("login response missing ticket")
	}

	c.PVEAuthCookie = ticket
	c.CSRFPreventionToken = csrf
	return nil
}

// ClientOption defines a function signature for applying configuration options to the Client.
// This pattern allows for flexible and clear client customization (e.g., setting a timeout or cache TTL).
type ClientOption func(*Client)

// CacheEntry represents a cached API response.
// It stores the raw response data, the timestamp of when it was cached, and its Time-To-Live (TTL).
type CacheEntry struct {
	Data      []byte
	Timestamp time.Time
	TTL       time.Duration
}

// WithTimeout returns a ClientOption that sets a custom timeout for all API requests made by the client.
// This overrides the default timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = timeout
	}
}

// WithCache returns a ClientOption that enables response caching.
// It initializes the cache map and sets a custom Time-To-Live (TTL) for all cached items.
func WithCache(ttl time.Duration) ClientOption {
	return func(c *Client) {
		if ttl > 0 {
			c.cache = make(map[string]*CacheEntry)
			c.cacheTTL = ttl
		}
	}
}

// NewClient creates a new Proxmox API client with default settings for timeout and caching.
// It serves as a simplified constructor, calling NewClientWithOptions with default values.
func NewClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool) (*Client, error) {
	return NewClientWithOptions(apiURL, apiTokenID, apiTokenSecret, insecureSkipVerify)
}

// NewClientWithOptions constructs a new Proxmox API client, configures the underlying HTTP transport
// with connection pooling and TLS settings, and applies any custom functional options provided.
// It is the main constructor for creating a customized client instance.
func NewClientWithOptions(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool, opts ...ClientOption) (*Client, error) {
	if apiURL == "" || apiTokenID == "" || apiTokenSecret == "" {
		logger.Get().Error().Str("apiURL", apiURL).Str("apiTokenID", apiTokenID).Msg("Missing required Proxmox API credentials")
		return nil, fmt.Errorf("apiURL, apiTokenID, and apiTokenSecret are required")
	}

	// Normalize and validate base API URL
	normalizedURL, err := normalizeBaseURL(apiURL)
	if err != nil {
		logger.Get().Error().Err(err).Str("apiURL", apiURL).Msg("Invalid PROXMOX_URL; must include host and will be normalized to end with /api2/json")
		return nil, err
	}
	apiURL = normalizedURL

	// Set up TLS configuration with connection pooling
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       60 * time.Second,
		DisableCompression:    false,
		MaxConnsPerHost:       32,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	httpClient := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	// Create the underlying Proxmox client
	pxClient, err := px.NewClient(apiURL, httpClient, "", nil, "", 300)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to create Proxmox client")
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	// Set API token for authentication
	pxClient.SetAPIToken(apiTokenID, apiTokenSecret)
	authToken := fmt.Sprintf("%s=%s", apiTokenID, apiTokenSecret)

	// Create our client wrapper with default cache
	client := &Client{
		Client:     pxClient,
		HttpClient: httpClient,
		ApiUrl:     apiURL,
		AuthToken:  authToken,
		Timeout:    10 * time.Second,             // Default timeout
		cache:      make(map[string]*CacheEntry), // Default cache
		cacheTTL:   2 * time.Minute,              // Default TTL: 2 minutes
	}

	// Apply any provided options
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Get performs a GET request to the Proxmox API using the client's default timeout.
// It is a convenience wrapper around GetWithContext.
func (c *Client) Get(path string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	logger.Get().Info().Str("path", path).Msg("Performing GET request to Proxmox API")
	return c.GetWithContext(ctx, path)
}

// GetWithContext performs a GET request to the Proxmox API, handling the entire request-response cycle.
// It checks the cache for a valid response before making a live request.
// If the response is not cached, it calls GetRawWithContext, unmarshals the JSON response, and caches it.
func (c *Client) GetWithContext(ctx context.Context, path string) (map[string]interface{}, error) {
	// Try to get from cache first if caching is enabled
	if c.cache != nil {
		c.mux.RLock()
		cacheEntry, found := c.cache[path]
		c.mux.RUnlock()

		if found && time.Since(cacheEntry.Timestamp) < c.cacheTTL {
			logger.Get().Debug().Str("path", path).Msg("Using cached response")
			var result map[string]interface{}
			if err := json.Unmarshal(cacheEntry.Data, &result); err == nil {
				return result, nil
			} else {
				logger.Get().Warn().Str("path", path).Err(err).Msg("Failed to unmarshal cached data")
			}
		}
	}

	// Not in cache, make the request
	rawData, err := c.GetRawWithContext(ctx, path)
	if err != nil {
		logger.Get().Error().Err(err).Str("path", path).Msg("Error in Proxmox API request")
		return nil, err
	}

	// Parse the response
	var result map[string]interface{}
	if err := json.Unmarshal(rawData, &result); err != nil {
		logger.Get().Error().Err(err).Str("path", path).Msg("Failed to decode API response")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Cache the raw response if caching is enabled
	if c.cache != nil {
		c.mux.Lock()
		c.cache[path] = &CacheEntry{
			Data:      rawData,
			Timestamp: time.Now(),
		}
		c.mux.Unlock()
		logger.Get().Debug().Str("path", path).Msg("Fetching from Proxmox API")
	}

	return result, nil
}

// GetRawWithContext is the core method for making GET requests.
// It checks the cache first, and if a valid entry is not found, it constructs and executes an HTTP request
// with the appropriate context and authorization headers, returning the raw response body.
func (c *Client) GetRawWithContext(ctx context.Context, path string) ([]byte, error) {
	// Check cache first
	if c.cache != nil {
		c.mux.RLock()
		if entry, ok := c.cache[path]; ok {
			// If TTL is set and entry is expired, delete under write lock
			if c.cacheTTL > 0 && time.Since(entry.Timestamp) > c.cacheTTL {
				c.mux.RUnlock()
				c.mux.Lock()
				delete(c.cache, path)
				c.mux.Unlock()
				logger.Get().Debug().Str("path", path).Msg("Cache entry expired")
			} else {
				c.mux.RUnlock()
				return entry.Data, nil
			}
		} else {
			c.mux.RUnlock()
		}
	}

	// Delegate GET to Telmate client with retries
	var m map[string]any
	if err := c.Client.GetJsonRetryable(ctx, path, &m, 3); err != nil {
		logger.Get().Error().Err(err).Str("path", path).Msg("Proxmox GET failed via Telmate client")
		return nil, err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Telmate response: %w", err)
	}

	// Cache the response if caching is enabled
	if c.cache != nil {
		c.mux.Lock()
		c.cache[path] = &CacheEntry{
			Data:      b,
			Timestamp: time.Now(),
		}
		c.mux.Unlock()
		logger.Get().Debug().Str("path", path).Msg("Cached raw API response")
	}

	return b, nil
}

// PostFormWithContext performs a POST request with form-encoded data to the Proxmox API.
// It is primarily used for VM actions such as start/stop/reset/reboot/shutdown which
// require POSTing to status endpoints.
func (c *Client) PostFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.ApiUrl+path, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// PutFormWithContext performs a PUT request with form-encoded data to the Proxmox API.
// Some endpoints such as ACL management require PUT instead of POST.
func (c *Client) PutFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "PUT", c.ApiUrl+path, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(b))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetVNCProxy requests a VNC ticket for a specific VM.
// It makes a POST request to the vncproxy endpoint and returns the ticket details.
func (c *Client) GetVNCProxy(ctx context.Context, node string, vmID int) (map[string]interface{}, error) {
	fullURL := fmt.Sprintf("%s/nodes/%s/qemu/%d/vncproxy", c.ApiUrl, node, vmID)
	params := url.Values{}
	params.Set("websocket", "1")

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create vncproxy request: %w", err)
	}

	// Set required headers for Proxmox API
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	// Prefer cookie-based auth when available (needed for noVNC websocket flow)
	if c.PVEAuthCookie != "" {
		req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", c.PVEAuthCookie))
		if c.CSRFPreventionToken != "" {
			req.Header.Set("CSRFPreventionToken", c.CSRFPreventionToken)
		}
	} else if c.AuthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		logger.Get().Error().Err(err).Str("path", fullURL).Msg("Proxmox vncproxy POST failed")
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vncproxy request failed with status: %s, body: %s", resp.Status, string(b))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read vncproxy response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		logger.Get().Error().Err(err).Str("path", fullURL).Msg("Failed to unmarshal vncproxy response")
		return nil, err
	}

	return result, nil
}

// GetApiUrl returns the base URL of the Proxmox API.
func (c *Client) GetApiUrl() string {
	return c.ApiUrl
}

// SetTimeout sets the timeout for the HTTP client.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.HttpClient.Timeout = timeout
}

// GetTimeout returns the client's configured timeout.
func (c *Client) GetTimeout() time.Duration {
	return c.Timeout
}

// GetJSON fetches data from the Proxmox API and unmarshals it into the target interface.
// It leverages GetRawWithContext to handle caching and data retrieval.
func (c *Client) GetJSON(ctx context.Context, path string, target interface{}) error {
	rawData, err := c.GetRawWithContext(ctx, path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(rawData, target); err != nil {
		logger.Get().Error().Err(err).Str("path", path).Msg("Failed to decode API response for GetJSON")
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
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

// normalizeBaseURL ensures the Proxmox API URL is correctly formatted.
func normalizeBaseURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
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
