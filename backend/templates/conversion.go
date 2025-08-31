package templates

import (
	"fmt"
	"strconv"
)

// convertToInt converts a value of various types to an integer.
// It handles standard integer types, floating-point numbers (with truncation),
// and strings. If a string cannot be parsed as an integer, or if the input
// type is unsupported, it returns 0.
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

// convertToString converts any given value to its string representation using
// the default formatting provided by fmt.Sprintf.
func convertToString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

// convertToFloat64 converts a value of various types to a float64.
// It handles standard integer and floating-point types, as well as strings.
// If a string cannot be parsed as a float, or if the input type is
// unsupported, it returns 0.0.
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
