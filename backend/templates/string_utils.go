package templates

import (
	"fmt"
	"html/template"
	"reflect"
	"strings"
)

// toUpper converts a string to its uppercase equivalent.
func toUpper(s string) string {
	return strings.ToUpper(s)
}

// toLower converts a string to its lowercase equivalent.
func toLower(s string) string {
	return strings.ToLower(s)
}

// truncateString truncates a string to a specified length.
// If the string is longer than the given length, it is shortened and an ellipsis ("...") is appended.
// The function is UTF-8 safe.
func truncateString(s string, length int) string {
	if length <= 0 {
		return ""
	}
	// UTF-8 safe truncation via runes
	r := []rune(s)
	if len(r) <= length {
		return s
	}
	return string(r[:length]) + "..."
}

// join concatenates the elements of a slice into a single string, separated by a given separator.
// It accepts a slice of any type, converting each element to its string representation.
func join(parts interface{}, sep string) string {
	v := reflect.ValueOf(parts)
	if v.Kind() != reflect.Slice {
		return "" // Return empty string if not a slice, for template safety.
	}

	length := v.Len()
	if length == 0 {
		return ""
	}

	// Pre-allocate with exact capacity and use direct indexing
	strParts := make([]string, length)
	for i := 0; i < length; i++ {
		strParts[i] = fmt.Sprintf("%v", v.Index(i).Interface())
	}

	return strings.Join(strParts, sep)
}

// basename extracts the last component of a path (after the last slash or colon)
func basename(s string) string {
	// Find the last slash or colon
	lastSlash := strings.LastIndex(s, "/")
	lastColon := strings.LastIndex(s, ":")
	var lastSep int
	if lastSlash > lastColon {
		lastSep = lastSlash
	} else {
		lastSep = lastColon
	}

	if lastSep == -1 {
		return s
	}
	return s[lastSep+1:]
}

// startsWith checks if a string starts with a given prefix
func startsWith(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

// eqPath compares two paths for equality, ignoring trailing slashes
func eqPath(a, b string) bool {
	return normalizePath(a) == normalizePath(b)
}

// activeFor returns true when path matches base exactly (ignoring trailing slash)
// or when path is a subpath of base (e.g., /admin/iso/toggle).
func activeFor(path, base string) bool {
	p := normalizePath(path)
	b := normalizePath(base)
	if p == b {
		return true
	}
	if b == "/" {
		// Any non-root path is a subpath of the root
		return true
	}
	return strings.HasPrefix(p, b+"/")
}

// safeHTML marks a string as safe HTML to prevent auto-escaping
// Use with caution - only for trusted content
func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

// safeHTMLAttr marks a string as a safe HTML attribute
// Use with caution - only for trusted content
func safeHTMLAttr(s string) template.HTMLAttr {
	return template.HTMLAttr(s)
}
