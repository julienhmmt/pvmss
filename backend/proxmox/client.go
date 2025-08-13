package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	px "github.com/Telmate/proxmox-api-go/proxmox"
	"pvmss/logger"
	"pvmss/metrics"
)

// Client is a custom wrapper around the standard Proxmox API client.
// It enhances the base client with features like request timeouts, response caching,
// and a simplified authentication mechanism using API tokens.
// It implements the ClientInterface for better testability and abstraction.
type Client struct {
	*px.Client
	HttpClient *http.Client
	ApiUrl     string
	AuthToken  string
	Timeout    time.Duration
	cache      map[string]*CacheEntry
	cacheTTL   time.Duration
	mux        sync.RWMutex
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

	// Build URL
	fullURL := c.ApiUrl + path
	logger.Get().Debug().Str("url", fullURL).Msg("Making GET request to Proxmox")

	// Retry policy
	const maxAttempts = 3
	var resp *http.Response
	var body []byte
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		start := time.Now()

		// Create request with context for each attempt
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			metrics.ObserveProxmox("GET", path, 0, "request_build_error", start)
			logger.Get().Error().Err(err).Str("url", fullURL).Msg("Failed to create request for Proxmox API")
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		// Headers
		req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "pvmss-proxmox-client/1.0")

		resp, err = c.HttpClient.Do(req)
		if err != nil {
			metrics.ObserveProxmox("GET", path, 0, "network_error", start)
			if attempt < maxAttempts {
				backoff(attempt)
				continue
			}
			logger.Get().Error().Err(err).Str("url", fullURL).Msg("HTTP request to Proxmox API failed")
			return nil, fmt.Errorf("request failed: %w", err)
		}

		// Always close on exit of loop body
		defer resp.Body.Close()

		// Read the response body for status handling and caching
		var readErr error
		body, readErr = io.ReadAll(resp.Body)
		if readErr != nil {
			metrics.ObserveProxmox("GET", path, resp.StatusCode, "read_error", start)
			if attempt < maxAttempts {
				backoff(attempt)
				continue
			}
			logger.Get().Error().Err(readErr).Str("url", fullURL).Msg("Failed to read response body from Proxmox API")
			return nil, fmt.Errorf("failed to read response body: %w", readErr)
		}

		// Non-2xx: decide retry for 5xx/429
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Log truncated body for context without relying on SDK types
			const maxLog = 512
			logBody := body
			if len(logBody) > maxLog {
				logBody = logBody[:maxLog]
			}
			logger.Get().Error().Int("status", resp.StatusCode).Str("url", fullURL).RawJSON("body", logBody).Msg("Proxmox API non-2xx")
			outcome := "error"
			if shouldRetryStatus(resp.StatusCode) && attempt < maxAttempts {
				outcome = "retry"
				metrics.ObserveProxmox("GET", path, resp.StatusCode, outcome, start)
				backoff(attempt)
				continue
			}
			metrics.ObserveProxmox("GET", path, resp.StatusCode, outcome, start)
			return nil, fmt.Errorf("API returned non-200 status: %d - %s", resp.StatusCode, string(body))
		}

		// Success
		metrics.ObserveProxmox("GET", path, resp.StatusCode, "success", start)
		break
	}

	// body holds successful response at this point
	data := body

	// Cache the response if caching is enabled
	if c.cache != nil {
		c.mux.Lock()
		c.cache[path] = &CacheEntry{
			Data:      data,
			Timestamp: time.Now(),
		}
		c.mux.Unlock()
		logger.Get().Debug().Str("path", path).Msg("Cached raw API response")
	}

	return data, nil
}

// PostFormWithContext performs a POST request with form-encoded data to the Proxmox API.
// It is primarily used for VM actions such as start/stop/reset/reboot/shutdown which
// require POSTing to status endpoints.
func (c *Client) PostFormWithContext(ctx context.Context, path string, form map[string]string) ([]byte, error) {
	urlStr := c.ApiUrl + path
	logger.Get().Info().Str("url", urlStr).Msg("Making POST request to Proxmox")

	vals := url.Values{}
	for k, v := range form {
		vals.Set(k, v)
	}

	const maxAttempts = 3
	var resp *http.Response
	var err error
	var body []byte
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		start := time.Now()

		req, buildErr := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, strings.NewReader(vals.Encode()))
		if buildErr != nil {
			metrics.ObserveProxmox("POST", path, 0, "request_build_error", start)
			return nil, fmt.Errorf("failed to create POST request: %w", buildErr)
		}
		req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", "pvmss-proxmox-client/1.0")

		resp, err = c.HttpClient.Do(req)
		if err != nil {
			metrics.ObserveProxmox("POST", path, 0, "network_error", start)
			if attempt < maxAttempts {
				backoff(attempt)
				continue
			}
			return nil, fmt.Errorf("POST request failed: %w", err)
		}
		defer resp.Body.Close()

		body, _ = io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			outcome := "error"
			if shouldRetryStatus(resp.StatusCode) && attempt < maxAttempts {
				outcome = "retry"
				metrics.ObserveProxmox("POST", path, resp.StatusCode, outcome, start)
				backoff(attempt)
				continue
			}
			metrics.ObserveProxmox("POST", path, resp.StatusCode, outcome, start)
			return nil, fmt.Errorf("API returned non-success status: %d - %s", resp.StatusCode, string(body))
		}
		metrics.ObserveProxmox("POST", path, resp.StatusCode, "success", start)
		break
	}

	c.InvalidateCache("")
	return body, nil
}

// shouldRetryStatus returns true for transient statuses
func shouldRetryStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// backoff sleeps with exponential backoff and jitter
func backoff(attempt int) {
	base := 200 * time.Millisecond
	max := 2 * time.Second
	d := time.Duration(1<<uint(attempt-1)) * base
	if d > max {
		d = max
	}
	// jitter +/- 20%
	jitter := 0.2 - rand.Float64()*0.4
	sleep := time.Duration(float64(d) * (1 + jitter))
	time.Sleep(sleep)
}

// InvalidateCache removes entries from the client's response cache.
// If a specific path is provided, only that entry is deleted.
// If the path is empty, the entire cache is cleared.
func (c *Client) InvalidateCache(path string) {
	if c.cache == nil {
		return
	}

	c.mux.Lock()
	defer c.mux.Unlock()

	if path == "" {
		// Invalidate all cache
		c.cache = make(map[string]*CacheEntry)
		logger.Get().Debug().Msg("Cache cleared")
	} else {
		// Invalidate specific path
		delete(c.cache, path)
		logger.Get().Debug().Str("path", path).Msg("Cache entry invalidated")
	}
}

// GetTimeout returns the client's configured timeout duration
func (c *Client) GetTimeout() time.Duration {
	return c.Timeout
}

// SetTimeout sets the client's request timeout
func (c *Client) SetTimeout(timeout time.Duration) {
	c.Timeout = timeout
	if c.HttpClient != nil {
		c.HttpClient.Timeout = timeout
	}
}

// GetJSON performs a GET request and unmarshals the response into the target interface
func (c *Client) GetJSON(ctx context.Context, path string, target interface{}) error {
	// Get raw response
	data, err := c.GetRawWithContext(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to get data: %w", err)
	}

	// Unmarshal into target
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}
