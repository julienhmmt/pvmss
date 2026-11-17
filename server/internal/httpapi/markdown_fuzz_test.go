package httpapi

import (
	"html"
	"regexp"
	"strings"
	"testing"
)

// allowedTagRe lists every tag renderMarkdownToHTML may emit; any other "<"
// in the output means user input escaped into markup.
var allowedTagRe = regexp.MustCompile(`^<(/?(p|ul|ol|li|strong|em|code|pre|h[1-6]|a)|a href="[^"<>]*"( target="_blank" rel="noopener noreferrer")?)>`)

var hrefRe = regexp.MustCompile(`<a href="([^"]*)"`)

func assertSafeMarkdownHTML(t *testing.T, in, out string) {
	t.Helper()

	for i := strings.IndexByte(out, '<'); i >= 0; {
		if !allowedTagRe.MatchString(out[i:]) {
			t.Fatalf("unexpected markup for %q: %q", in, out)
		}

		next := strings.IndexByte(out[i+1:], '<')
		if next < 0 {
			break
		}

		i += next + 1
	}

	for _, m := range hrefRe.FindAllStringSubmatch(out, -1) {
		if dangerousHref(html.UnescapeString(m[1])) {
			t.Fatalf("unsafe href %q for %q", m[1], in)
		}
	}
}

// dangerousHref mimics browser URL parsing: strip what browsers strip, then look for an executable scheme or a protocol-relative host.
func dangerousHref(href string) bool {
	// Browsers drop tab/CR/LF anywhere and trim leading/trailing C0 + space.
	clean := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}

		return r
	}, strings.ToLower(href))
	clean = strings.TrimLeft(clean, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f ")
	clean = strings.ReplaceAll(clean, `\`, "/")

	for _, p := range []string{"javascript:", "data:", "vbscript:", "//"} {
		if strings.HasPrefix(clean, p) {
			return true
		}
	}

	return false
}

func FuzzRenderMarkdownToHTML(f *testing.F) {
	for _, seed := range []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror=alert(1)>`,
		`[x](javascript:alert(1))`,
		`[x](JaVaScRiPt:alert(1))`,
		"[x](java\tscript:alert(1))",
		`[x](&#106;avascript:alert(1))`,
		`[x](data:text/html,<script>alert(1)</script>)`,
		`[x](//evil.example)`,
		`[x](///evil.example)`,
		`[x](/\\evil.example)`,
		`[x](" onmouseover="alert(1))`,
		`[x](https://a.example/" onmouseover="alert(1))`,
		"[`x`](`\" onclick=alert(1)`)",
		`**[a](https://x.example/**b**)**`,
		"```\n</pre><script>alert(1)</script>\n```",
		"# <svg onload=alert(1)>",
		"- [x](vbscript:msgbox(1))",
		"_[a](https://x/_y_)_",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		assertSafeMarkdownHTML(t, in, renderMarkdownToHTML(in))
	})
}
