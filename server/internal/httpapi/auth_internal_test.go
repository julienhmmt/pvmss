package httpapi

import "testing"

func TestWithPoolPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		username string
		want     string
		wantOK   bool
	}{
		{name: "bare pool user", username: "jho@pve", want: "pvmss-jho@pve", wantOK: true},
		{name: "already prefixed", username: "pvmss-jho@pve", want: "", wantOK: false},
		{name: "other realm preserved", username: "jho@pam", want: "pvmss-jho@pam", wantOK: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := withPoolPrefix(tc.username)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("withPoolPrefix(%q) = (%q, %v), want (%q, %v)", tc.username, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
