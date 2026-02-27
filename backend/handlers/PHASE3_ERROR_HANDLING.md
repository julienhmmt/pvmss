# Phase 3: Error Handling Standardization

**Status**: In Progress  
**Date**: 2026-02-15

## Overview

Phase 3 standardizes error handling across the PVMSS backend using the custom error types from Phase 1. This ensures consistent error wrapping, logging, and HTTP response codes throughout the application.

## Key Components

### 1. ErrorHandler (`error_handling.go`)

Provides standardized error handling utilities for HTTP handlers.

**Main Functions**:

- `ErrorHandlerWith(log)` - Create error handler with logging
- `HandleError(w, err, message)` - Log and respond with HTTP error
- `HandleErrorWithContext(w, err, message, context)` - Log with additional context
- `codeToHTTPStatus(code)` - Convert error codes to HTTP status codes
- `WrapHandlerError(err, operation)` - Wrap handler errors with context
- `WrapProxmoxHandlerError(err, endpoint, statusCode, operation)` - Wrap Proxmox errors
- `WrapValidationError(field, value, message)` - Create validation errors
- `LogError(log, err, operation, context)` - Log errors with structured context
- `LogWarning(log, message, context)` - Log warnings with context

### 2. Error Code to HTTP Status Mapping

| Error Code | HTTP Status | Description |
|-----------|------------|-------------|
| NOT_FOUND | 404 | Resource not found |
| UNAUTHORIZED | 401 | Authentication required |
| FORBIDDEN | 403 | Permission denied |
| VALIDATION_ERROR | 400 | Invalid input |
| CONFLICT | 409 | Resource conflict |
| RATE_LIMITED | 429 | Too many requests |
| TIMEOUT | 504 | Gateway timeout |
| UNAVAILABLE | 503 | Service unavailable |
| NOT_IMPLEMENTED | 501 | Not implemented |
| PROXMOX_ERROR, VM_ERROR, AUTH_ERROR, CONFIG_ERROR | 500 | Internal server error |

## Implementation Pattern

### Before (Old Pattern)

```go
if err != nil {
    log.Error().Err(err).Msg("Failed to get VNC proxy")
    return "", 0, fmt.Errorf("failed to get VNC proxy: %w", err)
}
```

### After (New Pattern)

```go
if err != nil {
    log.Error().Err(err).Msg("Failed to get VNC proxy")
    return "", 0, errors.WrapVM(err, vmidInt, node, "failed to get VNC proxy")
}
```

## Usage Examples

### Validation Errors

```go
if vmid <= 0 {
    return errors.ValidationErr("vmid", vmid, "invalid VM ID format")
}
```

### Configuration Errors

```go
if proxmoxURL == "" {
    return errors.ConfigErr("PROXMOX_URL", "proxmox URL not configured")
}
```

### Authentication Errors

```go
if !authenticated {
    return errors.AuthErr(username, "operation", "authentication required")
}
```

### VM Operation Errors

```go
if err != nil {
    return errors.WrapVM(err, vmid, node, "failed to start VM")
}
```

### Proxmox API Errors

```go
if err != nil {
    return errors.WrapProxmox(err, endpoint, statusCode, "API call failed")
}
```

## Files Updated

### Completed

- `handlers/error_handling.go` - New error handling utilities
- `handlers/vm_console_helpers.go` - Standardized error handling for VNC proxy

### In Progress

- Update remaining handler files to use standardized errors
- Add error handling tests
- Document error handling patterns

## Best Practices

1. **Always wrap errors with context**:

   ```go
   return errors.Wrap(err, code, "operation description")
   ```

2. **Use specific error types**:

   ```go
   return errors.VMErr(vmid, node, "operation failed")
   ```

3. **Log before returning**:

   ```go
   log.Error().Err(err).Msg("operation failed")
   return errors.Wrap(err, code, "operation failed")
   ```

4. **Include relevant context**:

   ```go
   return errors.ValidationErr("field_name", value, "validation message")
   ```

5. **Check error types, not strings**:

   ```go
   if errors.IsVMError(err) { ... }
   ```

## Testing

All tests pass with standardized error handling:

```
ok      pvmss/handlers  0.317s
ok      pvmss/errors    (cached)
```

## Next Steps

1. Update remaining handler files (auth, user_pool, vm_actions, etc.)
2. Add error handling tests for critical paths
3. Document error handling in API responses
4. Create error recovery patterns for critical operations
5. Implement error metrics and monitoring
