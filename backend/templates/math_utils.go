package templates

// Math utility functions for template operations.
// All functions automatically convert arguments to float64 for consistent behavior.

// addNumbers adds two numbers.
// It converts both arguments to float64 before performing the addition.
func addNumbers(a, b interface{}) float64 {
	return convertToFloat64(a) + convertToFloat64(b)
}

// subtractNumbers subtracts the second number from the first.
// It converts both arguments to float64 before performing the subtraction.
func subtractNumbers(a, b interface{}) float64 {
	return convertToFloat64(a) - convertToFloat64(b)
}

// multiplyNumbers multiplies two numbers.
// It converts both arguments to float64 before performing the multiplication.
func multiplyNumbers(a, b interface{}) float64 {
	return convertToFloat64(a) * convertToFloat64(b)
}

// divideNumbers divides the first number by the second.
// It converts both arguments to float64. If the divisor is zero, it returns 0.
func divideNumbers(a, b interface{}) float64 {
	bVal := convertToFloat64(b)
	if bVal == 0 {
		return 0
	}
	return convertToFloat64(a) / bVal
}

// iterate returns a slice of integers from 0 to n-1.
// This is useful for range loops in templates.
// Example: {{range $i := iterate 5}} will loop from 0 to 4.
func iterate(count interface{}) []int {
	n := int(convertToFloat64(count))
	if n <= 0 {
		return []int{}
	}
	result := make([]int, n)
	for i := 0; i < n; i++ {
		result[i] = i
	}
	return result
}

// toFloat converts any numeric type to float64.
// This is useful for template comparisons where types need to match.
func toFloat(val interface{}) float64 {
	return convertToFloat64(val)
}

// toInt converts any numeric type to int.
// This is useful for template comparisons and range operations.
func toInt(val interface{}) int {
	return int(convertToFloat64(val))
}

// addInt adds two numbers and returns an int result.
// Useful in templates when working with integer indices.
func addInt(a, b interface{}) int {
	return int(convertToFloat64(a) + convertToFloat64(b))
}
