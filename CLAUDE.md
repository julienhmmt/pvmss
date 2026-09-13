# CLAUDE.md

Guidance for Claude Code (claude.ai/code) in this repo.

The single source of truth for all agents is `AGENTS.md` (project overview,
commands, architecture, the mandatory locate-before-you-read workflow,
conventions). It is
imported below — follow it in full:

@AGENTS.md

## Claude Code-Specific Notes

- **Locating code**: use `tools/pq` (see AGENTS.md) before any Grep/Glob.
  `pq def NAME` to find a declaration, `pq callers NAME` to find usage,
  `pq api <dir>` for a package's surface. The graphify and code-review-graph
  workflows were removed on 2026-09-11.
- **Skills**: project skills live in `.devin/skills/` (golang-*,
  svelte-code-writer, tailwind-design-system, todo-planning,
  backend-refactor). Invoke the matching skill at the start of matching tasks
  — e.g. `svelte-code-writer` whenever creating or editing `.svelte` files.
- **Rules**: `.devin/rules/coding-style.md` (Go/TS style) and
  `.devin/rules/ui-quality.md` (admin pages, forms) apply to all code you write.

## Agent skills

### Issue tracker

Issues and specs live as markdown files under `.scratch/`. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: `CONTEXT.md` + `docs/adr/` at repo root (created lazily). See `docs/agents/domain.md`.

### Planning/TDD/review ownership (overrides global agents.md)

This repo uses the mattpocock-skills flow (`/grill-with-docs` → `/to-spec` →
`/to-tickets` → `/implement`). `/implement` already drives `/tdd` and
`/code-review` internally per ticket. Do **not** also auto-invoke the global
`planner`, `tdd-guide`, or `code-reviewer` agents from `~/.claude/rules/agents.md`
in this repo — that duplicates the same work through a second process. Use
`/diagnosing-bugs` for bug reports instead of jumping straight to a fix.
