package templates

import "reflect"

// defaultValue returns the default value if the provided value is empty
func defaultValue(value, defaultVal interface{}) interface{} {
	if isEmpty(value) {
		return defaultVal
	}
	return value
}

// isEmpty checks if a value is empty
// It returns true for nil, empty strings, empty slices, empty maps, and nil pointers
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
