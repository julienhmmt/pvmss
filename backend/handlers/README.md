# Handlers Package

The `handlers` package provides HTTP request handlers for the PVMSS application.

## Package Organization

The handlers package is organized into logical domains by file naming convention:

### VM Operations (`vm_*.go`)

| File Group | Description |
|------------|-------------|
| `vm_create*.go` | VM creation workflow |
| `vm_details*.go` | VM details and info display |
| `vm_actions*.go` | VM lifecycle actions (start, stop, etc.) |
| `vm_console*.go` | VNC/console access |
| `vm_snapshots.go` | Snapshot management |
| `vm_delete.go` | VM deletion |
| `vm_guest_agent.go` | Guest agent integration |

### Authentication (`auth*.go`)

| File | Description |
|------|-------------|
| `auth.go` | Login/logout handlers |
| `auth_guard.go` | Authentication middleware |

### Admin Operations (`admin*.go`)

| File | Description |
|------|-------------|
| `admin.go` | Admin dashboard |
| `admin_cloudinit.go` | Cloud-init templates |
| `admin_vms.go` | Admin VM management |

### Settings (`settings*.go`)

| File | Description |
|------|-------------|
| `settings.go` | General settings |
| `settings_iso.go` | ISO management |
| `settings_limits.go` | Resource limits |

### Infrastructure

| File | Description |
|------|-------------|
| `handlers.go` | Route registration |
| `handler_context.go` | Request context |
| `helpers.go` | HTTP utilities |
| `formatting.go` | Data formatting |
| `notifications.go` | Notification scripts |
| `error_handling.go` | Error utilities |
| `validation.go` | Input validation |
| `sanitize.go` | Input sanitization |

## Error Handling

All handlers use standardized error types from `pvmss/errors`:

```go
import "pvmss/errors"

// Validation error
return errors.ValidationErr("field", value, "message")

// VM operation error
return errors.WrapVM(err, vmid, node, "operation failed")

// Proxmox API error
return errors.WrapProxmox(err, endpoint, statusCode, "API call failed")

// Authentication error
return errors.AuthErr(username, action, "auth required")

// Configuration error
return errors.ConfigErr("KEY", "config missing")
```

## Handler Registration

Handlers are registered via `RegisterRoutes` methods:

```go
router := httprouter.New()

// VM creation
vmHandler := handlers.VMCreateOptimizedHandler(stateManager)
vmHandler.RegisterRoutes(router)

// Authentication
authHandler := handlers.AuthHandler(stateManager)
authHandler.RegisterRoutes(router)

// User pools
poolHandler := handlers.UserPoolHandler(stateManager)
poolHandler.RegisterRoutes(router)
```

## Testing

Run handler tests:

```bash
go test ./handlers/...
```

Test files:

- `auth_guard_test.go`
- `errors_test.go`
- `security_test.go`
- `user_pool_test.go`
- `vm_actions_test.go`
- `vm_create_test.go`
