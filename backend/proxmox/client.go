package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: false,
		MaxConnsPerHost:    5,
	}
	httpClient := &http.Client{Transport: tr}

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
	// Try to get from cache first if caching is enabled
	if c.cache != nil {
		c.mux.RLock()
		cacheEntry, found := c.cache[path]
		c.mux.RUnlock()

		if found && time.Since(cacheEntry.Timestamp) < c.cacheTTL {
			logger.Get().Debug().Str("path", path).Msg("Using cached raw response")
			return cacheEntry.Data, nil
		}
	}

	url := c.ApiUrl + path
	logger.Get().Info().Str("url", url).Msg("Making API request to Proxmox")

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Get().Error().Err(err).Str("url", url).Msg("Failed to create HTTP request for Proxmox API")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authorization header
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))

	// Execute the request
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		logger.Get().Error().Err(err).Str("url", url).Msg("HTTP request to Proxmox API failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Get().Error().Int("status", resp.StatusCode).Str("url", url).Msg("Proxmox API returned non-200 status")
		return nil, fmt.Errorf("API returned non-200 status: %d - %s", resp.StatusCode, string(body))
	}

	// Read the response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Get().Error().Err(err).Str("url", url).Msg("Failed to read response body from Proxmox API")
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

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
