# Research: Go Best Practices Refactoring

**Feature**: Go Best Practices Refactoring  
**Branch**: `001-go-best-practices-refactor`  
**Date**: 2026-02-15  
**Status**: Complete

---

## Executive Summary

This research document analyzes the PVMSS backend codebase to establish baseline metrics and identify refactoring priorities. The analysis reveals a functional but organically-grown codebase with opportunities for improvement in code organization, test coverage, error handling patterns, and type safety.

**Key Findings**:

- **Total Go code**: ~64,000 lines (33,500 excluding generated templ files)
- **Test coverage**: Low overall (~3-5% in handlers, ~85% in cloudinit)
- **Linter status**: ✅ 0 issues (golangci-lint passes cleanly)
- **Files >500 lines**: 12 handler files need splitting
- **interface{} usage**: 302 occurrences (reduction target: >80%)
- **Error wrapping**: 142 uses of `%w` (good foundation, needs expansion)

---

## 1. Code Structure Analysis

### 1.1 Files Exceeding 500 Lines (Excluding Generated)

| File | Lines | Priority | Recommendation |
|------|-------|----------|----------------|
| `handlers/vm_create.go` | 1,162 | HIGH | Split into vm_create_form.go, vm_create_validation.go, vm_create_submit.go |
| `handlers/user_pool.go` | 931 | HIGH | Split into user_pool_handlers.go, user_pool_helpers.go |
| `handlers/search.go` | 882 | MEDIUM | Extract search logic into service layer |
| `handlers/auth.go` | 881 | HIGH | Split into auth_handlers.go, auth_helpers.go, auth_session.go |
| `handlers/storage.go` | 825 | MEDIUM | Extract storage operations into service |
| `handlers/vm_create_handler.go` | 797 | HIGH | Consolidate with vm_create.go split |
| `tests/routes_test.go` | 758 | LOW | Acceptable for test file |
| `handlers/helpers.go` | 688 | HIGH | Split by domain (template, validation, formatting) |
| `handlers/vm_details_info.go` | 673 | MEDIUM | Extract info gathering logic |
| `handlers/admin_cloudinit.go` | 642 | MEDIUM | Extract cloudinit operations |
| `handlers/settings_iso.go` | 621 | MEDIUM | Extract ISO management logic |
| `proxmox/vms.go` | 607 | MEDIUM | Split into vms_crud.go, vms_status.go, vms_config.go |
| `proxmox/cloudinit.go` | 570 | LOW | Acceptable, focused responsibility |
| `handlers/limits_helpers.go` | 564 | MEDIUM | Consolidate with settings_limits.go |
| `handlers/settings_limits.go` | 525 | MEDIUM | Merge helpers, extract validation |
| `handlers/vm_actions_resources.go` | 521 | MEDIUM | Good split already done |
| `handlers/profile.go` | 506 | MEDIUM | Extract profile operations |

**Total files >500 lines**: 17 (12 in handlers/, 2 in proxmox/, 1 in tests/)

### 1.2 Package Distribution

| Package | Files | Lines (approx) | Purpose |
|---------|-------|----------------|---------|
| handlers/ | 64 | ~15,000 | HTTP request handlers |
| components/ | 86 | ~30,000 | Templ templates (generated) |
| proxmox/ | 19 | ~4,500 | Proxmox API client |
| state/ | 9 | ~1,500 | Application state management |
| i18n/ | 57 | ~3,000 | Internationalization |
| tests/ | 11 | ~2,000 | Test infrastructure |
| utils/ | 5 | ~500 | Utility functions |
| middleware/ | 4 | ~400 | HTTP middleware |
| security/ | 7 | ~600 | Security utilities |
| constants/ | 8 | ~400 | Constants and config |
| templates/ | 9 | ~500 | Template utilities |
| logger/ | 2 | ~300 | Logging configuration |
| cloudinit/ | 2 | ~200 | Cloud-init functionality |
| app/ | 1 | ~100 | Application initialization |

### 1.3 Circular Dependencies

**Status**: ✅ No circular dependencies detected

The `go mod graph` output shows clean dependency relationships. All imports flow in expected directions:

- main → handlers, state, proxmox
- handlers → proxmox, state, utils, templates
- proxmox → (external only)
- state → proxmox (interface only)

---

## 2. Test Coverage Analysis

### 2.1 Current Coverage by Package

| Package | Coverage | Status | Priority |
|---------|----------|--------|----------|
| cloudinit | 85.7% | ✅ Good | LOW |
| middleware | 78.7% | ✅ Good | LOW |
| logger | 70.7% | ✅ Good | LOW |
| utils | 29.2% | ⚠️ Needs improvement | MEDIUM |
| proxmox | 4.4% | ❌ Critical | HIGH |
| handlers | 3.3% | ❌ Critical | HIGH |
| main | 0.0% | ❌ Critical | MEDIUM |
| app | 0.0% | ❌ Critical | MEDIUM |
| components | 0.0% | N/A (generated) | SKIP |
| i18n | 0.0% | N/A (data) | SKIP |
| security | 0.0% | ❌ Critical | HIGH |
| state | 0.0% | ❌ Critical | HIGH |
| templates | 0.0% | ⚠️ Needs improvement | MEDIUM |

### 2.2 Coverage Gaps

**Critical gaps** (security/reliability impact):

1. **handlers/** - Core business logic untested
2. **proxmox/** - API client operations untested
3. **state/** - State management untested
4. **security/** - Security utilities untested

**Estimated effort to reach 70%**:

- handlers/: ~40 new test files, ~3,000 lines of tests
- proxmox/: ~10 new test files, ~1,000 lines of tests
- state/: ~5 new test files, ~500 lines of tests
- security/: ~3 new test files, ~300 lines of tests

---

## 3. Error Handling Analysis

### 3.1 Current Patterns

| Pattern | Count | Status |
|---------|-------|--------|
| `fmt.Errorf` (total) | 224 | Good foundation |
| `fmt.Errorf("%w", ...)` (wrapping) | 142 | ✅ 63% use wrapping |
| `errors.New` | 3 | Low usage |
| `errors.Is/As` | 2 | ⚠️ Underutilized |
| `panic()` (non-test) | 1 | ✅ Minimal (in utils/errors.go) |

### 3.2 Error Handling Findings

**Positive**:

- 63% of `fmt.Errorf` calls use `%w` for error wrapping
- Only 1 panic in production code (acceptable for fatal init errors)
- Structured logging with zerolog is consistent

**Needs Improvement**:

- `errors.Is/As` barely used (only 2 occurrences)
- No custom error types defined
- Error messages inconsistent in format
- Some handlers return generic errors without context

### 3.3 Recommendations

1. **Create custom error types** in `backend/errors/`:
   - `VMError` - VM operation failures
   - `ProxmoxError` - API communication failures
   - `ValidationError` - Input validation failures
   - `AuthError` - Authentication/authorization failures

2. **Standardize error wrapping**:
   - Always wrap with context: `fmt.Errorf("operation %s failed: %w", op, err)`
   - Use `errors.Is/As` for error type checking

3. **Remove panic usage**:
   - Replace `utils/errors.go:75` panic with proper error return

---

## 4. Type Safety Analysis

### 4.1 interface{} Usage

**Total occurrences**: 302

**Distribution by package**:

| Package | Count | Priority |
|---------|-------|----------|
| handlers/ | ~150 | HIGH |
| templates/ | ~80 | MEDIUM |
| proxmox/ | ~40 | MEDIUM |
| state/ | ~20 | LOW |
| utils/ | ~12 | LOW |

### 4.2 Common interface{} Patterns

1. **Template data maps** (`map[string]interface{}`) - ~100 occurrences
   - Recommendation: Create typed `TemplateData` structs per page

2. **JSON unmarshaling** - ~50 occurrences
   - Recommendation: Define concrete types for all API responses

3. **Generic utilities** - ~30 occurrences
   - Recommendation: Use Go generics where applicable

4. **Configuration values** - ~20 occurrences
   - Recommendation: Use typed configuration structs

### 4.3 Generics Opportunities

| Pattern | Current | Proposed |
|---------|---------|----------|
| Cache operations | `interface{}` values | `Cache[K, V any]` |
| Collection utilities | Multiple implementations | `Slice[T any]` helpers |
| Result types | Error + value returns | `Result[T any]` type |
| Optional values | Pointer + nil checks | `Optional[T any]` type |

---

## 5. Concurrency Analysis

### 5.1 Current Patterns

| Pattern | Count | Status |
|---------|-------|--------|
| `context.Context` usage | 119 | ✅ Good adoption |
| `go func()` goroutines | 14 | ⚠️ Review needed |
| `defer` statements | 395 | ✅ Good cleanup |
| `sync.Mutex` usage | ~10 | ✅ Appropriate |

### 5.2 Goroutine Analysis

**14 goroutines identified**:

- Background monitoring (state/manager.go)
- Connection recovery (state/manager.go)
- Async operations (various handlers)

**Potential issues**:

- Some goroutines may not have proper lifecycle management
- Context cancellation not consistently propagated

### 5.3 Recommendations

1. **Add context to all long-running operations**
2. **Implement goroutine lifecycle management** with sync.WaitGroup
3. **Add timeout handling** to external API calls
4. **Review mutex usage** in state/manager.go (previous deadlock fixed)

---

## 6. Dependency Analysis

### 6.1 Outdated Dependencies

| Dependency | Current | Latest | Priority |
|------------|---------|--------|----------|
| github.com/andybalholm/brotli | v1.1.0 | v1.2.0 | LOW |
| github.com/coreos/go-systemd/v22 | v22.5.0 | v22.7.0 | LOW |
| github.com/fatih/color | v1.16.0 | v1.18.0 | LOW |
| github.com/fsnotify/fsnotify | v1.7.0 | v1.9.0 | LOW |
| github.com/go-resty/resty/v2 | v2.17.1 | v2.17.2 | LOW |
| github.com/google/go-cmp | v0.6.0 | v0.7.0 | LOW |
| github.com/rs/cors | v1.11.0 | v1.11.1 | LOW |
| golang.org/x/mod | v0.32.0 | v0.33.0 | LOW |
| golang.org/x/time | v0.12.0 | v0.14.0 | LOW |
| golang.org/x/tools | v0.41.0 | v0.42.0 | LOW |

### 6.2 Security Vulnerabilities

**Status**: Unable to run `govulncheck` (not installed)

**Recommendation**: Install and run `govulncheck ./...` before deployment

### 6.3 Unused Dependencies

**Analysis needed**: Run `go mod tidy` and verify no unused imports

---

## 7. Performance Baseline

### 7.1 Current Metrics

**Note**: Benchmarks not yet established. Recommendations:

1. **Create benchmark suite** for:
   - VM list retrieval
   - VM details page load
   - Search operations
   - Template rendering

2. **Profile hot paths**:
   - Request handling latency
   - Memory allocations per request
   - Proxmox API call latency

### 7.2 Optimization Opportunities

1. **Template rendering**: Pre-compile templates
2. **API caching**: Implement TTL-based caching for Proxmox data
3. **Connection pooling**: Reuse HTTP connections to Proxmox
4. **Memory allocation**: Use sync.Pool for frequent allocations

---

## 8. Documentation Analysis

### 8.1 Current State

| Area | Status | Priority |
|------|--------|----------|
| Package documentation | ⚠️ Incomplete | HIGH |
| Exported function docs | ⚠️ ~30% documented | HIGH |
| README.md | ✅ Comprehensive | LOW |
| API documentation | ❌ Missing | MEDIUM |
| Architecture docs | ❌ Missing | MEDIUM |

### 8.2 Recommendations

1. **Add package-level documentation** to all packages
2. **Document all exported functions** with godoc comments
3. **Create ARCHITECTURE.md** explaining code organization
4. **Add inline comments** for complex algorithms

---

## 9. Refactoring Priorities

### 9.1 Priority Matrix

| Priority | Area | Impact | Effort | Recommendation |
|----------|------|--------|--------|----------------|
| P1 | Test coverage | HIGH | HIGH | Start with handlers/, proxmox/ |
| P2 | Code organization | HIGH | MEDIUM | Split large handler files |
| P3 | Error handling | MEDIUM | LOW | Add custom error types |
| P4 | Type safety | MEDIUM | MEDIUM | Reduce interface{} usage |
| P5 | Documentation | MEDIUM | LOW | Add godoc comments |
| P6 | Performance | LOW | MEDIUM | Establish benchmarks first |
| P7 | Dependencies | LOW | LOW | Update after other work |

### 9.2 Quick Wins

1. **Run `go mod tidy`** - Clean up dependencies
2. **Add package docs** - 1 line per package
3. **Create error types** - 1 file, reusable everywhere
4. **Split helpers.go** - Immediate organization improvement

### 9.3 High-Impact Changes

1. **Split vm_create.go** (1,162 lines) - Most complex handler
2. **Add handler tests** - Critical for reliability
3. **Create typed template data** - Reduce interface{} by ~100

---

## 10. Decisions & Rationale

### Decision 1: Start with Code Organization

**Decision**: Begin refactoring with code organization (Phase 2) before testing (Phase 6)

**Rationale**:

- Smaller files are easier to test
- Clear responsibilities make test boundaries obvious
- Reduces test rewrite when code moves

**Alternatives Considered**:

- Test-first approach: Rejected because testing monolithic files is harder
- Performance-first: Rejected because optimization without tests is risky

### Decision 2: Custom Error Types Before Wrapping

**Decision**: Create custom error types before standardizing error wrapping

**Rationale**:

- Error types define the vocabulary for wrapping
- Enables `errors.Is/As` usage immediately
- One-time investment with broad impact

**Alternatives Considered**:

- Wrap first, types later: Rejected because it requires double work
- Use sentinel errors: Rejected because types are more flexible

### Decision 3: Table-Driven Tests for All New Tests

**Decision**: All new tests MUST use table-driven format

**Rationale**:

- Consistent with Go conventions
- Easier to add test cases
- Better test coverage visibility
- Aligns with constitution principle III

**Alternatives Considered**:

- Individual test functions: Rejected for verbosity
- Behavior-driven testing: Rejected for complexity

---

## 11. Baseline Metrics Summary

| Metric | Current | Target | Gap |
|--------|---------|--------|-----|
| Test coverage (overall) | ~10% | >70% | 60% |
| Test coverage (handlers) | 3.3% | >70% | 67% |
| Test coverage (proxmox) | 4.4% | >70% | 66% |
| Linter warnings | 0 | 0 | ✅ Met |
| Files >500 lines | 17 | 0 | 17 files |
| interface{} usage | 302 | <60 | 242 |
| Error wrapping rate | 63% | >95% | 32% |
| Documented exports | ~30% | 100% | 70% |
| Panic usage | 1 | 0 | 1 |
| Outdated dependencies | 10 | 0 | 10 |

---

## 12. Next Steps

1. **Phase 1**: Create error types and generic utilities
2. **Phase 2**: Split large handler files (start with vm_create.go)
3. **Phase 3**: Standardize error handling
4. **Phase 4**: Reduce interface{} usage
5. **Phase 5**: Improve concurrency patterns
6. **Phase 6**: Add comprehensive tests
7. **Phase 7**: Optimize performance
8. **Phase 8**: Complete documentation
9. **Phase 9**: Update dependencies and polish

---

## Appendix A: Commands Used

```bash
# File size analysis
find . -name "*.go" -type f ! -path "./vendor/*" | xargs wc -l | sort -rn

# Test coverage
go test -cover ./...

# Linter check
golangci-lint run --timeout=3m

# Dependency updates
go list -u -m all | grep -E '\[.*\]'

# Pattern searches
grep -rn "interface{}" --include="*.go" | wc -l
grep -rn "panic(" --include="*.go" | grep -v "_test.go"
grep -rn "fmt.Errorf" --include="*.go" | wc -l
grep -rn "%w" --include="*.go" | wc -l
grep -rn "context.Context" --include="*.go" | wc -l
grep -rn "go func" --include="*.go" | wc -l
grep -rn "defer " --include="*.go" | wc -l
```
