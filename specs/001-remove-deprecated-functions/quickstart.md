# Quickstart: Remove Deprecated Functions

## Goal

Complete the deprecated handler context cleanup by migrating maintained references to `HandlerContextWith`, removing the deprecated wrapper, and validating that the repository remains safe to merge.

## Prerequisites

- You are on branch `001-remove-deprecated-functions`.
- You are working from the repository root: `/Users/jh/git/gh/pvmss`.
- Standard Go and project tooling are available.

## Steps

1. Review the feature artifacts:
   - `specs/001-remove-deprecated-functions/spec.md`
   - `specs/001-remove-deprecated-functions/plan.md`
   - `specs/001-remove-deprecated-functions/research.md`

2. Update all maintained references to the deprecated handler context entry point:
   - Replace in-scope uses of `NewHandlerContext` with `HandlerContextWith`.
   - Include maintained tests and helper code when they still reference the deprecated symbol.

3. Remove the deprecated wrapper definition once no in-scope references remain.

4. Verify completion using the repository maintenance workflow:
   - Run formatting.
   - Run linting.
   - Run offline automated tests.
   - Confirm no maintained application source still references `NewHandlerContext`.

5. Review the final diff to ensure Telmate migration TODO cleanup was not included.

## Expected Outcome

- No maintained source file depends on the deprecated handler context entry point.
- The deprecated wrapper is absent from the codebase.
- Repository validation passes.
- Scope remains limited to the planned maintenance cleanup.
