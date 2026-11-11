package store_test

import (
	"errors"
	"pvmss/server/internal/store"
	"testing"
)

//nolint:gosec // G101: intentionally fake credentials embedded in a URL to exercise the userinfo refusal
const testURLWithUserinfo = "https://user:pw@pve"

func TestValidateClusterURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "https accepted", raw: "https://pve:8006", wantErr: false},
		{name: "https with api path accepted", raw: "https://pve.example:8006/api2/json", wantErr: false},
		{name: "http on ipv4 loopback accepted", raw: "http://127.0.0.1:1234", wantErr: false},
		{name: "http on localhost accepted", raw: "http://localhost:8006", wantErr: false},
		{name: "http on ipv6 loopback accepted", raw: "http://[::1]:8006", wantErr: false},
		{name: "http on remote host refused", raw: "http://pve:8006", wantErr: true},
		{name: "bare host and port refused", raw: "pve:8006", wantErr: true},
		{name: "userinfo refused", raw: testURLWithUserinfo, wantErr: true},
		{name: "query refused", raw: "https://pve:8006/?x=1", wantErr: true},
		{name: "other scheme refused", raw: "ftp://pve:8006", wantErr: true},
		{name: "empty refused", raw: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := store.ValidateClusterURL(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateClusterURL(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}

			if err != nil && !errors.Is(err, store.ErrInvalidClusterURL) {
				t.Fatalf("ValidateClusterURL(%q) error = %v, want ErrInvalidClusterURL", tt.raw, err)
			}
		})
	}
}
