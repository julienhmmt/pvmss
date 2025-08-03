package templates

import (
	"fmt"
	"reflect"
	"time"
)

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

// formatDuration formats a time.Duration to a human-readable string
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// formatBytes formats bytes to human readable format
func formatBytes(bytes interface{}) string {
	b := convertToFloat64(bytes)
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", int64(b))
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
