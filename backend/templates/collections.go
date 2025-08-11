package templates

import (
	"fmt"
	"reflect"
	"sort"
)

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
		// Use DeepEqual to avoid panics on non-comparable element types
		if reflect.DeepEqual(val.Index(i).Interface(), value) {
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

// sortStrings sorts a slice of strings ascending
func sortStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}

// sortInts sorts a slice of ints ascending
func sortInts(in []int) []int {
	if in == nil {
		return []int{}
	}
	out := make([]int, len(in))
	copy(out, in)
	sort.Ints(out)
	return out
}

// seq returns a sequence of integers from start to end-1
// Example: seq(3, 7) => [3,4,5,6]
func seq(start, end int) []int {
	if end <= start {
		return []int{}
	}
	n := end - start
	res := make([]int, n)
	for i := 0; i < n; i++ {
		res[i] = start + i
	}
	return res
}
