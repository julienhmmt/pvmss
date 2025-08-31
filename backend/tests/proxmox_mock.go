package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"pvmss/proxmox"
)

// MockStorage represents a Proxmox storage entry for testing
type MockStorage struct {
	Storage     string      `json:"storage"`
	Type        string      `json:"type"`
	Used        json.Number `json:"used,omitempty"`
	Total       json.Number `json:"total,omitempty"`
	Avail       json.Number `json:"avail,omitempty"`
	Active      int         `json:"active,omitempty"`
	Enabled     int         `json:"enabled,omitempty"`
	Shared      int         `json:"shared,omitempty"`
	Content     string      `json:"content,omitempty"`
	Nodes       string      `json:"nodes,omitempty"`
	Description string      `json:"description,omitempty"`
}

// MockStorageListResponse represents the mock response from the /api2/json/storage endpoint
type MockStorageListResponse struct {
	Data []MockStorage `json:"data"`
}

// MockVM represents a Proxmox VM entry for testing
type MockVM struct {
	VMID    int    `json:"vmid"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Node    string `json:"node"`
	Cpus    int    `json:"cpus"`
	Maxdisk int64  `json:"maxdisk"`
	Maxmem  int64  `json:"maxmem"`
	Mem     int64  `json:"mem"`
	Disk    int64  `json:"disk"`
	Uptime  int64  `json:"uptime"`
}

// MockVMBR represents a Proxmox network bridge entry for testing
type MockVMBR struct {
	Iface string `json:"iface"`
	Type  string `json:"type"`
}

// MockISO represents a Proxmox ISO entry for testing
type MockISO struct {
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	CreationTime string `json:"ctime"`
}

// MockNode represents a Proxmox node entry for testing
type MockNode struct {
	Node   string `json:"node"`
	Status string `json:"status"`
	Cpus   int    `json:"cpus"`
	Maxcpu int    `json:"maxcpu"`
	Mem    int64  `json:"mem"`
	Maxmem int64  `json:"maxmem"`
}

// MockCacheEntry represents a cached API response for testing
type MockCacheEntry struct {
	Data      []byte
	Timestamp time.Time
	TTL       time.Duration
}

// MockClientOption defines a function signature for applying configuration options to the MockProxmoxClient
type MockClientOption func(*MockProxmoxClient)

// WithMockStorages returns a MockClientOption that sets mock storage data
func WithMockStorages(storages []MockStorage) MockClientOption {
	return func(c *MockProxmoxClient) {
		c.mockStorages = storages
	}
}

// WithMockVMBRs returns a MockClientOption that sets mock VMBR data
func WithMockVMBRs(vmbRs []MockVMBR) MockClientOption {
	return func(c *MockProxmoxClient) {
		c.mockVMBRs = vmbRs
	}
}

// WithMockISOs returns a MockClientOption that sets mock ISO data
func WithMockISOs(isos []MockISO) MockClientOption {
	return func(c *MockProxmoxClient) {
		c.mockISOs = isos
	}
}

// WithMockVMs returns a MockClientOption that sets mock VM data
func WithMockVMs(vms []MockVM) MockClientOption {
	return func(c *MockProxmoxClient) {
		c.mockVMs = vms
	}
}

// WithMockNodes returns a MockClientOption that sets mock Node data
func WithMockNodes(nodes []MockNode) MockClientOption {
	return func(c *MockProxmoxClient) {
		c.mockNodes = nodes
	}
}

// MockProxmoxClient is a mock implementation of the Proxmox client for testing
type MockProxmoxClient struct {
	ApiUrl    string
	AuthToken string
	Timeout   time.Duration
	cache     map[string]*MockCacheEntry
	cacheTTL  time.Duration
	mux       sync.RWMutex

	// Mock data
	mockStorages     []MockStorage
	mockVMBRs        []MockVMBR
	mockISOs         []MockISO
	mockVMs          []MockVM
	mockNodes        []MockNode
	mockVNCProxyData map[string]interface{}
}

// WithMockVNCProxy returns a MockClientOption that sets mock VNC proxy data
func WithMockVNCProxy(vncProxyData map[string]interface{}) MockClientOption {
	return func(c *MockProxmoxClient) {
		c.mockVNCProxyData = vncProxyData
	}
}

// Ensure MockProxmoxClient implements proxmox.ClientInterface
var _ proxmox.ClientInterface = (*MockProxmoxClient)(nil)

// NewMockProxmoxClient creates a new mock Proxmox client for testing
func NewMockProxmoxClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool, opts ...MockClientOption) *MockProxmoxClient {
	// Create auth token
	authToken := fmt.Sprintf("%s=%s", apiTokenID, apiTokenSecret)

	// Create mock client
	client := &MockProxmoxClient{
		ApiUrl:    apiURL,
		AuthToken: authToken,
		Timeout:   10 * time.Second,
		cache:     make(map[string]*MockCacheEntry),
		cacheTTL:  2 * time.Minute,
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	return client
}

// GetRawWithContext is the core method for making GET requests in the mock client
func (c *MockProxmoxClient) GetRawWithContext(ctx context.Context, path string) ([]byte, error) {
	// Try to get from cache first if caching is enabled
	if c.cache != nil {
		c.mux.RLock()
		cacheEntry, found := c.cache[path]
		c.mux.RUnlock()

		if found && time.Since(cacheEntry.Timestamp) < c.cacheTTL {
			return cacheEntry.Data, nil
		}
	}

	// Handle different paths with mock data
	var data []byte
	var err error

	switch path {
	case "/storage":
		// Return mock storage data
		response := MockStorageListResponse{
			Data: c.mockStorages,
		}
		data, err = json.Marshal(response)
		if err != nil {
			return nil, err
		}
	case "/cluster/resources":
		// Return mock cluster resources data
		vmData := make([]map[string]interface{}, len(c.mockVMs))
		for i, vm := range c.mockVMs {
			vmData[i] = map[string]interface{}{
				"vmid":    vm.VMID,
				"name":    vm.Name,
				"status":  vm.Status,
				"node":    vm.Node,
				"cpus":    vm.Cpus,
				"maxdisk": vm.Maxdisk,
				"maxmem":  vm.Maxmem,
				"mem":     vm.Mem,
				"disk":    vm.Disk,
				"uptime":  vm.Uptime,
			}
		}

		data, err = json.Marshal(map[string]interface{}{
			"data": vmData,
		})
		if err != nil {
			return nil, err
		}
	case "/nodes":
		// Return mock nodes data
		nodeData := make([]map[string]interface{}, len(c.mockNodes))
		for i, node := range c.mockNodes {
			nodeData[i] = map[string]interface{}{
				"node":   node.Node,
				"status": node.Status,
				"cpus":   node.Cpus,
				"maxcpu": node.Maxcpu,
				"mem":    node.Mem,
				"maxmem": node.Maxmem,
			}
		}

		data, err = json.Marshal(map[string]interface{}{
			"data": nodeData,
		})
		if err != nil {
			return nil, err
		}
	default:
		// For other paths, return empty JSON object
		data = []byte("{}")
	}

	// Cache the response if caching is enabled
	if c.cache != nil {
		c.mux.Lock()
		c.cache[path] = &MockCacheEntry{
			Data:      data,
			Timestamp: time.Now(),
		}
		c.mux.Unlock()
	}

	return data, nil
}

// GetWithContext performs a GET request in the mock client
func (c *MockProxmoxClient) GetWithContext(ctx context.Context, path string) (map[string]interface{}, error) {
	// Try to get from cache first if caching is enabled
	if c.cache != nil {
		c.mux.RLock()
		cacheEntry, found := c.cache[path]
		c.mux.RUnlock()

		if found && time.Since(cacheEntry.Timestamp) < c.cacheTTL {
			var result map[string]interface{}
			if err := json.Unmarshal(cacheEntry.Data, &result); err == nil {
				return result, nil
			}
		}
	}

	// Not in cache, make the request
	rawData, err := c.GetRawWithContext(ctx, path)
	if err != nil {
		return nil, err
	}

	// Parse the response
	var result map[string]interface{}
	if err := json.Unmarshal(rawData, &result); err != nil {
		return nil, err
	}

	// Cache the raw response if caching is enabled
	if c.cache != nil {
		c.mux.Lock()
		c.cache[path] = &MockCacheEntry{
			Data:      rawData,
			Timestamp: time.Now(),
		}
		c.mux.Unlock()
	}

	return result, nil
}

// Get performs a GET request using the client's default timeout
func (c *MockProxmoxClient) Get(path string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	return c.GetWithContext(ctx, path)
}

// InvalidateCache removes entries from the client's response cache
func (m *MockProxmoxClient) InvalidateCache(path string) {
	m.mux.Lock()
	defer m.mux.Unlock()

	if path == "" {
		// Clear all cache if no specific path is provided
		m.cache = make(map[string]*MockCacheEntry)
	} else {
		// Clear specific cache entry
		delete(m.cache, path)
	}
}

// GetTimeout returns the client's configured timeout duration.
func (m *MockProxmoxClient) GetTimeout() time.Duration {
	return m.Timeout
}

// SetTimeout sets the client's request timeout.
func (m *MockProxmoxClient) SetTimeout(timeout time.Duration) {
	m.Timeout = timeout
}

// GetApiUrl returns the base URL of the Proxmox API.
func (m *MockProxmoxClient) GetApiUrl() string {
	return m.ApiUrl
}

// GetPVEAuthCookie returns a mock PVEAuthCookie.
func (m *MockProxmoxClient) GetPVEAuthCookie() string {
	return "mock-pve-auth-cookie"
}

// GetCSRFPreventionToken returns a mock CSRF token.
func (m *MockProxmoxClient) GetCSRFPreventionToken() string {
	return "mock-csrf-token"
}

// GetJSON performs a GET request and unmarshals the response into the target interface
func (c *MockProxmoxClient) GetJSON(ctx context.Context, path string, target interface{}) error {
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

// PostFormWithContext performs a mock POST operation and returns a fake UPID
func (c *MockProxmoxClient) PostFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error) {
	// For now, just return a deterministic UPID-like response
	resp := map[string]interface{}{
		"data": "UPID:MOCK-12345",
	}
	return resp, nil
}

// PutFormWithContext performs a mock PUT operation and returns a fake UPID
func (c *MockProxmoxClient) PutFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error) {
	// Mirror Post mock behaviour
	resp := map[string]interface{}{
		"data": "UPID:MOCK-12345",
	}
	return resp, nil
}

// PostFormAndGetJSON performs a mock POST request and unmarshals the response.
func (c *MockProxmoxClient) PostFormAndGetJSON(ctx context.Context, path string, data url.Values, v interface{}) error {
	// For now, just return a deterministic UPID-like response
	resp := map[string]interface{}{
		"data": "UPID:MOCK-12345",
	}
	// Marshal the mock response and then unmarshal it into v
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal mock response: %w", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("failed to unmarshal mock response: %w", err)
	}
	return nil
}

// GetVNCProxy returns mock VNC proxy data.
func (c *MockProxmoxClient) GetVNCProxy(ctx context.Context, node string, vmID int) (map[string]interface{}, error) {
	if c.mockVNCProxyData != nil {
		return c.mockVNCProxyData, nil
	}
	// Return a default mock response if no specific data is set
	return map[string]interface{}{
		"data": map[string]interface{}{
			"port":   5900,
			"ticket": "mock-ticket",
			"upid":   "mock-upid",
		},
	}, nil
}
