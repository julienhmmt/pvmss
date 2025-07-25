// Package templates provides template functions and utilities for the PVMSS application
package templates

import (
	"fmt"
	"html/template"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// GetFuncMap returns the function map for template functions
func GetFuncMap() template.FuncMap {
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
	}
}

// convertToInt converts various types to int
func convertToInt(v interface{}) int {
	switch v := v.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		return 0
	default:
		return 0
	}
}

// convertToString converts interface to string
func convertToString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

// convertToFloat64 converts various types to float64
func convertToFloat64(v interface{}) float64 {
	switch v := v.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
		return 0.0
	default:
		return 0.0
	}
}

// formatMemory formats memory bytes to human readable format
func formatMemory(memBytes interface{}) string {
	var memInt int64
	switch v := memBytes.(type) {
	case int:
		memInt = int64(v)
	case int64:
		memInt = v
	case float64:
		memInt = int64(v)
	default:
		return "0 MB"
	}

	if memInt >= 1024*1024*1024 {
		return fmt.Sprintf("%.1f GB", float64(memInt)/(1024*1024*1024))
	}
	return fmt.Sprintf("%d MB", memInt/(1024*1024))
}

// nodeMemMaxGB extracts maximum memory in GB from a node object
func nodeMemMaxGB(node interface{}) int64 {
	if node == nil {
		return 8 // Default to 8GB
	}

	nodeValue := reflect.ValueOf(node)
	if nodeValue.Kind() == reflect.Ptr {
		nodeValue = nodeValue.Elem()
	}

	if nodeValue.Kind() == reflect.Struct {
		if field := nodeValue.FieldByName("MaxMem"); field.IsValid() {
			maxMem := convertToInt(field.Interface())
			return int64(maxMem / (1024 * 1024 * 1024)) // Convert bytes to GB
		}
	}

	return 8 // Default to 8GB
}

// formatBytes formats bytes to human readable format
func formatBytes(bytes interface{}) string {
	b := convertToFloat64(bytes)
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%.0f B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", b/float64(div), "KMGTPE"[exp])
}

// sortSlice sorts a slice of strings
func sortSlice(slice interface{}) []string {
	if slice == nil {
		return []string{}
	}

	val := reflect.ValueOf(slice)
	if val.Kind() != reflect.Slice {
		return []string{}
	}

	result := make([]string, val.Len())
	for i := 0; i < val.Len(); i++ {
		result[i] = fmt.Sprintf("%v", val.Index(i).Interface())
	}

	sort.Strings(result)
	return result
}

// reverseSlice reverses a slice
func reverseSlice(slice interface{}) []interface{} {
	if slice == nil {
		return []interface{}{}
	}

	val := reflect.ValueOf(slice)
	if val.Kind() != reflect.Slice {
		return []interface{}{}
	}

	result := make([]interface{}, val.Len())
	for i := 0; i < val.Len(); i++ {
		result[val.Len()-1-i] = val.Index(i).Interface()
	}

	return result
}

// containsValue checks if a slice contains a value
func containsValue(slice interface{}, value interface{}) bool {
	if slice == nil {
		return false
	}

	val := reflect.ValueOf(slice)
	if val.Kind() != reflect.Slice {
		return false
	}

	for i := 0; i < val.Len(); i++ {
		if val.Index(i).Interface() == value {
			return true
		}
	}

	return false
}

// getLength returns the length of a slice, map, or string
func getLength(v interface{}) int {
	if v == nil {
		return 0
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
		return val.Len()
	default:
		return 0
	}
}

// String utility functions
func toUpper(s string) string {
	return strings.ToUpper(s)
}

func toLower(s string) string {
	return strings.ToLower(s)
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

// Math functions
func addNumbers(a, b interface{}) float64 {
	return convertToFloat64(a) + convertToFloat64(b)
}

func subtractNumbers(a, b interface{}) float64 {
	return convertToFloat64(a) - convertToFloat64(b)
}

func multiplyNumbers(a, b interface{}) float64 {
	return convertToFloat64(a) * convertToFloat64(b)
}

func divideNumbers(a, b interface{}) float64 {
	bVal := convertToFloat64(b)
	if bVal == 0 {
		return 0
	}
	return convertToFloat64(a) / bVal
}

// Utility functions
func defaultValue(value, defaultVal interface{}) interface{} {
	if isEmpty(value) {
		return defaultVal
	}
	return value
}

func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.String:
		return val.Len() == 0
	case reflect.Slice, reflect.Array, reflect.Map:
		return val.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return val.IsNil()
	default:
		return false
	}
}

// isNotEmpty checks if a value is not empty
func isNotEmpty(v interface{}) bool {
	return !isEmpty(v)
}

// until returns a slice of integers from 0 to count-1
// This is commonly used in templates for iteration
// Example: {{range $i := until 5}}{{end}} will iterate 5 times (0-4)
func until(count int) []int {
	if count <= 0 {
		return []int{}
	}
	result := make([]int, count)
	for i := 0; i < count; i++ {
		result[i] = i
	}
	return result
}
