package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// secretCipherPrefix marks a value produced by EncryptSecret so DecryptSecret
// can tell ciphertext apart from a legacy plaintext value.
const secretCipherPrefix = "encv1:"

// deriveAESKey derives a 32-byte AES-256 key from an arbitrary-length secret
// (e.g. SESSION_SECRET) via SHA-256.
func deriveAESKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// EncryptSecret encrypts plaintext with AES-256-GCM using a key derived from
// secret, returning a prefixed, base64-encoded string. Empty plaintext returns
// "" (nothing to protect). Used to store sensitive values (e.g. an SSH private
// key) at rest in the database.
func EncryptSecret(plaintext, secret string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if secret == "" {
		return "", errors.New("encrypt secret: empty session secret")
	}
	block, err := aes.NewCipher(deriveAESKey(secret))
	if err != nil {
		return "", fmt.Errorf("encrypt secret: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encrypt secret: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encrypt secret: nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return secretCipherPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptSecret reverses EncryptSecret. A value without the encryption prefix is
// returned unchanged (treated as already-plaintext), so callers can pass either
// form transparently during/after a migration.
func DecryptSecret(value, secret string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, secretCipherPrefix) {
		return value, nil
	}
	if secret == "" {
		return "", errors.New("decrypt secret: empty session secret")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, secretCipherPrefix))
	if err != nil {
		return "", fmt.Errorf("decrypt secret: base64: %w", err)
	}
	block, err := aes.NewCipher(deriveAESKey(secret))
	if err != nil {
		return "", fmt.Errorf("decrypt secret: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: new gcm: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("decrypt secret: ciphertext too short")
	}
	nonce, sealed := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: open: %w", err)
	}
	return string(plaintext), nil
}

// IsEncrypted reports whether value carries the encryption prefix.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, secretCipherPrefix)
}
