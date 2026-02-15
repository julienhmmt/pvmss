package handlers

import (
	"fmt"
	"strings"
)

// CreateNotificationScript generates JavaScript to show a notification
func CreateNotificationScript(notificationType, title, message string, options map[string]interface{}) string {
	script := fmt.Sprintf(`
		<script>
			document.addEventListener('DOMContentLoaded', function() {
				window.showNotification({
					type: '%s',
					title: '%s',
					message: '%s'`,
		notificationType, strings.ReplaceAll(title, "'", "\\'"), strings.ReplaceAll(message, "'", "\\'"))

	if options != nil {
		if icon, ok := options["icon"].(string); ok && icon != "" {
			script += fmt.Sprintf(",\n\t\t\t\t\ticon: '%s'", icon)
		}
		if duration, ok := options["duration"].(int); ok && duration > 0 {
			script += fmt.Sprintf(",\n\t\t\t\t\tduration: %d", duration)
		}
		if dismissible, ok := options["dismissible"].(bool); ok {
			script += fmt.Sprintf(",\n\t\t\t\t\tdismissible: %t", dismissible)
		}
		if urgent, ok := options["urgent"].(bool); ok {
			script += fmt.Sprintf(",\n\t\t\t\t\turgent: %t", urgent)
		}
	}

	script += `
				});
			});
		</script>`

	return script
}

// ShowSuccessNotification creates a success notification script
func ShowSuccessNotification(title, message string, options map[string]interface{}) string {
	if options == nil {
		options = make(map[string]interface{})
	}
	if _, ok := options["icon"]; !ok {
		options["icon"] = "fas fa-check-circle"
	}
	return CreateNotificationScript("success", title, message, options)
}

// ShowErrorNotification creates an error notification script
func ShowErrorNotification(title, message string, options map[string]interface{}) string {
	if options == nil {
		options = make(map[string]interface{})
	}
	if _, ok := options["icon"]; !ok {
		options["icon"] = "fas fa-exclamation-circle"
	}
	if _, ok := options["urgent"]; !ok {
		options["urgent"] = true
	}
	return CreateNotificationScript("danger", title, message, options)
}

// ShowWarningNotification creates a warning notification script
func ShowWarningNotification(title, message string, options map[string]interface{}) string {
	if options == nil {
		options = make(map[string]interface{})
	}
	if _, ok := options["icon"]; !ok {
		options["icon"] = "fas fa-exclamation-triangle"
	}
	return CreateNotificationScript("warning", title, message, options)
}

// ShowInfoNotification creates an info notification script
func ShowInfoNotification(title, message string, options map[string]interface{}) string {
	if options == nil {
		options = make(map[string]interface{})
	}
	if _, ok := options["icon"]; !ok {
		options["icon"] = "fas fa-info-circle"
	}
	return CreateNotificationScript("info", title, message, options)
}

// CreateProgressScript generates JavaScript to show a progress bar
func CreateProgressScript(title, message string, options map[string]interface{}) string {
	script := fmt.Sprintf(`
		<script>
			document.addEventListener('DOMContentLoaded', function() {
				window.showProgress({
					title: '%s',
					message: '%s'`,
		strings.ReplaceAll(title, "'", "\\'"), strings.ReplaceAll(message, "'",
			"\\'"))

	if options != nil {
		if progressType, ok := options["type"].(string); ok && progressType != "" {
			script += fmt.Sprintf(",\n\t\t\t\t\ttype: '%s'", progressType)
		}
		if icon, ok := options["icon"].(string); ok && icon != "" {
			script += fmt.Sprintf(",\n\t\t\t\t\ticon: '%s'", icon)
		}
		if showClose, ok := options["showClose"].(bool); ok {
			script += fmt.Sprintf(",\n\t\t\t\t\tshowClose: %t", showClose)
		}
		if showDetails, ok := options["showDetails"].(bool); ok {
			script += fmt.Sprintf(",\n\t\t\t\t\tshowDetails: %t", showDetails)
		}
		if onCancel, ok := options["onCancel"].(string); ok && onCancel != "" {
			script += fmt.Sprintf(",\n\t\t\t\t\tonCancel: function() { %s }", onCancel)
		}
	}

	script += `
				});
			});
		</script>`

	return script
}

// UpdateProgressScript generates JavaScript to update a progress bar
func UpdateProgressScript(progressId string, progress int, message string) string {
	return fmt.Sprintf(`
		<script>
			document.addEventListener('DOMContentLoaded', function() {
				window.updateProgress('%s', {
					progress: %d,
					message: '%s'
				});
			});
		</script>`,
		strings.ReplaceAll(progressId, "'", "\\'"),
		progress,
		strings.ReplaceAll(message, "'", "\\'"))
}

// HideProgressScript generates JavaScript to hide a progress bar
func HideProgressScript(progressId string) string {
	return fmt.Sprintf(`
		<script>
			document.addEventListener('DOMContentLoaded', function() {
				window.hideProgress('%s');
			});
		</script>`,
		strings.ReplaceAll(progressId, "'", "\\'"))
}
