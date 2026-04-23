package proxmox

import "testing"

func TestParseTagStyle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    map[string]TagColor
	}{
		{
			name:  "empty",
			input: "",
			want:  map[string]TagColor{},
		},
		{
			name:  "no color-map",
			input: "shape=full,ordering=config",
			want:  map[string]TagColor{},
		},
		{
			name:  "single bg only",
			input: "color-map=production:ff0000",
			want: map[string]TagColor{
				"production": {Background: "ff0000"},
			},
		},
		{
			name:  "bg and text",
			input: "color-map=prod:112233:ffffff",
			want: map[string]TagColor{
				"prod": {Background: "112233", Text: "ffffff"},
			},
		},
		{
			name:  "multiple entries",
			input: "shape=full,color-map=a:aabbcc;b:ddeeff:000000;c:112233",
			want: map[string]TagColor{
				"a": {Background: "aabbcc"},
				"b": {Background: "ddeeff", Text: "000000"},
				"c": {Background: "112233"},
			},
		},
		{
			name:  "hash prefix stripped and lowercased",
			input: "color-map=dev:#AABBCC:#FF0000",
			want: map[string]TagColor{
				"dev": {Background: "aabbcc", Text: "ff0000"},
			},
		},
		{
			name:  "ignores empty entries and invalid rows",
			input: "color-map=;;valid:abcdef;:bad;onlyname",
			want: map[string]TagColor{
				"valid": {Background: "abcdef"},
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseTagStyle(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %d (%v), want %d (%v)", len(got), got, len(tc.want), tc.want)
			}
			for k, v := range tc.want {
				gv, ok := got[k]
				if !ok {
					t.Errorf("missing key %q in result", k)
					continue
				}
				if gv != v {
					t.Errorf("key %q: got %+v, want %+v", k, gv, v)
				}
			}
		})
	}
}
