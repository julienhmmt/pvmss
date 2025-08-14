// Package templates provides template functions and utilities for the PVMSS application
package templates

import (
	"html/template"
	"net/http"
)

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
		"sort":        sortSlice,
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
		"humanBytes": formatBytes,

		// Math functions
		"add":      addNumbers,
		"subtract": subtractNumbers,
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
			// Normalize trailing slashes except root
			norm := func(s string) string {
				if s == "" {
					return "/"
				}
				if s != "/" && s[len(s)-1] == '/' {
					return s[:len(s)-1]
				}
				return s
			}
			return norm(a) == norm(b)
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
	}

	return funcMap
}
