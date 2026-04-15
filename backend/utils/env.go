package utils

import (
	"strings"
)

// IsProduction checks if the application is running in production mode.
// Accepts: "production", "prod"
func IsProduction(env string) bool {
	e := strings.ToLower(strings.TrimSpace(env))
	return e == "production" || e == "prod"
}

// IsDevelopment checks if the application is running in development mode.
// Accepts: "dev", "development", "developpement"
func IsDevelopment(env string) bool {
	e := strings.ToLower(strings.TrimSpace(env))
	return e == "dev" || e == "development" || e == "developpement"
}
