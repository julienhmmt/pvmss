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
		"formatMemory": formatMemory,
		"nodeMemMaxGB": nodeMemMaxGB,
		"formatBytes":  formatBytes,

		// Collection functions
		"sort":     sortSlice,
		"reverse":  reverseSlice,
		"contains": containsValue,
		"length":   getLength,
		"until":    until,

		// String functions
		"upper":      toUpper,
		"lower":      toLower,
		"truncate":   truncateString,
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

		// Utility functions that don't depend on the request
		"formatDuration": formatDuration,
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
	}

	return funcMap
}
