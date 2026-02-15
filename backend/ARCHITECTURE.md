# PVMSS Backend Architecture

This document describes the architecture and package organization of the PVMSS backend.

## Overview

PVMSS is a Go web application that provides a self-service portal for Proxmox VE. The backend follows a layered architecture with clear separation of concerns.

## Package Structure

```bash
backend/
├── app/                 # Application initialization
├── cloudinit/           # Cloud-init functionality
├── components/          # Templ templates (generated)
├── constants/           # Constants and configuration
├── errors/              # Custom error types (NEW)
├── handlers/            # HTTP request handlers
├── i18n/                # Internationalization
├── logger/              # Logging configuration
├── middleware/          # HTTP middleware
├── proxmox/             # Proxmox API client
├── security/            # Security utilities
├── state/               # Application state management
├── templates/           # Template utilities
├── tests/               # Test infrastructure
├── utils/               # Utility functions
├── main.go              # Application entry point
└── go.mod               # Go module definition
```

## Package Responsibilities

### Core Packages

#### `handlers/`

HTTP request handlers organized by domain:

- **VM handlers**: vm_create, vm_details, vm_actions, vm_snapshots
- **Admin handlers**: admin, admin_appinfo, admin_cloudinit, admin_vms
- **Auth handlers**: auth, profile
- **Settings handlers**: settings_iso, settings_limits, storage, tags, vmbr
- **Utility handlers**: common, helpers, template_data

#### `proxmox/`

Proxmox API client with typed operations:

- **client.go**: HTTP client and authentication
- **vms.go**: VM CRUD operations
- **nodes.go**: Node management
- **storage.go**: Storage operations
- **console.go**: VNC console access
- **access.go**: User and permission management

#### `state/`

Application state management:

- **manager.go**: Central state manager
- **interface.go**: StateManager interface
- **settings.go**: Settings loading and persistence
- **proxmox_status.go**: Proxmox connection status

#### `errors/` (NEW)

Custom error types for consistent error handling:

- **AppError**: Base error type with code and message
- **VMError**: VM-specific errors with VMID and node
- **ProxmoxError**: API errors with endpoint and status
- **ValidationError**: Input validation errors
- **AuthError**: Authentication/authorization errors
- **ConfigError**: Configuration errors

### Support Packages

#### `utils/`

Generic utility functions:

- **generics.go**: Type-safe utilities (Optional, Result, Cache, slice operations)
- **errors.go**: Error handling utilities
- **env.go**: Environment detection

#### `middleware/`

HTTP middleware:

- Rate limiting
- Request logging
- Security headers

#### `security/`

Security utilities:

- CSRF protection
- Session management
- Input sanitization

#### `templates/`

Template utilities:

- Function maps
- Type conversions
- HTTP helpers

#### `tests/`

Testing infrastructure:

- **helpers.go**: Test utilities and assertions
- **benchmarks_test.go**: Performance benchmarks
- **integration_test.go**: Integration tests
- **routes_test.go**: Route tests

## Design Principles

### 1. Single Responsibility

Each package and file has a clear, focused purpose. Large files (>500 lines) should be split into focused submodules.

### 2. Error Handling

- Use custom error types from `errors/` package
- Always wrap errors with context using `fmt.Errorf("%w", err)`
- Use `errors.Is` and `errors.As` for error type checking
- Log errors with structured context

### 3. Type Safety

- Prefer concrete types over `interface{}`
- Use generics for reusable utilities
- Define typed structs for all data structures

### 4. Concurrency

- Use `context.Context` for cancellation and timeouts
- Manage goroutine lifecycle properly
- Use sync primitives appropriately

### 5. Testing

- Use table-driven tests
- Aim for >70% code coverage
- Include benchmarks for performance-sensitive code

## Data Flow

```bash
HTTP Request
    │
    ▼
┌─────────────┐
│  Middleware │  (auth, logging, security)
└─────────────┘
    │
    ▼
┌─────────────┐
│  Handlers   │  (request handling, validation)
└─────────────┘
    │
    ▼
┌─────────────┐
│   State     │  (application state, settings)
└─────────────┘
    │
    ▼
┌─────────────┐
│  Proxmox    │  (API client)
└─────────────┘
    │
    ▼
Proxmox VE API
```

## Error Handling Pattern

```go
import apperrors "pvmss/errors"

// Creating errors
err := apperrors.VMErr(vmid, node, "failed to start")

// Wrapping errors
err := apperrors.WrapProxmox(apiErr, endpoint, statusCode, "API call failed")

// Checking error types
if apperrors.IsVMError(err) {
    // Handle VM-specific error
}

var vmErr *apperrors.VMError
if errors.As(err, &vmErr) {
    log.Error().Int("vmid", vmErr.VMID).Msg(vmErr.Message)
}
```

## Generic Utilities

```go
import "pvmss/utils"

// Optional values
opt := utils.Some(42)
if opt.IsPresent() {
    value, _ := opt.Get()
}

// Cache with TTL
cache := utils.CacheWith[string, int](time.Minute, 100)
cache.Set("key", 42)
value, ok := cache.Get("key")

// Slice operations
filtered := utils.Filter(slice, func(n int) bool { return n > 0 })
mapped := utils.MapSlice(slice, func(n int) int { return n * 2 })
sum := utils.Reduce(slice, 0, func(acc, n int) int { return acc + n })
```

## Testing Pattern

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   int
        want    int
        wantErr bool
    }{
        {
            name:  "positive input",
            input: 5,
            want:  10,
        },
        {
            name:    "negative input",
            input:   -1,
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Configuration

Configuration is managed through:

- **Environment variables**: Secrets, API tokens, URLs
- **settings.json**: Application settings (tags, limits, storages)
- **constants/**: Compile-time constants

## Logging

Structured logging with zerolog:

```go
log.Info().
    Str("component", "handlers").
    Int("vmid", vmid).
    Msg("VM created successfully")
```

## Future Improvements

1. **Split large handlers**: vm_create.go, user_pool.go, auth.go
2. **Increase test coverage**: Target >70% overall
3. **Reduce interface{} usage**: Use typed template data
4. **Add benchmarks**: Performance-sensitive operations
5. **Improve documentation**: 100% exported function coverage
