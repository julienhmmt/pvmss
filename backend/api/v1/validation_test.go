package apiv1

import "testing"

func TestValidateTagName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "webserver", false},
		{"digits", "web01", false},
		{"hyphen_underscore", "web-server_01", false},
		{"max_length", repeat("a", 50), false},
		{"empty", "", true},
		{"too_long", repeat("a", 51), true},
		{"slash", "web/server", true},
		{"dotdot", "..", true},
		{"space", "web server", true},
		{"colon", "web:server", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTagName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateTagName(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePoolName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "alice", false},
		{"with_prefix", "pvmss-alice", false},
		{"digits_underscore", "team_01", false},
		{"max_length", repeat("a", 50), false},
		{"empty", "", true},
		{"too_long", repeat("a", 51), true},
		{"slash", "alice/../root", true},
		{"space", "alice bob", true},
		{"at_sign", "alice@pve", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePoolName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validatePoolName(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for range n {
		out = append(out, s...)
	}
	return string(out)
}
