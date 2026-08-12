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
		{name: "leading whitespace allowed", content: " \n\t#cloud-config\n"},
		{name: "wrong marker", content: "users: {}", wantErr: cloudinit.ErrSnippetPrefix},
		{name: "exact size", content: "#cloud-config\n" + strings.Repeat("x", cloudinit.MaxSnippetSize-len("#cloud-config\n"))},
		{name: "too large", content: strings.Repeat("x", cloudinit.MaxSnippetSize+1), wantErr: cloudinit.ErrSnippetTooLarge},
		{name: "invalid UTF-8", content: "#cloud-config\n\xff", wantErr: cloudinit.ErrSnippetInvalidUTF8},
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
