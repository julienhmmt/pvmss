package security

import (
	"net/url"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ValidateInput validates and sanitizes input
func ValidateInput(input string, maxLength int) string {
	if input == "" {
		return ""
	}

	// Trim and limit length
	cleaned := strings.TrimSpace(input)
	if len(cleaned) > maxLength {
		cleaned = cleaned[:maxLength]
	}

	return cleaned
}

// ValidateURL validates a URL
func ValidateURL(urlStr string) bool {
	u, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// SanitizePath cleans and validates a file path
func SanitizePath(path string) string {
	// Remove any directory traversal attempts
	path = strings.ReplaceAll(path, "../", "")
	return path
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash verifies a password against its hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
