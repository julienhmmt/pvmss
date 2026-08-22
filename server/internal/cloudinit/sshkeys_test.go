package cloudinit_test

import (
	"errors"
	"pvmss/server/internal/cloudinit"
	"testing"
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
