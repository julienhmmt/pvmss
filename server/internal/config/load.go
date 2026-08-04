package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Load reads and validates the runtime configuration from the environment.
// It returns an error as soon as a required value is missing or malformed.
func Load() (Configuration, error) {
	var cfg Configuration
	portStr, ok := os.LookupEnv("PVMSS_PORT")
	portStr = strings.TrimSpace(portStr)
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

	cfg.DBPath = strings.TrimSpace(os.Getenv("PVMSS_DB_PATH"))
	if cfg.DBPath == "" {
		return cfg, fmt.Errorf("PVMSS_DB_PATH is required")
	}

	cfg.LogLevel = strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if cfg.LogLevel == "" {
		return cfg, fmt.Errorf("LOG_LEVEL is required")
	}
	if !isValidLogLevel(cfg.LogLevel) {
		return cfg, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error, got %q", cfg.LogLevel)
	}

	cfg.LogFormat = strings.TrimSpace(os.Getenv("LOG_FORMAT"))
	if cfg.LogFormat == "" {
		return cfg, fmt.Errorf("LOG_FORMAT is required")
	}
	if !isValidLogFormat(cfg.LogFormat) {
		return cfg, fmt.Errorf("LOG_FORMAT must be one of json, console, got %q", cfg.LogFormat)
	}

	cfg.LogOutput = strings.TrimSpace(os.Getenv("LOG_OUTPUT"))
	if cfg.LogOutput == "" {
		return cfg, fmt.Errorf("LOG_OUTPUT is required")
	}

	host := strings.TrimSpace(os.Getenv("PVMSS_HOST"))
	if host == "" {
		host = "127.0.0.1"
	}
	cfg.Host = host

	cfg.WebDir = strings.TrimSpace(os.Getenv("PVMSS_WEB_DIR"))

	clusterSource := strings.TrimSpace(os.Getenv("PVMSS_CLUSTER_SOURCE"))
	if clusterSource == "" {
		clusterSource = "fake"
	}
	if !isValidClusterSource(clusterSource) {
		return cfg, fmt.Errorf("PVMSS_CLUSTER_SOURCE must be one of fake, proxmox, got %q", clusterSource)
	}
	cfg.ClusterSource = clusterSource

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

func isValidClusterSource(source string) bool {
	switch source {
	case "fake", "proxmox":
		return true
	}
	return false
}
