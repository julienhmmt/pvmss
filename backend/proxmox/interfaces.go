package proxmox

import (
	"context"
	"net/url"
	"time"
)

// ClientInterface defines the methods needed to interact with the Proxmox API.
type ClientInterface interface {
	DeleteWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error)
	Get(path string) (map[string]interface{}, error)
	GetApiUrl() string
	GetCSRFPreventionToken() string
	GetJSON(ctx context.Context, path string, target interface{}) error
	GetPVEAuthCookie() string
	GetTimeout() time.Duration
	GetWithContext(ctx context.Context, path string) (map[string]interface{}, error)
	InvalidateCache(path string)
	PostFormAndGetJSON(ctx context.Context, path string, data url.Values, v interface{}) error
	PostFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error)
	PutFormWithContext(ctx context.Context, path string, data url.Values) (map[string]interface{}, error)
	SetTimeout(timeout time.Duration)
}
