// Package templates provides template functions and utilities for the PVMSS application
package templates

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"pvmss/i18n"
)

// normalizePath ensures a path has a consistent format, removing trailing slashes
// except for the root path, which is always "/".
func normalizePath(s string) string {
	if s == "" {
		return "/"
	}
	if len(s) > 1 && s[len(s)-1] == '/' {
		return s[:len(s)-1]
	}
	return s
}

// GetBaseFuncMap returns functions that don't depend on the request
func GetBaseFuncMap() template.FuncMap {
	return template.FuncMap{
		// Conversion functions
		"int":     convertToInt,
		"string":  convertToString,
		"float64": convertToFloat64,

		// Memory and formatting functions
		"formatMemory":  formatMemory,
		"formatBytes":   formatBytes,
		"formatBytesSI": formatBytesSI,
		"formatGiB":     formatGiB,

		// Collection functions
		"sort": func(slice interface{}) (interface{}, error) {
			return sortSlice(slice)
		},
		"reverse":     reverseSlice,
		"contains":    containsValue,
		"length":      getLength,
		"until":       until,
		"sortStrings": sortStrings,
		"sortInts":    sortInts,
		"seq":         seq,

		// String functions
		"upper":      toUpper,
		"lower":      toLower,
		"truncate":   truncateString,
		"join":       join,
		"split":      strings.Split,
		"humanBytes": formatBytes,

		// Math functions
		"add":      addNumbers,
		"subtract": subtractNumbers,
		"sub":      subtractNumbers,
		"multiply": multiplyNumbers,
		"mul":      multiplyNumbers,
		"divide":   divideNumbers,
		"div":      divideNumbers,

		// Utility functions
		"default":  defaultValue,
		"empty":    isEmpty,
		"notEmpty": isNotEmpty,
		"coalesce": coalesce,

		// Utility functions that don't depend on the request
		"formatDuration": formatDuration,
		"since":          since,
		"untilTime":      untilTime,
		"dateFormat":     dateFormat,
		"dateRFC3339":    dateRFC3339,
		"dateISO8601":    dateISO8601,
		"dateShort":      dateShort,
		"dateTimeShort":  dateTimeShort,
		"toJSON":         toJSON,
		"toJSONIndent":   toJSONIndent,

		// Request-agnostic fallbacks (overridden by GetFuncMap when request is available)
		"currentPath": func() string { return "/" },
		"urlWithLang": func(lang string) string { return "/?lang=" + lang },

		// Path/string helpers
		"eqPath": func(a, b string) bool {
			return normalizePath(a) == normalizePath(b)
		},
		// activeFor returns true when path matches base exactly (ignoring trailing slash)
		// or when path is a subpath of base (e.g., /admin/iso/toggle).
		"activeFor": func(path, base string) bool {
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
		},
		"basename": func(s string) string {
			// Find the last slash or colon
			lastSlash := strings.LastIndex(s, "/")
			lastColon := strings.LastIndex(s, ":")
			lastSep := -1
			if lastSlash > lastColon {
				lastSep = lastSlash
			} else {
				lastSep = lastColon
			}

			if lastSep == -1 {
				return s
			}
			return s[lastSep+1:]
		},
		"startsWith": func(s, prefix string) bool {
			if len(prefix) == 0 {
				return true
			}
			if len(s) < len(prefix) {
				return false
			}
			return s[:len(prefix)] == prefix
		},
		
		// Template helper functions for creating maps and slices
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of arguments")
			}
			result := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				result[key] = values[i+1]
			}
			return result, nil
		},
		"slice": func(values ...interface{}) []interface{} {
			return values
		},
		"printf": fmt.Sprintf,
	}
}

// GetFuncMap returns a function map with request-aware functions
func GetFuncMap(r *http.Request) template.FuncMap {
	// Start with base functions
	funcMap := GetBaseFuncMap()

	// Add request-aware functions if request is provided
	if r != nil {
		// Add CSRF token functions
		funcMap["csrfToken"] = func() template.HTML {
			return csrfToken(r)
		}

		funcMap["csrfMeta"] = func() template.HTML {
			return csrfMeta(r)
		}

		// Add request info functions
		funcMap["isHTTPS"] = func() bool { return isHTTPS(r) }
		funcMap["host"] = func() string { return getHost(r) }

		// Path helpers
		funcMap["currentPath"] = func() string {
			if r.URL == nil {
				return "/"
			}
			if r.URL.Path == "" {
				return "/"
			}
			return r.URL.Path
		}

		// Build current URL while overriding only the lang parameter
		funcMap["urlWithLang"] = func(lang string) string {
			if r.URL == nil {
				return "/?lang=" + lang
			}
			u := *r.URL // shallow copy
			q := u.Query()
			q.Set("lang", lang)
			u.RawQuery = q.Encode()
			if u.RawQuery == "" {
				return u.Path
			}
			return u.Path + "?" + u.RawQuery
		}

		// Add translation function
		funcMap["T"] = func(key string, args ...interface{}) string {
			localizer := i18n.GetLocalizerFromRequest(r)

			var count []int
			if len(args) > 0 {
				// The template engine might pass numbers as int, int64 or float64
				switch c := args[0].(type) {
				case int:
					count = append(count, c)
				case int64:
					count = append(count, int(c))
				case float64:
					count = append(count, int(c))
				}
			}

			return i18n.Localize(localizer, key, count...)
		}
	}

	return funcMap
}
