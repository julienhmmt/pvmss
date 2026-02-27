# Data Model: Go Best Practices Refactoring

**Feature**: Go Best Practices Refactoring  
**Branch**: `001-go-best-practices-refactor`  
**Date**: 2026-02-15

---

## Overview

This document defines the data models and types introduced during the Go best practices refactoring. These types provide the foundation for consistent error handling, type-safe utilities, and improved code organization.

## Error Types

### Error Codes

```go
type ErrorCode string

const (
    CodeNotFound       ErrorCode = "NOT_FOUND"
    CodeUnauthorized   ErrorCode = "UNAUTHORIZED"
    CodeForbidden      ErrorCode = "FORBIDDEN"
    CodeValidation     ErrorCode = "VALIDATION_ERROR"
    CodeInternal       ErrorCode = "INTERNAL_ERROR"
    CodeTimeout        ErrorCode = "TIMEOUT"
    CodeUnavailable    ErrorCode = "UNAVAILABLE"
    CodeConflict       ErrorCode = "CONFLICT"
    CodeRateLimited    ErrorCode = "RATE_LIMITED"
    CodeProxmox        ErrorCode = "PROXMOX_ERROR"
    CodeVM             ErrorCode = "VM_ERROR"
    CodeAuth           ErrorCode = "AUTH_ERROR"
    CodeConfig         ErrorCode = "CONFIG_ERROR"
    CodeNotImplemented ErrorCode = "NOT_IMPLEMENTED"
)
```

### AppError (Base Type)

```go
type AppError struct {
    Code    ErrorCode
    Message string
    Err     error                  // Wrapped error
    Details map[string]interface{} // Additional context
}
```

**Usage**: Base error type for all application errors. Supports error wrapping and additional context.

### VMError

```go
type VMError struct {
    AppError
    VMID int
    Node string
}
```

**Usage**: Errors related to VM operations. Includes VM identifier and node for debugging.

### ProxmoxError

```go
type ProxmoxError struct {
    AppError
    Endpoint   string
    StatusCode int
}
```

**Usage**: Errors from Proxmox API calls. Includes endpoint and HTTP status code.

### ValidationError

```go
type ValidationError struct {
    AppError
    Field string
    Value interface{}
}
```

**Usage**: Input validation errors. Includes the field name and invalid value.

### AuthError

```go
type AuthError struct {
    AppError
    Username string
    Action   string
}
```

**Usage**: Authentication and authorization errors. Includes username and attempted action.

### ConfigError

```go
type ConfigError struct {
    AppError
    Key string
}
```

**Usage**: Configuration errors. Includes the configuration key that caused the error.

## Generic Utility Types

### Optional[T]

```go
type Optional[T any] struct {
    value   T
    present bool
}
```

**Usage**: Represents a value that may or may not be present. Eliminates nil pointer checks.

**Methods**:

- `Some[T](value T) Optional[T]` - Create with value
- `None[T]() Optional[T]` - Create empty
- `IsPresent() bool` - Check if value exists
- `Get() (T, bool)` - Get value and presence
- `GetOrDefault(T) T` - Get value or default
- `GetOrElse(func() T) T` - Get value or compute

### Result[T]

```go
type Result[T any] struct {
    value T
    err   error
}
```

**Usage**: Represents the result of an operation that may fail. Combines value and error.

**Methods**:

- `Ok[T](value T) Result[T]` - Create success
- `Err[T](err error) Result[T]` - Create failure
- `IsOk() bool` - Check success
- `IsErr() bool` - Check failure
- `Unwrap() (T, error)` - Get both
- `UnwrapOr(T) T` - Get value or default

### Cache[K, V]

```go
type Cache[K comparable, V any] struct {
    mu      sync.RWMutex
    items   map[K]cacheItem[V]
    ttl     time.Duration
    maxSize int
}
```

**Usage**: Thread-safe cache with TTL and size limits.

**Methods**:

- `CacheWith[K, V](ttl, maxSize)` - Create cache
- `Get(key K) (V, bool)` - Retrieve value
- `Set(key K, value V)` - Store value
- `SetWithTTL(key K, value V, ttl)` - Store with custom TTL
- `Delete(key K)` - Remove value
- `Clear()` - Remove all
- `Size() int` - Get count
- `GetOrSet(key K, fn func() V) V` - Get or compute

## Slice Utility Functions

### Filter

```go
func Filter[T any](slice []T, predicate func(T) bool) []T
```

Returns elements matching the predicate.

### MapSlice

```go
func MapSlice[T, U any](slice []T, fn func(T) U) []U
```

Transforms each element.

### Reduce

```go
func Reduce[T, U any](slice []T, initial U, fn func(U, T) U) U
```

Reduces to a single value.

### Find

```go
func Find[T any](slice []T, predicate func(T) bool) Optional[T]
```

Finds first matching element.

### Contains

```go
func Contains[T comparable](slice []T, element T) bool
```

Checks if element exists.

### Unique

```go
func Unique[T comparable](slice []T) []T
```

Removes duplicates.

### GroupBy

```go
func GroupBy[T any, K comparable](slice []T, keyFn func(T) K) map[K][]T
```

Groups elements by key.

### First / Last

```go
func First[T any](slice []T) Optional[T]
func Last[T any](slice []T) Optional[T]
```

Gets first/last element safely.

## Map Utility Functions

### Keys / Values

```go
func Keys[K comparable, V any](m map[K]V) []K
func Values[K comparable, V any](m map[K]V) []V
```

Extracts keys or values from a map.

## Pointer Utility Functions

### Ptr

```go
func Ptr[T any](v T) *T
```

Returns pointer to value.

### Deref / DerefOr

```go
func Deref[T any](p *T) T
func DerefOr[T any](p *T, defaultValue T) T
```

Safely dereferences pointers.

### Coalesce

```go
func Coalesce[T comparable](values ...T) T
```

Returns first non-zero value.

## Relationships

```bash
AppError (base)
    ├── VMError (embeds AppError)
    ├── ProxmoxError (embeds AppError)
    ├── ValidationError (embeds AppError)
    ├── AuthError (embeds AppError)
    └── ConfigError (embeds AppError)

Optional[T] ←── Find, First, Last, Map
Result[T] ←── Ok, Err
Cache[K,V] ←── NewCache, GetOrSet
```

## Migration Guide

### From interface{} to Typed Errors

**Before**:

```go
return fmt.Errorf("VM %d failed: %v", vmid, err)
```

**After**:

```go
return errors.WrapVMError(err, vmid, node, "operation failed")
```

### From nil Checks to Optional

**Before**:

```go
if result != nil {
    // use result
}
```

**After**:

```go
opt := utils.Find(slice, predicate)
if opt.IsPresent() {
    value, _ := opt.Get()
    // use value
}
```

### From Manual Caching to Cache[K,V]

**Before**:

```go
var cache = make(map[string]interface{})
var mu sync.RWMutex
// manual locking and expiration
```

**After**:

```go
cache := utils.NewCache[string, MyType](time.Minute, 100)
value, ok := cache.Get("key")
```
