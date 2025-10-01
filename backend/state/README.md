# State Management Package

This package provides a centralized and thread-safe way to manage the application state using dependency injection and interface-based design.

## Architecture

The state management system is built around these key principles:

1. **Interface-based Design**: `StateManager` interface defines all state operations
2. **Dependency Injection**: Pass the state manager to components that need it (RECOMMENDED)
3. **Thread Safety**: Built-in mutex protection for all operations
4. **Backward Compatibility**: Legacy global functions maintained with deprecation notices

## ⚠️ Important Note on Package Import

When you import `"pvmss/state"`, you're importing the **entire package** including:

- ✅ `StateManager` interface (RECOMMENDED - use this!)
- ✅ `NewAppState()` constructor
- ❌ Global helper functions in `global.go` (DEPRECATED - avoid these!)

**The package itself is NOT deprecated.** Only the global helper functions in `global.go` are deprecated.
Use dependency injection with the `StateManager` interface instead.

## Usage

### Initialization (RECOMMENDED)

```go
// Create a new instance with dependency injection (RECOMMENDED)
stateManager := state.NewAppState()

// Pass it to your handlers/components
handler := NewMyHandler(stateManager)
```

### Legacy Initialization (DEPRECATED - DO NOT USE)

```go
// DEPRECATED: Initialize global state manager (for backward compatibility only)
stateManager := state.InitGlobalState()

// DEPRECATED: Access global state directly
settings := state.GetSettings() // DON'T DO THIS!
```

### Using the State Manager

```go
// Set templates
tmpl := template.Must(template.ParseGlob("templates/*.html"))
if err := stateManager.SetTemplates(tmpl); err != nil {
    log.Fatal(err)
}

// Get templates
templates := stateManager.GetTemplates()

// Update settings
settings := &state.AppSettings{
    AdminPassword: "hashed_password_here",
    Tags:          []string{"dev", "prod"},
    // ... other settings
}
if err := stateManager.SetSettings(settings); err != nil {
    log.Fatal(err)
}
```

### Migrating from Legacy Code

| Old Function | New Method |
|-------------|------------|
| `state.GetAppSettings()` | `stateManager.GetSettings()` |
| `state.SetAppSettings(settings)` | `stateManager.SetSettings(settings)` |
| `state.GetAdminPassword()` | `stateManager.GetAdminPassword()` |
| `state.GetTags()` | `stateManager.GetTags()` |
| `state.GetISOs()` | `stateManager.GetISOs()` |
| `state.GetVMBRs()` | `stateManager.GetVMBRs()` |
| `state.GetLimits()` | `stateManager.GetLimits()` |

## Best Practices

1. **Prefer Dependency Injection**:

   ```go
   func NewMyHandler(stateManager StateManager) *MyHandler {
       return &MyHandler{state: stateManager}
   }
   ```

2. **Avoid Global State**:
   - Pass the state manager to components that need it
   - Use context for request-scoped state

3. **Error Handling**:
   - Always check errors from state operations
   - Use proper error wrapping and context

## Thread Safety

All state operations are thread-safe using read/write mutexes. The package ensures that:

- Multiple readers can access the state concurrently
- Only one writer can modify the state at a time
- No data races during state updates

## Testing

The interface-based design makes it easy to mock the state manager in tests:

```go
type MockStateManager struct {
    state.StateManager
    // Override methods as needed
}

func TestMyHandler(t *testing.T) {
    mockState := &MockStateManager{}
    handler := NewMyHandler(mockState)
    // Test with mock state
}
```
