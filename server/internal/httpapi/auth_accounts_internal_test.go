package httpapi

import "testing"

const (
	normalizedPVEUser = "alice@pve"
	normalizedPAMUser = "root@pam"
)

func TestNormalizePVEUsername(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		username string
		realm    string
		want     string
		wantErr  bool
	}{
		{name: "bare defaults to pve", username: "alice", realm: "", want: normalizedPVEUser},
		{name: "bare explicit pve", username: "alice", realm: "pve", want: normalizedPVEUser},
		{name: "bare pam", username: "root", realm: "pam", want: normalizedPAMUser},
		{name: "typed realm wins", username: normalizedPVEUser, realm: "", want: normalizedPVEUser},
		{name: "typed pam with matching realm", username: normalizedPAMUser, realm: "pam", want: normalizedPAMUser},
		{name: "typed realm case-insensitive match", username: normalizedPAMUser, realm: "PAM", want: normalizedPAMUser},
		{name: "typed realm contradicting selector", username: normalizedPAMUser, realm: "pve", wantErr: true},
		{name: "oidc realm refused", username: "alice", realm: "oidc", wantErr: true},
		{name: "typed oidc realm refused", username: "alice@oidc", realm: "", want: "alice@oidc"},
		{name: "empty local part refused", username: "@pve", realm: "", wantErr: true},
		{name: "empty realm suffix refused", username: "alice@", realm: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizePVEUsername(tc.username, tc.realm)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizePVEUsername(%q, %q) = %q, want error", tc.username, tc.realm, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("normalizePVEUsername(%q, %q) error = %v", tc.username, tc.realm, err)
			}

			if got != tc.want {
				t.Fatalf("normalizePVEUsername(%q, %q) = %q, want %q", tc.username, tc.realm, got, tc.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	if err := validatePassword("1234567"); err == nil {
		t.Error("7-character password accepted")
	}

	if err := validatePassword("12345678"); err != nil {
		t.Errorf("8-character password rejected: %v", err)
	}

	long := make([]byte, maxPasswordLength+1)
	for i := range long {
		long[i] = 'a'
	}

	if err := validatePassword(string(long)); err == nil {
		t.Error("73-byte password accepted")
	}

	exact := make([]byte, maxPasswordLength)
	for i := range exact {
		exact[i] = 'a'
	}

	if err := validatePassword(string(exact)); err != nil {
		t.Errorf("72-byte password rejected: %v", err)
	}
}

func TestValidateLocalUsername(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "albert", want: "albert"},
		{raw: "  Albert  ", want: "albert"},
		{raw: "ops.team-2", want: "ops.team-2"},
		{raw: "admin", wantErr: true},
		{raw: "ADMIN", wantErr: true},
		{raw: "ab", wantErr: true},
		{raw: "alice@pve", wantErr: true},
		{raw: "-alice", wantErr: true},
		{raw: "alice-", wantErr: true},
		{raw: "ali ce", wantErr: true},
	}

	for _, tc := range cases {
		got, err := validateLocalUsername(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("validateLocalUsername(%q) = %q, want error", tc.raw, got)
			}

			continue
		}

		if err != nil {
			t.Errorf("validateLocalUsername(%q) error = %v", tc.raw, err)
			continue
		}

		if got != tc.want {
			t.Errorf("validateLocalUsername(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
