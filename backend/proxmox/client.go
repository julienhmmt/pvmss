package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
}

// ClientOption defines functional options for configuring the Client.
type ClientOption func(*Client)

// WithTimeout sets a custom timeout for API requests.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = timeout
	}
}

// NewClient creates a new Proxmox API client with default options.
func NewClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool) (*Client, error) {
	return NewClientWithOptions(apiURL, apiTokenID, apiTokenSecret, insecureSkipVerify)
}

// NewClientWithOptions creates a new Proxmox API client with the given options.
func NewClientWithOptions(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool, opts ...ClientOption) (*Client, error) {
	if apiURL == "" || apiTokenID == "" || apiTokenSecret == "" {
		return nil, fmt.Errorf("apiURL, apiTokenID, and apiTokenSecret are required")
	}

	// Set up TLS configuration
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
	}
	httpClient := &http.Client{Transport: tr}

	// Create the underlying Proxmox client
	pxClient, err := px.NewClient(apiURL, httpClient, "", nil, "", 300)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	// Set API token for authentication
	pxClient.SetAPIToken(apiTokenID, apiTokenSecret)
	authToken := fmt.Sprintf("%s=%s", apiTokenID, apiTokenSecret)

	// Create our client wrapper
	client := &Client{
		Client:     pxClient,
		HttpClient: httpClient,
		ApiUrl:     apiURL,
		AuthToken:  authToken,
		Timeout:    10 * time.Second, // Default timeout
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

	return c.GetWithContext(ctx, path)
}

// GetWithContext performs a direct GET request to the Proxmox API with the provided context.
func (c *Client) GetWithContext(ctx context.Context, path string) (map[string]interface{}, error) {
	url := c.ApiUrl + path
	
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authorization header
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))

	// Execute the request
	log.Debug().Str("url", url).Msg("Making API request to Proxmox")
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned non-200 status: %d - %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetRawWithContext performs a direct GET request to the Proxmox API and returns the raw response body.
func (c *Client) GetRawWithContext(ctx context.Context, path string) ([]byte, error) {
	url := c.ApiUrl + path
	
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authorization header
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))

	// Execute the request
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned non-200 status: %d - %s", resp.StatusCode, string(body))
	}

	// Read the response body
	return io.ReadAll(resp.Body)
}
