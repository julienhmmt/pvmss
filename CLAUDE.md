# CLAUDE.md

Guidance for Claude Code (claude.ai/code) in this repo.

The single source of truth for all agents is `AGENTS.md` (project overview,
commands, architecture, mandatory graph-first workflow, conventions). It is
imported below — follow it in full:

@AGENTS.md

## Claude Code-Specific Notes

- **Graphify**: when the user types `/graphify`, invoke the graphify skill
  before doing anything else. Per the graph-first workflow in AGENTS.md, build
  or refresh a folder's snapshot (`/graphify <folder>` /
  `/graphify <folder> --update`) before creating code there.
- **Skills**: project skills live in `.devin/skills/` (golang-*,
  svelte-code-writer, tailwind-design-system, speckit.*, todo-planning,
  backend-refactor). Invoke the matching skill at the start of matching tasks
  — e.g. `svelte-code-writer` whenever creating or editing `.svelte` files.
- **Rules**: `.devin/rules/coding-style.md` (Go/TS style) and
  `.devin/rules/ui-quality.md` (admin pages, forms) apply to all code you write.
