package config_test

import (
	"pvmss/server/internal/config"
	"strings"
	"testing"
	"time"
)

const (
	envPort          = "PVMSS_PORT"
	envDBPath        = "PVMSS_DB_PATH"
	envLogLevel      = "LOG_LEVEL"
	envLogFormat     = "LOG_FORMAT"
	envLogOutput     = "LOG_OUTPUT"
	envClusterSource = "PVMSS_CLUSTER_SOURCE"
	testDBPath       = "./tmp/pvmss.db"
	testMemoryDB     = ":memory:"
	testHost         = "127.0.0.1"
	testLogLevel     = "info"
	testLogFormat    = "json"
	testLogOutput    = "stdout"
	testCluster      = "fake"
)

//nolint:funlen // comprehensive table-driven test covering all env vars
func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
		want    config.Configuration
	}{
		{
			name: "valid config",
			env: map[string]string{
				envPort:          "50001",
				envDBPath:        testDBPath,
				envLogLevel:      testLogLevel,
				envLogFormat:     testLogFormat,
				envLogOutput:     testLogOutput,
				envClusterSource: testCluster,
			},
			want: config.Configuration{
				Host:                              testHost,
				Port:                              50001,
				DBPath:                            testDBPath,
				LogLevel:                          testLogLevel,
				LogFormat:                         testLogFormat,
				LogOutput:                         testLogOutput,
				ClusterSource:                     testCluster,
				InventoryRefreshInterval:          30 * time.Second,
				InventoryManualRefreshMinInterval: 5 * time.Second,
				InventoryRefreshTimeout:           15 * time.Second,
				MaxListPageSize:                   100,
			},
		},
		{
			name: "explicit host",
			env: map[string]string{
				"PVMSS_HOST":     "0.0.0.0",
				envPort:          "50001",
				envDBPath:        testDBPath,
				envLogLevel:      testLogLevel,
				envLogFormat:     testLogFormat,
				envLogOutput:     testLogOutput,
				envClusterSource: testCluster,
			},
			want: config.Configuration{
				Host:                              "0.0.0.0",
				Port:                              50001,
				DBPath:                            testDBPath,
				LogLevel:                          testLogLevel,
				LogFormat:                         testLogFormat,
				LogOutput:                         testLogOutput,
				ClusterSource:                     testCluster,
				InventoryRefreshInterval:          30 * time.Second,
				InventoryManualRefreshMinInterval: 5 * time.Second,
				InventoryRefreshTimeout:           15 * time.Second,
				MaxListPageSize:                   100,
			},
		},
		{
			name: "missing port",
			env: map[string]string{
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
			},
			wantErr: "PVMSS_PORT is required",
		},
		{
			name: "missing db path",
			env: map[string]string{
				envPort:      "50001",
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
			},
			wantErr: "PVMSS_DB_PATH is required",
		},
		{
			name: "missing log level",
			env: map[string]string{
				envPort:      "50001",
				envDBPath:    testDBPath,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
			},
			wantErr: "LOG_LEVEL is required",
		},
		{
			name: "missing log format",
			env: map[string]string{
				envPort:      "50001",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogOutput: testLogOutput,
			},
			wantErr: "LOG_FORMAT is required",
		},
		{
			name: "missing log output",
			env: map[string]string{
				envPort:      "50001",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
			},
			wantErr: "LOG_OUTPUT is required",
		},
		{
			name: "port not a number",
			env: map[string]string{
				envPort:      "abc",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
			},
			wantErr: "PVMSS_PORT must be an integer",
		},
		{
			name: "port too low",
			env: map[string]string{
				envPort:      "0",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
			},
			wantErr: "PVMSS_PORT must be between 1 and 65535",
		},
		{
			name: "port too high",
			env: map[string]string{
				envPort:      "65536",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
			},
			wantErr: "PVMSS_PORT must be between 1 and 65535",
		},
		{
			name: "invalid log level",
			env: map[string]string{
				envPort:      "50001",
				envDBPath:    testDBPath,
				envLogLevel:  "verbose",
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
			},
			wantErr: "LOG_LEVEL must be one of",
		},
		{
			name: "invalid log format",
			env: map[string]string{
				envPort:      "50001",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: "xml",
				envLogOutput: testLogOutput,
			},
			wantErr: "LOG_FORMAT must be one of",
		},
		{
			name: "explicit cluster source proxmox",
			env: map[string]string{ //nolint:gosec // test-only fake Proxmox credentials
				envPort:                   "50001",
				envDBPath:                 testDBPath,
				envLogLevel:               testLogLevel,
				envLogFormat:              testLogFormat,
				envLogOutput:              testLogOutput,
				envClusterSource:          "proxmox",
				"PROXMOX_URL":             "https://proxmox.example.com",
				"PROXMOX_API_TOKEN_NAME":  "root@pam!pvmss",
				"PROXMOX_API_TOKEN_VALUE": "token-value",
			},
			want: config.Configuration{ //nolint:gosec // test-only fake Proxmox credentials
				Host:                              testHost,
				Port:                              50001,
				DBPath:                            testDBPath,
				LogLevel:                          testLogLevel,
				LogFormat:                         testLogFormat,
				LogOutput:                         testLogOutput,
				ClusterSource:                     "proxmox",
				ProxmoxURL:                        "https://proxmox.example.com",
				ProxmoxAPITokenName:               "root@pam!pvmss",
				ProxmoxAPITokenValue:              "token-value",
				InventoryRefreshInterval:          30 * time.Second,
				InventoryManualRefreshMinInterval: 5 * time.Second,
				InventoryRefreshTimeout:           15 * time.Second,
				MaxListPageSize:                   100,
			},
		},
		{
			name: "missing cluster source",
			env: map[string]string{
				envPort:      "50001",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
			},
			wantErr: "PVMSS_CLUSTER_SOURCE is required",
		},
		{
			name: "explicit cluster source vmware",
			env: map[string]string{
				envPort:          "50001",
				envDBPath:        testDBPath,
				envLogLevel:      testLogLevel,
				envLogFormat:     testLogFormat,
				envLogOutput:     testLogOutput,
				envClusterSource: "vmware",
			},
			wantErr: "PVMSS_CLUSTER_SOURCE must be one of",
		},
		{
			name: "explicit inventory intervals",
			env: map[string]string{
				envPort:                            "50001",
				envDBPath:                          testDBPath,
				envLogLevel:                        testLogLevel,
				envLogFormat:                       testLogFormat,
				envLogOutput:                       testLogOutput,
				envClusterSource:                   testCluster,
				"PVMSS_INVENTORY_REFRESH_INTERVAL": "10s",
				"PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL": "2s",
			},
			want: config.Configuration{
				Host:                              testHost,
				Port:                              50001,
				DBPath:                            testDBPath,
				LogLevel:                          testLogLevel,
				LogFormat:                         testLogFormat,
				LogOutput:                         testLogOutput,
				ClusterSource:                     testCluster,
				InventoryRefreshInterval:          10 * time.Second,
				InventoryManualRefreshMinInterval: 2 * time.Second,
				InventoryRefreshTimeout:           15 * time.Second,
				MaxListPageSize:                   100,
			},
		},
		{
			name: "invalid inventory refresh interval",
			env: map[string]string{
				envPort:                            "50001",
				envDBPath:                          testDBPath,
				envLogLevel:                        testLogLevel,
				envLogFormat:                       testLogFormat,
				envLogOutput:                       testLogOutput,
				envClusterSource:                   testCluster,
				"PVMSS_INVENTORY_REFRESH_INTERVAL": "not-a-duration",
			},
			wantErr: "PVMSS_INVENTORY_REFRESH_INTERVAL must be a duration",
		},
		{
			name: "non-positive inventory refresh interval",
			env: map[string]string{
				envPort:                            "50001",
				envDBPath:                          testDBPath,
				envLogLevel:                        testLogLevel,
				envLogFormat:                       testLogFormat,
				envLogOutput:                       testLogOutput,
				envClusterSource:                   testCluster,
				"PVMSS_INVENTORY_REFRESH_INTERVAL": "0s",
			},
			wantErr: "PVMSS_INVENTORY_REFRESH_INTERVAL must be a positive duration",
		},
		{
			name: "non-positive manual refresh min interval",
			env: map[string]string{
				envPort:          "50001",
				envDBPath:        testDBPath,
				envLogLevel:      testLogLevel,
				envLogFormat:     testLogFormat,
				envLogOutput:     testLogOutput,
				envClusterSource: testCluster,
				"PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL": "-1s",
			},
			wantErr: "PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL must be a positive duration",
		},
		{
			name: "explicit inventory refresh timeout",
			env: map[string]string{
				envPort:                           "50001",
				envDBPath:                         testDBPath,
				envLogLevel:                       testLogLevel,
				envLogFormat:                      testLogFormat,
				envLogOutput:                      testLogOutput,
				envClusterSource:                  testCluster,
				"PVMSS_INVENTORY_REFRESH_TIMEOUT": "45s",
			},
			want: config.Configuration{
				Host:                              testHost,
				Port:                              50001,
				DBPath:                            testDBPath,
				LogLevel:                          testLogLevel,
				LogFormat:                         testLogFormat,
				LogOutput:                         testLogOutput,
				ClusterSource:                     testCluster,
				InventoryRefreshInterval:          30 * time.Second,
				InventoryManualRefreshMinInterval: 5 * time.Second,
				InventoryRefreshTimeout:           45 * time.Second,
				MaxListPageSize:                   100,
			},
		},
		{
			name: "invalid inventory refresh timeout",
			env: map[string]string{
				envPort:                           "50001",
				envDBPath:                         testDBPath,
				envLogLevel:                       testLogLevel,
				envLogFormat:                      testLogFormat,
				envLogOutput:                      testLogOutput,
				envClusterSource:                  testCluster,
				"PVMSS_INVENTORY_REFRESH_TIMEOUT": "not-a-duration",
			},
			wantErr: "PVMSS_INVENTORY_REFRESH_TIMEOUT must be a duration",
		},
		{
			name: "non-positive inventory refresh timeout",
			env: map[string]string{
				envPort:                           "50001",
				envDBPath:                         testDBPath,
				envLogLevel:                       testLogLevel,
				envLogFormat:                      testLogFormat,
				envLogOutput:                      testLogOutput,
				envClusterSource:                  testCluster,
				"PVMSS_INVENTORY_REFRESH_TIMEOUT": "0s",
			},
			wantErr: "PVMSS_INVENTORY_REFRESH_TIMEOUT must be a positive duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runLoadCase(t, tt.env, tt.want, tt.wantErr)
		})
	}
}

// configLoadCase is the named test-case struct for TestLoad, extracted so the
// loop body can be delegated to a helper (SonarQube go:S3776).
type configLoadCase struct {
	name    string
	env     map[string]string
	wantErr string
	want    config.Configuration
}

// runLoadCase sets up the environment for a single TestLoad case, calls
// config.Load, and asserts the expected error or configuration. Extracted
// from TestLoad to keep its Cognitive Complexity under the SonarQube go:S3776
// threshold.
func runLoadCase(t *testing.T, env map[string]string, want config.Configuration, wantErr string) {
	t.Helper()

	t.Setenv(envPort, env[envPort])
	t.Setenv(envDBPath, env[envDBPath])
	t.Setenv(envLogLevel, env[envLogLevel])
	t.Setenv(envLogFormat, env[envLogFormat])
	t.Setenv(envLogOutput, env[envLogOutput])
	t.Setenv("PVMSS_HOST", env["PVMSS_HOST"])
	t.Setenv("PVMSS_WEB_DIR", env["PVMSS_WEB_DIR"])
	t.Setenv(envClusterSource, env[envClusterSource])
	t.Setenv("PVMSS_INVENTORY_REFRESH_INTERVAL", env["PVMSS_INVENTORY_REFRESH_INTERVAL"])
	t.Setenv("PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL", env["PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL"])
	t.Setenv("PVMSS_INVENTORY_REFRESH_TIMEOUT", env["PVMSS_INVENTORY_REFRESH_TIMEOUT"])
	t.Setenv("PROXMOX_URL", env["PROXMOX_URL"])
	t.Setenv("PROXMOX_API_TOKEN_NAME", env["PROXMOX_API_TOKEN_NAME"])
	t.Setenv("PROXMOX_API_TOKEN_VALUE", env["PROXMOX_API_TOKEN_VALUE"])
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))

	want.SessionSecret = strings.Repeat("s", 32)
	want.CookieSecure = true

	got, err := config.Load()
	if wantErr != "" {
		if err == nil {
			t.Fatalf("expected error containing %q, got nil", wantErr)
		}

		if !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("expected error containing %q, got %q", wantErr, err.Error())
		}

		return
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != want {
		t.Fatalf("config mismatch: got %+v, want %+v", got, want)
	}
}
