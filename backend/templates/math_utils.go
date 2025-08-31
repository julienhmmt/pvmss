package templates

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
