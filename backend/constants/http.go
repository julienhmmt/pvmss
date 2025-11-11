// Package constants defines application-wide constants for timeouts, limits, and configuration values.
package constants

import "time"

// HTTP and Form Configuration
const (
	// MaxFormSize is the maximum size for form submissions (10 MB)
	MaxFormSize = 10 * 1024 * 1024

	// MaxHeaderBytes is the maximum size for HTTP headers (1 MB)
	MaxHeaderBytes = 1 << 20
)

// Server Timeouts
const (
	// ServerReadTimeout is the maximum duration for reading the entire request
	ServerReadTimeout = 10 * time.Second

	// ServerWriteTimeout is the maximum duration before timing out writes of the response
	ServerWriteTimeout = 30 * time.Second

	// ServerIdleTimeout is the maximum amount of time to wait for the next request
	ServerIdleTimeout = 120 * time.Second

	// ServerReadHeaderTimeout is the amount of time allowed to read request headers
	ServerReadHeaderTimeout = 5 * time.Second
)

// HTTP Transport Configuration
const (
	// HTTPMaxIdleConns is the maximum number of idle connections across all hosts
	HTTPMaxIdleConns = 100

	// HTTPMaxIdleConnsPerHost is the maximum number of idle connections per host
	HTTPMaxIdleConnsPerHost = 50

	// HTTPIdleConnTimeout is the maximum amount of time an idle connection will remain idle
	HTTPIdleConnTimeout = 90 * time.Second

	// HTTPTLSHandshakeTimeout is the maximum amount of time to wait for a TLS handshake
	HTTPTLSHandshakeTimeout = 10 * time.Second

	// HTTPExpectContinueTimeout is the maximum amount of time to wait for a server's first
	// response headers after fully writing the request headers if the request has "Expect: 100-continue"
	HTTPExpectContinueTimeout = 1 * time.Second

	// HTTPResponseHeaderTimeout is the amount of time to wait for a server's response headers
	HTTPResponseHeaderTimeout = 15 * time.Second
)
