package security

import (
	"net/url"
	"path/filepath"
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
	// Clean collapses things like /a/../b and removes redundant separators
	cleaned := filepath.Clean(path)
	// Disallow parent traversal that escapes root by removing any leading ../
	for strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		cleaned = strings.TrimPrefix(cleaned, "../")
		if cleaned == ".." {
			cleaned = "."
		}
	}
	return cleaned
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	cost := GetConfig().BcryptCost
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

// CheckPasswordHash verifies a password against its hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
