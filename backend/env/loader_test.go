package env_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pvmss/env"
)

// validBase returns a map of environment variables that satisfy all required fields.
func validBase() map[string]string {
	return map[string]string{
		"JWT_SECRET":              strings.Repeat("a", 32),
		"SESSION_SECRET":          strings.Repeat("b", 32),
		"ADMIN_PASSWORD_HASH":     "$2b$12$" + strings.Repeat("x", 53),
		"PROXMOX_URL":             "https://pve.example.com:8006",
		"PROXMOX_API_TOKEN_NAME":  "root@pam!pvmss",
		"PROXMOX_API_TOKEN_VALUE": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
	}
}

// setEnv installs the provided key-value pairs as environment variables and
// returns a cleanup function that restores the previous state.
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, key := range envKeys() {
		t.Setenv(key, "")
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func envKeys() []string {
	return []string{
		"JWT_SECRET",
		"SESSION_SECRET",
		"ADMIN_PASSWORD_HASH",
		"PROXMOX_URL",
		"PROXMOX_API_TOKEN_NAME",
		"PROXMOX_API_TOKEN_VALUE",
		"PROXMOX_VERIFY_SSL",
		"PVMSS_DB_PATH",
		"PVMSS_ENV",
		"PVMSS_OFFLINE",
		"LOG_LEVEL",
		"LOG_OUTPUT",
		"LOG_FORMAT",
		"TZ",
		"PORT",
	}
}

// TestLoadAndValidate_AllRequired_Valid verifies that a fully populated,
// valid environment loads without errors.
func TestLoadAndValidate_AllRequired_Valid(t *testing.T) {
	setEnv(t, validBase())
	cfg, err := env.LoadAndValidate()
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("a", 32), cfg.JWTSecret)
	assert.Equal(t, "https://pve.example.com:8006", cfg.ProxmoxURL)
	assert.Equal(t, "root@pam!pvmss", cfg.ProxmoxAPITokenName)
}

// TestLoadAndValidate_MissingRequiredVars verifies that all missing required
// variables are reported in a single error.
func TestLoadAndValidate_MissingRequiredVars(t *testing.T) {
	tests := []struct {
		name    string
		missing string
		wantMsg string
	}{
		{"missing JWT_SECRET", "JWT_SECRET", "JWT_SECRET is required"},
		{"missing ADMIN_PASSWORD_HASH", "ADMIN_PASSWORD_HASH", "ADMIN_PASSWORD_HASH is required"},
		{"missing SESSION_SECRET", "SESSION_SECRET", "SESSION_SECRET is required"},
		{"missing PROXMOX_URL", "PROXMOX_URL", "PROXMOX_URL is required"},
		{"missing PROXMOX_API_TOKEN_NAME", "PROXMOX_API_TOKEN_NAME", "PROXMOX_API_TOKEN_NAME is required"},
		{"missing PROXMOX_API_TOKEN_VALUE", "PROXMOX_API_TOKEN_VALUE", "PROXMOX_API_TOKEN_VALUE is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vars := validBase()
			delete(vars, tc.missing)
			setEnv(t, vars)
			_, err := env.LoadAndValidate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

// TestLoadAndValidate_InvalidValues verifies individual constraint violations.
func TestLoadAndValidate_InvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantMsg string
	}{
		{
			name:    "JWT_SECRET too short",
			key:     "JWT_SECRET",
			value:   "tooshort",
			wantMsg: "JWT_SECRET must be at least 32 bytes",
		},
		{
			name:    "SESSION_SECRET too short",
			key:     "SESSION_SECRET",
			value:   "tooshort",
			wantMsg: "SESSION_SECRET must be at least 32 bytes",
		},
		{
			name:    "ADMIN_PASSWORD_HASH not bcrypt",
			key:     "ADMIN_PASSWORD_HASH",
			value:   "plaintextpassword",
			wantMsg: "ADMIN_PASSWORD_HASH must be a valid bcrypt hash",
		},
		{
			name:    "PROXMOX_URL not https",
			key:     "PROXMOX_URL",
			value:   "http://pve.example.com:8006",
			wantMsg: "must use https scheme",
		},
		{
			name:    "PROXMOX_URL missing host",
			key:     "PROXMOX_URL",
			value:   "https://",
			wantMsg: "missing host in URL",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vars := validBase()
			vars[tc.key] = tc.value
			setEnv(t, vars)
			_, err := env.LoadAndValidate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

// TestLoadAndValidate_DefaultValues verifies that optional variables fall back
// to their documented defaults when unset.
func TestLoadAndValidate_DefaultValues(t *testing.T) {
	setEnv(t, validBase())
	cfg, err := env.LoadAndValidate()
	require.NoError(t, err)

	assert.Equal(t, "pvmss.db", cfg.DBPath)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "stdout", cfg.LogOutput)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "UTC", cfg.Timezone)
	assert.Equal(t, "50000", cfg.Port)
	assert.True(t, cfg.ProxmoxSSLVerify)
	assert.False(t, cfg.Offline)
}

// TestLoadAndValidate_CustomOptionalValues verifies that optional variables
// override defaults when set.
func TestLoadAndValidate_CustomOptionalValues(t *testing.T) {
	vars := validBase()
	vars["PVMSS_DB_PATH"] = "/data/pvmss.db"
	vars["PVMSS_ENV"] = "production"
	vars["LOG_LEVEL"] = "debug"
	vars["LOG_OUTPUT"] = "stderr"
	vars["LOG_FORMAT"] = "pretty"
	vars["TZ"] = "Europe/Paris"
	vars["PROXMOX_VERIFY_SSL"] = "false"
	setEnv(t, vars)

	cfg, err := env.LoadAndValidate()
	require.NoError(t, err)

	assert.Equal(t, "/data/pvmss.db", cfg.DBPath)
	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "stderr", cfg.LogOutput)
	assert.Equal(t, "pretty", cfg.LogFormat)
	assert.Equal(t, "Europe/Paris", cfg.Timezone)
	assert.False(t, cfg.ProxmoxSSLVerify)
}

// TestLoadAndValidate_OfflineMode verifies that Proxmox variables are not
// required when PVMSS_OFFLINE=true.
func TestLoadAndValidate_OfflineMode(t *testing.T) {
	setEnv(t, map[string]string{
		"JWT_SECRET":          strings.Repeat("a", 32),
		"SESSION_SECRET":      strings.Repeat("b", 32),
		"ADMIN_PASSWORD_HASH": "$2b$12$" + strings.Repeat("x", 53),
		"PVMSS_OFFLINE":       "true",
	})
	cfg, err := env.LoadAndValidate()
	require.NoError(t, err)
	assert.True(t, cfg.Offline)
	assert.Empty(t, cfg.ProxmoxURL)
	assert.Empty(t, cfg.ProxmoxAPITokenName)
	assert.Empty(t, cfg.ProxmoxAPITokenValue)
}

// TestLoadAndValidate_OfflineMode_ProxmoxVarsOptional verifies that Proxmox
// variables may be set in offline mode without causing errors.
func TestLoadAndValidate_OfflineMode_ProxmoxVarsOptional(t *testing.T) {
	vars := validBase()
	vars["PVMSS_OFFLINE"] = "true"
	setEnv(t, vars)
	_, err := env.LoadAndValidate()
	require.NoError(t, err)
}

// TestLoadAndValidate_OfflineMode_CaseInsensitive verifies that "TRUE", "True",
// etc. are all accepted for PVMSS_OFFLINE.
func TestLoadAndValidate_OfflineMode_CaseInsensitive(t *testing.T) {
	for _, v := range []string{"true", "TRUE", "True"} {
		t.Run(v, func(t *testing.T) {
			setEnv(t, map[string]string{
				"JWT_SECRET":          strings.Repeat("a", 32),
				"SESSION_SECRET":      strings.Repeat("b", 32),
				"ADMIN_PASSWORD_HASH": "$2b$12$" + strings.Repeat("x", 53),
				"PVMSS_OFFLINE":       v,
			})
			cfg, err := env.LoadAndValidate()
			require.NoError(t, err)
			assert.True(t, cfg.Offline)
		})
	}
}

// TestLoadAndValidate_AllErrorsReportedTogether verifies that all missing required
// variables are collected and reported in a single error (fail-fast / batch reporting).
func TestLoadAndValidate_AllErrorsReportedTogether(t *testing.T) {
	_, err := env.LoadAndValidate()
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "JWT_SECRET")
	assert.Contains(t, msg, "SESSION_SECRET")
	assert.Contains(t, msg, "ADMIN_PASSWORD_HASH")
}

// TestLoadAndValidate_ProxmoxSSLVerify_DefaultTrue verifies that PROXMOX_VERIFY_SSL
// defaults to true when unset.
func TestLoadAndValidate_ProxmoxSSLVerify_DefaultTrue(t *testing.T) {
	setEnv(t, validBase())
	cfg, err := env.LoadAndValidate()
	require.NoError(t, err)
	assert.True(t, cfg.ProxmoxSSLVerify)
}

// TestLoadAndValidate_ProxmoxSSLVerify_FalseValues verifies both "false" and "0"
// disable SSL verification.
func TestLoadAndValidate_ProxmoxSSLVerify_FalseValues(t *testing.T) {
	for _, v := range []string{"false", "0", "FALSE", "False"} {
		t.Run(v, func(t *testing.T) {
			vars := validBase()
			vars["PROXMOX_VERIFY_SSL"] = v
			setEnv(t, vars)
			cfg, err := env.LoadAndValidate()
			require.NoError(t, err)
			assert.False(t, cfg.ProxmoxSSLVerify)
		})
	}
}

// TestLoadAndValidate_AdminPasswordHash_AllBcryptPrefixes verifies that all
// valid bcrypt version prefixes ($2a$, $2b$, $2y$) are accepted.
func TestLoadAndValidate_AdminPasswordHash_AllBcryptPrefixes(t *testing.T) {
	for _, prefix := range []string{"$2a$", "$2b$", "$2y$"} {
		t.Run(prefix, func(t *testing.T) {
			vars := validBase()
			vars["ADMIN_PASSWORD_HASH"] = prefix + "12$" + strings.Repeat("x", 53)
			setEnv(t, vars)
			_, err := env.LoadAndValidate()
			require.NoError(t, err)
		})
	}
}
