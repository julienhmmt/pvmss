// Package constants defines standardized i18n message keys used throughout the application.
// This ensures consistency and makes it easier to track which messages are used where.
package constants

// Authentication & Authorization Messages
const (
	MsgAuthRequired       = "Auth.Required"
	MsgAuthInvalidCreds   = "Auth.InvalidCredentials"
	MsgAuthLoginSuccess   = "Auth.LoginSuccess"
	MsgAuthLogoutSuccess  = "Auth.LogoutSuccess"
	MsgAuthSessionExpired = "Auth.SessionExpired"
	MsgAuthUnauthorized   = "Auth.Unauthorized"
	MsgAuthForbidden      = "Auth.Forbidden"
)

// Generic Error Messages
const (
	MsgErrorGeneric        = "Error.Generic"
	MsgErrorNotFound       = "Error.NotFound"
	MsgErrorBadRequest     = "Error.BadRequest"
	MsgErrorInternalServer = "Error.InternalServer"
	MsgErrorInvalidInput   = "Error.InvalidInput"
	MsgErrorInvalidVMID    = "Error.InvalidVMID"
)

// VM Operation Messages
const (
	MsgVMCreated       = "VM.Created"
	MsgVMCreateError   = "VM.CreateError"
	MsgVMDeleted       = "VMDelete.Success"
	MsgVMDeleteError   = "VMDelete.Error"
	MsgVMActionSuccess = "VMDetails.Action.Success"
	MsgVMActionFailed  = "Message.ActionFailed"
	MsgVMNotFound      = "VM.NotFound"
	MsgVMUpdateSuccess = "Message.UpdatedSuccessfully"
	MsgVMStarted       = "VM.Started"
	MsgVMStopped       = "VM.Stopped"
	MsgVMRebooted      = "VM.Rebooted"
)

// Form & Validation Messages
const (
	MsgFormInvalidInput    = "Form.InvalidInput"
	MsgFormMissingField    = "Form.MissingField"
	MsgFormSaveSuccess     = "Form.SaveSuccess"
	MsgFormSaveError       = "Form.SaveError"
	MsgFormValidationError = "Form.ValidationError"
)

// Admin Messages
const (
	MsgAdminNodeAdded     = "Admin.Node.Added"
	MsgAdminNodeRemoved   = "Admin.Node.Removed"
	MsgAdminNodeError     = "Admin.Node.Error"
	MsgAdminSettingsSaved = "Admin.Settings.Saved"
	MsgAdminSettingsError = "Admin.Settings.Error"
	MsgAdminTagCreated    = "Admin.Tag.Created"
	MsgAdminTagDeleted    = "Admin.Tag.Deleted"
	MsgAdminTagError      = "Admin.Tag.Error"
)

// Profile & User Messages
const (
	MsgProfileUpdated      = "Profile.Updated"
	MsgProfileUpdateError  = "Profile.UpdateError"
	MsgPasswordChanged     = "Profile.PasswordChanged"
	MsgPasswordChangeError = "Profile.PasswordChangeError"
)

// Proxmox Connection Messages
const (
	MsgProxmoxConnected    = "Proxmox.Connected"
	MsgProxmoxDisconnected = "Proxmox.Disconnected"
	MsgProxmoxError        = "Proxmox.Error"
	MsgProxmoxTimeout      = "Proxmox.Timeout"
	MsgProxmoxOfflineMode  = "Proxmox.OfflineModeEnabled"
	MsgProxmoxClientNil    = "Proxmox.ClientNotInitialized"
)

// Console Messages
const (
	MsgConsoleOpened      = "Console.Opened"
	MsgConsoleError       = "Console.Error"
	MsgConsoleUnavailable = "Console.Unavailable"
	MsgConsoleTicketError = "Console.TicketError"
)

// Resource Limit Messages
const (
	MsgLimitExceeded = "Limit.Exceeded"
	MsgLimitCPU      = "Limit.CPU"
	MsgLimitMemory   = "Limit.Memory"
	MsgLimitDisk     = "Limit.Disk"
	MsgLimitVMCount  = "Limit.VMCount"
)

// Storage Messages
const (
	MsgStorageAdded   = "Storage.Added"
	MsgStorageRemoved = "Storage.Removed"
	MsgStorageError   = "Storage.Error"
	MsgStorageFull    = "Storage.Full"
)

// ISO Messages
const (
	MsgISOAdded    = "ISO.Added"
	MsgISORemoved  = "ISO.Removed"
	MsgISOError    = "ISO.Error"
	MsgISONotFound = "ISO.NotFound"
)

// Network Messages
const (
	MsgNetworkBridgeAdded   = "Network.Bridge.Added"
	MsgNetworkBridgeRemoved = "Network.Bridge.Removed"
	MsgNetworkBridgeError   = "Network.Bridge.Error"
	MsgNetworkConfigError   = "Network.ConfigError"
)

// Success/Info Messages
const (
	MsgSuccess           = "Message.Success"
	MsgOperationComplete = "Message.OperationComplete"
	MsgPleaseWait        = "Message.PleaseWait"
	MsgProcessing        = "Message.Processing"
)

// Warning Messages
const (
	MsgWarningGeneric      = "Warning.Generic"
	MsgWarningConfirm      = "Warning.Confirm"
	MsgWarningIrreversible = "Warning.Irreversible"
)
