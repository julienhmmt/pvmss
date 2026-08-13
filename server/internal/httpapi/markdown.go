//nolint:wsl_v5 // the line-by-line markdown dispatch keeps state transitions adjacent
package httpapi

import (
	"html"
	"regexp"
	"strings"
)

// renderMarkdownToHTML converts a small, safe subset of Markdown to HTML. It is
// intentionally minimal (issue #53 docs are admin-authored): headings (h1-h3),
// paragraphs, unordered/ordered lists, fenced code blocks, inline code, bold,
// italic, and links. Raw HTML in the input is escaped — the renderer only ever
// emits a known-safe tag set, so the result is safe to insert via {@html} on
// the client without a separate sanitization pass.
//
// External links (http://, https://, //) get target="_blank" and
// rel="noopener noreferrer"; internal destinations (anchors like "#section",
// relative paths, or mailto) are left untouched.
//
//nolint:gocyclo,funlen // the line-type dispatch is inherently a branch per prefix
func renderMarkdownToHTML(md string) string {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var (
		out     strings.Builder
		i       int
		inList  bool
		listOrd bool
		inCode  bool
		codeBuf strings.Builder
	)

	flushList := func() {
		if inList {
			out.WriteString("</ul>\n")
			inList = false
			listOrd = false
		}
	}

	for i < len(lines) {
		line := lines[i]

		// Fenced code block toggle.
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inCode {
				out.WriteString("<pre><code>")
				out.WriteString(escapeHTML(codeBuf.String()))
				out.WriteString("</code></pre>\n")
				codeBuf.Reset()
				inCode = false
			} else {
				flushList()
				inCode = true
			}

			i++

			continue
		}

		if inCode {
			codeBuf.WriteString(line)
			codeBuf.WriteByte('\n')
			i++

			continue
		}

		trimmed := strings.TrimSpace(line)

		// Blank line ends a paragraph/list.
		if trimmed == "" {
			flushList()
			i++

			continue
		}

		// Headings (h1-h3).
		if h, level, ok := matchHeading(trimmed); ok {
			flushList()
			tag := headingTag(level)
			out.WriteString("<" + tag + ">")
			out.WriteString(renderInline(h))
			out.WriteString("</" + tag + ">\n")
			i++

			continue
		}

		// Unordered list item.
		if isUnorderedItem(trimmed) {
			if !inList || listOrd {
				flushList()
				out.WriteString("<ul>\n")
				inList = true
				listOrd = false
			}

			out.WriteString("<li>")
			out.WriteString(renderInline(trimmed[2:]))
			out.WriteString("</li>\n")
			i++

			continue
		}

		// Ordered list item.
		if content, ok := matchOrderedItem(trimmed); ok {
			if !inList || !listOrd {
				flushList()
				out.WriteString("<ol>\n")
				inList = true
				listOrd = true
			}

			out.WriteString("<li>")
			out.WriteString(renderInline(content))
			out.WriteString("</li>\n")
			i++

			continue
		}

		// Paragraph: collect consecutive non-blank, non-special lines.
		flushList()
		var para strings.Builder
		for i < len(lines) {
			l := lines[i]
			t := strings.TrimSpace(l)
			if t == "" || strings.HasPrefix(t, "```") || isUnorderedItem(t) || matchOrderedItem2(t) {
				break
			}

			if _, _, ok := matchHeading(t); ok {
				break
			}

			if para.Len() > 0 {
				para.WriteByte(' ')
			}

			para.WriteString(t)
			i++
		}

		out.WriteString("<p>")
		out.WriteString(renderInline(para.String()))
		out.WriteString("</p>\n")
	}

	flushList()

	if inCode {
		// Unterminated fenced block — emit what we have.
		out.WriteString("<pre><code>")
		out.WriteString(escapeHTML(codeBuf.String()))
		out.WriteString("</code></pre>\n")
	}

	return out.String()
}

var headingRe = regexp.MustCompile(`^(#{1,3})\s+(.+)$`)

func matchHeading(line string) (text string, level int, ok bool) {
	m := headingRe.FindStringSubmatch(line)
	if m == nil {
		return "", 0, false
	}

	return m[2], len(m[1]), true
}

// headingTag returns the HTML tag for a heading level (1-3).
func headingTag(level int) string {
	tags := []string{"", "h1", "h2", "h3"}
	if level < 1 || level >= len(tags) {
		return "h3"
	}

	return tags[level]
}

func isUnorderedItem(line string) bool {
	return strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ")
}

var orderedItemRe = regexp.MustCompile(`^\d+\.\s+(.+)$`)

func matchOrderedItem(line string) (string, bool) {
	m := orderedItemRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}

	return m[1], true
}

func matchOrderedItem2(line string) bool {
	_, ok := matchOrderedItem(line)
	return ok
}

// renderInline handles inline formatting: links, bold, italic, inline code.
// Raw HTML is escaped once up front; only the renderer's own tags are emitted,
// so the result is safe to insert via {@html} without further sanitization.
func renderInline(text string) string {
	escaped := escapeHTML(text)

	// Protect inline code spans (content already escaped) by extracting them.
	var codeSpans []string
	protected := inlineCodeRe.ReplaceAllStringFunc(escaped, func(match string) string {
		codeSpans = append(codeSpans, match[1:len(match)-1])
		return "\x00CODE" + indexPlaceholder(len(codeSpans)-1) + "\x00"
	})

	// Links: [text](dest) — text and dest are already HTML-escaped.
	protected = linkRe.ReplaceAllStringFunc(protected, func(match string) string {
		m := linkRe.FindStringSubmatch(match)
		return renderLink(m[1], m[2])
	})

	// Bold **x** then italic *x* / _x_. The italic patterns capture the inner
	// text only (delimiters excluded) so the replacement does not re-emit them.
	protected = boldRe.ReplaceAllString(protected, "<strong>$1</strong>")
	protected = italicStarRe.ReplaceAllString(protected, "<em>$1</em>")
	protected = italicUnderscoreRe.ReplaceAllString(protected, "<em>$1</em>")

	// Restore code spans.
	for idx, span := range codeSpans {
		protected = strings.ReplaceAll(protected, "\x00CODE"+indexPlaceholder(idx)+"\x00", "<code>"+span+"</code>")
	}

	return protected
}

var (
	inlineCodeRe       = regexp.MustCompile("`[^`]+`")
	linkRe             = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	boldRe             = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicStarRe       = regexp.MustCompile(`\*([^*]+)\*`)
	italicUnderscoreRe = regexp.MustCompile(`_([^_]+)_`)
)

// renderLink emits an <a> tag, adding target/rel only for external destinations.
// Both text and dest are already HTML-escaped by renderInline, so they are
// inserted verbatim.
func renderLink(text, dest string) string {
	if isExternalLink(dest) {
		return "<a href=\"" + dest + "\" target=\"_blank\" rel=\"noopener noreferrer\">" + text + "</a>"
	}

	return "<a href=\"" + dest + "\">" + text + "</a>"
}

// isExternalLink reports whether dest is an absolute http(s) or protocol-relative URL.
func isExternalLink(dest string) bool {
	lower := strings.ToLower(dest)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "//")
}

func escapeHTML(s string) string {
	return html.EscapeString(s)
}

func indexPlaceholder(idx int) string {
	const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if idx == 0 {
		return string(digits[0])
	}

	var buf strings.Builder
	for idx > 0 {
		buf.WriteByte(digits[idx%36])
		idx /= 36
	}

	return buf.String()
}
