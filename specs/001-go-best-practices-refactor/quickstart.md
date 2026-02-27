# Quickstart: Go Best Practices Refactoring

**Feature**: Go Best Practices Refactoring  
**Branch**: `001-go-best-practices-refactor`  
**Date**: 2026-02-15

---

## Overview

This guide provides quick examples for using the new error types and generic utilities introduced in the Go best practices refactoring.

## Error Handling

### Import

```go
import apperrors "pvmss/errors"
```

### Creating Errors

```go
// Simple error
err := apperrors.AppErr(apperrors.CodeInternal, "something went wrong")

// VM error
err := apperrors.VMErr(100, "pve1", "failed to start VM")

// Proxmox API error
err := apperrors.ProxmoxErr("/api2/json/nodes", 500, "server error")

// Validation error
err := apperrors.ValidationErr("cpu_cores", -1, "must be positive")

// Auth error
err := apperrors.AuthErr("admin", "login", "invalid credentials")
```

### Wrapping Errors

```go
// Wrap any error with context
if err != nil {
    return apperrors.Wrap(err, apperrors.CodeInternal, "operation failed")
}

// Wrap as VM error
if err != nil {
    return apperrors.WrapVM(err, vmid, node, "failed to start")
}

// Wrap as Proxmox error
if err != nil {
    return apperrors.WrapProxmox(err, endpoint, resp.StatusCode, "API call failed")
}
```

### Checking Error Types

```go
// Check specific error types
if apperrors.IsVMError(err) {
    // Handle VM-specific error
}

if apperrors.IsProxmoxError(err) {
    // Handle Proxmox API error
}

if apperrors.IsValidation(err) {
    // Handle validation error
}

// Check sentinel errors
if apperrors.IsNotFound(err) {
    // Handle not found
}

if apperrors.IsUnauthorized(err) {
    // Handle unauthorized
}
```

### Extracting Error Details

```go
import "errors"

// Extract VM error details
var vmErr *apperrors.VMError
if errors.As(err, &vmErr) {
    log.Error().
        Int("vmid", vmErr.VMID).
        Str("node", vmErr.Node).
        Msg(vmErr.Message)
}

// Get error code
code := apperrors.GetCode(err)
```

### Adding Context

```go
err := apperrors.New(apperrors.CodeInternal, "operation failed").
    WithDetail("vmid", 100).
    WithDetail("operation", "start")
```

## Generic Utilities

### Imports

```go
import "pvmss/utils"
```

### Optional Values

```go
// Create optional
opt := utils.Some(42)
empty := utils.None[int]()

// Check and get
if opt.IsPresent() {
    value, _ := opt.Get()
    fmt.Println(value) // 42
}

// Get with default
value := opt.GetOrDefault(0) // 42
value := empty.GetOrDefault(0) // 0

// Get with function
value := empty.GetOrElse(func() int { return computeDefault() })
```

### Result Type

```go
// Create results
success := utils.Ok(42)
failure := utils.Err[int](errors.New("failed"))

// Check and use
if success.IsOk() {
    value, _ := success.Unwrap()
    fmt.Println(value)
}

// Get with default
value := failure.UnwrapOr(0) // 0
```

### Cache

```go
// Create cache with 5-minute TTL and max 100 items
cache := utils.CacheWith[string, User](5*time.Minute, 100)

// Set and get
cache.Set("user:123", user)
user, ok := cache.Get("user:123")

// Get or compute
user := cache.GetOrSet("user:123", func() User {
    return fetchUserFromDB("123")
})

// Custom TTL
cache.SetWithTTL("session:abc", session, 30*time.Minute)

// Clear
cache.Delete("user:123")
cache.Clear()
```

### Slice Operations

```go
numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

// Filter
evens := utils.Filter(numbers, func(n int) bool {
    return n%2 == 0
})
// [2, 4, 6, 8, 10]

// Map
doubled := utils.MapSlice(numbers, func(n int) int {
    return n * 2
})
// [2, 4, 6, 8, 10, 12, 14, 16, 18, 20]

// Reduce
sum := utils.Reduce(numbers, 0, func(acc, n int) int {
    return acc + n
})
// 55

// Find
found := utils.Find(numbers, func(n int) bool {
    return n > 5
})
if found.IsPresent() {
    value, _ := found.Get() // 6
}

// Contains
hasThree := utils.Contains(numbers, 3) // true

// Unique
unique := utils.Unique([]int{1, 2, 2, 3, 3, 3})
// [1, 2, 3]

// First/Last
first := utils.First(numbers) // Some(1)
last := utils.Last(numbers)   // Some(10)
```

### GroupBy

```go
type Person struct {
    Name string
    Age  int
}

people := []Person{
    {"Alice", 30},
    {"Bob", 25},
    {"Charlie", 30},
}

byAge := utils.GroupBy(people, func(p Person) int {
    return p.Age
})
// map[25:[Bob] 30:[Alice, Charlie]]
```

### Map Operations

```go
m := map[string]int{"a": 1, "b": 2, "c": 3}

keys := utils.Keys(m)     // ["a", "b", "c"] (order varies)
values := utils.Values(m) // [1, 2, 3] (order varies)
```

### Pointer Utilities

```go
// Create pointer
ptr := utils.Ptr(42)

// Safe dereference
value := utils.Deref(ptr)        // 42
value := utils.Deref[int](nil)   // 0 (zero value)

// Dereference with default
value := utils.DerefOr(ptr, 0)   // 42
value := utils.DerefOr(nil, 99)  // 99

// Coalesce (first non-zero)
result := utils.Coalesce("", "", "hello", "world")
// "hello"
```

## Testing Helpers

### Imports 2

```go
import "pvmss/tests"
```

### HTTP Request Builder

```go
func TestMyHandler(t *testing.T) {
    handler := http.HandlerFunc(MyHandler)
    
    resp := tests.NewRequest(t, "POST", "/api/vms").
        WithBody(map[string]interface{}{"name": "test"}).
        WithHeader("X-Custom", "value").
        WithAuth("token123").
        Execute(handler)
    
    resp.AssertStatus(t, http.StatusOK).
        AssertBodyContains(t, "success")
}
```

### Assertions

```go
tests.AssertEqual(t, got, want, "values should match")
tests.AssertNotEqual(t, got, notWant, "values should differ")
tests.AssertNil(t, err, "error should be nil")
tests.AssertNotNil(t, result, "result should exist")
tests.AssertError(t, err, "should return error")
tests.AssertNoError(t, err, "should not error")
tests.AssertTrue(t, condition, "should be true")
tests.AssertFalse(t, condition, "should be false")
tests.AssertLen(t, slice, 3, "should have 3 items")
tests.AssertContains(t, slice, item, "should contain item")
```

### Table-Driven Tests

```go
func TestDouble(t *testing.T) {
    tests := []tests.TableTest[int, int]{
        {Name: "positive", Input: 5, Expected: 10},
        {Name: "zero", Input: 0, Expected: 0},
        {Name: "negative", Input: -3, Expected: -6},
    }
    
    tests.RunTableTests(t, tests, func(input int) (int, error) {
        return input * 2, nil
    })
}
```

## Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./tests/

# Run specific benchmark
go test -bench=BenchmarkCache -benchmem ./tests/

# Run with memory profiling
go test -bench=. -benchmem -memprofile=mem.out ./tests/
```

## Best Practices

### Error Handling 2

1. **Always wrap errors with context**:

   ```go
   return apperrors.Wrap(err, apperrors.CodeInternal, "failed to process request")
   ```

2. **Use specific error types**:

   ```go
   return apperrors.NewVMError(vmid, node, "VM not found")
   ```

3. **Check error types, not strings**:

   ```go
   if apperrors.IsVMError(err) { ... }
   ```

### Type Safety

1. **Prefer Optional over nil**:

   ```go
   func FindUser(id string) utils.Optional[User]
   ```

2. **Use Cache for thread-safe caching**:

   ```go
   cache := utils.CacheWith[string, Data](time.Minute, 100)
   ```

3. **Use slice utilities for clarity**:

   ```go
   filtered := utils.Filter(items, predicate)
   ```

### Testing

1. **Use table-driven tests**:

   ```go
   tests := []struct{ ... }{ ... }
   for _, tt := range tests { ... }
   ```

2. **Use test helpers**:

   ```go
   tests.AssertEqual(t, got, want, "message")
   ```

3. **Run benchmarks for performance-sensitive code**:

   ```bash
   go test -bench=. -benchmem ./...
   ```
