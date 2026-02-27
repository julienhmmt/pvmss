# Tasks: Go Best Practices Refactoring

**Feature**: Go Best Practices Refactoring  
**Branch**: `001-go-best-practices-refactor`  
**Status**: Ready for Implementation  
**Created**: 2026-02-15

---

## Overview

This document defines the actionable tasks for refactoring the PVMSS backend to follow Go best practices. Tasks are organized by phase and user story, with clear dependencies and parallel execution opportunities.

**Total Tasks**: 87  
**Estimated Duration**: 4-6 weeks  
**Team Size**: 1-3 developers

---

## Phase 0: Research & Analysis

**Goal**: Establish baseline metrics and identify refactoring priorities  
**Duration**: 1-2 days  
**Independent Test Criteria**:

- Baseline metrics documented (coverage, performance, complexity)
- Files >500 lines identified
- Circular dependencies documented
- Performance profile established

### Setup Tasks

- [ ] T001 Establish baseline code coverage with `go test -cover ./...` in backend/
- [ ] T002 Run golangci-lint with full configuration and document all warnings
- [ ] T003 Identify all handler files >500 lines and document structure
- [ ] T004 Profile application with pprof to identify hot paths
- [ ] T005 Audit dependencies with `go list -u -m all` and `govulncheck ./...`
- [ ] T006 Document current error handling patterns across all packages
- [ ] T007 Identify circular dependencies with `go mod graph` analysis
- [ ] T008 Create baseline performance benchmarks for critical paths
- [ ] T009 Document test coverage gaps by package
- [ ] T010 Create research.md summarizing all findings and recommendations

---

## Phase 1: Foundation & Infrastructure

**Goal**: Create reusable infrastructure and patterns for refactoring  
**Duration**: 3-5 days  
**Independent Test Criteria**:

- Error types compile and are usable
- Generic utilities work with multiple types
- Benchmarking infrastructure functional
- Testing helpers reduce boilerplate

### Infrastructure Tasks

- [ ] T011 [P] Create error type definitions in backend/errors/errors.go (VM, Proxmox, Handler, Validation errors)
- [ ] T012 [P] Implement generic utility functions in backend/utils/generics.go (Cache, Pool, Collection types)
- [ ] T013 [P] Set up benchmarking infrastructure in backend/tests/benchmarks_test.go
- [ ] T014 [P] Create testing utilities in backend/tests/helpers.go (mock builders, assertions)
- [ ] T015 [P] Document package organization strategy in backend/ARCHITECTURE.md
- [ ] T016 Create data-model.md defining refactoring entities and relationships
- [ ] T017 Create contracts/ directory structure for API contracts
- [ ] T018 Create quickstart.md with refactoring patterns and examples
- [ ] T019 Update agent context with refactoring patterns and conventions
- [ ] T020 Create REFACTORING_GUIDE.md documenting best practices to follow

---

## Phase 2: Code Organization & Structure

**Goal**: Split monolithic files and improve package organization  
**Duration**: 5-7 days  
**Independent Test Criteria**:

- All handler files <500 lines
- No circular dependencies
- Package documentation complete
- All routes continue to function

### Code Organization Tasks

- [ ] T021 [P] [US1] Split backend/handlers/vm_details.go into vm_details_base.go, vm_details_info.go, vm_details_actions.go
- [ ] T022 [P] [US1] Split backend/handlers/vm_create.go into vm_create_handler.go, vm_create_helpers.go, vm_create_validation.go
- [ ] T023 [P] [US1] Split backend/handlers/vm_actions.go into vm_actions_lifecycle.go, vm_actions_resources.go, vm_actions_helpers.go
- [ ] T024 [P] [US1] Reorganize backend/proxmox/ into client.go, vms.go, nodes.go, storage.go, console.go
- [ ] T025 [P] [US1] Improve backend/state/ organization with manager.go, proxmox_status.go, cache.go
- [ ] T026 [P] [US1] Create backend/handlers/auth/ subpackage for authentication logic
- [ ] T027 [P] [US1] Create backend/handlers/admin/ subpackage for admin handlers
- [ ] T028 [P] [US1] Create backend/handlers/vm/ subpackage for VM-related handlers
- [ ] T029 [P] [US1] Update all imports in main.go and route registration
- [ ] T030 [US1] Verify all routes continue to function with integration tests
- [ ] T031 [US1] Add package documentation to all refactored packages
- [ ] T032 [US1] Remove circular dependencies and verify with `go mod graph`

---

## Phase 3: Error Handling Standardization

**Goal**: Implement consistent, idiomatic error handling  
**Duration**: 3-4 days  
**Independent Test Criteria**:

- All errors wrapped with context
- Custom error types used appropriately
- Error logging structured and consistent
- No panics in request handlers

### Error Handling Tasks

- [ ] T033 [P] [US2] Implement error wrapping in backend/handlers/common.go with fmt.Errorf("%w")
- [ ] T034 [P] [US2] Add custom error types to backend/errors/errors.go (NotFound, Unauthorized, ValidationError, etc.)
- [ ] T035 [P] [US2] Standardize error logging in backend/handlers/ using structured logging
- [ ] T036 [P] [US2] Replace panic calls with proper error handling in backend/handlers/
- [ ] T037 [P] [US2] Implement error recovery in critical paths (VM operations, API calls)
- [ ] T038 [P] [US2] Update error messages for clarity and debugging
- [ ] T039 [US2] Add error handling tests in backend/handlers/*_test.go
- [ ] T040 [US2] Verify error chains preserve context with integration tests

---

## Phase 4: Type Safety & Generics

**Goal**: Reduce interface{} usage and leverage Go generics  
**Duration**: 2-3 days  
**Independent Test Criteria**:

- interface{} usage reduced by >80%
- Generic utilities compile and work
- Type safety verified by compiler
- No breaking changes to public APIs

### Type Safety Tasks

- [ ] T041 [P] [US3] Identify all interface{} usage with grep and document locations
- [ ] T042 [P] [US3] Create typed wrappers for caching in backend/utils/cache.go
- [ ] T043 [P] [US3] Create typed wrappers for collections in backend/utils/collections.go
- [ ] T044 [P] [US3] Implement generic utility functions in backend/utils/generics.go
- [ ] T045 [P] [US3] Replace interface{} in template data structures
- [ ] T046 [P] [US3] Replace interface{} in configuration structures
- [ ] T047 [P] [US3] Replace interface{} in API response structures
- [ ] T048 [US3] Document type constraints and generic patterns
- [ ] T049 [US3] Verify type safety with `go vet` and compiler checks

---

## Phase 5: Concurrency & Resource Management

**Goal**: Optimize concurrent operations and resource management  
**Duration**: 2-3 days  
**Independent Test Criteria**:

- All long-running operations accept context.Context
- Goroutines properly cleaned up
- Resource cleanup guaranteed with defer
- Timeout handling consistent

### Concurrency Tasks

- [ ] T050 [P] [US4] Add context.Context parameter to long-running operations in backend/handlers/
- [ ] T051 [P] [US4] Implement goroutine lifecycle management in backend/state/manager.go
- [ ] T052 [P] [US4] Add connection pooling for Proxmox API calls in backend/proxmox/client.go
- [ ] T053 [P] [US4] Implement sync.Pool for frequently allocated objects
- [ ] T054 [P] [US4] Ensure proper cleanup with defer statements throughout codebase
- [ ] T055 [P] [US4] Add timeout handling to all external API calls
- [ ] T056 [US4] Test goroutine cleanup with `go test -race` to detect race conditions
- [ ] T057 [US4] Add context cancellation tests in backend/tests/

---

## Phase 6: Testing & Coverage

**Goal**: Improve test coverage to >70% with table-driven tests  
**Duration**: 4-5 days  
**Independent Test Criteria**:

- Code coverage >70% overall
- All new tests use table-driven format
- Critical paths have integration tests
- Performance benchmarks established

### Testing Tasks

- [ ] T058 [P] [US5] Convert existing tests to table-driven format in backend/handlers/*_test.go
- [ ] T059 [P] [US5] Add unit tests for utility functions in backend/utils/*_test.go
- [ ] T060 [P] [US5] Implement integration tests for critical paths in backend/tests/integration_test.go
- [ ] T061 [P] [US5] Add benchmarks for performance-sensitive code in backend/tests/benchmarks_test.go
- [ ] T062 [P] [US5] Add tests for error handling in backend/errors/*_test.go
- [ ] T063 [P] [US5] Add tests for concurrency patterns in backend/tests/concurrency_test.go
- [ ] T064 [P] [US5] Add tests for type safety improvements in backend/utils/*_test.go
- [ ] T065 [US5] Achieve >70% code coverage with `go test -cover ./...`
- [ ] T066 [US5] Verify test execution time <5 minutes
- [ ] T067 [US5] Document test coverage gaps and create issues for future work

---

## Phase 7: Performance Optimization

**Goal**: Achieve >10% improvement in critical path performance  
**Duration**: 3-4 days  
**Independent Test Criteria**:

- Benchmarks show measurable improvements
- Memory allocations reduced in hot paths
- No performance regressions
- Optimizations documented

### Performance Tasks

- [ ] T068 [P] [US6] Profile hot paths with pprof in backend/tests/
- [ ] T069 [P] [US6] Optimize memory allocations in request handlers
- [ ] T070 [P] [US6] Implement caching strategies for frequently accessed data
- [ ] T071 [P] [US6] Optimize Proxmox API queries (batch operations, caching)
- [ ] T072 [P] [US6] Optimize template rendering performance
- [ ] T073 [US6] Run benchmarks and verify >10% improvement in p95 latency
- [ ] T074 [US6] Document performance optimizations in PERFORMANCE.md
- [ ] T075 [US6] Add performance regression tests to CI/CD

---

## Phase 8: Documentation & Comments

**Goal**: Achieve 100% exported function documentation  
**Duration**: 2-3 days  
**Independent Test Criteria**:

- 100% of exported functions documented
- Package documentation complete
- godoc renders cleanly
- No outdated comments

### Documentation Tasks

- [ ] T076 [P] [US7] Add godoc comments to all exported functions in backend/handlers/
- [ ] T077 [P] [US7] Add godoc comments to all exported functions in backend/proxmox/
- [ ] T078 [P] [US7] Add godoc comments to all exported functions in backend/state/
- [ ] T079 [P] [US7] Add godoc comments to all exported functions in backend/utils/
- [ ] T080 [P] [US7] Document package purposes in all backend packages
- [ ] T081 [P] [US7] Include usage examples for complex types
- [ ] T082 [P] [US7] Document assumptions and constraints in code comments
- [ ] T083 [US7] Verify godoc rendering with `godoc -http=:6060`
- [ ] T084 [US7] Update README.md with refactoring improvements

---

## Phase 9: Dependency Management & Polish

**Goal**: Clean dependencies and achieve zero linter warnings  
**Duration**: 2-3 days  
**Independent Test Criteria**:

- All dependencies up-to-date
- No security vulnerabilities
- Zero linter warnings
- All tests pass

### Dependency & Polish Tasks

- [ ] T085 [P] [US8] Audit dependencies with `go list -u -m all` and update outdated packages
- [ ] T086 [P] [US8] Remove unused dependencies from go.mod
- [ ] T087 [P] [US8] Run security audit with `govulncheck ./...`
- [ ] T088 [P] [US8] Fix all golangci-lint warnings with `golangci-lint run --fix`
- [ ] T089 [P] [US8] Run `go vet ./...` and fix all issues
- [ ] T090 [P] [US8] Run `go fmt ./...` to ensure consistent formatting
- [ ] T091 [US8] Run full test suite: `go test -race -cover ./...`
- [ ] T092 [US8] Verify backward compatibility with integration tests
- [ ] T093 [US8] Create final summary document with metrics and improvements

---

## Dependencies & Execution Order

### Critical Path (Blocking Dependencies)

```text
Phase 0 (Research) → Phase 1 (Infrastructure) → Phase 2 (Organization)
                                                      ↓
                                    Phase 3-5 (Quality Improvements)
                                                      ↓
                                    Phase 6-9 (Testing & Polish)
```

### Parallelization Opportunities

**Can be done in parallel**:

- Phase 1 tasks (T011-T020) - All independent infrastructure work
- Phase 2 tasks (T021-T028) - Different handler files can be split simultaneously
- Phase 3-5 tasks - Different packages can be improved in parallel
- Phase 6-9 tasks - Different test/documentation work can be parallelized

**Example parallel execution**:

```text
Developer 1: T021-T023 (VM handlers)
Developer 2: T024-T025 (Proxmox & state)
Developer 3: T026-T028 (Auth & admin)
All: T030-T032 (Integration & verification)
```

---

## Independent Test Criteria by Phase

### Phase 0 Complete When

- ✅ Baseline metrics documented
- ✅ All files >500 lines identified
- ✅ Circular dependencies documented
- ✅ Performance profile established

### Phase 1 Complete When

- ✅ Error types compile and are usable
- ✅ Generic utilities work with multiple types
- ✅ Benchmarking infrastructure functional
- ✅ All documentation artifacts created

### Phase 2 Complete When

- ✅ All handler files <500 lines
- ✅ No circular dependencies
- ✅ Package documentation complete
- ✅ All routes continue to function

### Phase 3 Complete When

- ✅ All errors wrapped with context
- ✅ Custom error types used appropriately
- ✅ Error logging structured and consistent
- ✅ No panics in request handlers

### Phase 4 Complete When

- ✅ interface{} usage reduced by >80%
- ✅ Generic utilities compile and work
- ✅ Type safety verified by compiler
- ✅ No breaking changes to public APIs

### Phase 5 Complete When

- ✅ All long-running operations accept context.Context
- ✅ Goroutines properly cleaned up
- ✅ Resource cleanup guaranteed with defer
- ✅ Timeout handling consistent

### Phase 6 Complete When

- ✅ Code coverage >70% overall
- ✅ All new tests use table-driven format
- ✅ Critical paths have integration tests
- ✅ Performance benchmarks established

### Phase 7 Complete When

- ✅ Benchmarks show measurable improvements
- ✅ Memory allocations reduced in hot paths
- ✅ No performance regressions
- ✅ Optimizations documented

### Phase 8 Complete When

- ✅ 100% of exported functions documented
- ✅ Package documentation complete
- ✅ godoc renders cleanly
- ✅ No outdated comments

### Phase 9 Complete When

- ✅ All dependencies up-to-date
- ✅ No security vulnerabilities
- ✅ Zero linter warnings
- ✅ All tests pass

---

## Success Metrics

### Code Quality

- golangci-lint: 0 warnings
- go vet: 0 issues
- gosec: 0 real vulnerabilities
- Duplication: <5%
- Cyclomatic complexity: average <10

### Test Coverage

- Overall coverage: >70%
- Critical paths: 100% coverage
- New code: >80% coverage
- Test execution: <5 minutes

### Performance

- Request latency p95: >10% improvement
- Memory allocations: >15% reduction in hot paths
- No performance regressions
- Benchmark suite established

### Documentation

- 100% exported functions documented
- All packages documented
- No outdated comments
- godoc renders cleanly

### Maintainability

- Average function length: <30 lines
- Package organization: clear and logical
- Error handling: consistent patterns
- No circular dependencies

---

## Implementation Notes

### Approach

- **Incremental**: Refactor one package/area at a time
- **Non-breaking**: All changes maintain backward compatibility
- **Test-driven**: Write tests before refactoring
- **Phased**: Complete phases sequentially to avoid blocking features

### Risk Mitigation

- Baseline metrics established before starting
- Full test suite run after each phase
- All changes reviewed for quality and compatibility
- Metrics tracked throughout refactoring

### Timeline

- **Total Duration**: 4-6 weeks (with parallel work on features)
- **Can be distributed**: Across team members
- **Phases can be parallelized**: Where independent
- **No blocking**: Of feature development
