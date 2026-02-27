# Error Contracts

**Feature**: Go Best Practices Refactoring  
**Date**: 2026-02-15

---

## Overview

This document defines the contracts for error handling in the PVMSS application.

## Error Interface Contract

All custom errors MUST implement the standard `error` interface:

```go
type error interface {
    Error() string
}
```

All custom errors SHOULD implement `Unwrap()` for error chaining:

```go
type unwrapper interface {
    Unwrap() error
}
```

## AppError Contract

**Package**: `pvmss/errors`

### Structure

```go
type AppError struct {
    Code    ErrorCode              // REQUIRED: Machine-readable code
    Message string                 // REQUIRED: Human-readable message
    Err     error                  // OPTIONAL: Wrapped error
    Details map[string]interface{} // OPTIONAL: Additional context
}
```

### Invariants

1. `Code` MUST be a valid `ErrorCode` constant
2. `Message` MUST be non-empty
3. `Error()` MUST return a string in format: `"CODE: message"` or `"CODE: message: wrapped_error"`
4. `Unwrap()` MUST return the wrapped error or nil

## VMError Contract

**Package**: `pvmss/errors`

### Structure

```go
type VMError struct {
    AppError
    VMID int    // REQUIRED: VM identifier
    Node string // REQUIRED: Proxmox node name
}
```

### Invariants

1. `VMID` MUST be a valid VM ID (positive integer)
2. `Node` MUST be a non-empty string
3. `Error()` MUST include VMID and Node in output

## ProxmoxError Contract

**Package**: `pvmss/errors`

### Structure

```go
type ProxmoxError struct {
    AppError
    Endpoint   string // REQUIRED: API endpoint
    StatusCode int    // REQUIRED: HTTP status code
}
```

### Invariants

1. `Endpoint` MUST be a non-empty string
2. `StatusCode` MUST be a valid HTTP status code (100-599)
3. `Error()` MUST include endpoint and status code

## ValidationError Contract

**Package**: `pvmss/errors`

### Structure

```go
type ValidationError struct {
    AppError
    Field string      // REQUIRED: Field name
    Value interface{} // OPTIONAL: Invalid value
}
```

### Invariants

1. `Field` MUST be a non-empty string
2. `Error()` MUST include field name

## AuthError Contract

**Package**: `pvmss/errors`

### Structure

```go
type AuthError struct {
    AppError
    Username string // REQUIRED: Username
    Action   string // REQUIRED: Attempted action
}
```

### Invariants

1. `Username` MUST be a non-empty string
2. `Action` MUST be a non-empty string
3. `Error()` MUST include username and action

## Error Code Contract

### Valid Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| NOT_FOUND | 404 | Resource not found |
| UNAUTHORIZED | 401 | Authentication required |
| FORBIDDEN | 403 | Permission denied |
| VALIDATION_ERROR | 400 | Invalid input |
| INTERNAL_ERROR | 500 | Internal server error |
| TIMEOUT | 504 | Operation timed out |
| UNAVAILABLE | 503 | Service unavailable |
| CONFLICT | 409 | Resource conflict |
| RATE_LIMITED | 429 | Too many requests |
| PROXMOX_ERROR | 502 | Proxmox API error |
| VM_ERROR | 500 | VM operation error |
| AUTH_ERROR | 401/403 | Auth error |
| CONFIG_ERROR | 500 | Configuration error |
| NOT_IMPLEMENTED | 501 | Not implemented |

## Usage Contract

### Creating Errors

```go
// MUST use constructor functions
err := apperrors.VMErr(vmid, node, message)
err := apperrors.ProxmoxErr(endpoint, statusCode, message)
err := apperrors.ValidationErr(field, value, message)
err := apperrors.AuthErr(username, action, message)

// MUST NOT create structs directly
err := &apperrors.VMError{...} // WRONG
```

### Wrapping Errors

```go
// MUST wrap with context
if err != nil {
    return apperrors.WrapVM(err, vmid, node, "operation failed")
}

// MUST NOT lose original error
if err != nil {
    return apperrors.VMErr(vmid, node, err.Error()) // WRONG - loses chain
}
```

### Checking Errors

```go
// MUST use type checking functions
if apperrors.IsVMError(err) { ... }

// SHOULD use errors.As for extracting details
var vmErr *apperrors.VMError
if errors.As(err, &vmErr) {
    // Use vmErr.VMID, vmErr.Node
}

// MUST NOT use string matching
if strings.Contains(err.Error(), "VM") { ... } // WRONG
```

## Logging Contract

Errors MUST be logged with structured context:

```go
log.Error().
    Err(err).
    Str("code", string(apperrors.GetCode(err))).
    Msg("operation failed")
```

For specific error types, include relevant fields:

```go
var vmErr *apperrors.VMError
if errors.As(err, &vmErr) {
    log.Error().
        Err(err).
        Int("vmid", vmErr.VMID).
        Str("node", vmErr.Node).
        Msg(vmErr.Message)
}
```
