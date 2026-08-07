package config_test

import (
	"pvmss/server/internal/config"
	"strings"
	"testing"
	"time"
)

const (
	envPort       = "PVMSS_PORT"
	envDBPath     = "PVMSS_DB_PATH"
	envLogLevel   = "LOG_LEVEL"
	envLogFormat  = "LOG_FORMAT"
	envLogOutput  = "LOG_OUTPUT"
	testDBPath    = "./tmp/pvmss.db"
	testMemoryDB  = ":memory:"
	testHost      = "127.0.0.1"
	testLogLevel  = "info"
	testLogFormat = "json"
	testLogOutput = "stdout"
	testCluster   = "fake"
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
				envPort:      "50001",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
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
				DefaultUserQuota:                  -1,
			},
		},
		{
			name: "explicit host",
			env: map[string]string{
				"PVMSS_HOST": "0.0.0.0",
				envPort:      "50001",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
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
				DefaultUserQuota:                  -1,
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
			env: map[string]string{
				envPort:                "50001",
				envDBPath:              testDBPath,
				envLogLevel:            testLogLevel,
				envLogFormat:           testLogFormat,
				envLogOutput:           testLogOutput,
				"PVMSS_CLUSTER_SOURCE": "proxmox",
			},
			want: config.Configuration{
				Host:                              testHost,
				Port:                              50001,
				DBPath:                            testDBPath,
				LogLevel:                          testLogLevel,
				LogFormat:                         testLogFormat,
				LogOutput:                         testLogOutput,
				ClusterSource:                     "proxmox",
				InventoryRefreshInterval:          30 * time.Second,
				InventoryManualRefreshMinInterval: 5 * time.Second,
				InventoryRefreshTimeout:           15 * time.Second,
				MaxListPageSize:                   100,
				DefaultUserQuota:                  -1,
			},
		},
		{
			name: "invalid cluster source",
			env: map[string]string{
				envPort:                "50001",
				envDBPath:              testDBPath,
				envLogLevel:            testLogLevel,
				envLogFormat:           testLogFormat,
				envLogOutput:           testLogOutput,
				"PVMSS_CLUSTER_SOURCE": "vmware",
			},
			wantErr: "PVMSS_CLUSTER_SOURCE must be one of",
		},
		{
			name: "explicit inventory intervals",
			env: map[string]string{
				envPort:                                "50001",
				envDBPath:                              testDBPath,
				envLogLevel:                            testLogLevel,
				envLogFormat:                           testLogFormat,
				envLogOutput:                           testLogOutput,
				"PVMSS_V04_INVENTORY_REFRESH_INTERVAL": "10s",
				"PVMSS_V04_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL": "2s",
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
				DefaultUserQuota:                  -1,
			},
		},
		{
			name: "invalid inventory refresh interval",
			env: map[string]string{
				envPort:                                "50001",
				envDBPath:                              testDBPath,
				envLogLevel:                            testLogLevel,
				envLogFormat:                           testLogFormat,
				envLogOutput:                           testLogOutput,
				"PVMSS_V04_INVENTORY_REFRESH_INTERVAL": "not-a-duration",
			},
			wantErr: "PVMSS_V04_INVENTORY_REFRESH_INTERVAL must be a duration",
		},
		{
			name: "non-positive inventory refresh interval",
			env: map[string]string{
				envPort:                                "50001",
				envDBPath:                              testDBPath,
				envLogLevel:                            testLogLevel,
				envLogFormat:                           testLogFormat,
				envLogOutput:                           testLogOutput,
				"PVMSS_V04_INVENTORY_REFRESH_INTERVAL": "0s",
			},
			wantErr: "PVMSS_V04_INVENTORY_REFRESH_INTERVAL must be a positive duration",
		},
		{
			name: "non-positive manual refresh min interval",
			env: map[string]string{
				envPort:      "50001",
				envDBPath:    testDBPath,
				envLogLevel:  testLogLevel,
				envLogFormat: testLogFormat,
				envLogOutput: testLogOutput,
				"PVMSS_V04_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL": "-1s",
			},
			wantErr: "PVMSS_V04_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL must be a positive duration",
		},
		{
			name: "explicit inventory refresh timeout",
			env: map[string]string{
				envPort:                               "50001",
				envDBPath:                             testDBPath,
				envLogLevel:                           testLogLevel,
				envLogFormat:                          testLogFormat,
				envLogOutput:                          testLogOutput,
				"PVMSS_V04_INVENTORY_REFRESH_TIMEOUT": "45s",
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
				DefaultUserQuota:                  -1,
			},
		},
		{
			name: "invalid inventory refresh timeout",
			env: map[string]string{
				envPort:                               "50001",
				envDBPath:                             testDBPath,
				envLogLevel:                           testLogLevel,
				envLogFormat:                          testLogFormat,
				envLogOutput:                          testLogOutput,
				"PVMSS_V04_INVENTORY_REFRESH_TIMEOUT": "not-a-duration",
			},
			wantErr: "PVMSS_V04_INVENTORY_REFRESH_TIMEOUT must be a duration",
		},
		{
			name: "non-positive inventory refresh timeout",
			env: map[string]string{
				envPort:                               "50001",
				envDBPath:                             testDBPath,
				envLogLevel:                           testLogLevel,
				envLogFormat:                          testLogFormat,
				envLogOutput:                          testLogOutput,
				"PVMSS_V04_INVENTORY_REFRESH_TIMEOUT": "0s",
			},
			wantErr: "PVMSS_V04_INVENTORY_REFRESH_TIMEOUT must be a positive duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envPort, tt.env[envPort])
			t.Setenv(envDBPath, tt.env[envDBPath])
			t.Setenv(envLogLevel, tt.env[envLogLevel])
			t.Setenv(envLogFormat, tt.env[envLogFormat])
			t.Setenv(envLogOutput, tt.env[envLogOutput])
			t.Setenv("PVMSS_HOST", tt.env["PVMSS_HOST"])
			t.Setenv("PVMSS_WEB_DIR", tt.env["PVMSS_WEB_DIR"])
			t.Setenv("PVMSS_CLUSTER_SOURCE", tt.env["PVMSS_CLUSTER_SOURCE"])
			t.Setenv("PVMSS_V04_INVENTORY_REFRESH_INTERVAL", tt.env["PVMSS_V04_INVENTORY_REFRESH_INTERVAL"])
			t.Setenv("PVMSS_V04_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL", tt.env["PVMSS_V04_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL"])
			t.Setenv("PVMSS_V04_INVENTORY_REFRESH_TIMEOUT", tt.env["PVMSS_V04_INVENTORY_REFRESH_TIMEOUT"])
			t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))

			want := tt.want
			want.SessionSecret = strings.Repeat("s", 32)

			got, err := config.Load()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != want {
				t.Fatalf("config mismatch: got %+v, want %+v", got, want)
			}
		})
	}
}
