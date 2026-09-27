# Plan: Restore web checks with TypeScript 7

## Goal

Make `make web-check` and `make web-lint` pass without changing the TypeScript version the application intentionally uses or suppressing compatibility errors.

## Current state

- `web/package.json` uses TypeScript `^7.0.2`, `svelte-check` `^4.7.6`, and `typescript-eslint` `^8.70.1`.
- `make web-check` compiles Paraglide, then `svelte-check` exits before checking because TypeScript 7 requires TypeScript 6 alongside it and the `--tsgo-experimental-api` flag.
- `make web-lint` exits at startup because the current `typescript-eslint` rejects TypeScript 7.0 and directs users to run it with the TypeScript 6 API.
- Web tests and the production build pass; this plan concerns only the check/lint toolchain.

## Proposed approach

Keep TypeScript 7 as the project compiler and provide the TypeScript 6 compatibility API required by the current check/lint tools. Confirm package and module-resolution details against current upstream guidance before changing dependencies. If this side-by-side setup is not supported by the Svelte and ESLint integrations, pin the project to TypeScript 6 instead of bypassing their version guards.

## Steps

1. Confirm whether the project needs TypeScript 7-specific behavior; record the decision before changing package versions.
2. Verify supported TypeScript 6/7 package aliases and `svelte-check` CLI options against current Svelte-check, typescript-eslint, and TypeScript documentation.
3. Update `web/package.json` and `web/bun.lock` through Bun to install compatible TypeScript 6 and TypeScript 7 packages. Keep typescript-eslint on the supported TypeScript 6 API and update the web check script to use the documented experimental TypeScript 7 API where required.
4. Run `bun install --frozen-lockfile`, `make web-check`, `make web-lint`, `make web-test`, and `bun run build` from `web/` or their root Make targets. Fix any integration or lockfile issues without disabling lint rules or type checking.
5. Review the final dependency diff and confirm the lockfile reproduces the same checks in a clean install.

## Acceptance criteria

- `make web-check` completes with no Svelte/type errors.
- `make web-lint` completes without an unsupported-TypeScript error.
- Web tests and the production build remain green.
- The final package and lockfile changes use supported, pinned versions and contain no lint/type-check bypasses.

## References

- `web/package.json`
- `web/eslint.config.js`
- `web` `check` script error: TypeScript 7 requires TypeScript 6 alongside it and `--tsgo-experimental-api`.
- `typescript-eslint` 8.70 error: TypeScript 7.0 is rejected; run its tooling on the TypeScript 6 API while keeping TypeScript 7 for the project where supported.
