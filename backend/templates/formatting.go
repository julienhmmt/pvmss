package templates

import (
	"encoding/json"
	"fmt"
	"time"
)

// formatMemory formats memory bytes to human readable format
func formatMemory(memBytes interface{}) string {
	// Delegate to formatBytes for consistent formatting
	return formatBytes(memBytes)
}

// formatDuration formats a time.Duration to a human-readable string like "1h 2m 3s".
// Handles negative durations by prefixing with "-".
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	neg := d < 0
	if neg {
		d = -d
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	var out string
	if h > 0 {
		out = fmt.Sprintf("%dh %dm %ds", h, m, s)
	} else if m > 0 {
		out = fmt.Sprintf("%dm %ds", m, s)
	} else {
		out = fmt.Sprintf("%ds", s)
	}
	if neg {
		return "-" + out
	}
	return out
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

// formatBytesSI formats bytes using SI units (base 1000): kB, MB, GB, ...
func formatBytesSI(bytes interface{}) string {
	b := convertToFloat64(bytes)
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", int64(b))
	}
	div, exp := float64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/div, "kMGTPE"[exp])
}

// formatGiB formats a byte value as GiB with fixed precision (e.g., "8.0 GiB").
func formatGiB(bytes interface{}) string {
	b := convertToFloat64(bytes)
	const gib = 1024 * 1024 * 1024
	return fmt.Sprintf("%.1f GiB", b/float64(gib))
}

// since returns a human-friendly relative time string like "5m ago" for a past time.
func since(t time.Time) string {
	return humanizeRelative(time.Since(t)) + " ago"
}

// untilTime returns a human-friendly relative time string like "in 2h" for a future time.
func untilTime(t time.Time) string {
	d := time.Until(t)
	if d <= 0 {
		// already past or now
		return humanizeRelative(-d) + " ago"
	}
	return "in " + humanizeRelative(d)
}

// humanizeRelative renders the largest unit among days, hours, minutes, seconds.
func humanizeRelative(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d >= 24*time.Hour {
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	}
	if d >= time.Hour {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	s := int(d / time.Second)
	if s <= 0 {
		return "0s"
	}
	return fmt.Sprintf("%ds", s)
}

// dateFormat formats a time.Time with the provided layout (Go time layout strings)
func dateFormat(t time.Time, layout string) string {
	return t.Format(layout)
}

// dateRFC3339 formats a time.Time as RFC3339.
func dateRFC3339(t time.Time) string { return t.Format(time.RFC3339) }

// dateISO8601 is an alias of RFC3339 formatting.
func dateISO8601(t time.Time) string { return t.Format(time.RFC3339) }

// dateShort returns a short date like 2006-01-02
func dateShort(t time.Time) string { return t.Format("2006-01-02") }

// dateTimeShort returns a short datetime like 2006-01-02 15:04
func dateTimeShort(t time.Time) string { return t.Format("2006-01-02 15:04") }

// toJSON marshals a value to a JSON string; returns "" on error
func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// toJSONIndent marshals a value to pretty JSON; returns "" on error.
// If indent is empty, two spaces are used.
func toJSONIndent(v interface{}, indent string) string {
	if indent == "" {
		indent = "  "
	}
	b, err := json.MarshalIndent(v, "", indent)
	if err != nil {
		return ""
	}
	return string(b)
}
