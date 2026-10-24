package cloudinit_test

import (
	"errors"
	"pvmss/server/internal/cloudinit"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{name: "empty clears", content: ""},
		{name: "valid marker", content: "#cloud-config\nusers: {}"},
		{name: "valid users list", content: "#cloud-config\nusers:\n  - name: alice"},
		{name: "leading whitespace allowed", content: " \n\t#cloud-config\n"},
		{name: "wrong marker", content: "users: {}", wantErr: cloudinit.ErrSnippetPrefix},
		{
			name:    "exact size",
			content: "#cloud-config\nkey: " + strings.Repeat("x", cloudinit.MaxSnippetSize-len("#cloud-config\nkey: ")),
		},
		{name: "too large", content: strings.Repeat("x", cloudinit.MaxSnippetSize+1), wantErr: cloudinit.ErrSnippetTooLarge},
		{name: "invalid UTF-8", content: "#cloud-config\n\xff", wantErr: cloudinit.ErrSnippetInvalidUTF8},
		{name: "malformed yaml", content: "#cloud-config\ninvalid: [", wantErr: cloudinit.ErrSnippetInvalidYAML},
		{name: "scalar root", content: "#cloud-config\nhello", wantErr: cloudinit.ErrSnippetInvalidYAML},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := cloudinit.Validate(tt.content)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
