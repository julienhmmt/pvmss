package cloudinit

import (
	"strings"
	"testing"
)

func TestValidateCloudInitYAML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple YAML",
			input:   "key: value",
			wantErr: false,
		},
		{
			name: "valid cloud-init config",
			input: `#cloud-config
users:
  - name: testuser
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - ssh-rsa AAAAB...`,
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "whitespace only",
			input:   "   \n\t\n   ",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "invalid YAML syntax",
			input:   "key: value\n  invalid: indentation",
			wantErr: true,
			errMsg:  "YAML",
		},
		{
			name:    "invalid YAML - unclosed quote",
			input:   `key: "unclosed string`,
			wantErr: true,
			errMsg:  "YAML",
		},
		{
			name: "valid nested structure",
			input: `runcmd:
  - echo "hello"
  - apt-get update
packages:
  - nginx
  - vim`,
			wantErr: false,
		},
		{
			name:    "too large",
			input:   strings.Repeat("a", MaxYAMLSize+1),
			wantErr: true,
			errMsg:  "exceeds maximum size",
		},
		{
			name:    "too many lines",
			input:   strings.Repeat("key: value\n", MaxLineCount+1),
			wantErr: true,
			errMsg:  "exceeds maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCloudInitYAML(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCloudInitYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateCloudInitYAML() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateCloudInitYAMLStrict(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid with cloud-config header",
			input: `#cloud-config
users:
  - name: test`,
			wantErr: false,
		},
		{
			name:    "missing cloud-config header",
			input:   "users:\n  - name: test",
			wantErr: true,
			errMsg:  "#cloud-config",
		},
		{
			name: "header with leading whitespace on first line",
			input: `  #cloud-config
users:
  - name: test`,
			wantErr: false, // TrimSpace removes leading whitespace, so header is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCloudInitYAMLStrict(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCloudInitYAMLStrict() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateCloudInitYAMLStrict() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestIsValidYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid YAML",
			input: "key: value",
			want:  true,
		},
		{
			name:  "invalid YAML",
			input: "key: value\n  bad: indent",
			want:  false,
		},
		{
			name:  "empty",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidYAML(tt.input); got != tt.want {
				t.Errorf("IsValidYAML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCloudInitYAML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantKey string
	}{
		{
			name:    "simple map",
			input:   "key: value",
			wantErr: false,
			wantKey: "key",
		},
		{
			name: "cloud-init config",
			input: `#cloud-config
users:
  - name: test`,
			wantErr: false,
			wantKey: "users",
		},
		{
			name:    "invalid YAML",
			input:   "not: valid: yaml:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCloudInitYAML(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCloudInitYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.wantKey != "" {
				if _, ok := got[tt.wantKey]; !ok {
					t.Errorf("ParseCloudInitYAML() missing expected key %q", tt.wantKey)
				}
			}
		})
	}
}

func TestSanitizeYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no change needed",
			input: "key: value",
			want:  "key: value",
		},
		{
			name:  "trim whitespace",
			input: "  \n  key: value  \n  ",
			want:  "key: value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeYAML(tt.input); got != tt.want {
				t.Errorf("SanitizeYAML() = %v, want %v", got, tt.want)
			}
		})
	}
}
