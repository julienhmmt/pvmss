//nolint:wsl_v5 // the line-by-line markdown dispatch keeps state transitions adjacent
package httpapi

import (
	"html"
	"net/url"
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
	out        strings.Builder
	lines      []string
	i          int
	inList     bool
	listOrd    bool
	inListItem bool
	inCode     bool
	codeBuf    strings.Builder
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

		if m.continueListItem(trimmed) {
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

// flushList closes an open list item and its list, resetting the list state.
func (m *mdRenderer) flushList() {
	m.closeListItem()

	if m.inList {
		if m.listOrd {
			m.out.WriteString("</ol>\n")
		} else {
			m.out.WriteString("</ul>\n")
		}

		m.inList = false
		m.listOrd = false
	}
}

// closeListItem closes the current list item if one is open.
func (m *mdRenderer) closeListItem() {
	if m.inListItem {
		m.out.WriteString("</li>\n")
		m.inListItem = false
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
// list when the list kind changes. The item is left open so continuation lines
// can be appended. Returns true when the line was consumed.
func (m *mdRenderer) handleListItem(trimmed string) bool {
	if isUnorderedItem(trimmed) {
		m.closeListItem()
		m.openList(false)
		m.inListItem = true
		m.out.WriteString("<li>")
		m.out.WriteString(renderInline(trimmed[2:]))
		m.i++

		return true
	}

	content, ok := matchOrderedItem(trimmed)
	if !ok {
		return false
	}

	m.closeListItem()
	m.openList(true)
	m.inListItem = true
	m.out.WriteString("<li>")
	m.out.WriteString(renderInline(content))
	m.i++

	return true
}

// continueListItem appends a continuation line to the open list item. It is
// reached after code fences, blank lines, headings and list items have already
// been handled, so the remaining lines are treated as part of the current item.
func (m *mdRenderer) continueListItem(trimmed string) bool {
	if !m.inListItem {
		return false
	}

	m.out.WriteByte(' ')
	m.out.WriteString(renderInline(trimmed))
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
	protected = renderLinks(protected)

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
	boldRe             = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicStarRe       = regexp.MustCompile(`\*([^*]+)\*`)
	italicUnderscoreRe = regexp.MustCompile(`_([^_]+)_`)
)

// renderLink emits an <a> tag for allowed URL schemes. External http(s) links
// get target="_blank" and rel="noopener noreferrer"; mailto and relative paths
// do not. Dangerous schemes (javascript:, data:, vbscript:, etc.) and
// protocol-relative references are rendered as plain text so they cannot be
// activated.
//
// Both text and dest are already HTML-escaped by renderInline, so they are
// inserted verbatim when a link is emitted.
func renderLink(text, dest string) string {
	safe, external := safeURL(dest)
	if !safe {
		return text
	}

	if external {
		return "<a href=\"" + dest + "\" target=\"_blank\" rel=\"noopener noreferrer\">" + text + "</a>"
	}

	return "<a href=\"" + dest + "\">" + text + "</a>"
}

// safeURL parses a link destination and classifies it as safe or unsafe. It
// returns safe=true for http:, https:, mailto:, and relative paths; it rejects
// javascript:, data:, vbscript:, and mixed-case/whitespace variants.
// external is true only for http and https destinations.
func safeURL(dest string) (safe, external bool) {
	u, err := url.Parse(strings.TrimSpace(dest))
	if err != nil || u == nil {
		return false, false
	}

	if u.Scheme == "" {
		// Reject protocol-relative references like //example.com because the
		// browser resolves them against the current scheme and they are not
		// relative paths.
		if u.Host != "" {
			return false, false
		}

		return true, false
	}

	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return true, true
	case "mailto":
		return true, false
	default:
		return false, false
	}
}

// renderLinks scans s for [text](dest) links and emits them through renderLink.
// It handles URLs that contain their own parentheses by tracking the paren
// balance, so [click](javascript:alert(1)) is parsed as a single link.
func renderLinks(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '[' {
			out.WriteByte(s[i])
			i++

			continue
		}

		next, text, dest, ok := extractLink(s, i)
		if !ok {
			out.WriteByte(s[i])
			i++

			continue
		}

		out.WriteString(renderLink(text, dest))
		i = next
	}

	return out.String()
}

// extractLink attempts to parse a Markdown link starting at s[start] where
// s[start] == '['. It returns the index just after the closing ')', the link
// text, the destination, and ok=true on success.
func extractLink(s string, start int) (next int, text, dest string, ok bool) {
	if s[start] != '[' {
		return 0, "", "", false
	}

	// Find the closing ']' for the link text.
	closeBracket := strings.IndexByte(s[start+1:], ']')
	if closeBracket == -1 {
		return 0, "", "", false
	}

	textEnd := start + 1 + closeBracket
	if textEnd+1 >= len(s) || s[textEnd+1] != '(' {
		return 0, "", "", false
	}

	destStart := textEnd + 2
	parenCount := 0
	for j := destStart; j < len(s); j++ {
		c := s[j]
		if c == '(' {
			parenCount++

			continue
		}

		if c == ')' {
			if parenCount > 0 {
				parenCount--

				continue
			}

			text = s[start+1 : textEnd]
			dest = s[destStart:j]

			return j + 1, text, dest, true
		}
	}

	return 0, "", "", false
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
