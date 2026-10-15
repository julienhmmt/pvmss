package cloudinit

import (
	"errors"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

// MaxSSHKeyBytes is the maximum size accepted for a profile SSH public key.
// A 4096-bit RSA public key line is under 800 bytes; 1024 leaves headroom for
// a comment without letting a profile row carry arbitrary blobs.
const MaxSSHKeyBytes = 1024

var (
	// ErrSSHKeyEmpty reports a blank key in a key list.
	ErrSSHKeyEmpty = errors.New("cloud-init ssh key must not be empty")
	// ErrSSHKeyMultiline reports a key spanning more than one line, which
	// would smuggle extra keys into authorized_keys on a naive paste.
	ErrSSHKeyMultiline = errors.New("cloud-init ssh key must be a single line")
	// ErrSSHKeyType reports an unsupported or malformed key type prefix.
	ErrSSHKeyType = errors.New("cloud-init ssh key has an unsupported type")
	// ErrSSHKeyFormat reports a key that is not a well-formed OpenSSH public key.
	ErrSSHKeyFormat = errors.New("cloud-init ssh key is not a valid openssh public key")
	// ErrSSHKeyPrivate reports a pasted private key block, which must never
	// be stored where a public key is expected.
	ErrSSHKeyPrivate = errors.New("ssh key is a private key, not a public key")
	// ErrSSHKeyTooLong reports a key over MaxSSHKeyBytes.
	ErrSSHKeyTooLong = errors.New("ssh key exceeds the maximum size")
)

// sshKeyTypeRE matches the type prefixes Proxmox/cloud-init accept as public
// keys. It mirrors ProxMate's isValidPublicKey allowlist.
var sshKeyTypeRE = regexp.MustCompile(`^(ssh-rsa|ssh-ed25519|ssh-dss|ecdsa-sha2-nistp(256|384|521)|sk-ssh-ed25519@openssh\.com|sk-ecdsa-sha2-nistp256@openssh\.com)$`)

// sshKeyBlobRE matches the base64 key body of an OpenSSH public key. It is
// intentionally lenient about the alphabet (it accepts the standard base64
// set plus a hyphen/underscore) so it never rejects a legitimate key whose
// blob uses an unusual encoding; the security property that matters is that
// the value is a single line with a known type prefix, not that the blob is
// cryptographically perfect (the guest validates that on apply).
var sshKeyBlobRE = regexp.MustCompile(`^[A-Za-z0-9+/=_-]+$`)

// ValidateSSHKey checks one public key. It rejects empty strings, multi-line
// values (a pasted block would inject extra keys into authorized_keys), and
// anything that is not a well-formed OpenSSH public key
// ("<type> <base64> [comment]"). The comment is optional and may contain
// spaces, so it is not validated beyond the first two fields.
func ValidateSSHKey(key string) error {
	if key == "" || strings.TrimSpace(key) == "" {
		return ErrSSHKeyEmpty
	}

	if strings.ContainsAny(key, "\n\r") {
		return ErrSSHKeyMultiline
	}

	fields := strings.Fields(key)
	if len(fields) < 2 {
		return ErrSSHKeyFormat
	}

	if !sshKeyTypeRE.MatchString(fields[0]) {
		return ErrSSHKeyType
	}

	if !sshKeyBlobRE.MatchString(fields[1]) {
		return ErrSSHKeyFormat
	}

	return nil
}

// ValidateSSHKeys validates every key in a list, returning the first error.
func ValidateSSHKeys(keys []string) error {
	for _, key := range keys {
		if err := ValidateSSHKey(key); err != nil {
			return err
		}
	}

	return nil
}

// ValidateProfileSSHKey validates one SSH public key for the per-user profile
// store and returns its SHA256 fingerprint. On top of ValidateSSHKey it
// rejects pasted private keys (checked first so a multi-line PEM block reports
// private rather than multiline), keys over MaxSSHKeyBytes, and key blobs that
// look structurally fine but cannot be parsed as a real OpenSSH public key.
func ValidateProfileSSHKey(key string) (string, error) {
	if strings.Contains(key, "PRIVATE KEY") && strings.Contains(key, "-----BEGIN") {
		return "", ErrSSHKeyPrivate
	}

	if len(key) > MaxSSHKeyBytes {
		return "", ErrSSHKeyTooLong
	}

	if err := ValidateSSHKey(key); err != nil {
		return "", err
	}

	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
	if err != nil {
		return "", ErrSSHKeyFormat
	}

	return ssh.FingerprintSHA256(pub), nil
}

// SSHKeyFingerprint returns the SHA256 fingerprint of an OpenSSH public key
// line ("<type> <base64> [comment]"). The comment does not participate, so the
// same key blob always yields the same fingerprint.
func SSHKeyFingerprint(key string) (string, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
	if err != nil {
		return "", ErrSSHKeyFormat
	}

	return ssh.FingerprintSHA256(pub), nil
}
