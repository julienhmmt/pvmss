package templates

import "reflect"

// defaultValue returns the default value if the provided value is empty
func defaultValue(value, defaultVal interface{}) interface{} {
	if isEmpty(value) {
		return defaultVal
	}
	return value
}

// isEmpty checks if a value is considered "empty".
// It returns true for nil, false, numeric zero, empty strings, empty collections (slices, maps, arrays),
// and nil pointers or interfaces.
func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return val.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return val.IsNil()
	case reflect.Bool:
		return !val.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return val.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return val.Float() == 0
	default:
		// For other types like structs, we consider them non-empty.
		// A more comprehensive check could use reflect.DeepEqual with the zero value,
		// but this is sufficient for typical template usage.
		return false
	}
}

// isNotEmpty checks if a value is not empty
func isNotEmpty(v interface{}) bool {
	return !isEmpty(v)
}

// coalesce returns the first non-empty value from the provided arguments
// Example: {{ coalesce .Name .Fallback "Unknown" }}
func coalesce(values ...interface{}) interface{} {
	for _, v := range values {
		if !isEmpty(v) {
			return v
		}
	}
	return nil
}
