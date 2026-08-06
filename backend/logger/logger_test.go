package logger_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"pvmss/logger"
)

// setEnvHelper sets an environment variable and fails the test if it cannot.
func setEnvHelper(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env %s: %v", key, err)
	}
}

// unsetEnvHelper unsets an environment variable and fails the test if it cannot.
func unsetEnvHelper(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset env %s: %v", key, err)
	}
}

// TestInit verifies that the logger initializes correctly with various log levels.
func TestInit(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		logLevel string // level to log at
		wantLog  bool   // whether we expect the message to appear
	}{
		{"debug level logs debug", "debug", "debug", true},
		{"debug level logs info", "debug", "info", true},
		{"info level logs info", "info", "info", true},
		{"info level skips debug", "info", "debug", false},
		{"warn level logs warn", "warn", "warn", true},
		{"warn level skips info", "warn", "info", false},
		{"error level logs error", "error", "error", true},
		{"error level skips warn", "error", "warn", false},
		{"invalid level defaults to info", "invalid", "info", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset environment
			unsetEnvHelper(t, "LOG_OUTPUT")
			unsetEnvHelper(t, "LOG_FILE_PATH")
			unsetEnvHelper(t, "LOG_FORMAT")

			// Capture output
			var buf bytes.Buffer
			logger.Init(tt.level)
			logger.SetOutput(&buf)

			// Log a message at the specified level
			switch tt.logLevel {
			case "debug":
				logger.Get().Debug().Msg("test message")
			case "info":
				logger.Get().Info().Msg("test message")
			case "warn":
				logger.Get().Warn().Msg("test message")
			case "error":
				logger.Get().Error().Msg("test message")
			}

			hasMessage := strings.Contains(buf.String(), "test message")
			if tt.wantLog && !hasMessage {
				t.Errorf("Expected log output to contain 'test message', got: %s", buf.String())
			}
			if !tt.wantLog && hasMessage {
				t.Errorf("Expected log output NOT to contain 'test message', got: %s", buf.String())
			}
		})
	}
}

// TestLoggerGet verifies that Get returns a non-nil logger.
func TestLoggerGet(t *testing.T) {
	logger.Init("info")
	log := logger.Get()
	if log == nil {
		t.Error("Expected Get() to return a non-nil logger")
	}
}

// TestSetOutput verifies that SetOutput changes the log destination.
func TestSetOutput(t *testing.T) {
	logger.Init("info")

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.Get().Info().Msg("custom output test")

	if !strings.Contains(buf.String(), "custom output test") {
		t.Errorf("Expected log output to contain 'custom output test', got: %s", buf.String())
	}
}

// TestJSONFormat verifies that JSON format produces valid JSON output.
func TestJSONFormat(t *testing.T) {
	setEnvHelper(t, "LOG_OUTPUT", "stdout")
	setEnvHelper(t, "LOG_FORMAT", "json")
	defer func() {
		unsetEnvHelper(t, "LOG_OUTPUT")
		unsetEnvHelper(t, "LOG_FORMAT")
	}()

	logger.Init("info")

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.Get().Info().Str("key", "value").Msg("json test")

	// Parse the JSON output
	output := strings.TrimSpace(buf.String())
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Expected valid JSON output, got error: %v, output: %s", err, output)
	}

	// Verify expected fields
	if result["message"] != "json test" {
		t.Errorf("Expected message 'json test', got: %v", result["message"])
	}
	if result["key"] != "value" {
		t.Errorf("Expected key 'value', got: %v", result["key"])
	}
}

// TestAuthEvent verifies AuthEvent helper produces correct structured fields.
func TestAuthEvent(t *testing.T) {
	setEnvHelper(t, "LOG_FORMAT", "json")
	defer unsetEnvHelper(t, "LOG_FORMAT")

	logger.Init("info")

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.AuthEvent("login_success").
		Str("username", "testuser").
		Msg("User logged in")

	output := strings.TrimSpace(buf.String())
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v, output: %s", err, output)
	}

	if result["event_category"] != "auth" {
		t.Errorf("Expected event_category 'auth', got: %v", result["event_category"])
	}
	if result["event_type"] != "login_success" {
		t.Errorf("Expected event_type 'login_success', got: %v", result["event_type"])
	}
	if result["username"] != "testuser" {
		t.Errorf("Expected username 'testuser', got: %v", result["username"])
	}
}

// TestVMEvent verifies VMEvent helper produces correct structured fields.
func TestVMEvent(t *testing.T) {
	setEnvHelper(t, "LOG_FORMAT", "json")
	defer unsetEnvHelper(t, "LOG_FORMAT")

	logger.Init("info")

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.VMEvent("vm_create", 100, "node1").
		Str("vm_name", "test-vm").
		Msg("VM created")

	output := strings.TrimSpace(buf.String())
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v, output: %s", err, output)
	}

	if result["event_category"] != "vm" {
		t.Errorf("Expected event_category 'vm', got: %v", result["event_category"])
	}
	if result["event_type"] != "vm_create" {
		t.Errorf("Expected event_type 'vm_create', got: %v", result["event_type"])
	}
	if result["vmid"] != float64(100) {
		t.Errorf("Expected vmid 100, got: %v", result["vmid"])
	}
	if result["node"] != "node1" {
		t.Errorf("Expected node 'node1', got: %v", result["node"])
	}
}

// TestSecurityEvent verifies SecurityEvent helper produces correct structured fields.
func TestSecurityEvent(t *testing.T) {
	setEnvHelper(t, "LOG_FORMAT", "json")
	defer unsetEnvHelper(t, "LOG_FORMAT")

	logger.Init("info")

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.SecurityEvent("csrf_validation_failed").
		Str("client_ip", "192.168.1.1").
		Msg("CSRF validation failed")

	output := strings.TrimSpace(buf.String())
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v, output: %s", err, output)
	}

	if result["event_category"] != "security" {
		t.Errorf("Expected event_category 'security', got: %v", result["event_category"])
	}
	if result["event_type"] != "csrf_validation_failed" {
		t.Errorf("Expected event_type 'csrf_validation_failed', got: %v", result["event_type"])
	}
	if result["level"] != "warn" {
		t.Errorf("Expected level 'warn', got: %v", result["level"])
	}
}

// TestProxmoxEvent verifies ProxmoxEvent helper produces correct structured fields.
func TestProxmoxEvent(t *testing.T) {
	setEnvHelper(t, "LOG_FORMAT", "json")
	defer unsetEnvHelper(t, "LOG_FORMAT")

	logger.Init("info")

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.ProxmoxEvent("connection_restored").
		Int("node_count", 3).
		Msg("Proxmox connection restored")

	output := strings.TrimSpace(buf.String())
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v, output: %s", err, output)
	}

	if result["event_category"] != "proxmox" {
		t.Errorf("Expected event_category 'proxmox', got: %v", result["event_category"])
	}
	if result["event_type"] != "connection_restored" {
		t.Errorf("Expected event_type 'connection_restored', got: %v", result["event_type"])
	}
}

// TestProxmoxFailure verifies ProxmoxFailure helper produces correct structured fields.
func TestProxmoxFailure(t *testing.T) {
	setEnvHelper(t, "LOG_FORMAT", "json")
	defer unsetEnvHelper(t, "LOG_FORMAT")

	logger.Init("info")

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.ProxmoxFailure("connection_check", "timeout").
		Msg("Proxmox connection failed")

	output := strings.TrimSpace(buf.String())
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v, output: %s", err, output)
	}

	if result["event_category"] != "proxmox" {
		t.Errorf("Expected event_category 'proxmox', got: %v", result["event_category"])
	}
	if result["failure_reason"] != "timeout" {
		t.Errorf("Expected failure_reason 'timeout', got: %v", result["failure_reason"])
	}
	if result["level"] != "error" {
		t.Errorf("Expected level 'error', got: %v", result["level"])
	}
}
