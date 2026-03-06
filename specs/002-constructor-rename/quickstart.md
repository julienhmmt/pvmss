# Quickstart: Constructor Rename to Make

## Goal

Apply the approved constructor rename set from `New...` to `Make...` without changing runtime behavior, then verify the refactor using the project’s standard quality gates.

## Prerequisites

- Work from branch `001-constructor-rename`.
- Use the approved rename mapping captured in `spec.md`.
- Ensure local development tooling for Go formatting, Go tests, and `golangci-lint` is available.

## Workflow

1. Review `spec.md`, `plan.md`, and `research.md` to confirm the rename scope and constraints.
2. Update constructor definitions to the approved `Make...` names.
3. Update all compile-relevant call sites for those constructors in backend packages, tests, and supporting helpers.
4. Review duplicate constructor names that exist in different packages and confirm each rename remains package-local.
5. Update any developer-facing artifact that would otherwise become misleading for normal implementation or verification workflows.
6. Run the standard repository verification steps:
   - `make go-fmt`
   - `make test-offline`
   - `make go-lint`
7. Resolve any missed rename references surfaced by the verification steps.
8. Prepare the feature for task breakdown once the verification workflow is clean.

## Expected Outcome

- All approved constructor definitions use the `Make...` prefix.
- All compile-relevant references use the updated names.
- No behavior or public contract changes are introduced.
- Formatting, offline tests, and linting pass without rename-related failures.
