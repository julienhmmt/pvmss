package config

import (
	"fmt"
	"os"
	"strconv"
)

// Load reads and validates the runtime configuration from the environment.
// It returns an error as soon as a required value is missing or malformed.
func Load() (Configuration, error) {
	var cfg Configuration
	portStr, ok := os.LookupEnv("PVMSS_PORT")
	if !ok || portStr == "" {
		return cfg, fmt.Errorf("PVMSS_PORT is required")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return cfg, fmt.Errorf("PVMSS_PORT must be an integer, got %q", portStr)
	}
	if port < 1 || port > 65535 {
		return cfg, fmt.Errorf("PVMSS_PORT must be between 1 and 65535, got %d", port)
	}
	cfg.Port = port

	cfg.DBPath, ok = os.LookupEnv("PVMSS_DB_PATH")
	if !ok || cfg.DBPath == "" {
		return cfg, fmt.Errorf("PVMSS_DB_PATH is required")
	}

	cfg.LogLevel, ok = os.LookupEnv("LOG_LEVEL")
	if !ok || cfg.LogLevel == "" {
		return cfg, fmt.Errorf("LOG_LEVEL is required")
	}
	if !isValidLogLevel(cfg.LogLevel) {
		return cfg, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error, got %q", cfg.LogLevel)
	}

	cfg.LogFormat, ok = os.LookupEnv("LOG_FORMAT")
	if !ok || cfg.LogFormat == "" {
		return cfg, fmt.Errorf("LOG_FORMAT is required")
	}
	if !isValidLogFormat(cfg.LogFormat) {
		return cfg, fmt.Errorf("LOG_FORMAT must be one of json, console, got %q", cfg.LogFormat)
	}

	cfg.LogOutput, ok = os.LookupEnv("LOG_OUTPUT")
	if !ok || cfg.LogOutput == "" {
		return cfg, fmt.Errorf("LOG_OUTPUT is required")
	}

	cfg.WebDir, _ = os.LookupEnv("PVMSS_WEB_DIR")

	return cfg, nil
}

func isValidLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	}
	return false
}

func isValidLogFormat(format string) bool {
	switch format {
	case "json", "console":
		return true
	}
	return false
}
