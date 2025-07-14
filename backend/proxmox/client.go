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

	"github.com/rs/zerolog/log"
	px "github.com/Telmate/proxmox-api-go/proxmox"
)

// Client is a wrapper around the Proxmox API client.
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

// ClientOption defines functional options for configuring the Client.
type ClientOption func(*Client)

// CacheEntry represents a cached API response
type CacheEntry struct {
	Data      []byte
	Timestamp time.Time
	TTL       time.Duration
}

// WithTimeout sets a custom timeout for API requests.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = timeout
	}
}

// WithCache enables response caching with a specific TTL.
func WithCache(ttl time.Duration) ClientOption {
	return func(c *Client) {
		if ttl > 0 {
			c.cache = make(map[string]*CacheEntry)
			c.cacheTTL = ttl
		}
	}
}

// NewClient creates a new Proxmox API client with default options.
func NewClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool) (*Client, error) {
	return NewClientWithOptions(apiURL, apiTokenID, apiTokenSecret, insecureSkipVerify)
}

// NewClientWithOptions creates a new Proxmox API client with the given options.
func NewClientWithOptions(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool, opts ...ClientOption) (*Client, error) {
	if apiURL == "" || apiTokenID == "" || apiTokenSecret == "" {
		log.Error().Str("apiURL", apiURL).Str("apiTokenID", apiTokenID).Msg("Missing required Proxmox API credentials")
		return nil, fmt.Errorf("apiURL, apiTokenID, and apiTokenSecret are required")
	}

	// Set up TLS configuration with connection pooling
	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: false,
		MaxConnsPerHost:    5,
	}
	httpClient := &http.Client{Transport: tr}

	// Create the underlying Proxmox client
	pxClient, err := px.NewClient(apiURL, httpClient, "", nil, "", 300)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox client")
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
		Timeout:    10 * time.Second, // Default timeout
		cache:      make(map[string]*CacheEntry), // Default cache
		cacheTTL:   2 * time.Minute, // Default TTL: 2 minutes
	}

	// Apply any provided options
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Get performs a direct GET request to the Proxmox API with context support.
func (c *Client) Get(path string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	log.Info().Str("path", path).Msg("Performing GET request to Proxmox API")
	return c.GetWithContext(ctx, path)
}

// GetWithContext performs a direct GET request to the Proxmox API with the provided context.
func (c *Client) GetWithContext(ctx context.Context, path string) (map[string]interface{}, error) {
	// Try to get from cache first if caching is enabled
	if c.cache != nil {
		c.mux.RLock()
		cacheEntry, found := c.cache[path]
		c.mux.RUnlock()
		
		if found && time.Since(cacheEntry.Timestamp) < c.cacheTTL {
			log.Debug().Str("path", path).Msg("Using cached response")
			var result map[string]interface{}
			if err := json.Unmarshal(cacheEntry.Data, &result); err == nil {
				return result, nil
			} else {
				log.Warn().Str("path", path).Err(err).Msg("Failed to unmarshal cached data")
				// If unmarshaling fails, proceed to make a fresh request
			}
		}
	}

	// Not in cache, make the request
	rawData, err := c.GetRawWithContext(ctx, path)
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to get raw API response")
		return nil, err
	}

	// Parse the response
	var result map[string]interface{}
	if err := json.Unmarshal(rawData, &result); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to decode API response")
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
		log.Debug().Str("path", path).Msg("Cached API response")
	}

	return result, nil
}

// GetRawWithContext performs a direct GET request to the Proxmox API and returns the raw response body.
func (c *Client) GetRawWithContext(ctx context.Context, path string) ([]byte, error) {
	// Try to get from cache first if caching is enabled
	if c.cache != nil {
		c.mux.RLock()
		cacheEntry, found := c.cache[path]
		c.mux.RUnlock()
		
		if found && time.Since(cacheEntry.Timestamp) < c.cacheTTL {
			log.Debug().Str("path", path).Msg("Using cached raw response")
			return cacheEntry.Data, nil
		}
	}

	url := c.ApiUrl + path
	log.Info().Str("url", url).Msg("Making API request to Proxmox")
	
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Failed to create HTTP request for Proxmox API")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authorization header
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))

	// Execute the request
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("HTTP request to Proxmox API failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error().Int("status", resp.StatusCode).Str("url", url).Msg("Proxmox API returned non-200 status")
		return nil, fmt.Errorf("API returned non-200 status: %d - %s", resp.StatusCode, string(body))
	}

	// Read the response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Failed to read response body from Proxmox API")
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
		log.Debug().Str("path", path).Msg("Cached raw API response")
	}

	return data, nil
}

// InvalidateCache invalidates all cached responses or a specific path
func (c *Client) InvalidateCache(path string) {
	if c.cache == nil {
		return
	}

	c.mux.Lock()
	defer c.mux.Unlock()
	
	if path == "" {
		// Invalidate all cache
		c.cache = make(map[string]*CacheEntry)
		log.Debug().Msg("Invalidated entire API cache")
	} else {
		// Invalidate specific path
		delete(c.cache, path)
		log.Debug().Str("path", path).Msg("Invalidated specific API cache entry")
	}
}
