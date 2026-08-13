//nolint:wsl_v5 // test scaffolding keeps setup and assertions adjacent
package httpapi

import (
	"strings"
	"testing"
)

// TestRenderMarkdownToHTML_ExternalLinkGetsTargetBlank — external links get
// target="_blank" + rel="noopener noreferrer"; internal anchors do not.
func TestRenderMarkdownToHTML_ExternalLinkGetsTargetBlank(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		md      string
		want    string
		notWant string
	}{
		{
			name: "external https link",
			md:   "[docs](https://example.com/x)",
			want: `target="_blank" rel="noopener noreferrer"`,
		},
		{
			name: "external http link",
			md:   "[docs](http://example.com/x)",
			want: `target="_blank" rel="noopener noreferrer"`,
		},
		{
			name:    "internal anchor untouched",
			md:      "[section](#section)",
			notWant: `target="_blank"`,
		},
		{
			name:    "relative path untouched",
			md:      "[other](/docs/other)",
			notWant: `target="_blank"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderMarkdownToHTML(tc.md)
			if tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatalf("output %q missing %q", got, tc.want)
			}

			if tc.notWant != "" && strings.Contains(got, tc.notWant) {
				t.Fatalf("output %q should not contain %q", got, tc.notWant)
			}
		})
	}
}

// TestRenderMarkdownToHTML_RawHTMLEscaped — raw HTML in the input is escaped,
// never passed through (XSS defense at the source).
func TestRenderMarkdownToHTML_RawHTMLEscaped(t *testing.T) {
	t.Parallel()
	got := renderMarkdownToHTML("<script>alert(1)</script>")
	if strings.Contains(got, "<script>") {
		t.Fatalf("raw <script> leaked into output: %q", got)
	}

	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("expected escaped script tag, got %q", got)
	}
}

// TestRenderMarkdownToHTML_BasicElements — headings, lists, code, emphasis.
func TestRenderMarkdownToHTML_BasicElements(t *testing.T) {
	t.Parallel()
	md := "# Title\n\n- one\n- two\n\nA **bold** and *italic* and `code`.\n\n```\ncode block\n```"
	got := renderMarkdownToHTML(md)

	checks := []string{"<h1>", "<ul>", "<li>one</li>", "<strong>bold</strong>", "<em>italic</em>", "<code>code</code>", "<pre><code>"}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Fatalf("output missing %q: %s", c, got)
		}
	}
}
