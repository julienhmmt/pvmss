package utils

import (
	"os"
	"strings"
)

// IsProduction checks if the application is running in production mode.
// Accepts: "production", "prod", "production", "dev", "development", "developpement"
func IsProduction() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("PVMSS_ENV")))
	return env == "production" || env == "prod"
}

// IsDevelopment checks if the application is running in development mode.
// Accepts: "dev", "development", "developpement"
func IsDevelopment() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("PVMSS_ENV")))
	return env == "dev" || env == "development" || env == "developpement"
}
