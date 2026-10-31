package httpapi

import "testing"

func TestCSVSafe_NeutralisesFormulaPrefixes(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"=cmd()":   "'=cmd()",
		"+1":       "'+1",
		"-1":       "'-1",
		"@SUM(A1)": "'@SUM(A1)",
		"\tx":      "'\tx",
		"bob@pve":  "bob@pve",
		"":         "",
	}
	for in, want := range cases {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}
