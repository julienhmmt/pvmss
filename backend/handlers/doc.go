// Package handlers provides HTTP request handlers for the PVMSS application.
//
// # Package Organization
//
// The handlers package is organized into logical domains, each with focused
// responsibilities. While all handlers reside in a single package for import
// simplicity, they are logically grouped by file naming convention.
//
// # Domain Groups
//
// ## VM Operations (vm_*.go)
//
// Handlers for virtual machine lifecycle and management:
//   - vm_create.go, vm_create_handler.go, vm_create_helpers.go, vm_create_render.go
//   - vm_details_*.go (base, helpers, info, metrics, render, validation)
//   - vm_actions_*.go (helpers, lifecycle, misc, resources)
//   - vm_console_*.go (api, helpers, websocket)
//   - vm_delete.go, vm_snapshots.go, vm_status_partial.go, vm_guest_agent.go
//
// ## Authentication (auth*.go)
//
// Handlers for user and admin authentication:
//   - auth.go - Main authentication handlers (login, logout)
//   - auth_guard.go - Authentication middleware and guards
//
// ## Admin Operations (admin*.go)
//
// Handlers for administrative functions:
//   - admin.go - Admin dashboard and routing
//   - admin_cloudinit.go - Cloud-init template management
//   - admin_vms.go - Admin VM management
//
// ## Settings (settings*.go)
//
// Handlers for application settings:
//   - settings.go - General settings
//   - settings_iso.go - ISO image settings
//   - settings_limits.go - Resource limits settings
//
// ## User Pool (user_pool*.go)
//
// Handlers for user pool management:
//   - user_pool.go - User pool CRUD operations
//
// ## Infrastructure
//
// Core handler infrastructure and utilities:
//   - handlers.go - Main handler registration
//   - handler_context.go - Request context management
//   - helpers.go - HTTP helper functions
//   - formatting.go - Data formatting utilities
//   - notifications.go - Notification script generation
//   - error_handling.go - Standardized error handling
//   - validation.go - Input validation
//   - sanitize.go - Input sanitization
//
// ## Middleware
//
// Request processing middleware:
//   - form_middleware.go - Form parsing middleware
//   - form_session.go - Form session handling
//   - middleware_utils.go - Middleware utilities
//   - route_helpers.go - Route helper functions
//
// ## Other Domains
//
//   - health.go - Health check endpoints
//   - search.go - Search functionality
//   - storage.go - Storage management
//   - tags.go - Tag management
//   - vmbr.go - Network bridge management
//   - disks.go - Disk management
//   - language.go - Language/i18n handling
//   - css.go - CSS serving
//
// # Error Handling
//
// All handlers use standardized error types from the pvmss/errors package.
// See error_handling.go for utilities and PHASE3_ERROR_HANDLING.md for patterns.
//
// # Testing
//
// Test files follow the *_test.go convention:
//   - auth_guard_test.go, errors_test.go, security_test.go
//   - user_pool_test.go, vm_actions_test.go, vm_create_test.go
//
// # Usage
//
// Handlers are registered via the RegisterRoutes functions in handlers.go
// and domain-specific handler structs (e.g., VMCreateOptimizedHandler).
//
// Example:
//
//	router := httprouter.New()
//	vmHandler := handlers.MakeVMCreateOptimizedHandler(stateManager)
//	vmHandler.RegisterRoutes(router)
package handlers
