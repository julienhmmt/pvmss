package templates

// addNumbers adds two numbers of any numeric type
func addNumbers(a, b interface{}) float64 {
	return convertToFloat64(a) + convertToFloat64(b)
}

// subtractNumbers subtracts b from a
func subtractNumbers(a, b interface{}) float64 {
	return convertToFloat64(a) - convertToFloat64(b)
}

// multiplyNumbers multiplies two numbers
func multiplyNumbers(a, b interface{}) float64 {
	return convertToFloat64(a) * convertToFloat64(b)
}

// divideNumbers divides a by b, returns 0 if dividing by zero
func divideNumbers(a, b interface{}) float64 {
	bVal := convertToFloat64(b)
	if bVal == 0 {
		return 0
	}
	return convertToFloat64(a) / bVal
}
