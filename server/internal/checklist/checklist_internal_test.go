package checklist

import "testing"

// wantAuth is the ficheDir display name for the auth prefix, asserted twice below.
const wantAuth = "auth"

// TestLabelFromFilename — the label is the filename with its .md suffix and
// leading "ID-" prefix stripped, then hyphens replaced by spaces. Covers the
// normal case, no-prefix case, multi-hyphen case, and malformed inputs.
func TestLabelFromFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"normal single word", "A01-Login.md", "Login"},
		{"multi word label", "V27-Console VNC.md", "Console VNC"},
		{"multi hyphen label", "X12-Configure SFTP cloud-init.md", "Configure SFTP cloud init"},
		{"no id prefix", "no-id-here.md", "id here"},
		{"no hyphen at all", "readme.md", "readme"},
		{"only id no label", "A01.md", "A01"},
		{"empty filename", "", ""},
		{"prefix only with trailing hyphen", "A01-.md", ""},
		{"trailing hyphen", "trailing-.md", ""},
		{"double extension stripped once", "A01-Notes.md.md", "Notes.md"},
		{"no md suffix", "A01-NoSuffix", "NoSuffix"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := labelFromFilename(tc.filename)
			if got != tc.want {
				t.Errorf("labelFromFilename(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

// TestFicheDirForID — each known prefix maps to its display name; an empty id
// or an unknown prefix returns the empty string.
func TestFicheDirForID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want string
	}{
		{"auth prefix", "A01", wantAuth},
		{"vm prefix", "V27", "vm"},
		{"admin prefix", "X01", "admin"},
		{"plateforme prefix", "P01", "plateforme"},
		{"empty id", "", ""},
		{"unknown prefix", "Z99", ""},
		{"lowercase prefix does not match", "a01", ""},
		{"single char id", "A", wantAuth},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ficheDirForID(tc.id)
			if got != tc.want {
				t.Errorf("ficheDirForID(%q) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}
