# Research: Constructor Rename to Make

## Decision 1: Use an explicit rename mapping as the source of truth

- **Decision**: Drive the refactor from the approved list of constructor renames captured in the feature specification.
- **Rationale**: The feature already defines the exact intended scope. Treating that list as authoritative prevents accidental renames of unrelated `New...` symbols and gives maintainers a clear audit surface.
- **Alternatives considered**:
  - Broad pattern-based replacement of every `New`-prefixed function in the repository. Rejected because it could rename out-of-scope helpers or third-party references.
  - Manual file-by-file discovery during implementation. Rejected because it increases the risk of omissions and inconsistent scope control.

## Decision 2: Treat the refactor as a mechanical symbol rename only

- **Decision**: Rename constructor definitions and compile-relevant references only, while preserving signatures, return values, package ownership, and runtime behavior.
- **Rationale**: The feature is explicitly non-breaking and cosmetic. Limiting the change to symbol renames preserves backward compatibility expectations and keeps review focused.
- **Alternatives considered**:
  - Combining the rename with broader cleanup of constructors, types, or package structure. Rejected because it expands risk and weakens the guarantee of no logic changes.
  - Introducing temporary alias wrappers. Rejected because the feature does not require compatibility shims inside the codebase and wrappers would add unnecessary complexity.

## Decision 3: Let existing verification commands define completeness

- **Decision**: Use the project’s established formatting, offline test, and lint workflows as the primary completion gates for the refactor.
- **Rationale**: A mechanical rename should be proven by successful compilation, automated tests, and static analysis. The repository already provides `make go-fmt`, `make test-offline`, and `make go-lint` as standard verification entry points.
- **Alternatives considered**:
  - Relying only on compiler feedback. Rejected because tests and linting can catch missed or inconsistent references outside the immediate build path.
  - Adding feature-specific runtime tests. Rejected because the change does not alter behavior and is better validated through existing suite coverage.

## Decision 4: Keep non-runtime artifacts in scope only when they affect developer workflow

- **Decision**: Update documentation or helper artifacts only when stale constructor names would mislead maintainers or break expected development workflows.
- **Rationale**: The primary requirement is code correctness. Supporting artifacts should stay accurate where they materially affect implementation, review, or verification, but the refactor should not become a documentation rewrite.
- **Alternatives considered**:
  - Updating every textual occurrence across the repository. Rejected because it adds noise without improving runtime safety.
  - Ignoring all non-code references. Rejected because some developer-facing artifacts may become confusing if they describe outdated constructor names.

## Decision 5: Preserve package-local naming even for duplicate constructor names

- **Decision**: Rename duplicate constructor names independently within their existing packages, such as `NewVMHandler` and `NewAuthHandler`, without introducing shared abstractions.
- **Rationale**: Package boundaries already disambiguate these names. Preserving that structure avoids unnecessary architectural changes.
- **Alternatives considered**:
  - Renaming duplicate constructors differently to make them globally unique. Rejected because package scope already solves the ambiguity and different names would reduce consistency.
  - Consolidating duplicated constructor names into shared packages. Rejected because it would change ownership and exceed the feature scope.
