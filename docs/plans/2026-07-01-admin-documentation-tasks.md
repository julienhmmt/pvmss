# Tasks: Admin-Authored Documentation (issue #53)

**Input**: `docs/plans/2026-07-01-admin-documentation.md`
**Prerequisites**: plan (required)
**Tests**: Included — this repo mandates `make test-offline` green, so test tasks are required, not optional.
**Organization**: Phased by dependency order. Three user stories are derived from the plan:

- **US1 — Admin authors documentation** (P1): admin can create/edit/delete/toggle Markdown pages with audience + lang + category.
- **US2 — User reads documentation** (P1): users browse a docs index and read rendered pages; admins see admin-audience pages.
- **US3 — Unify documentation in the DB** (P2): migrate the 4 built-in static docs into the new table and retire the file-based read path.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no dependencies on the same phase).
- **[USx]**: User story the task belongs to.

## Path Conventions

- **Web app**: `backend/` (Go), `frontend/src/` (Svelte 5 + TS).

---

## Phase 1: Foundation — DB schema, DB layer, embed seeding

**Purpose**: DDL migration + CRUD data access + one-time seed of built-in docs. BLOCKS all user-story work.

- [ ] T001 [US3] Add `schemaV4` DDL for `documentation_pages` (composite PK `(id, lang)`, `audience` CHECK) in `backend/database/schema.go`
- [ ] T002 [US3] Register `4: schemaV4` and bump `currentSchemaVersion` to 4 in `backend/database/migrations.go` (coordinate vs. issue #54 — renumber to v6 if #54 lands first)
- [ ] T003 [P] [US3] Write migration test: load v1 → run to v4, assert table + composite PK in `backend/database/migrations_test.go`
- [ ] T004 [US1] Define `DocumentationPage` struct in new `backend/database/docs.go`
- [ ] T005 [US1] Implement `ListDocumentationPages`, `GetDocumentationPage(id, lang)`, `ListDocumentationPagesByID(id)` on `*sqliteDB` in `backend/database/docs.go`
- [ ] T006 [US1] Implement `CreateDocumentationPage`, `UpdateDocumentationPage` (refuse id/lang change + `is_system`), `DeleteDocumentationPage` (refuse `is_system`), `ToggleDocumentationPage` on `*sqliteDB` — all with `appendAudit(tx, "documentation_pages", id+"@"+lang, ...)` in `backend/database/docs.go`
- [ ] T007 [US1] Extend `database.DB` interface in `backend/database/database.go` with the 7 new methods
- [ ] T008 [US3] Add `DocumentationPages []DocumentationPage` to `database.AppSettings` and populate in `LoadAppSettings` (ordered by `sort_order, title`) in `backend/database/settings.go`
- [ ] T009 [P] [US1] Write DB layer tests (table-driven, `_test` package): CRUD round-trip, duplicate `(id,lang)` rejected, `is_system` protected, lang fallback, `LoadAppSettings` includes docs in `backend/database/docs_test.go`
- [ ] T010 [US3] Add `//go:embed docs/*.md` (or relocate to `backend/database/seed/docs/`) in new `backend/database/docs_seed.go`
- [ ] T011 [US3] Implement `SeedDocumentationPages(db DB) error` (idempotent insert of missing `(id,lang)` rows, `is_system=1`, audience mapping, fixed `sort_order`) in `backend/database/docs_seed.go`
- [ ] T012 [US3] Implement `SeedDocumentationPagesIntoSettings(s *state.AppSettings)` for offline/no-DB mode in `backend/database/docs_seed.go`
- [ ] T013 [US3] Call `SeedDocumentationPages(db)` after `RunMigrations` in `backend/main.go` `initializeApp`; call settings seeder in the no-DB/offline path
- [ ] T014 [P] [US3] Write seed tests: idempotent (run twice → no duplicates, no clobber of edited title), offline settings seeder yields 4 enabled system pages in `backend/database/docs_seed_test.go`

**Checkpoint**: Schema migrated, DB CRUD works, built-in docs seed into table. No user-story UI/API yet.

---

## Phase 2: State cache + shared Markdown rendering

**Purpose**: Wire docs into the in-memory settings cache and extract the renderer so the read path can go DB-backed. Depends on Phase 1.

- [ ] T015 [US1] Add `state.DocumentationPage` type + `DocumentationPages []DocumentationPage` to `state.AppSettings`; init `[]DocumentationPage{}` in `defaultSettings` in `backend/state/settings.go`
- [ ] T016 [US1] Add `mapDocumentationPagesFromDB` and assign in the `LoadSettingsFromDB` path in `backend/state/manager_db.go`
- [ ] T017 [US1] Add `copyDocumentationPages` and wire into the settings deep-copy in `backend/state/manager_settings.go`
- [ ] T018 [US1] Add `AppSettings` helpers: `GetEnabledDocumentationPages`, `GetDocumentationPage(id, lang)` (en fallback), `AddOrUpdateDocumentationPage`, `RemoveDocumentationPage` in `backend/state/settings.go`
- [ ] T019 [P] [US1] Write state round-trip tests (create → GetSettings has it → update → delete → gone) in `backend/state/manager_db_test.go`
- [ ] T020 [US2] Extract `renderMarkdownToHTML(md string) string` (parser + renderer + `addExternalLinkAttrs`) from `docs.go` into new `backend/api/v1/markdown.go`
- [ ] T021 [P] [US2] Write renderer tests: external links get `target="_blank"`, internal anchors untouched in `backend/api/v1/markdown_test.go`

**Checkpoint**: Docs flow through the settings cache; Markdown rendering is a reusable function.

---

## Phase 3: API — public read + admin CRUD

**Purpose**: Expose docs over HTTP. Depends on Phases 1 and 2.

- [ ] T022 [US2] Refactor `DocsAPIHandler.renderDoc` to read from `state.AppSettings.DocumentationPages` instead of the filesystem; resolve `(type, lang)` → page → `renderMarkdownToHTML` in `backend/api/v1/docs.go`
- [ ] T023 [US2] Replace hardcoded `allowed`/`adminOnlyTypes` maps with: page must exist + be `enabled`; `audience=="admin"` requires valid JWT + `IsAdmin`; `audience=="user"` is public, in `backend/api/v1/docs.go`
- [ ] T024 [US2] Move rendered-HTML cache to key `(id, lang)`; add invalidation helper callable by admin mutations in `backend/api/v1/docs.go`
- [ ] T025 [P] [US2] Write `GetDoc` tests: 404 unknown id, 403 non-admin on admin page, en-fallback when fr absent, in `backend/api/v1/docs_test.go` (new)
- [ ] T026 [US2] Implement `GET /api/v1/docs` public list (audience-filtered by JWT) in new `backend/api/v1/docs_list.go`
- [ ] T027 [US1] Add request/response types `AdminDocResponse`, `AdminDocListResponse`, `CreateDocRequest`, `UpdateDocRequest`, `DocSummary` in `backend/api/v1/admin_types.go`
- [ ] T028 [US1] Add `generateDocSlug(title)` (clone of `generateCloudInitID`) and validators: `id` `^[a-z0-9-]{1,60}$`, `lang` ∈ {en,fr}, `title` 1–120, `audience` ∈ {user,admin}, `body_md` ≤ 256 KB, `category` ≤ 40 in `backend/api/v1/validation.go`
- [ ] T029 [US1] Implement admin CRUD handlers `ListAdminDocs`, `CreateDoc`, `UpdateDoc`, `DeleteDoc`, `ToggleDoc` (offline `HasDB()` branch mirrors `CreateCloudInit`) in new `backend/api/v1/admin_docs.go`
- [ ] T030 [US1] Register routes in `backend/api/v1/router.go`: `GET /api/v1/docs` (public), `GET /api/v1/admin/docs`, `POST /api/v1/admin/docs`, `PUT /api/v1/admin/docs/:id/:lang`, `DELETE /api/v1/admin/docs/:id/:lang`, `POST /api/v1/admin/docs/:id/:lang/toggle`
- [ ] T031 [US1] Add `"documentation_pages": true` to `validTables` audit allowlist in `backend/api/v1/admin_db.go`
- [ ] T032 [P] [US1] Write admin handler tests: validation rejects bad slugs/langs/audience/oversized body; `is_system` protected; toggle flips enabled; offline-mode mirrors cloudinit; list hides admin pages from non-admin in `backend/api/v1/admin_docs_test.go`

**Checkpoint**: Full HTTP API works. US1 and US2 are functionally complete via API.

---

## Phase 4: Frontend — user-facing docs (US2)

**Purpose**: Replace the redirect-only `/docs` index and generalize the viewer. Depends on Phase 3 API.

- [ ] T033 [US2] Create typed API client `listDocs()` → `GET /api/v1/docs`, `getDocHtml(id, lang)` → `GET /api/v1/docs/:id?lang=` in new `frontend/src/lib/api/docs.ts`
- [ ] T034 [US2] Rewrite `frontend/src/routes/docs/+page.svelte`: fetch `listDocs()`, group by `category`, render cards to `/docs/[id]`, lang selector (en/fr with en fallback), show admin pages only when `auth.isAdmin`
- [ ] T035 [US2] Generalize `frontend/src/routes/docs/[type]/+page.svelte`: relax hardcoded `ALLOWED`/`ADMIN_ONLY`, validate `:type` against fetched list, keep TOC/search/reading-time/DOMPurify/IntersectionObserver UX, gate admin pages on `auth.isAdmin` via list lookup
- [ ] T036 [US2] Update `frontend/src/lib/components/layout/Navbar.svelte`: point "Documentation" at `/docs` (index), keep visible to both users and admins (fix the filter that hides non-adminOnly links from admins), keep "Admin Documentation" shortcut to `/docs/admin` for admins
- [ ] T037 [P] [US2] Add i18n keys `nav.docs`, `docs.indexTitle`, `docs.availableDocs`, category labels, `docs.empty` to `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/fr.json`

**Checkpoint**: Users can browse and read all docs via the UI.

---

## Phase 5: Frontend — admin editor (US1)

**Purpose**: Admin UI to author docs. Depends on Phase 3 API; can run in parallel with Phase 4 once API is ready.

- [ ] T038 [US1] Create typed admin API client `listAdminDocs`, `createDoc`, `updateDoc`, `deleteDoc`, `toggleDoc` in new `frontend/src/lib/api/admin/docs.ts` (mirror `admin/cloudinit.ts`)
- [ ] T039 [US1] Build `frontend/src/routes/admin/docs/+page.svelte`: table of all pages (all langs) with title, category, audience badge, lang, enabled toggle, edit, delete (delete disabled for `is_system`); "New page" form + edit modal (title, auto-slug, lang, category, audience, enabled, body Markdown textarea); "Save" + "Save & View" → `goto('/docs/:id')`; loading skeleton, error banner, toast (mirror `admin/cloudinit/+page.svelte`)
- [ ] T040 [US1] Add `{ href: '/admin/docs', icon: BookOpen, label: $t('nav.docs') }` to `frontend/src/lib/components/layout/AdminSidebar.svelte` (import `BookOpen` from phosphor-svelte), placed near `profiles`/`settings`
- [ ] T041 [P] [US1] Add i18n keys `nav.docs`, `admin.docs.*` (title, newPage, editPage, fields, audienceUser, audienceAdmin, confirmDelete, systemProtected) to `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/fr.json`

**Checkpoint**: Admins can create, edit, delete, toggle docs end-to-end through the UI.

---

## Phase 6: Cleanup & verification (US3)

**Purpose**: Retire the file-based read path, update docs, run the QA pipeline. Depends on all prior phases.

- [ ] T042 [US3] Remove `findAPIDocsDir`, `findFile`, `docsDir` field, and the `os.ReadFile` branch from `backend/api/v1/docs.go`; keep `backend/docs/*.md` as embed sources only
- [ ] T043 [US3] Update `CLAUDE.md` "Configuration"/"Architecture" notes: docs are now DB-backed and admin-managed; `backend/docs/*.md` are embed seeds
- [ ] T044 [US3] Run `make qualif` (fmt → lint → test → dev) and fix anything that surfaces; confirm `make test-offline` green
- [ ] T045 [US3] Manual verification: `make dev` → admin creates a "VM creation guidelines" page (audience user) → user sees it at `/docs` and `/docs/<slug>` renders with TOC → edit a built-in doc body persists, delete disabled → non-admin on an admin-audience page gets 403 → `?lang=fr` on en-only page falls back to en → `PVMSS_OFFLINE=true` shows the 4 built-in docs

**Checkpoint**: Done — issue #53 fully delivered.

---

## Dependencies & Execution Order

### Phase dependencies

- **Phase 1 (Foundation)**: No dependencies — start immediately. BLOCKS all user-story work.
- **Phase 2 (State + rendering)**: Depends on Phase 1.
- **Phase 3 (API)**: Depends on Phases 1 and 2.
- **Phase 4 (Frontend user docs)**: Depends on Phase 3 API.
- **Phase 5 (Frontend admin editor)**: Depends on Phase 3 API; can run in parallel with Phase 4.
- **Phase 6 (Cleanup & verification)**: Depends on all prior phases.

### Within each phase

- DDL before DB methods (T001 → T005/T006).
- DB methods before interface extension (T005/T006 → T007).
- DB layer before state cache (Phase 1 → T015–T018).
- Renderer extraction before read-path refactor (T020 → T022).
- API handlers before route registration (T029 → T030).
- Frontend API client before pages that use it (T033 → T034/T035; T038 → T039).

### Parallel opportunities

- T003, T009, T014, T019, T021, T025, T032, T037, T041 are marked `[P]` — independent test/i18n tasks runnable in parallel within their phase.
- Phases 4 and 5 can proceed in parallel once Phase 3 lands.

---

## Implementation Strategy

### MVP first (US1 + US2, skip US3 migration)

1. Complete Phase 1 (foundation) — but skip T010–T014 seeding; start with an empty table.
2. Complete Phases 2–5.
3. **Stop and validate**: admin can create docs, users can read them.
4. Deploy/demo the MVP.

### Incremental delivery (add US3)

5. Add T010–T014 seeding (Phase 1 remainder).
6. Add Phase 6 cleanup — retire the file-based read path, migrate the 4 built-in docs.
7. Final `make qualif` + manual verification (T044, T045).

> Note: doing US3 last keeps the static file system as a fallback during development and avoids a big-bang migration. The plan's Task ordering assumes US3 is part of the foundation, but the tasks above are sequenced so US3's seeding/cleanup can be deferred without breaking US1/US2.
