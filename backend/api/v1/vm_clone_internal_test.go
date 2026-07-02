package apiv1

import "testing"

func TestIsValidHostname(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"simple", "web01", true},
		{"with hyphen", "web-clone-01", true},
		{"dotted", "web.example.com", true},
		{"digits", "12345", true},
		{"empty", "", false},
		{"leading hyphen", "-web", false},
		{"trailing hyphen", "web-", false},
		{"space", "web 01", false},
		{"underscore", "web_01", false},
		{"empty label", "web..com", false},
		{"too long", string(make([]byte, 64)), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidHostname(tt.in); got != tt.want {
				t.Fatalf("isValidHostname(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNodeAllowed(t *testing.T) {
	if !nodeAllowed(nil, "pve1") {
		t.Fatal("empty allowlist must permit any node")
	}
	if !nodeAllowed([]string{"pve1", "pve2"}, "pve2") {
		t.Fatal("listed node must be allowed")
	}
	if nodeAllowed([]string{"pve1"}, "pve9") {
		t.Fatal("unlisted node must be rejected")
	}
}

func TestStorageAllowed(t *testing.T) {
	if !storageAllowed(nil, "local-lvm") {
		t.Fatal("empty allowlist must permit any storage")
	}
	// entries use the node:storage form; the storage name is compared.
	if !storageAllowed([]string{"pve1:local-lvm", "pve2:ceph"}, "ceph") {
		t.Fatal("listed storage must be allowed")
	}
	if !storageAllowed([]string{"local-zfs"}, "local-zfs") {
		t.Fatal("bare storage entry must match")
	}
	if storageAllowed([]string{"pve1:local-lvm"}, "nfs") {
		t.Fatal("unlisted storage must be rejected")
	}
}
