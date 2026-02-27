package handlers

import (
	"fmt"
	"net/http"

	"pvmss/i18n"
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

// FormatUptime formats uptime in seconds to human-readable format (days, hours, minutes, seconds)
// with i18n support
func FormatUptime(seconds int64, r *http.Request) string {
	localizer := i18n.GetLocalizerFromRequest(r)

	if seconds == 0 {
		return i18n.Localize(localizer, "Uptime.NotRunning")
	}

	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	var parts []string
	if days > 0 {
		if days == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", i18n.Localize(localizer, "Uptime.Day")))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", days, i18n.Localize(localizer, "Uptime.Days")))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", i18n.Localize(localizer, "Uptime.Hour")))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", hours, i18n.Localize(localizer, "Uptime.Hours")))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", i18n.Localize(localizer, "Uptime.Minute")))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", minutes, i18n.Localize(localizer, "Uptime.Minutes")))
		}
	}
	if secs > 0 || len(parts) == 0 {
		if secs == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", i18n.Localize(localizer, "Uptime.Second")))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", secs, i18n.Localize(localizer, "Uptime.Seconds")))
		}
	}

	// Join parts with commas - simple format for all languages
	if len(parts) == 1 {
		return parts[0]
	}
	result := ""
	for i, part := range parts {
		if i == 0 {
			result = part
		} else {
			result += ", " + part
		}
	}
	return result
}
