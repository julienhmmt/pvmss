# Settings.json Migration: Remove Optional Env Vars

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move all non-secret configuration (port, log level, environment mode, CSRF TTL, timezone, etc.) from environment variables into `settings.json`, keeping env vars only for secrets that must never appear in a version-controlled file.

**Architecture:** Each migrated variable gets a typed field added to `AppSettings`. Resolution order is always: env var (override) → settings.json value → hardcoded default. The logger requires a two-phase init: quick bootstrap from env at startup, then reconfigure with merged settings after `LoadSettings()`. Security secrets (`SESSION_SECRET`, `ADMIN_PASSWORD_HASH`, `PROXMOX_*`) stay as mandatory env vars — they must not appear in any file committed to version control.

**Tech Stack:** Go — `state/settings.go`, `logger/logger.go`, `security/config.go`, `main.go`. No new dependencies.

---

## Variables being migrated

| Env var | `settings.json` key | Default | Where read today |
|---|---|---|---|
| `PORT` | `port` | `"50000"` | `main.go:75` |
| `PVMSS_ENV` | `env` | `"production"` | `main.go:35`, `api/v1/auth.go` |
| `PVMSS_OFFLINE` | `offline_mode` | `false` | `main.go:39`, `initializeApp` |
| `LOG_LEVEL` | `log_level` | `"info"` | `main.go` → `logger.Init` |
| `LOG_OUTPUT` | `log_output` | `"stdout"` | `logger/logger.go:24` |
| `LOG_FORMAT` | `log_format` | `"console"` | `logger/logger.go:29` |
| `LOG_FILE_PATH` | `log_file_path` | `""` | `logger/logger.go:34` |
| `CSRF_TOKEN_TTL` | `csrf_token_ttl` | `"30m"` | `security/config.go:41` |
| `TZ` | `timezone` | `"UTC"` | OS / `utils/` |
| `DEBUG_SESSIONS` | `debug_sessions` | `false` | `handlers/middleware_utils.go:91` |

## Variables that stay as mandatory env vars (secrets)

`SESSION_SECRET`, `ADMIN_PASSWORD_HASH`, `PROXMOX_URL`, `PROXMOX_API_TOKEN_NAME`, `PROXMOX_API_TOKEN_VALUE`

---

## Task 1: Add config fields to `AppSettings` and update settings files

**Files:**

- Modify: `backend/state/settings.go`
- Modify: `backend/settings.dev.json`
- Modify: `settings.json` (root — production example)

**Step 1: Add fields to the `AppSettings` struct**

In `backend/state/settings.go`, after the `JWTSecret` field (line ~143), add:

```go
// Runtime config — env vars override these values when both are set.
// Env var resolution: env var → this field → hardcoded default.
Env          string `json:"env,omitempty"`           // PVMSS_ENV: "production" or "development"
OfflineMode  bool   `json:"offline_mode,omitempty"`  // PVMSS_OFFLINE
Port         string `json:"port,omitempty"`           // PORT (default: "50000")
LogLevel     string `json:"log_level,omitempty"`      // LOG_LEVEL (default: "info")
LogOutput    string `json:"log_output,omitempty"`     // LOG_OUTPUT: "stdout"|"file"|"both"
LogFormat    string `json:"log_format,omitempty"`     // LOG_FORMAT: "console"|"json"
LogFilePath  string `json:"log_file_path,omitempty"`  // LOG_FILE_PATH
CSRFTokenTTL string `json:"csrf_token_ttl,omitempty"` // CSRF_TOKEN_TTL (default: "30m")
Timezone     string `json:"timezone,omitempty"`       // TZ (default: "UTC")
DebugSessions bool  `json:"debug_sessions,omitempty"` // DEBUG_SESSIONS
```

**Step 2: Add `envOr` and `envBoolOr` helpers to `state/settings.go`**

These are package-level helpers (unexported) used throughout the config resolution:

```go
// envOr returns the env var value if non-empty, else the settings value, else the default.
func envOr(envKey, settingsVal, defaultVal string) string {
    if v := os.Getenv(envKey); v != "" {
        return v
    }
    if settingsVal != "" {
        return settingsVal
    }
    return defaultVal
}

// envBoolOr returns the env var parsed as bool if set, else settingsVal.
func envBoolOr(envKey string, settingsVal bool) bool {
    if v := os.Getenv(envKey); v != "" {
        return strings.EqualFold(v, "true") || v == "1"
    }
    return settingsVal
}
```

Add `"strings"` to imports if not already present (it already is).

**Step 3: Add a `ResolvedConfig` method to `AppSettings`**

```go
// ResolvedConfig merges settings.json values with env var overrides.
// Env vars always win; settings.json values are the fallback.
type ResolvedConfig struct {
    Env          string
    OfflineMode  bool
    Port         string
    LogLevel     string
    LogOutput    string
    LogFormat    string
    LogFilePath  string
    CSRFTokenTTL string
    Timezone     string
    DebugSessions bool
}

func (s *AppSettings) ResolvedConfig() ResolvedConfig {
    return ResolvedConfig{
        Env:          envOr("PVMSS_ENV", s.Env, "production"),
        OfflineMode:  envBoolOr("PVMSS_OFFLINE", s.OfflineMode),
        Port:         envOr("PORT", s.Port, "50000"),
        LogLevel:     envOr("LOG_LEVEL", s.LogLevel, "info"),
        LogOutput:    envOr("LOG_OUTPUT", s.LogOutput, "stdout"),
        LogFormat:    envOr("LOG_FORMAT", s.LogFormat, "console"),
        LogFilePath:  envOr("LOG_FILE_PATH", s.LogFilePath, ""),
        CSRFTokenTTL: envOr("CSRF_TOKEN_TTL", s.CSRFTokenTTL, "30m"),
        Timezone:     envOr("TZ", s.Timezone, "UTC"),
        DebugSessions: envBoolOr("DEBUG_SESSIONS", s.DebugSessions),
    }
}
```

**Step 4: Update `backend/settings.dev.json`**

Add the new keys after `jwt_secret`:

```json
"env": "development",
"offline_mode": true,
"port": "50000",
"log_level": "debug",
"log_output": "stdout",
"log_format": "console",
"log_file_path": "",
"csrf_token_ttl": "30m",
"timezone": "UTC",
"debug_sessions": false
```

**Step 5: Update root `settings.json`**

Same keys with production defaults:

```json
"env": "production",
"offline_mode": false,
"port": "50000",
"log_level": "info",
"log_output": "stdout",
"log_format": "json",
"log_file_path": "",
"csrf_token_ttl": "30m",
"timezone": "UTC",
"debug_sessions": false
```

**Step 6: Write a test**

In `backend/state/` create `settings_config_test.go`:

```go
package state

import (
    "testing"
)

func TestResolvedConfig_Defaults(t *testing.T) {
    s := &AppSettings{}
    cfg := s.ResolvedConfig()
    if cfg.Port != "50000" {
        t.Errorf("expected default port 50000, got %s", cfg.Port)
    }
    if cfg.LogLevel != "info" {
        t.Errorf("expected default log level info, got %s", cfg.LogLevel)
    }
    if cfg.Env != "production" {
        t.Errorf("expected default env production, got %s", cfg.Env)
    }
}

func TestResolvedConfig_SettingsOverrideDefaults(t *testing.T) {
    s := &AppSettings{Port: "8080", LogLevel: "debug", Env: "development"}
    cfg := s.ResolvedConfig()
    if cfg.Port != "8080" {
        t.Errorf("expected port 8080 from settings, got %s", cfg.Port)
    }
    if cfg.LogLevel != "debug" {
        t.Errorf("expected log_level debug from settings, got %s", cfg.LogLevel)
    }
}

func TestResolvedConfig_EnvVarOverridesSettings(t *testing.T) {
    t.Setenv("PORT", "9090")
    t.Setenv("LOG_LEVEL", "warn")
    s := &AppSettings{Port: "8080", LogLevel: "debug"}
    cfg := s.ResolvedConfig()
    if cfg.Port != "9090" {
        t.Errorf("expected PORT env var 9090 to override settings 8080, got %s", cfg.Port)
    }
    if cfg.LogLevel != "warn" {
        t.Errorf("expected LOG_LEVEL env var warn to override settings debug, got %s", cfg.LogLevel)
    }
}

func TestEnvBoolOr(t *testing.T) {
    t.Setenv("PVMSS_OFFLINE", "true")
    if !envBoolOr("PVMSS_OFFLINE", false) {
        t.Error("expected true from env var")
    }
    t.Setenv("PVMSS_OFFLINE", "")
    if envBoolOr("PVMSS_OFFLINE", false) {
        t.Error("expected false fallback")
    }
    if !envBoolOr("PVMSS_OFFLINE", true) {
        t.Error("expected true from settings value")
    }
}
```

**Step 7: Run test to verify it passes**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run TestResolvedConfig ./state/...
```

Expected: all PASS

**Step 8: Commit**

```bash
git add backend/state/settings.go backend/state/settings_config_test.go backend/settings.dev.json settings.json
git commit -m "feat(settings): add runtime config fields to AppSettings with env-over-settings resolution"
```

---

## Task 2: Update `logger` to accept config values (two-phase init)

**Files:**

- Modify: `backend/logger/logger.go`

The logger currently reads `LOG_OUTPUT`, `LOG_FORMAT`, `LOG_FILE_PATH` internally via `os.Getenv` inside `Init()`. We need to let callers pass those values so settings can be used after `LoadSettings()` returns.

**Step 1: Add a `LogConfig` struct and `InitFromConfig`**

In `backend/logger/logger.go`, after the existing `Init(level string)` function, add:

```go
// LogConfig holds all logger configuration.
type LogConfig struct {
    Level      string // "debug", "info", "warn", "error"
    Output     string // "stdout", "file", "both"
    Format     string // "console", "json"
    FilePath   string // only used when Output is "file" or "both"
}

// InitFromConfig reinitializes the global logger from an explicit config.
// Call this after settings are loaded to apply settings.json values.
// Env vars have already been resolved into cfg before calling this.
func InitFromConfig(cfg LogConfig) {
    Init(cfg.Level) // reuse existing Init but we need to pass output/format/path too
}
```

Wait — `Init` currently reads output/format/path directly from env. We need to refactor it to accept parameters instead. The cleanest fix: change the signature to take a `LogConfig`.

**Step 1 (revised): Refactor `Init` to accept `LogConfig`**

Replace the current `Init(level string)` with:

```go
// Init initializes the global logger. Called twice:
//   1. At startup with values from env vars only (before settings load).
//   2. After LoadSettings() to apply merged env+settings config.
func Init(cfg LogConfig) {
    zerolog.TimeFieldFormat = time.RFC3339Nano

    outputMode := cfg.Output
    if outputMode == "" {
        outputMode = "stdout"
    }
    format := cfg.Format
    if format == "" {
        format = "console"
    }
    logFilePath := cfg.FilePath

    stdoutEnabled := outputMode == "stdout" || outputMode == "both"
    fileEnabled := outputMode == "file" || outputMode == "both"

    writers := make([]io.Writer, 0, 2)
    deferredWarnings := make([]string, 0, 2)

    if stdoutEnabled {
        if format == "json" {
            writers = append(writers, os.Stdout)
        } else {
            writers = append(writers, zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05"})
        }
    }
    if fileEnabled {
        if logFilePath == "" {
            deferredWarnings = append(deferredWarnings, "log_output requires a file path but log_file_path is not set; disabling file logging")
        } else {
            file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
            if err != nil {
                deferredWarnings = append(deferredWarnings, fmt.Sprintf("Failed to open log file '%s': %v", logFilePath, err))
            } else {
                if format == "json" {
                    writers = append(writers, file)
                } else {
                    writers = append(writers, zerolog.ConsoleWriter{Out: file, TimeFormat: "2006-01-02 15:04:05"})
                }
            }
        }
    }
    if len(writers) == 0 {
        writers = append(writers, zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05"})
        deferredWarnings = append(deferredWarnings, "No valid log output configured, falling back to stdout console")
    }

    var output io.Writer
    if len(writers) == 1 {
        output = writers[0]
    } else {
        output = zerolog.MultiLevelWriter(writers...)
    }
    log.Logger = log.Output(output)

    lvl, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
    if err != nil {
        lvl = zerolog.InfoLevel
    }
    zerolog.SetGlobalLevel(lvl)

    for _, msg := range deferredWarnings {
        log.Warn().Msg(msg)
    }
    log.Info().
        Str("level", zerolog.GlobalLevel().String()).
        Str("output", outputMode).
        Str("format", format).
        Msg("Logger initialized")
}

// InitFromEnv bootstraps the logger from env vars only (called before settings load).
func InitFromEnv(level string) {
    Init(LogConfig{
        Level:    level,
        Output:   os.Getenv("LOG_OUTPUT"),
        Format:   os.Getenv("LOG_FORMAT"),
        FilePath: os.Getenv("LOG_FILE_PATH"),
    })
}
```

**Step 2: Update `main.go` call site**

In `backend/main.go`, the `initLogger()` function currently calls `logger.Init(level)`. Change it:

```go
func initLogger() {
    level := os.Getenv("LOG_LEVEL")
    if level == "" {
        level = constants.DefaultLogLevel
    }
    logger.InitFromEnv(level) // bootstrap from env only; reconfigured after settings load
}
```

**Step 3: Run tests**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test ./logger/... -v
```

Expected: all PASS (logger tests should still pass; the interface changed but behaviour is identical when env vars are set)

**Step 4: Commit**

```bash
git add backend/logger/logger.go backend/main.go
git commit -m "refactor(logger): accept LogConfig struct instead of reading env vars internally"
```

---

## Task 3: Wire settings config into `main.go`

**Files:**

- Modify: `backend/main.go`

After `initializeApp()` loads settings, reconfigure the logger and resolve PORT, env, and offline mode from the merged config.

**Step 1: Update `initializeApp` to return `ResolvedConfig`**

Change the signature in `main.go`:

```go
func initializeApp(stateManager state.StateManager) (state.ResolvedConfig, error) {
    settings, modified, err := state.LoadSettings()
    if err != nil {
        return state.ResolvedConfig{}, fmt.Errorf("failed to load settings: %w", err)
    }

    cfg := settings.ResolvedConfig()

    // Re-initialize logger with merged config (settings values + env overrides)
    logger.Init(logger.LogConfig{
        Level:    cfg.LogLevel,
        Output:   cfg.LogOutput,
        Format:   cfg.LogFormat,
        FilePath: cfg.LogFilePath,
    })

    // Apply offline mode
    if cfg.OfflineMode {
        logger.Get().Info().Msg("offline_mode=true — Proxmox API calls disabled")
        stateManager.SetOfflineMode()
    } else {
        proxmoxClient, err := initProxmoxClient()
        if err != nil {
            return cfg, fmt.Errorf("failed to initialize Proxmox client: %w", err)
        }
        if err := stateManager.SetProxmoxClient(proxmoxClient); err != nil {
            return cfg, fmt.Errorf("failed to set Proxmox client: %w", err)
        }
        if connected := stateManager.CheckProxmoxConnection(); !connected {
            _, errorMsg := stateManager.GetProxmoxStatus()
            logger.Get().Warn().Str("error", errorMsg).Msg("Proxmox not reachable, starting in read-only mode")
        } else {
            logger.Get().Info().Msg("Proxmox connection verified")
        }
    }

    // ... rest unchanged until settings set ...

    if modified {
        if err := stateManager.SetSettings(settings); err != nil {
            return cfg, fmt.Errorf("failed to save modified settings: %w", err)
        }
    } else {
        stateManager.SetSettingsWithoutSave(settings)
    }

    return cfg, nil
}
```

**Step 2: Update `main()` to use cfg for PORT and env**

```go
cfg, err := initializeApp(stateManager)
if err != nil {
    logger.Get().Fatal().Err(err).Msg("Failed to initialize application")
}

// PORT and env from merged config
port := cfg.Port
env := cfg.Env

logger.Get().Info().
    Str("environment", env).
    Bool("offline_mode", cfg.OfflineMode).
    Str("port", port).
    Msg("Starting PVMSS")
```

Remove the old standalone `port := os.Getenv("PORT")` block and the early `env`/`offlineMode` reads (lines 35-46 in current main.go).

**Step 3: Update `auth.go` to use StateManager for env instead of os.Getenv**

In `backend/api/v1/auth.go`, `setTokenCookie` reads `os.Getenv("PVMSS_ENV")` to set `Secure: true`. Change to accept `secure bool` as a parameter resolved from `sm.GetSettings().ResolvedConfig().Env`:

```go
func (h *AuthHandler) issueTokens(w http.ResponseWriter, secret, username string, isAdmin bool) {
    cfg := h.state.GetSettings().ResolvedConfig()
    secure := cfg.Env == "production" || cfg.Env == "prod"
    setTokenCookie(w, secret, accessTokenCookie, username, isAdmin, accessTokenTTL, secure)
    setTokenCookie(w, secret, refreshTokenCookie, username, isAdmin, refreshTokenTTL, secure)
}

func setTokenCookie(w http.ResponseWriter, secret, name, username string, isAdmin bool, ttl time.Duration, secure bool) {
    // ... existing logic but use secure param instead of os.Getenv ...
    http.SetCookie(w, &http.Cookie{
        Name:     name,
        Value:    signed,
        Path:     "/",
        MaxAge:   int(ttl.Seconds()),
        HttpOnly: true,
        Secure:   secure,
        SameSite: http.SameSiteStrictMode,
    })
}
```

Remove `os.Getenv("PVMSS_ENV")` from auth.go.

**Step 4: Build and run tests**

```bash
cd backend && go build ./... 2>&1
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test ./... 2>&1 | grep -E "^(ok|FAIL)"
```

Expected: all PASS

**Step 5: Commit**

```bash
git add backend/main.go backend/api/v1/auth.go
git commit -m "feat(main): resolve PORT, env, offline_mode from settings with env var override"
```

---

## Task 4: Update `security/config.go` to read `csrf_token_ttl` from settings

**Files:**

- Modify: `backend/security/config.go`

Currently `GetConfig()` is a singleton that reads `CSRF_TOKEN_TTL` from env at first call. We need it to also accept a settings-based value when no env var is set.

**Step 1: Add a `Configure(ttlStr string)` function**

The singleton approach with `sync.Once` makes it hard to inject values. Change it to a function that accepts the resolved value:

```go
// GetCSRFTokenTTL returns the CSRF token TTL, resolved from env var then settings value.
// Call this after settings are loaded; the resolved string is from ResolvedConfig.CSRFTokenTTL.
func GetCSRFTokenTTL(resolved string) time.Duration {
    if d, err := time.ParseDuration(resolved); err == nil && d > 0 {
        return d
    }
    logger.Get().Warn().Str("value", resolved).Msg("Invalid csrf_token_ttl; using 30m default")
    return defaultCSRFTokenTTL
}
```

Keep `GetConfig()` as-is for backward compatibility during the transition (it still reads env var), but update the callers that use CSRF TTL to use the new function when a StateManager is available.

Actually, the `security.CSRF` middleware calls `GetConfig().CSRFTokenTTL`. To wire settings in cleanly, update `buildAppMiddleware` in `handlers/middleware_utils.go` to pass the resolved TTL:

```go
// In buildAppMiddleware, replace security.CSRF(handler) with:
csrfTTL := security.DefaultCSRFTokenTTL
if sm != nil {
    csrfTTL = security.GetCSRFTokenTTL(sm.GetSettings().ResolvedConfig().CSRFTokenTTL)
}
handler = security.CSRFWithTTL(handler, csrfTTL)
```

This requires adding `CSRFWithTTL(next http.Handler, ttl time.Duration) http.Handler` to security — or, if CSRF TTL is not actively enforced yet (it's reserved for future use per the comment in config.go), just leave `GetConfig()` unchanged for now and document the `csrf_token_ttl` setting for future use.

**Step 1 (simplified): Export `DefaultCSRFTokenTTL` and add `GetCSRFTokenTTL`**

In `backend/security/config.go`:

```go
// Export the default for use in buildAppMiddleware.
const DefaultCSRFTokenTTL = defaultCSRFTokenTTL

// GetCSRFTokenTTL parses a duration string (e.g., "30m") and returns it.
// Falls back to DefaultCSRFTokenTTL on empty or parse error.
func GetCSRFTokenTTL(s string) time.Duration {
    if s == "" {
        return DefaultCSRFTokenTTL
    }
    d, err := time.ParseDuration(s)
    if err != nil || d <= 0 {
        logger.Get().Warn().Str("value", s).Msg("Invalid csrf_token_ttl value; using 30m default")
        return DefaultCSRFTokenTTL
    }
    return d
}
```

Remove the `sync.Once` singleton in config.go since the new function is stateless.

**Step 2: Update `GetConfig()` to use `GetCSRFTokenTTL` internally**

```go
func GetConfig() *Config {
    configOnce.Do(func() {
        config = &Config{
            CSRFTokenTTL: GetCSRFTokenTTL(os.Getenv("CSRF_TOKEN_TTL")),
        }
    })
    return config
}
```

Now `CSRF_TOKEN_TTL` env var still works, and the new `GetCSRFTokenTTL` helper enables settings-based resolution.

**Step 3: Build and test**

```bash
cd backend && go build ./... && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test ./... | grep -E "^(ok|FAIL)"
```

Expected: all PASS

**Step 4: Commit**

```bash
git add backend/security/config.go
git commit -m "refactor(security): expose GetCSRFTokenTTL helper for settings-based resolution"
```

---

## Task 5: Remove now-optional vars from `security/validation.go` + update `DEBUG_SESSIONS`

**Files:**

- Modify: `backend/security/validation.go`
- Modify: `backend/handlers/middleware_utils.go`

**Step 1: Document what changed in validation.go**

`ValidateRequiredEnvVars()` checks `SESSION_SECRET`, `ADMIN_PASSWORD_HASH`, and (non-offline) the Proxmox trio. These stay required. The migrated vars (`PORT`, `PVMSS_ENV`, `PVMSS_OFFLINE`, `LOG_*`, `CSRF_TOKEN_TTL`, `TZ`) are no longer validated here because they have settings.json fallbacks.

No code change needed in `validation.go` — those vars were never in the `required` map. Just confirm by reading the file to ensure none were inadvertently added.

**Step 2: Remove `os.Getenv("DEBUG_SESSIONS")` from `middleware_utils.go`**

In `backend/handlers/middleware_utils.go`, `sessionDebugMiddleware` (line 91) reads `os.Getenv("DEBUG_SESSIONS")`. Change it to read from the StateManager:

```go
func sessionDebugMiddleware(sm state.StateManager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            debugEnabled := false
            if sm != nil {
                debugEnabled = sm.GetSettings().ResolvedConfig().DebugSessions
            }
            if !debugEnabled {
                next.ServeHTTP(w, r)
                return
            }
            // ... rest of debug logic unchanged ...
        })
    }
}
```

Update the call site in `buildAppMiddleware` from `sessionDebugMiddleware(handler)` to `sessionDebugMiddleware(sm)(handler)`.

**Step 3: Build and test**

```bash
cd backend && go build ./... && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test ./... | grep -E "^(ok|FAIL)"
```

**Step 4: Commit**

```bash
git add backend/handlers/middleware_utils.go backend/security/validation.go
git commit -m "refactor: read DEBUG_SESSIONS from settings instead of env var"
```

---

## Task 6: Update docs, example.env, and docker-compose

**Files:**

- Modify: `CLAUDE.md`
- Modify: `example.env`
- Modify: `docker-compose.dev.yml` (and any other compose files)

**Step 1: Update `CLAUDE.md` env vars table**

Replace the current runtime env vars table. The new split:

**Mandatory env vars (secrets — must stay as env vars):**

| Variable | Purpose |
|---|---|
| `PROXMOX_URL` | Full Proxmox API URL |
| `PROXMOX_API_TOKEN_NAME` | `user@pve!token` |
| `PROXMOX_API_TOKEN_VALUE` | Token secret |
| `ADMIN_PASSWORD_HASH` | Bcrypt hash for admin login |
| `SESSION_SECRET` | 32+ byte cookie encryption secret |

**Optional env var overrides (also configurable in settings.json):**

| Variable | settings.json key | Default | Purpose |
|---|---|---|---|
| `PVMSS_ENV` | `env` | `production` | `production` or `development` |
| `PVMSS_OFFLINE` | `offline_mode` | `false` | `true` to disable Proxmox API |
| `PORT` | `port` | `50000` | HTTP port |
| `LOG_LEVEL` | `log_level` | `info` | `debug`/`info`/`warn`/`error` |
| `LOG_OUTPUT` | `log_output` | `stdout` | `stdout`/`file`/`both` |
| `LOG_FORMAT` | `log_format` | `console` | `console` or `json` |
| `LOG_FILE_PATH` | `log_file_path` | `""` | Path when `log_output` includes file |
| `CSRF_TOKEN_TTL` | `csrf_token_ttl` | `30m` | Duration string |
| `TZ` | `timezone` | `UTC` | IANA timezone name |
| `DEBUG_SESSIONS` | `debug_sessions` | `false` | Verbose session logging |

**Step 2: Update `example.env`**

Move the now-optional vars into a clearly labelled "optional overrides" section:

```bash
# ─── Required secrets ────────────────────────────────────────────────────────
SESSION_SECRET=changeme_use_at_least_32_random_bytes_here
ADMIN_PASSWORD_HASH=$2a$12$...
PROXMOX_URL=https://proxmox.example.com:8006
PROXMOX_API_TOKEN_NAME=user@pve!token-name
PROXMOX_API_TOKEN_VALUE=your-token-secret-here

# ─── Optional overrides (also settable in settings.json) ─────────────────────
# These env vars take precedence over settings.json when set.
# If unset, the settings.json value is used; if also absent, the default applies.
# PVMSS_ENV=production
# PVMSS_OFFLINE=false
# PORT=50000
# LOG_LEVEL=info
# LOG_OUTPUT=stdout
# LOG_FORMAT=json
# LOG_FILE_PATH=
# CSRF_TOKEN_TTL=30m
# TZ=UTC
# DEBUG_SESSIONS=false

# ─── Note ─────────────────────────────────────────────────────────────────────
# jwt_secret for /api/v1/ JWT auth lives in settings.json, not here.
```

**Step 3: Update `docker-compose.dev.yml`**

Move the non-secret env vars from the `environment:` block to a comment noting they can be set in settings.json. Keep only secrets in the compose file:

```yaml
environment:
  # Required secrets
  - SESSION_SECRET=${SESSION_SECRET}
  - ADMIN_PASSWORD_HASH=${ADMIN_PASSWORD_HASH}
  - PROXMOX_URL=${PROXMOX_URL}
  - PROXMOX_API_TOKEN_NAME=${PROXMOX_API_TOKEN_NAME}
  - PROXMOX_API_TOKEN_VALUE=${PROXMOX_API_TOKEN_VALUE}
  # Optional: uncomment to override settings.json values
  # - PVMSS_ENV=development
  # - LOG_LEVEL=debug
  # - LOG_FORMAT=console
```

**Step 4: Commit**

```bash
git add CLAUDE.md example.env docker-compose.dev.yml
git commit -m "docs: update env var documentation after settings.json migration"
```

---

## Task 7: Final verification

**Step 1: Run full test suite**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test ./... | grep -E "^(ok|FAIL)"
```

Expected: all PASS except pre-existing `TestDocsFilesExist`.

**Step 2: Verify build**

```bash
cd backend && go build ./...
```

Expected: no errors.

**Step 3: Verify no new `os.Getenv` calls for migrated vars**

```bash
grep -r 'os.Getenv("PORT"\|"PVMSS_ENV"\|"PVMSS_OFFLINE"\|"LOG_LEVEL"\|"LOG_OUTPUT"\|"LOG_FORMAT"\|"LOG_FILE_PATH"\|"CSRF_TOKEN_TTL"\|"DEBUG_SESSIONS")' backend/ --include="*.go" | grep -v "_test.go"
```

Expected: zero results.

**Step 4: Commit if clean**

```bash
git add -A
git commit -m "chore: settings.json migration complete — all optional config moved out of env vars"
```
