package handlers

// MBToGB converts MB to GB as integer (for calculations)
func MBToGB(mb int64) int64 {
	return mb / 1024
}
