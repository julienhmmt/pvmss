package proxmox

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	"pvmss/constants"
)

// sharedTransports caches one *http.Transport per InsecureSkipVerify setting.
//
// Design Decision: Transports are shared across all Proxmox servers with the same
// TLS verification setting. This is safe because HTTP connections are keyed by
// (host, port) internally, so connections to different servers won't interfere.
// This design maximizes connection reuse and minimizes resource overhead.
//
// If per-server transport configuration is ever needed (e.g., different proxy
// settings per server), the cache key would need to include the server URL or
// a configuration hash.
var (
	sharedTransportMu sync.Mutex
	sharedTransports  = make(map[bool]*http.Transport, 2)
)

// getSharedTransport returns the process-wide *http.Transport for the given
// InsecureSkipVerify setting. The transport is created lazily on first use.
func getSharedTransport(insecureSkipVerify bool) *http.Transport {
	sharedTransportMu.Lock()
	defer sharedTransportMu.Unlock()

	if t, ok := sharedTransports[insecureSkipVerify]; ok {
		return t
	}

	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   constants.HTTPDialTimeout,
			KeepAlive: constants.HTTPDialKeepAlive,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          constants.HTTPMaxIdleConns,
		MaxIdleConnsPerHost:   constants.HTTPMaxIdleConnsPerHost,
		IdleConnTimeout:       constants.HTTPIdleConnTimeout,
		TLSHandshakeTimeout:   constants.HTTPTLSHandshakeTimeout,
		ExpectContinueTimeout: constants.HTTPExpectContinueTimeout,
		ResponseHeaderTimeout: constants.HTTPResponseHeaderTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify, // #nosec G402 - controlled by PROXMOX_VERIFY_SSL
			MinVersion:         tls.VersionTLS12,
		},
	}

	sharedTransports[insecureSkipVerify] = t
	return t
}

// ResetSharedTransports clears the transport cache, forcing new transports to be
// created on the next call to getSharedTransport(). This is useful for testing
// scenarios or if TLS configuration is dynamically reloaded at runtime.
func ResetSharedTransports() {
	sharedTransportMu.Lock()
	defer sharedTransportMu.Unlock()

	sharedTransports = make(map[bool]*http.Transport, 2)
}

// CloseSharedTransports closes all idle connections in cached transports.
// This should be called during application shutdown to gracefully terminate connections.
// After calling this function, new transports will be created on demand.
func CloseSharedTransports() {
	sharedTransportMu.Lock()
	defer sharedTransportMu.Unlock()

	for _, t := range sharedTransports {
		t.CloseIdleConnections()
	}

	sharedTransports = make(map[bool]*http.Transport, 2)
}
