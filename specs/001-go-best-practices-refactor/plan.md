# Implementation Plan: Go Best Practices Refactoring

**Branch**: `001-go-best-practices-refactor` | **Date**: 2026-02-15 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-go-best-practices-refactor/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Systematically refactor the PVMSS backend codebase to follow Go best practices and idioms. This includes improving code organization (splitting monolithic files), standardizing error handling, improving type safety, optimizing concurrency and resource management, enhancing test coverage, optimizing performance, improving documentation, and auditing dependencies. The refactoring will be completed in phases without blocking feature development, maintaining full backward compatibility with existing APIs.

## Technical Context

**Language/Version**: Go 1.21+  
**Primary Dependencies**: zerolog (logging), resty (HTTP client), templ (templating), existing Proxmox client  
**Storage**: Proxmox API (no schema changes), local settings.json  
**Testing**: Go standard testing package, table-driven tests, benchmarks  
**Target Platform**: Linux server (backend only)  
**Project Type**: Web application backend (monolithic)  
**Performance Goals**: >10% improvement in request latency p95, >15% reduction in memory allocations in hot paths  
**Constraints**: No breaking changes to public APIs, all existing tests must pass, <5 minute test execution time  
**Scale/Scope**: ~15 backend packages, ~2000+ lines of code to refactor, 8 functional requirements

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Constitution Status**: Template-based (not yet customized for project)

- No blocking principles identified
- Refactoring aligns with code quality and maintainability goals
- All changes maintain backward compatibility (non-breaking)

## Project Structure

### Documentation (this feature)

```text
specs/001-go-best-practices-refactor/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
├── checklists/
│   └── requirements.md  # Specification quality checklist
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
backend/
├── app/                 # Application initialization
├── cloudinit/           # Cloud-init functionality
├── components/          # Templ components (86 files)
├── constants/           # Constants and configuration
├── handlers/            # HTTP request handlers (64 files - PRIMARY REFACTORING TARGET)
├── i18n/                # Internationalization (57 files)
├── logger/              # Logging configuration
├── middleware/          # HTTP middleware
├── proxmox/             # Proxmox API client (19 files)
├── security/            # Security utilities
├── state/               # Application state management (9 files)
├── templates/           # Template utilities
├── tests/               # Test infrastructure
├── utils/               # Utility functions
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── main.go             # Application entry point
└── main_test.go        # Main tests
```

**Structure Decision**: Web application backend (monolithic). Refactoring focuses on:

1. **handlers/** - Split large files (>500 lines) into focused subpackages
2. **proxmox/** - Improve client organization and error handling
3. **state/** - Enhance state management and concurrency patterns
4. **utils/** - Create typed utility functions with generics
5. **All packages** - Standardize error handling, improve documentation, add tests

## Complexity Tracking

No constitution violations identified. Refactoring is non-breaking and aligns with code quality goals.

---

## Implementation Phases

### Phase 0: Research & Analysis

**Duration**: 1-2 days  
**Deliverables**: research.md, baseline metrics

**Tasks**:

- Analyze current code structure and identify files >500 lines
- Profile application to establish performance baseline
- Audit dependencies for security vulnerabilities
- Document current error handling patterns
- Identify circular dependencies and import issues
- Establish test coverage baseline

**Output**: research.md with findings and recommendations

### Phase 1: Foundation & Infrastructure

**Duration**: 3-5 days  
**Deliverables**: data-model.md, contracts/, quickstart.md, agent context

**Tasks**:

- Create error type definitions for major domains
- Implement generic utility functions for common patterns
- Set up benchmarking infrastructure
- Create testing utilities and helpers
- Document package organization strategy
- Update agent context with refactoring patterns

**Output**: Reusable infrastructure for refactoring

### Phase 2: Code Organization & Structure

**Duration**: 5-7 days  
**Deliverables**: Refactored handler packages, improved imports

**Tasks**:

- Split handlers/ into focused subpackages (vm_details, vm_create, vm_actions, etc.)
- Reorganize proxmox/ client code
- Improve state/ management patterns
- Remove circular dependencies
- Add package documentation
- Ensure backward compatibility

**Output**: Reorganized, single-responsibility packages

### Phase 3: Error Handling Standardization

**Duration**: 3-4 days  
**Deliverables**: Consistent error handling throughout codebase

**Tasks**:

- Implement error wrapping with context
- Replace panic with proper error handling
- Standardize error logging
- Add error recovery in critical paths
- Update error messages for clarity

**Output**: Consistent, traceable error handling

### Phase 4: Type Safety & Generics

**Duration**: 2-3 days  
**Deliverables**: Reduced interface{} usage, typed utilities

**Tasks**:

- Identify and replace interface{} with concrete types
- Create generic utility functions
- Implement typed wrappers for caching/collections
- Document type constraints
- Verify type safety with compiler

**Output**: Improved type safety, reduced interface{} usage

### Phase 5: Concurrency & Resource Management

**Duration**: 2-3 days  
**Deliverables**: Optimized concurrent operations, proper cleanup

**Tasks**:

- Add context.Context to long-running operations
- Implement goroutine lifecycle management
- Add connection pooling where applicable
- Ensure proper cleanup with defer
- Add timeout handling

**Output**: Safe, efficient concurrent operations

### Phase 6: Testing & Coverage

**Duration**: 4-5 days  
**Deliverables**: >70% code coverage, table-driven tests

**Tasks**:

- Convert existing tests to table-driven format
- Add unit tests for utility functions
- Implement integration tests for critical paths
- Add benchmarks for performance-sensitive code
- Achieve >70% overall coverage

**Output**: Comprehensive test suite

### Phase 7: Performance Optimization

**Duration**: 3-4 days  
**Deliverables**: Benchmarks showing improvements

**Tasks**:

- Profile hot paths
- Optimize memory allocations
- Implement caching strategies
- Optimize database/API queries
- Verify >10% improvement in p95 latency

**Output**: Optimized performance with benchmarks

### Phase 8: Documentation & Comments

**Duration**: 2-3 days  
**Deliverables**: 100% exported function documentation

**Tasks**:

- Add godoc comments to all exported functions
- Document package purposes
- Include usage examples
- Document assumptions and constraints
- Verify godoc rendering

**Output**: Complete, clear documentation

### Phase 9: Dependency Management & Polish

**Duration**: 2-3 days  
**Deliverables**: Clean go.mod, zero linter warnings

**Tasks**:

- Audit and update dependencies
- Remove unused dependencies
- Run security audit
- Fix all linter warnings
- Final testing and verification

**Output**: Production-ready codebase

---

## Implementation Strategy

### Approach

- **Incremental**: Refactor one package/area at a time
- **Non-breaking**: All changes maintain backward compatibility
- **Test-driven**: Write tests before refactoring
- **Phased**: Complete phases sequentially to avoid blocking features

### Risk Mitigation

- **Baseline metrics**: Establish before starting (coverage, performance, complexity)
- **Continuous testing**: Run full test suite after each phase
- **Code review**: All changes reviewed for quality and compatibility
- **Monitoring**: Track metrics throughout refactoring

### Success Metrics

- golangci-lint: 0 warnings
- Code coverage: >70%
- Performance: >10% improvement in p95 latency
- Documentation: 100% exported functions documented
- Backward compatibility: All existing tests pass

### Timeline

**Total Duration**: 4-6 weeks (with parallel work on features)

- Can be distributed across team
- Phases can be parallelized where independent
- No blocking of feature development
