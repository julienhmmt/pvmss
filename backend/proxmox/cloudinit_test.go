package proxmox

import (
	"strings"
	"testing"
)

func TestValidateSnippetFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{name: "valid simple", filename: "pvmss-100.yml", wantErr: false},
		{name: "valid with dots", filename: "my.template.yaml", wantErr: false},
		{name: "valid probe", filename: ".pvmss-sftp-test-123", wantErr: false},
		{name: "empty rejected", filename: "", wantErr: true},
		{name: "dot rejected", filename: ".", wantErr: true},
		{name: "double-dot rejected", filename: "..", wantErr: true},
		{name: "unix traversal", filename: "../etc/passwd", wantErr: true},
		{name: "backslash traversal", filename: `..\windows\win.ini`, wantErr: true},
		{name: "leading slash absolute", filename: "/etc/passwd", wantErr: true},
		{name: "trailing slash", filename: "foo/", wantErr: true},
		{name: "nested path", filename: "sub/dir.yml", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSnippetFilename(tt.filename)
			if tt.wantErr && err == nil {
				t.Errorf("validateSnippetFilename(%q) expected error, got nil", tt.filename)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateSnippetFilename(%q) expected nil, got %v", tt.filename, err)
			}
		})
	}
}

func TestValidateSnippetFilename_ErrorMessages(t *testing.T) {
	// Path-traversal attempts must be rejected with a message that names the
	// offending filename so logs/audit trails are actionable.
	err := validateSnippetFilename("../etc/passwd")
	if err == nil {
		t.Fatal("expected error for traversal filename")
	}
	if !strings.Contains(err.Error(), "../etc/passwd") {
		t.Errorf("error should contain the filename, got: %v", err)
	}
}
