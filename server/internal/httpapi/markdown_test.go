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

// TestRenderMarkdownToHTML_OrderedListAndKindSwitch — ordered lists render as
// <ol>, and switching from unordered to ordered reopens the list.
func TestRenderMarkdownToHTML_OrderedListAndKindSwitch(t *testing.T) {
	t.Parallel()
	md := "- a\n- b\n1. c\n2. d"
	got := renderMarkdownToHTML(md)

	if !strings.Contains(got, "<ul>") {
		t.Fatalf("missing <ul>: %s", got)
	}

	if !strings.Contains(got, "<ol>") {
		t.Fatalf("missing <ol>: %s", got)
	}

	if !strings.Contains(got, "<li>c</li>") || !strings.Contains(got, "<li>d</li>") {
		t.Fatalf("missing ordered items: %s", got)
	}
}

// TestRenderMarkdownToHTML_StarBullet — the "* " prefix is also an unordered
// list item.
func TestRenderMarkdownToHTML_StarBullet(t *testing.T) {
	t.Parallel()
	got := renderMarkdownToHTML("* star item")
	if !strings.Contains(got, "<ul>") || !strings.Contains(got, "<li>star item</li>") {
		t.Fatalf("star bullet not rendered: %s", got)
	}
}

// TestRenderMarkdownToHTML_UnterminatedCodeBlock — a fenced block without a
// closing fence still emits its accumulated content.
func TestRenderMarkdownToHTML_UnterminatedCodeBlock(t *testing.T) {
	t.Parallel()
	got := renderMarkdownToHTML("```\nunfinished code")
	if !strings.Contains(got, "<pre><code>") || !strings.Contains(got, "unfinished code") {
		t.Fatalf("unterminated code block not emitted: %s", got)
	}
}

// TestRenderMarkdownToHTML_ParagraphBreaksOnSpecialLines — a paragraph stops at
// a following heading, list item, or code fence.
func TestRenderMarkdownToHTML_ParagraphBreaksOnSpecialLines(t *testing.T) {
	t.Parallel()
	md := "Intro line\n# Heading\n- item\n```\ncode\n```"
	got := renderMarkdownToHTML(md)

	if !strings.Contains(got, "<p>Intro line</p>") {
		t.Fatalf("paragraph not isolated: %s", got)
	}

	if !strings.Contains(got, "<h1>Heading</h1>") {
		t.Fatalf("heading after paragraph missing: %s", got)
	}
}

// TestRenderMarkdownToHTML_EmptyAndCRLF — empty input and CRLF line endings are
// handled without panics or stray tags.
func TestRenderMarkdownToHTML_EmptyAndCRLF(t *testing.T) {
	t.Parallel()
	if got := renderMarkdownToHTML(""); got != "" {
		t.Fatalf("empty input = %q, want empty", got)
	}

	got := renderMarkdownToHTML("line one\r\nline two\r\n")
	if !strings.Contains(got, "<p>line one line two</p>") {
		t.Fatalf("CRLF not normalized: %s", got)
	}
}

// TestRenderMarkdownToHTML_MultipleInlineCodeSpans — two code spans exercise the
// multi-index placeholder restore path in renderInline.
func TestRenderMarkdownToHTML_MultipleInlineCodeSpans(t *testing.T) {
	t.Parallel()
	got := renderMarkdownToHTML("Use `foo` and `bar` together.")
	if !strings.Contains(got, "<code>foo</code>") || !strings.Contains(got, "<code>bar</code>") {
		t.Fatalf("code spans not restored: %s", got)
	}
}

// TestHeadingTag_OutOfRange — levels outside 1-3 clamp to h3.
func TestHeadingTag_OutOfRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level int
		want  string
	}{
		{0, "h3"},
		{4, "h3"},
		{1, "h1"},
		{2, "h2"},
		{3, "h3"},
	}
	for _, tc := range cases {
		if got := headingTag(tc.level); got != tc.want {
			t.Errorf("headingTag(%d) = %q, want %q", tc.level, got, tc.want)
		}
	}
}

// TestMatchOrderedItem_NoMatch — a non-list line returns no match.
func TestMatchOrderedItem_NoMatch(t *testing.T) {
	t.Parallel()
	if content, ok := matchOrderedItem("not a list item"); ok || content != "" {
		t.Fatalf("matchOrderedItem(non-list) = %q, %v, want empty false", content, ok)
	}
}

// TestIndexPlaceholder_MultiDigit — indices above 0 exercise the base-36 loop.
func TestIndexPlaceholder_MultiDigit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		idx  int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{35, "Z"},
	}
	for _, tc := range cases {
		if got := indexPlaceholder(tc.idx); got != tc.want {
			t.Errorf("indexPlaceholder(%d) = %q, want %q", tc.idx, got, tc.want)
		}
	}
}

// TestRenderMarkdownToHTML_DangerousURLSchemesRejected — the renderer escapes
// or drops dangerous URLs while keeping http(s) links safe.
func TestRenderMarkdownToHTML_DangerousURLSchemesRejected(t *testing.T) {
	t.Parallel()

	const noAnchor = "<a href"

	cases := []struct {
		name    string
		md      string
		want    string
		notWant string
	}{
		{
			name:    "javascript scheme rejected",
			md:      "[click](javascript:alert(1))",
			want:    "<p>click</p>",
			notWant: noAnchor,
		},
		{
			name:    "mixed-case javascript scheme rejected",
			md:      "[click](JavaScript:alert(1))",
			want:    "<p>click</p>",
			notWant: noAnchor,
		},
		{
			name:    "raw javascript not rendered as a link",
			md:      "javascript:alert(1)",
			notWant: noAnchor,
		},
		{
			name:    "data scheme rejected",
			md:      "[open](data:text/html,<script>alert(1)</script>)",
			want:    "<p>open</p>",
			notWant: noAnchor,
		},
		{
			name:    "vbscript scheme rejected",
			md:      "[run](vbscript:msgbox(1))",
			want:    "<p>run</p>",
			notWant: noAnchor,
		},
		{
			name:    "raw img tag escaped",
			md:      `<img src=x onerror=alert(1)>`,
			want:    "&lt;img",
			notWant: `<img`,
		},
		{
			name: "valid https link gets target and rel",
			md:   "[docs](https://example.com)",
			want: `<a href="https://example.com" target="_blank" rel="noopener noreferrer">docs</a>`,
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
