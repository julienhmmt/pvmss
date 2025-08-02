package proxmox

import (
	"context"
	"time"
)

// ClientInterface defines the interface that both the real and mock Proxmox clients must implement
type ClientInterface interface {
	// GetRawWithContext performs a raw GET request with context support
	GetRawWithContext(ctx context.Context, path string) ([]byte, error)

	// GetWithContext performs a GET request with context support and returns the response as a map
	GetWithContext(ctx context.Context, path string) (map[string]interface{}, error)

	// GetJSON performs a GET request and unmarshals the response into the target interface
	GetJSON(ctx context.Context, path string, target interface{}) error

	// Get performs a GET request using the client's default timeout
	Get(path string) (map[string]interface{}, error)

	// InvalidateCache removes entries from the client's response cache
	InvalidateCache(path string)

	// GetTimeout returns the client's configured timeout duration
	GetTimeout() time.Duration

	// SetTimeout sets the client's request timeout
	SetTimeout(timeout time.Duration)
}
