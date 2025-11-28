// Package templates provides template functions and utilities for the PVMSS application
package templates

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"pvmss/constants"
	"pvmss/i18n"
	"pvmss/utils"
)

// stringsPkg provides namespace-style access to string functions in templates
type stringsPkg struct{}

func (sp *stringsPkg) Contains(s1, s2 string) bool        { return strings.Contains(s1, s2) }
func (sp *stringsPkg) HasPrefix(s1, s2 string) bool       { return strings.HasPrefix(s1, s2) }
func (sp *stringsPkg) HasSuffix(s1, s2 string) bool       { return strings.HasSuffix(s1, s2) }
func (sp *stringsPkg) Trim(s1, cutset string) string      { return strings.Trim(s1, cutset) }
func (sp *stringsPkg) TrimSpace(s string) string          { return strings.TrimSpace(s) }
func (sp *stringsPkg) Split(s, sep string) []string       { return strings.Split(s, sep) }
func (sp *stringsPkg) Join(a []string, sep string) string { return strings.Join(a, sep) }

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
		"FormatMemoryGB": func(value int64, isBytes bool) string {
			// Import the function from handlers/helpers.go
			var memoryMB int64
			if isBytes {
				memoryMB = value / (1024 * 1024)
			} else {
				memoryMB = value
			}
			memoryGB := float64(memoryMB) / 1024.0
			if memoryGB >= 1 {
				if memoryGB == float64(int64(memoryGB)) {
					return fmt.Sprintf("%d GB", int64(memoryGB))
				}
				return fmt.Sprintf("%.1f GB", memoryGB)
			}
			if memoryMB == int64(memoryMB) {
				return fmt.Sprintf("%d MB", memoryMB)
			}
			return fmt.Sprintf("%.0f MB", float64(memoryMB))
		},

		// Collection functions
		"sort": func(slice interface{}) (interface{}, error) {
			return sortSlice(slice)
		},
		"reverse":  reverseSlice,
		"hasValue": containsValue,
		"sliceContains": func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		},
		"length":      getLength,
		"sortStrings": sortStrings,
		"sortInts":    sortInts,
		"seq":         seq,

		// String functions
		"upper":      toUpper,
		"lower":      toLower,
		"truncate":   truncateString,
		"join":       join,
		"split":      strings.Split,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"trim":       strings.Trim,
		"trimSpace":  strings.TrimSpace,
		"strings":    func() *stringsPkg { return &stringsPkg{} },
		"humanBytes": formatBytes,

		// Math functions
		"add":      addNumbers,
		"addInt":   addInt,
		"subtract": subtractNumbers,
		"sub":      subtractNumbers,
		"multiply": multiplyNumbers,
		"mul":      multiplyNumbers,
		"divide":   divideNumbers,
		"div":      divideNumbers,
		"iterate":  iterate,
		"toFloat":  toFloat,
		"toInt":    toInt,

		// Utility functions
		"default":      defaultValue,
		"empty":        isEmpty,
		"notEmpty":     isNotEmpty,
		"coalesce":     coalesce,
		"safeHTML":     safeHTML,
		"safeHTMLAttr": safeHTMLAttr,

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

		// Application version
		"appVersion": func() string { return constants.AppVersion },

		// Time functions
		"now": func() time.Time { return time.Now().In(utils.GetAppLocation()) },
		"date": func(format string) string {
			return time.Now().In(utils.GetAppLocation()).Format(format)
		},
		"currentPath": func() string { return "/" },

		// Path/string helpers
		"eqPath":        eqPath,
		"activeFor":     activeFor,
		"basename":      basename,
		"startsWith":    startsWith,
		"normalizePath": normalizePath,

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
