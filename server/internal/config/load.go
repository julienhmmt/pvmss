package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultInventoryRefreshInterval          = 30 * time.Second
	defaultInventoryManualRefreshMinInterval = 5 * time.Second
	defaultInventoryRefreshTimeout           = 15 * time.Second
	defaultMaxListPageSize                   = 100
)

// Load reads and validates the runtime configuration from the environment.
// It returns an error as soon as a required value is missing or malformed.
func Load() (Configuration, error) {
	var cfg Configuration

	if err := loadCore(&cfg); err != nil {
		return cfg, err
	}

	if err := loadLogSettings(&cfg); err != nil {
		return cfg, err
	}

	if err := loadSecuritySettings(&cfg); err != nil {
		return cfg, err
	}

	if err := loadClusterSettings(&cfg); err != nil {
		return cfg, err
	}

	if err := loadInventorySettings(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// loadCore reads the required core settings: port, DB path, and host.
func loadCore(cfg *Configuration) error {
	portStr, ok := os.LookupEnv("PVMSS_PORT")

	portStr = strings.TrimSpace(portStr)
	if !ok || portStr == "" {
		return errors.New("PVMSS_PORT is required")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("PVMSS_PORT must be an integer, got %q", portStr)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("PVMSS_PORT must be between 1 and 65535, got %d", port)
	}

	cfg.Port = port

	cfg.DBPath = strings.TrimSpace(os.Getenv("PVMSS_DB_PATH"))
	if cfg.DBPath == "" {
		return errors.New("PVMSS_DB_PATH is required")
	}

	host := strings.TrimSpace(os.Getenv("PVMSS_HOST"))
	if host == "" {
		host = "127.0.0.1"
	}

	cfg.Host = host
	cfg.WebDir = strings.TrimSpace(os.Getenv("PVMSS_WEB_DIR"))

	return nil
}

// loadLogSettings reads and validates the log level, format, and output.
func loadLogSettings(cfg *Configuration) error {
	cfg.LogLevel = strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if cfg.LogLevel == "" {
		return errors.New("LOG_LEVEL is required")
	}

	if !isValidLogLevel(cfg.LogLevel) {
		return fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error, got %q", cfg.LogLevel)
	}

	cfg.LogFormat = strings.TrimSpace(os.Getenv("LOG_FORMAT"))
	if cfg.LogFormat == "" {
		return errors.New("LOG_FORMAT is required")
	}

	if !isValidLogFormat(cfg.LogFormat) {
		return fmt.Errorf("LOG_FORMAT must be one of json, console, got %q", cfg.LogFormat)
	}

	cfg.LogOutput = strings.TrimSpace(os.Getenv("LOG_OUTPUT"))
	if cfg.LogOutput == "" {
		return errors.New("LOG_OUTPUT is required")
	}

	return nil
}

// loadSecuritySettings reads the session secret and admin password hash.
func loadSecuritySettings(cfg *Configuration) error {
	cfg.SessionSecret = strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if len(cfg.SessionSecret) < 32 {
		return errors.New("SESSION_SECRET must be at least 32 bytes")
	}

	cfg.AdminPasswordHash = strings.TrimSpace(os.Getenv("ADMIN_PASSWORD_HASH"))
	if cfg.AdminPasswordHash != "" && !strings.HasPrefix(cfg.AdminPasswordHash, "$2") {
		return errors.New("ADMIN_PASSWORD_HASH must be a bcrypt hash")
	}

	return nil
}

// loadClusterSettings reads the cluster source selection and Proxmox credentials.
func loadClusterSettings(cfg *Configuration) error {
	clusterSource := strings.TrimSpace(os.Getenv("PVMSS_CLUSTER_SOURCE"))
	if clusterSource == "" {
		clusterSource = "fake"
	}

	if !isValidClusterSource(clusterSource) {
		return fmt.Errorf("PVMSS_CLUSTER_SOURCE must be one of fake, proxmox, got %q", clusterSource)
	}

	cfg.ClusterSource = clusterSource
	cfg.ProxmoxURL = strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	cfg.ProxmoxAPITokenName = strings.TrimSpace(os.Getenv("PROXMOX_API_TOKEN_NAME"))
	cfg.ProxmoxAPITokenValue = strings.TrimSpace(os.Getenv("PROXMOX_API_TOKEN_VALUE"))

	if cfg.ClusterSource == "proxmox" {
		if cfg.ProxmoxURL == "" {
			return errors.New("PROXMOX_URL is required when PVMSS_CLUSTER_SOURCE=proxmox")
		}

		if cfg.ProxmoxAPITokenName == "" || cfg.ProxmoxAPITokenValue == "" {
			return errors.New("PROXMOX_API_TOKEN_NAME and PROXMOX_API_TOKEN_VALUE are required when PVMSS_CLUSTER_SOURCE=proxmox")
		}
	}

	return nil
}

// loadInventorySettings reads the inventory refresh and quota settings.
func loadInventorySettings(cfg *Configuration) error {
	refreshInterval, err := loadPositiveDuration("PVMSS_INVENTORY_REFRESH_INTERVAL", defaultInventoryRefreshInterval)
	if err != nil {
		return err
	}

	cfg.InventoryRefreshInterval = refreshInterval

	manualMinInterval, err := loadPositiveDuration("PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL", defaultInventoryManualRefreshMinInterval)
	if err != nil {
		return err
	}

	cfg.InventoryManualRefreshMinInterval = manualMinInterval

	refreshTimeout, err := loadPositiveDuration("PVMSS_INVENTORY_REFRESH_TIMEOUT", defaultInventoryRefreshTimeout)
	if err != nil {
		return err
	}

	cfg.InventoryRefreshTimeout = refreshTimeout

	maxPageSize, err := loadInt("PVMSS_MAX_LIST_PAGE_SIZE", defaultMaxListPageSize)
	if err != nil {
		return err
	}

	if maxPageSize < 1 {
		return fmt.Errorf("PVMSS_MAX_LIST_PAGE_SIZE must be at least 1, got %d", maxPageSize)
	}

	cfg.MaxListPageSize = maxPageSize

	return nil
}

func loadInt(envKey string, defaultVal int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return defaultVal, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", envKey, raw)
	}

	return value, nil
}

func loadPositiveDuration(envKey string, defaultVal time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return defaultVal, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration, got %q", envKey, raw)
	}

	if d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration, got %q", envKey, raw)
	}

	return d, nil
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
