package templates

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// formatMemory formats memory bytes to human readable format
func formatMemory(memBytes interface{}) string {
	// Delegate to formatBytes for consistent formatting
	return formatBytes(memBytes)
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

// dateFormat formats a time.Time with the provided layout
// Example layout: "2006-01-02 15:04:05"
func dateFormat(t time.Time, layout string) string {
	return t.Format(layout)
}

// toJSON marshals a value to a JSON string; returns "" on error
func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
