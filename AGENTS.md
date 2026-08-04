<!-- code-review-graph MCP tools -->

# MCP Tools: code-review-graph

**IMPORTANT: This project has a knowledge graph. ALWAYS use the
code-review-graph MCP tools BEFORE using Grep/Glob/Read to explore
the codebase.** The graph is faster, cheaper (fewer tokens), and gives
you structural context (callers, dependents, test coverage) that file
scanning cannot.

## When to use graph tools FIRST

- **Exploring code**: `semantic_search_nodes` or `query_graph` instead of Grep
- **Understanding impact**: `get_impact_radius` instead of manually tracing imports
- **Code review**: `detect_changes` + `get_review_context` instead of reading entire files
- **Finding relationships**: `query_graph` with callers_of/callees_of/imports_of/tests_for
- **Architecture questions**: `get_architecture_overview` + `list_communities`

Fall back to Grep/Glob/Read **only** when the graph doesn't cover what you need.

## graphify-out (static architecture snapshot)

`server/graphify-out/` and `web/graphify-out/` hold a graphify snapshot of each
module (`GRAPH_REPORT.md`, `graph.json`, `wiki/`). Root `graphify-out/` has the
merged server+web graph. Use these for architecture orientation ("what talks to
what", onboarding to a module) — read `GRAPH_REPORT.md` first, then the
relevant `wiki/*.md` article, before Grep. Unlike code-review-graph this does
NOT auto-update; if it looks stale, run `/graphify server --update` /
`/graphify web --update`.

## Key Tools

| Tool | Use when |
| ------ | ---------- |
| `detect_changes` | Reviewing code changes — gives risk-scored analysis |
| `get_review_context` | Need source snippets for review — token-efficient |
| `get_impact_radius` | Understanding blast radius of a change |
| `get_affected_flows` | Finding which execution paths are impacted |
| `query_graph` | Tracing callers, callees, imports, tests, dependencies |
| `semantic_search_nodes` | Finding functions/classes by name or keyword |
| `get_architecture_overview` | Understanding high-level codebase structure |
| `refactor_tool` | Planning renames, finding dead code |

## Workflow

1. The graph auto-updates on file changes (via hooks).
2. Use `detect_changes` for code review.
3. Use `get_affected_flows` to understand impact.
4. Use `query_graph` pattern="tests_for" to check coverage.

## Project Conventions

- Follow `.devin/rules/coding-style.md` for Go and TypeScript style.
- Follow `.devin/rules/ui-quality.md` for admin page and form layouts.
- Use `.devin/skills/todo-planning.md` to track multi-step work.
- Use `.devin/skills/backend-refactor.md` when splitting monolithic backend files.
- Admin features are a SvelteKit SPA under `frontend/src/routes/admin/` backed by REST endpoints in `backend/api/v1/admin_*.go`.
- Admin API handlers must return complete response payloads required by the admin UI.
