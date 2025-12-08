package constants

import (
	"testing"
	"time"
)

func assertEqualString(t *testing.T, name string, got string, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

func assertEqualInt(t *testing.T, name string, got int, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %d, want %d", name, got, want)
	}
}

func assertEqualDuration(t *testing.T, name string, got time.Duration, want time.Duration) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %s, want %s", name, got, want)
	}
}

func TestDefaultValues(t *testing.T) {
	assertEqualString(t, "DefaultPort", DefaultPort, "50000")
	assertEqualString(t, "DefaultLogLevel", DefaultLogLevel, "info")
	assertEqualString(t, "DefaultLanguage", DefaultLanguage, "en")
	assertEqualString(t, "DefaultLoginRealm", DefaultLoginRealm, "pve")
	assertEqualString(t, "AppVersion", AppVersion, "0.3.0")
}

func TestValidationLimits(t *testing.T) {
	assertEqualInt(t, "MaxUsernameLength", MaxUsernameLength, 100)
	assertEqualInt(t, "MaxPasswordLength", MaxPasswordLength, 200)
	assertEqualInt(t, "MinPasswordLength", MinPasswordLength, 5)
	assertEqualInt(t, "MaxVMNameLength", MaxVMNameLength, 100)
}

func TestHTTPConfiguration(t *testing.T) {
	assertEqualInt(t, "MaxFormSize", MaxFormSize, 10*1024*1024)
	assertEqualInt(t, "MaxHeaderBytes", MaxHeaderBytes, 1<<20)
	assertEqualDuration(t, "ServerReadTimeout", ServerReadTimeout, 10*time.Second)
	assertEqualDuration(t, "ServerWriteTimeout", ServerWriteTimeout, 30*time.Second)
	assertEqualDuration(t, "ServerIdleTimeout", ServerIdleTimeout, 120*time.Second)
	assertEqualDuration(t, "ServerReadHeaderTimeout", ServerReadHeaderTimeout, 5*time.Second)
	assertEqualInt(t, "HTTPMaxIdleConns", HTTPMaxIdleConns, 100)
	assertEqualInt(t, "HTTPMaxIdleConnsPerHost", HTTPMaxIdleConnsPerHost, 50)
	assertEqualDuration(t, "HTTPIdleConnTimeout", HTTPIdleConnTimeout, 90*time.Second)
	assertEqualDuration(t, "HTTPTLSHandshakeTimeout", HTTPTLSHandshakeTimeout, 10*time.Second)
	assertEqualDuration(t, "HTTPExpectContinueTimeout", HTTPExpectContinueTimeout, 1*time.Second)
	assertEqualDuration(t, "HTTPResponseHeaderTimeout", HTTPResponseHeaderTimeout, 15*time.Second)
}

func TestContextTimeouts(t *testing.T) {
	assertEqualDuration(t, "DefaultContextTimeout", DefaultContextTimeout, 10*time.Second)
	assertEqualDuration(t, "LongContextTimeout", LongContextTimeout, 30*time.Second)
	assertEqualDuration(t, "ShortContextTimeout", ShortContextTimeout, 5*time.Second)
	assertEqualDuration(t, "FetchVMsTimeout", FetchVMsTimeout, 15*time.Second)
}

func TestProxmoxConfiguration(t *testing.T) {
	assertEqualDuration(t, "ProxmoxDefaultTimeout", ProxmoxDefaultTimeout, 10*time.Second)
	assertEqualDuration(t, "ProxmoxCacheTTL", ProxmoxCacheTTL, 2*time.Minute)
	assertEqualDuration(t, "ProxmoxConnectionCheckInterval", ProxmoxConnectionCheckInterval, 30*time.Second)
	assertEqualDuration(t, "ProxmoxConnectionCheckTimeout", ProxmoxConnectionCheckTimeout, 5*time.Second)
	assertEqualDuration(t, "ProxmoxOfflineThreshold", ProxmoxOfflineThreshold, 2*time.Minute)
	assertEqualDuration(t, "NodeCacheRefreshInterval", NodeCacheRefreshInterval, 30*time.Second)
	assertEqualDuration(t, "NodeCacheRequestTimeout", NodeCacheRequestTimeout, 10*time.Second)
	assertEqualDuration(t, "NodeCacheStaleThreshold", NodeCacheStaleThreshold, 2*time.Minute)
	assertEqualDuration(t, "ClusterCacheRefreshInterval", ClusterCacheRefreshInterval, 45*time.Second)
	assertEqualDuration(t, "ClusterCacheRequestMinRefreshInterval", ClusterCacheRequestMinRefreshInterval, 30*time.Second)
	assertEqualDuration(t, "ClusterCacheRequestTimeout", ClusterCacheRequestTimeout, 20*time.Second)
	assertEqualDuration(t, "ConsoleSessionTTL", ConsoleSessionTTL, 8*time.Second)
	assertEqualDuration(t, "VNCTicketValidityDuration", VNCTicketValidityDuration, 2*time.Hour)
	assertEqualDuration(t, "VNCTicketSafetyMargin", VNCTicketSafetyMargin, 5*time.Minute)
	assertEqualDuration(t, "GuestAgentCacheTTL", GuestAgentCacheTTL, 1*time.Minute)
	assertEqualDuration(t, "GuestAgentTimeout", GuestAgentTimeout, 3*time.Second)
	assertEqualDuration(t, "GuestAgentShutdownPollInterval", GuestAgentShutdownPollInterval, 3*time.Second)
	assertEqualInt(t, "GuestAgentShutdownMaxAttempts", GuestAgentShutdownMaxAttempts, 5)
}

func TestSecurityConfiguration(t *testing.T) {
	assertEqualInt(t, "CSRFTokenLength", CSRFTokenLength, 32)
	assertEqualDuration(t, "CSRFTokenTTL", CSRFTokenTTL, 30*time.Minute)
	assertEqualDuration(t, "CSRFCleanupInterval", CSRFCleanupInterval, 30*time.Minute)
	assertEqualDuration(t, "SessionCleanupInterval", SessionCleanupInterval, 30*time.Minute)
	assertEqualDuration(t, "RateLimitWindow", RateLimitWindow, 5*time.Minute)
	assertEqualDuration(t, "RateLimitCleanup", RateLimitCleanup, 15*time.Minute)
	assertEqualInt(t, "LoginRateLimitCapacity", LoginRateLimitCapacity, 5)
	assertEqualDuration(t, "LoginRateLimitRefill", LoginRateLimitRefill, 12*time.Second)
	assertEqualString(t, "SessionKeyAuthenticated", SessionKeyAuthenticated, "authenticated")
	assertEqualString(t, "SessionKeyIsAdmin", SessionKeyIsAdmin, "is_admin")
	assertEqualString(t, "SessionKeyUsername", SessionKeyUsername, "username")
	assertEqualString(t, "SessionKeyCSRFToken", SessionKeyCSRFToken, "csrf_token")
	assertEqualString(t, "SessionKeyPVEAuthCookie", SessionKeyPVEAuthCookie, "pve_auth_cookie")
	assertEqualString(t, "SessionKeyPVECSRFToken", SessionKeyPVECSRFToken, "pve_csrf_token")
	assertEqualString(t, "SessionKeyPVEUsername", SessionKeyPVEUsername, "pve_username")
	assertEqualString(t, "SessionKeyPVETicketCreated", SessionKeyPVETicketCreated, "pve_ticket_created")
}

func TestMessageKeysNotEmpty(t *testing.T) {
	messageKeys := map[string]string{
		"MsgAuthRequired":           MsgAuthRequired,
		"MsgAuthInvalidCreds":       MsgAuthInvalidCreds,
		"MsgAuthLoginSuccess":       MsgAuthLoginSuccess,
		"MsgAuthLogoutSuccess":      MsgAuthLogoutSuccess,
		"MsgAuthSessionExpired":     MsgAuthSessionExpired,
		"MsgAuthUnauthorized":       MsgAuthUnauthorized,
		"MsgAuthForbidden":          MsgAuthForbidden,
		"MsgErrorGeneric":           MsgErrorGeneric,
		"MsgErrorNotFound":          MsgErrorNotFound,
		"MsgErrorBadRequest":        MsgErrorBadRequest,
		"MsgErrorInternalServer":    MsgErrorInternalServer,
		"MsgErrorInvalidInput":      MsgErrorInvalidInput,
		"MsgErrorInvalidVMID":       MsgErrorInvalidVMID,
		"MsgVMCreated":              MsgVMCreated,
		"MsgVMCreateError":          MsgVMCreateError,
		"MsgVMDeleted":              MsgVMDeleted,
		"MsgVMDeleteError":          MsgVMDeleteError,
		"MsgVMActionSuccess":        MsgVMActionSuccess,
		"MsgVMActionFailed":         MsgVMActionFailed,
		"MsgVMNotFound":             MsgVMNotFound,
		"MsgVMUpdateSuccess":        MsgVMUpdateSuccess,
		"MsgVMStarted":              MsgVMStarted,
		"MsgVMStopped":              MsgVMStopped,
		"MsgVMRebooted":             MsgVMRebooted,
		"MsgFormInvalidInput":       MsgFormInvalidInput,
		"MsgFormMissingField":       MsgFormMissingField,
		"MsgFormSaveSuccess":        MsgFormSaveSuccess,
		"MsgFormSaveError":          MsgFormSaveError,
		"MsgFormValidationError":    MsgFormValidationError,
		"MsgAdminNodeAdded":         MsgAdminNodeAdded,
		"MsgAdminNodeRemoved":       MsgAdminNodeRemoved,
		"MsgAdminNodeError":         MsgAdminNodeError,
		"MsgAdminSettingsSaved":     MsgAdminSettingsSaved,
		"MsgAdminSettingsError":     MsgAdminSettingsError,
		"MsgAdminTagCreated":        MsgAdminTagCreated,
		"MsgAdminTagDeleted":        MsgAdminTagDeleted,
		"MsgAdminTagError":          MsgAdminTagError,
		"MsgProfileUpdated":         MsgProfileUpdated,
		"MsgProfileUpdateError":     MsgProfileUpdateError,
		"MsgPasswordChanged":        MsgPasswordChanged,
		"MsgPasswordChangeError":    MsgPasswordChangeError,
		"MsgProxmoxConnected":       MsgProxmoxConnected,
		"MsgProxmoxDisconnected":    MsgProxmoxDisconnected,
		"MsgProxmoxError":           MsgProxmoxError,
		"MsgProxmoxTimeout":         MsgProxmoxTimeout,
		"MsgProxmoxOfflineMode":     MsgProxmoxOfflineMode,
		"MsgProxmoxAutoOfflineMode": MsgProxmoxAutoOfflineMode,
		"MsgProxmoxClientNil":       MsgProxmoxClientNil,
		"MsgConsoleOpened":          MsgConsoleOpened,
		"MsgConsoleError":           MsgConsoleError,
		"MsgConsoleUnavailable":     MsgConsoleUnavailable,
		"MsgConsoleTicketError":     MsgConsoleTicketError,
		"MsgLimitExceeded":          MsgLimitExceeded,
		"MsgLimitCPU":               MsgLimitCPU,
		"MsgLimitMemory":            MsgLimitMemory,
		"MsgLimitDisk":              MsgLimitDisk,
		"MsgLimitVMCount":           MsgLimitVMCount,
		"MsgStorageAdded":           MsgStorageAdded,
		"MsgStorageRemoved":         MsgStorageRemoved,
		"MsgStorageError":           MsgStorageError,
		"MsgStorageFull":            MsgStorageFull,
		"MsgISOAdded":               MsgISOAdded,
		"MsgISORemoved":             MsgISORemoved,
		"MsgISOError":               MsgISOError,
		"MsgISONotFound":            MsgISONotFound,
		"MsgNetworkBridgeAdded":     MsgNetworkBridgeAdded,
		"MsgNetworkBridgeRemoved":   MsgNetworkBridgeRemoved,
		"MsgNetworkBridgeError":     MsgNetworkBridgeError,
		"MsgNetworkConfigError":     MsgNetworkConfigError,
		"MsgSuccess":                MsgSuccess,
		"MsgOperationComplete":      MsgOperationComplete,
		"MsgPleaseWait":             MsgPleaseWait,
		"MsgProcessing":             MsgProcessing,
		"MsgWarningGeneric":         MsgWarningGeneric,
		"MsgWarningConfirm":         MsgWarningConfirm,
		"MsgWarningIrreversible":    MsgWarningIrreversible,
	}

	for name, value := range messageKeys {
		if value == "" {
			t.Fatalf("%s is empty, expected non-empty message key", name)
		}
	}
}

func TestSessionKeyUniqueness(t *testing.T) {
	sessionKeys := []string{
		SessionKeyAuthenticated,
		SessionKeyIsAdmin,
		SessionKeyUsername,
		SessionKeyCSRFToken,
		SessionKeyPVEAuthCookie,
		SessionKeyPVECSRFToken,
		SessionKeyPVEUsername,
		SessionKeyPVETicketCreated,
	}

	seen := make(map[string]bool)
	for _, key := range sessionKeys {
		if seen[key] {
			t.Fatalf("duplicate session key value detected: %s", key)
		}
		seen[key] = true
	}
}
