package cloudinit_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"pvmss/server/internal/cloudinit"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestValidateSSHKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		wantErr error
	}{
		{name: "empty", key: "", wantErr: cloudinit.ErrSSHKeyEmpty},
		{name: "whitespace only", key: "   ", wantErr: cloudinit.ErrSSHKeyEmpty},
		{name: "multiline smuggle", key: "ssh-rsa AAAAB1\nssh-rsa AAAAB2", wantErr: cloudinit.ErrSSHKeyMultiline},
		{name: "crlf smuggle", key: "ssh-rsa AAAAB1\r\nssh-rsa AAAAB2", wantErr: cloudinit.ErrSSHKeyMultiline},
		{name: "single field", key: "ssh-rsa", wantErr: cloudinit.ErrSSHKeyFormat},
		{name: "unknown type", key: "ssh-bad AAAAB1 comment", wantErr: cloudinit.ErrSSHKeyType},
		{name: "bad base64 blob", key: "ssh-rsa @@@notbase64 comment", wantErr: cloudinit.ErrSSHKeyFormat},
		{name: "rsa valid", key: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDemo comment here"},
		{name: "ed25519 valid", key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDemo comment"},
		{name: "ecdsa valid", key: "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAA"},
		{name: "sk ed25519 valid", key: "sk-ssh-ed25519@openssh.com AAAAInJlZm9ybS1rZXktdh== comment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := cloudinit.ValidateSSHKey(tt.key)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateSSHKey() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSSHKeys(t *testing.T) {
	t.Parallel()

	if err := cloudinit.ValidateSSHKeys(nil); err != nil {
		t.Fatalf("ValidateSSHKeys(nil) error = %v, want nil", err)
	}

	if err := cloudinit.ValidateSSHKeys([]string{"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDemo"}); err != nil {
		t.Fatalf("ValidateSSHKeys(valid) error = %v, want nil", err)
	}

	err := cloudinit.ValidateSSHKeys([]string{"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDemo", "ssh-rsa AAAAB1\nssh-rsa AAAAB2"})
	if !errors.Is(err, cloudinit.ErrSSHKeyMultiline) {
		t.Fatalf("ValidateSSHKeys(bad) error = %v, want %v", err, cloudinit.ErrSSHKeyMultiline)
	}
}

// testPublicKey generates a real OpenSSH public key line of the requested
// kind, so fingerprint tests exercise ssh.ParseAuthorizedKey rather than a
// regex-shaped stand-in.
func testPublicKey(t *testing.T, kind string) (string, ssh.PublicKey) {
	t.Helper()

	var pub ssh.PublicKey

	var err error

	switch kind {
	case "ed25519":
		var edPub ed25519.PublicKey
		if edPub, _, err = ed25519.GenerateKey(rand.Reader); err == nil {
			pub, err = ssh.NewPublicKey(edPub)
		}
	case "rsa":
		var key *rsa.PrivateKey
		if key, err = rsa.GenerateKey(rand.Reader, 2048); err == nil {
			pub, err = ssh.NewPublicKey(&key.PublicKey)
		}
	case "ecdsa":
		var key *ecdsa.PrivateKey
		if key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); err == nil {
			pub, err = ssh.NewPublicKey(&key.PublicKey)
		}
	default:
		t.Fatalf("unknown key kind %q", kind)
	}

	if err != nil {
		t.Fatalf("generate %s key: %v", kind, err)
	}

	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub))), pub
}

func TestValidateProfileSSHKey(t *testing.T) {
	t.Parallel()

	edKey, edPub := testPublicKey(t, "ed25519")
	rsaKey, _ := testPublicKey(t, "rsa")
	ecKey, _ := testPublicKey(t, "ecdsa")
	wantFingerprint := ssh.FingerprintSHA256(edPub)

	tests := []struct {
		name            string
		key             string
		wantErr         error
		wantFingerprint string
	}{
		{name: "ed25519", key: edKey, wantFingerprint: wantFingerprint},
		{name: "ed25519 with comment", key: edKey + " me@example", wantFingerprint: wantFingerprint},
		{name: "rsa", key: rsaKey},
		{name: "ecdsa", key: ecKey},
		{name: "empty", key: "", wantErr: cloudinit.ErrSSHKeyEmpty},
		{name: "private key pem block", key: "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA\n-----END OPENSSH PRIVATE KEY-----", wantErr: cloudinit.ErrSSHKeyPrivate},
		{name: "private key single line", key: "-----BEGIN PRIVATE KEY----- abc", wantErr: cloudinit.ErrSSHKeyPrivate},
		{name: "multiline", key: edKey + "\n" + rsaKey, wantErr: cloudinit.ErrSSHKeyMultiline},
		{name: "unknown type", key: "ssh-bad AAAAB1 comment", wantErr: cloudinit.ErrSSHKeyType},
		{name: "unparseable blob", key: "ssh-ed25519 AAAA", wantErr: cloudinit.ErrSSHKeyFormat},
		{name: "too long", key: edKey + " " + strings.Repeat("x", cloudinit.MaxSSHKeyBytes), wantErr: cloudinit.ErrSSHKeyTooLong},
		{name: "private key over size limit still reports private", key: "-----BEGIN OPENSSH PRIVATE KEY----- " + strings.Repeat("x", cloudinit.MaxSSHKeyBytes), wantErr: cloudinit.ErrSSHKeyPrivate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fingerprint, err := cloudinit.ValidateProfileSSHKey(tt.key)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateProfileSSHKey() error = %v, want %v", err, tt.wantErr)
			}

			if tt.wantErr != nil {
				return
			}

			if !strings.HasPrefix(fingerprint, "SHA256:") {
				t.Fatalf("fingerprint = %q, want SHA256: prefix", fingerprint)
			}

			if tt.wantFingerprint != "" && fingerprint != tt.wantFingerprint {
				t.Fatalf("fingerprint = %q, want %q", fingerprint, tt.wantFingerprint)
			}
		})
	}
}

func TestSSHKeyFingerprint(t *testing.T) {
	t.Parallel()

	key, pub := testPublicKey(t, "ed25519")
	want := ssh.FingerprintSHA256(pub)

	got, err := cloudinit.SSHKeyFingerprint(key)
	if err != nil {
		t.Fatalf("SSHKeyFingerprint: %v", err)
	}

	if got != want {
		t.Fatalf("SSHKeyFingerprint = %q, want %q", got, want)
	}

	withComment, err := cloudinit.SSHKeyFingerprint(key + " another-comment")
	if err != nil {
		t.Fatalf("SSHKeyFingerprint with comment: %v", err)
	}

	if withComment != want {
		t.Fatalf("SSHKeyFingerprint(comment) = %q, want %q (comment must not change the fingerprint)", withComment, want)
	}

	if _, err := cloudinit.SSHKeyFingerprint("not-a-key"); err == nil {
		t.Fatal("SSHKeyFingerprint(garbage) = nil error, want failure")
	}
}
