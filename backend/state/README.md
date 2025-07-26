# State Management Package

This package provides a centralized and thread-safe way to manage the application state using dependency injection and interface-based design.

## New Architecture

The new state management system is built around these key principles:

1. **Interface-based Design**: `StateManager` interface defines all state operations
2. **Dependency Injection**: Pass the state manager to components that need it
3. **Thread Safety**: Built-in mutex protection for all operations
4. **Backward Compatibility**: Legacy functions maintained with deprecation notices

## Usage

### Initialization

```go
// Initialize the global state manager (for backward compatibility)
stateManager := state.InitGlobalState()

// Or create a new instance for dependency injection
stateManager := state.NewAppState()
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
