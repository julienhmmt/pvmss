package config_test

import (
	"pvmss/server/internal/config"
	"strings"
	"testing"
)

func setMetricsBaseEnv(t *testing.T) {
	t.Helper()

	for k, v := range map[string]string{
		envPort: "50001", envDBPath: testDBPath, envLogLevel: testLogLevel,
		envLogFormat: testLogFormat, envLogOutput: testLogOutput, envClusterSource: testCluster,
		"SESSION_SECRET": strings.Repeat("s", 32),
	} {
		t.Setenv(k, v)
	}
}

func TestLoad_MetricsPort(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		want    int
		wantErr string
	}{
		{"unset means disabled", "", 0, ""},
		{"valid", "9464", 9464, ""},
		{"whitespace is trimmed", " 9464 ", 9464, ""},
		{"not an integer", "abc", 0, "PVMSS_METRICS_PORT must be an integer"},
		{"too low", "0", 0, "PVMSS_METRICS_PORT must be between 1 and 65535"},
		{"too high", "70000", 0, "PVMSS_METRICS_PORT must be between 1 and 65535"},
		{"same as main port", "50001", 0, "PVMSS_METRICS_PORT must differ from PVMSS_PORT"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setMetricsBaseEnv(t)
			t.Setenv("PVMSS_METRICS_PORT", c.value)

			cfg, err := config.Load()
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, c.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			if cfg.MetricsPort != c.want {
				t.Fatalf("MetricsPort = %d, want %d", cfg.MetricsPort, c.want)
			}
		})
	}
}
