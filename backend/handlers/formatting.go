package handlers

import (
	"fmt"
	"net/http"
	"strings"
)

// FormatBytes formats byte values to human-readable format (MB/GB)
func FormatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatMemoryGB converts memory from bytes or MB to GB with clean formatting
// Accepts bytes (from Proxmox API) or MB (from form input)
// Returns formatted string like "2 GB", "512 MB", "0.5 GB"
func FormatMemoryGB(value int64, isBytes bool) string {
	var memoryMB int64

	if isBytes {
		// Convert bytes to MB (Proxmox API returns bytes)
		memoryMB = value / (1024 * 1024)
	} else {
		// Already in MB (from form input)
		memoryMB = value
	}

	// Convert MB to GB
	memoryGB := float64(memoryMB) / 1024.0

	// Format based on size
	if memoryGB >= 1 {
		// For 1 GB or more, show as GB
		if memoryGB == float64(int64(memoryGB)) {
			// Whole number
			return fmt.Sprintf("%d GB", int64(memoryGB))
		}
		// Decimal
		return fmt.Sprintf("%.1f GB", memoryGB)
	}

	// Less than 1 GB, show as MB
	if memoryMB == int64(memoryMB) {
		return fmt.Sprintf("%d MB", memoryMB)
	}
	return fmt.Sprintf("%.0f MB", float64(memoryMB))
}

// BytesToGB converts bytes to GB as integer (for calculations)
func BytesToGB(bytes int64) int64 {
	return bytes / (1024 * 1024 * 1024)
}

// MBToGB converts MB to GB as integer (for calculations)
func MBToGB(mb int64) int64 {
	return mb / 1024
}

// FormatUptime formats uptime duration (simple English, no i18n)
func FormatUptime(seconds int64, r *http.Request) string {
	if seconds == 0 {
		return "Not running"
	}

	var parts []string
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}
	if secs > 0 || len(parts) == 0 {
		if secs == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", secs))
		}
	}

	return strings.Join(parts, " ")
}
