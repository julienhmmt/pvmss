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
func renderMarkdownToHTML(md string) string {
	r := &mdRenderer{lines: strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")}
	return r.render()
}

// mdRenderer carries the line-by-line rendering state, distributing the
// per-line dispatch across small helpers so each stays under the cognitive
// complexity threshold (go:S3776).
type mdRenderer struct {
	out     strings.Builder
	lines   []string
	i       int
	inList  bool
	listOrd bool
	inCode  bool
	codeBuf strings.Builder
}

// render runs the line dispatch loop and flushes any trailing state.
func (m *mdRenderer) render() string {
	for m.i < len(m.lines) {
		line := m.lines[m.i]

		if m.handleCodeFence(line) {
			continue
		}

		if m.inCode {
			m.codeBuf.WriteString(line)
			m.codeBuf.WriteByte('\n')
			m.i++

			continue
		}

		trimmed := strings.TrimSpace(line)

		// Blank line ends a paragraph/list.
		if trimmed == "" {
			m.flushList()
			m.i++

			continue
		}

		if m.handleHeading(trimmed) {
			continue
		}

		if m.handleListItem(trimmed) {
			continue
		}

		// Paragraph: collect consecutive non-blank, non-special lines.
		m.collectParagraph()
	}

	m.flushList()

	if m.inCode {
		// Unterminated fenced block — emit what we have.
		m.emitCodeBlock()
	}

	return m.out.String()
}

// flushList closes an open unordered list, resetting the list state.
func (m *mdRenderer) flushList() {
	if m.inList {
		m.out.WriteString("</ul>\n")
		m.inList = false
		m.listOrd = false
	}
}

// handleCodeFence toggles a fenced code block on a ``` line, emitting the
// accumulated buffer when closing. Returns true when the line was consumed.
func (m *mdRenderer) handleCodeFence(line string) bool {
	if !strings.HasPrefix(strings.TrimSpace(line), "```") {
		return false
	}

	if m.inCode {
		m.emitCodeBlock()
		m.codeBuf.Reset()
		m.inCode = false
	} else {
		m.flushList()
		m.inCode = true
	}

	m.i++

	return true
}

// emitCodeBlock writes the escaped, accumulated code buffer as a pre/code block.
func (m *mdRenderer) emitCodeBlock() {
	m.out.WriteString("<pre><code>")
	m.out.WriteString(escapeHTML(m.codeBuf.String()))
	m.out.WriteString("</code></pre>\n")
}

// handleHeading emits an h1-h3 heading when the line matches. Returns true when
// the line was consumed.
func (m *mdRenderer) handleHeading(trimmed string) bool {
	h, level, ok := matchHeading(trimmed)
	if !ok {
		return false
	}

	m.flushList()
	tag := headingTag(level)
	m.out.WriteString("<" + tag + ">")
	m.out.WriteString(renderInline(h))
	m.out.WriteString("</" + tag + ">\n")
	m.i++

	return true
}

// handleListItem emits an unordered or ordered list item, opening the matching
// list when the list kind changes. Returns true when the line was consumed.
func (m *mdRenderer) handleListItem(trimmed string) bool {
	if isUnorderedItem(trimmed) {
		m.openList(false)
		m.out.WriteString("<li>")
		m.out.WriteString(renderInline(trimmed[2:]))
		m.out.WriteString("</li>\n")
		m.i++

		return true
	}

	content, ok := matchOrderedItem(trimmed)
	if !ok {
		return false
	}

	m.openList(true)
	m.out.WriteString("<li>")
	m.out.WriteString(renderInline(content))
	m.out.WriteString("</li>\n")
	m.i++

	return true
}

// openList starts a list of the requested kind, closing the previous list first
// when the kind differs or no list is open.
func (m *mdRenderer) openList(ordered bool) {
	if m.inList && m.listOrd == ordered {
		return
	}

	m.flushList()
	if ordered {
		m.out.WriteString("<ol>\n")
	} else {
		m.out.WriteString("<ul>\n")
	}

	m.inList = true
	m.listOrd = ordered
}

// collectParagraph gathers consecutive non-blank, non-special lines into one
// paragraph, joining them with single spaces.
func (m *mdRenderer) collectParagraph() {
	m.flushList()
	var para strings.Builder
	for m.i < len(m.lines) {
		l := m.lines[m.i]
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
		m.i++
	}

	m.out.WriteString("<p>")
	m.out.WriteString(renderInline(para.String()))
	m.out.WriteString("</p>\n")
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
