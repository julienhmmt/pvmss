package handlers

import (
	"net/http"
	"strings"
)

func getSecureCookieFlag(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if isForwardedHTTPS(r) {
		return true
	}
	return false
}

func isForwardedHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if isXForwardedProtoHTTPS(r.Header.Get("X-Forwarded-Proto")) {
		return true
	}
	if isForwardedHeaderHTTPS(r.Header.Get("Forwarded")) {
		return true
	}
	if isXForwardedSSLHTTPS(r.Header.Get("X-Forwarded-Ssl")) {
		return true
	}
	return false
}

func isXForwardedProtoHTTPS(rawHeader string) bool {
	normalized := strings.TrimSpace(strings.ToLower(rawHeader))
	if normalized == "" {
		return false
	}
	firstProto := strings.TrimSpace(strings.Split(normalized, ",")[0])
	return firstProto == "https"
}

func isForwardedHeaderHTTPS(rawHeader string) bool {
	normalized := strings.TrimSpace(strings.ToLower(rawHeader))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "proto=https")
}

func isXForwardedSSLHTTPS(rawHeader string) bool {
	normalized := strings.TrimSpace(strings.ToLower(rawHeader))
	if normalized == "" {
		return false
	}
	return normalized == "on" || normalized == "true" || normalized == "1"
}
