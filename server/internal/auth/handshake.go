package auth

import (
	"crypto/hmac"
	"encoding/base64"
	"net/http"
	"strings"
)

// handshakeDomain separates handshake signatures from the session-hash HMAC
// that uses the same secret.
const handshakeDomain = "pvmss-handshake|"

// Sign returns value with an HMAC-SHA256 tag keyed by the application secret,
// for short-lived cookies the server must trust on the way back (the OIDC
// handshake). value is base64url-encoded, so it may hold any byte.
func (m *SessionManager) Sign(value string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(value))

	return encoded + "." + base64.RawURLEncoding.EncodeToString(m.tag(encoded))
}

// Verify returns the value Sign wrapped, or false when the tag does not match.
func (m *SessionManager) Verify(signed string) (string, bool) {
	encoded, tag, ok := strings.Cut(signed, ".")
	if !ok {
		return "", false
	}

	got, err := base64.RawURLEncoding.DecodeString(tag)
	if err != nil || !hmac.Equal(got, m.tag(encoded)) {
		return "", false
	}

	value, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}

	return string(value), true
}

func (m *SessionManager) tag(encoded string) []byte {
	return m.hash(handshakeDomain + encoded)
}

// HandshakeCookie builds a short-lived HttpOnly cookie for a login handshake.
// SameSite is Lax, not Strict: the IdP's redirect back is a cross-site
// navigation, and a Strict cookie would not ride along. maxAge < 0 clears it.
func (m *SessionManager) HandshakeCookie(name, value, path string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: path, MaxAge: maxAge, HttpOnly: true, Secure: m.isSecure, SameSite: http.SameSiteLaxMode} //nolint:gosec // Secure is intentionally conditional for dev HTTP mode
}
