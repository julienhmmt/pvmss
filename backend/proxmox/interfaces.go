package proxmox

import (
	"context"
	"net/url"
	"time"
)

// ClientInterface defines the methods for interacting with the Proxmox API.
// It's used for abstraction and testability.
type ClientInterface interface {
	Get(path string) (map[string]interface{}, error)
	GetWithContext(ctx context.Context, path string) (map[string]interface{}, error)
	GetJSON(ctx context.Context, path string, target interface{}) error
	PostFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error)
	PutFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error)
	GetVNCProxy(ctx context.Context, node string, vmID int) (map[string]interface{}, error)
	GetApiUrl() string
	SetTimeout(timeout time.Duration)
	GetTimeout() time.Duration
	InvalidateCache(path string)
}
