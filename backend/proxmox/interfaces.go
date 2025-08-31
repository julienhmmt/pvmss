package proxmox

import (
	"context"
	"net/url"
	"time"
)

// ClientInterface defines the methods needed to interact with the Proxmox API.
// This is used for mocking and abstracting the underlying client implementation.
type ClientInterface interface {
	Get(path string) (map[string]interface{}, error)
	GetWithContext(ctx context.Context, path string) (map[string]interface{}, error)
	GetJSON(ctx context.Context, path string, target interface{}) error
	PostFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error)
	PostFormAndGetJSON(ctx context.Context, path string, data url.Values, v interface{}) error
	PutFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error)
	GetVNCProxy(ctx context.Context, node string, vmID int) (map[string]interface{}, error)
	GetApiUrl() string
	SetTimeout(timeout time.Duration)
	GetTimeout() time.Duration
	InvalidateCache(path string)
	GetPVEAuthCookie() string
	GetCSRFPreventionToken() string
}
