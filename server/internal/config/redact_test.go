package config_test

import (
	"pvmss/server/internal/config"
	"strings"
	"testing"
)

// TestConfigurationRedacted_RedactsAllSecrets — SC-006: Redacted() returns
// every secret-shaped field with redacted=true and an empty value, regardless
// of the configured value. As of T14 the secret-shaped fields are
// AdminPasswordHash, SessionSecret, and ProxmoxAPITokenValue (every bearer
// credential or shared secret that lives in Configuration today).
//
//nolint:paralleltest // serial: no shared state, but kept consistent with suite
func TestConfigurationRedacted_RedactsAllSecrets(t *testing.T) {
	//nolint:gosec // test fixture, not a real credential
	cfg := config.Configuration{
		AdminPasswordHash:    "$2a$10$testbcryptvalue",
		SessionSecret:        strings.Repeat("s", 64),
		ProxmoxAPITokenValue: "secret-token-value",
		ProxmoxAPITokenName:  "pvmss@pve",
	}

	fields := cfg.Redacted()

	secretValues := map[string]string{
		"ADMIN_PASSWORD_HASH":     cfg.AdminPasswordHash,
		"SESSION_SECRET":          cfg.SessionSecret,
		"PROXMOX_API_TOKEN_VALUE": cfg.ProxmoxAPITokenValue,
	}

	for _, f := range fields {
		want, isSecret := secretValues[f.Name]
		if !isSecret {
			continue
		}

		if !f.Redacted {
			t.Errorf("field %s: redacted = false, want true", f.Name)
		}

		if f.Value != "" {
			t.Errorf("field %s: value = %q, want empty (redacted)", f.Name, f.Value)
		}
		// The real secret value must never appear in the redacted output.
		if want != "" && containsValue(fields, want) {
			t.Errorf("field %s: real secret value leaked into Redacted() output", f.Name)
		}
	}
}

// TestConfigurationRedacted_NonSecretFieldsShowRealValue — every non-secret
// Configuration field returns its real configured value, never redacted.
//
//nolint:paralleltest // serial: no shared state, but kept consistent with suite
func TestConfigurationRedacted_NonSecretFieldsShowRealValue(t *testing.T) {
	cfg := config.Configuration{
		Host:          "0.0.0.0",
		Port:          50001,
		DBPath:        "/data/pvmss.db",
		LogLevel:      "info",
		LogFormat:     "json",
		LogOutput:     "stdout",
		WebDir:        "/app/web",
		ClusterSource: "fake",
		CookieSecure:  true,
		ProxmoxURL:    "https://proxmox:8006",
	}

	fields := cfg.Redacted()

	byName := make(map[string]config.Field, len(fields))
	for _, f := range fields {
		byName[f.Name] = f
	}

	tests := []struct {
		name      string
		fieldName string
		wantValue string
	}{
		{"Host", "Host", cfg.Host},
		{"Port", "Port", "50001"},
		{"DBPath", "DBPath", cfg.DBPath},
		{"LogLevel", "LogLevel", cfg.LogLevel},
		{"LogFormat", "LogFormat", cfg.LogFormat},
		{"LogOutput", "LogOutput", cfg.LogOutput},
		{"ClusterSource", "ClusterSource", cfg.ClusterSource},
		{"ProxmoxURL", "ProxmoxURL", cfg.ProxmoxURL},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, ok := byName[tc.fieldName]
			if !ok {
				t.Fatalf("field %s not present in Redacted() output", tc.fieldName)
			}

			if f.Redacted {
				t.Errorf("field %s: redacted = true, want false", tc.fieldName)
			}

			if f.Value != tc.wantValue {
				t.Errorf("field %s: value = %q, want %q", tc.fieldName, f.Value, tc.wantValue)
			}
		})
	}
}

// TestConfigurationRedacted_Sc006_HashNeverInOutput — SC-006 at the unit
// layer: the configured admin password hash string is never a substring of
// any field's value or name in the Redacted() output.
//
//nolint:paralleltest // serial: no shared state, but kept consistent with suite
func TestConfigurationRedacted_Sc006_HashNeverInOutput(t *testing.T) {
	hash := "$2a$10$N9qo8uLOickgx2ZMRZoMy.MrqK0aVzJqDOKU3FqZwM9FqZwM9FqZwM9"
	cfg := config.Configuration{
		AdminPasswordHash: hash,
		SessionSecret:     strings.Repeat("s", 32),
	}

	fields := cfg.Redacted()
	for _, f := range fields {
		if strings.Contains(f.Name, hash) || strings.Contains(f.Value, hash) {
			t.Fatalf("admin password hash leaked into field %+v", f)
		}
	}
}

func containsValue(fields []config.Field, secret string) bool {
	for _, f := range fields {
		if strings.Contains(f.Value, secret) || strings.Contains(f.Name, secret) {
			return true
		}
	}

	return false
}
