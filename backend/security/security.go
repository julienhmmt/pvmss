package security

import (
	"net/url"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ValidateInput trims leading/trailing whitespace from a string and truncates it
// to a maximum length. It's a basic sanitization step for user-provided text.
func ValidateInput(input string, maxLength int) string {
	if input == "" {
		return ""
	}

	// Trim whitespace and enforce maximum length.
	cleaned := strings.TrimSpace(input)
	if len(cleaned) > maxLength {
		cleaned = cleaned[:maxLength]
	}

	return cleaned
}

// ValidateURL checks if a string is a valid HTTP or HTTPS URL.
func ValidateURL(urlStr string) bool {
	u, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// SanitizePath cleans a file path to prevent directory traversal attacks.
// It removes relative path components like '..' and ensures the path is clean.
func SanitizePath(path string) string {
	// Clean resolves '..' and removes redundant slashes.
	cleaned := filepath.Clean(path)

	// Repeatedly remove leading '..' to prevent escaping the intended directory.
	for strings.HasPrefix(cleaned, "../") {
		cleaned = strings.TrimPrefix(cleaned, "../")
	}

	return cleaned
}

// HashPassword creates a bcrypt hash of a password using the configured cost.
func HashPassword(password string) (string, error) {
	cost := GetConfig().BcryptCost
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

// CheckPasswordHash compares a plaintext password with a bcrypt hash to see if they match.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
