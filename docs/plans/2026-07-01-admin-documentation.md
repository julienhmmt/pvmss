# Admin-Authored Documentation — Plan for issue #53

**Goal:** Let administrators create, edit, and expose Markdown documentation
pages to users (VM-creation guidelines, template examples, cloud-init how-tos,
etc.). Unify documentation by migrating the existing bundled static docs
(`backend/docs/*.md`) into a new DB-backed `documentation_pages` table, retiring
the file-based read path while keeping the rich docs viewer UX.

**Architecture:** Mirror the proven cloudinit-templates / VM-profiles pattern: a
new DB table → `database.DB` interface methods → seeded into the `AppSettings`
cache (both `database.AppSettings` and `state.AppSettings`) → admin CRUD
handlers → admin frontend page + sidebar entry. Rendering reuses the existing
`gomarkdown` pipeline (extracted to a shared helper); the current
`DocsAPIHandler` switches from filesystem reads to the in-memory cache. The
existing `GET /api/v1/docs/:type` URL contract is preserved (now DB-backed), and
a new `GET /api/v1/docs` list endpoint is added.

**Tech Stack:** Go (stdlib + existing `gomarkdown`), SQLite (modernc),
`//go:embed` for one-time seeding, Svelte 5 runes + TypeScript + Tailwind. No
new runtime dependencies.

---

## Scope

### In scope

- New `documentation_pages` table (composite PK `(id, lang)`) with migration v4
  + one-time seed of the 4 built-in docs × 2 langs (en/fr) from embedded
  markdown.
- Admin CRUD API: list / create / update / delete / toggle, with slug + lang
  identity, audience (`user`/`admin`), category, sort_order, enabled,
  `is_system` protection.
- Public read API: `GET /api/v1/docs` (audience-filtered list) and
  `GET /api/v1/docs/:type` (rendered HTML, lang + en fallback) — replaces the
  filesystem read.
- State caching: `DocumentationPages` in both `database.AppSettings` and
  `state.AppSettings`, loaded in `LoadAppSettings`, deep-copied, rendered-HTML
  cache invalidated on mutation.
- Frontend: docs index page (`/docs`), generalized viewer (`/docs/[type]` no
  longer hardcoded allowlist), admin editor page (`/admin/docs`) with Markdown
  textarea + save, AdminSidebar entry, Navbar adjustment, i18n keys, typed API
  clients.
- Audit logging for all mutations; `documentation_pages` added to the audit
  table allowlist.
- Offline mode: built-in docs seeded into in-memory settings from embed; admin
  mutations update in-memory settings (mirrors cloudinit).

### Out of scope (deferred)

- Live Markdown preview in the editor (MVP uses textarea + "Save & View"; a
  client-side renderer e.g. `marked` can be added later).
- Rich-text / WYSIWYG authoring.
- Per-page access control beyond the `user`/`admin` audience flag (no per-user
  or per-group grants).
- Version history / draft workflow for docs (audit log already captures old/new
  diffs).
- Multi-cluster scoping of docs (issue #54); docs are global in this plan.

### Backwards compatibility

- `GET /api/v1/docs/:type` URL and response shape (`{html}`) unchanged →
  existing frontend viewer and Navbar links keep working.
- The 8 built-in doc rows are seeded as `is_system=1`: `id`/`lang` immutable,
  deletion forbidden, but title/category/body/audience/enabled/order editable so
  admins can customize.
- **Audience mapping (one behavior change):** `user` + `cloud-init` → `user`
  audience (public, no JWT — unchanged). `proxmox-permissions` → `admin`
  (admin-only — unchanged). `admin` → `admin` audience, which **tightens** it
  from "any authenticated user" to "admin-only". This is intentional (it is the
  admin documentation); flagged here so reviewers notice. Admins can relax it
  back by editing the audience since system docs allow audience edits.
- `make test-offline` must pass unchanged; offline mode keeps the 4 built-in
  docs available.

---

## Key design decisions

1. **Composite PK `(id, lang)` + en fallback.** Reconciles "single language per
   page" authoring with preserving the bilingual migrated content. Admins author
   one lang per create (default `en`); the read API resolves `?lang=fr` → fall
   back to `en` when the requested lang is absent. Optional translations can be
   added later as additional `(id, lang)` rows.
2. **`is_system` flag, not a separate table.** The 4 built-in docs live in the
   same table with `is_system=1`; protected from delete and id/lang change but
   otherwise editable. Avoids a parallel read path.
3. **`//go:embed` seeding, not SQL blobs.** The existing `backend/docs/*.md`
   files become embed sources for a one-time Go seeding step (idempotent: only
   inserts missing `(id, lang)` rows). The migration itself is DDL-only, keeping
   the migration system pure-DDL.
4. **Reuse `gomarkdown` rendering.** Extract `renderMarkdownToHTML(md)` (parser
   + renderer + `addExternalLinkAttrs`) from `docs.go` into a shared helper so
   static-migrated and admin-authored docs render identically. Frontend keeps
   DOMPurify sanitization as defense-in-depth.
5. **Rendered-HTML cache in the handler**, keyed `(id, lang)`, invalidated on
   any create/update/delete/toggle. Mirrors the existing per-`(type,lang)`
   cache.
6. **Audience = `user` (public) / `admin` (admin JWT).** Two-tier, per the
   decision in the planning conversation. JWT verification reuses the existing
   `parseClaims` path in `docs.go`.
7. **Slug identity is admin-chosen, immutable.** `id` matches
   `^[a-z0-9-]{1,60}$`; a `generateDocSlug(title)` helper mirrors
   `generateCloudInitID` for auto-slug from title.

---

## Database schema — migration v4

> **Version coordination:** `currentSchemaVersion` is currently 3. The #54
> multi-cluster plan also reserves v4/v5 but has not landed. **This plan takes
> v4.** If #54 lands first, renumber this to v6 and update the `migrations` map
> + `currentSchemaVersion` accordingly.

Add to `backend/database/schema.go`:

```sql
CREATE TABLE IF NOT EXISTS documentation_pages (
    id          TEXT NOT NULL,
    lang        TEXT NOT NULL DEFAULT 'en',
    title       TEXT NOT NULL,
    category    TEXT,
    body_md     TEXT NOT NULL,
    audience    TEXT NOT NULL DEFAULT 'user',   -- 'user' | 'admin'
    enabled     BOOLEAN NOT NULL DEFAULT 1,
    is_system   BOOLEAN NOT NULL DEFAULT 0,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, lang),
    CHECK (audience IN ('user','admin'))
);
```

Bump `currentSchemaVersion` to 4 in `backend/database/migrations.go`, register
`4: schemaV4`.

---

## Task 1 — DB layer for documentation pages

**Files:**

- New: `backend/database/docs.go`
- New: `backend/database/docs_test.go`
- Modify: `backend/database/database.go` (interface)
- Modify: `backend/database/settings.go` (`AppSettings` + `LoadAppSettings`)

**Steps:**

1. Define `type DocumentationPage struct { ID, Lang, Title, Category, BodyMD, Audience string; Enabled, IsSystem bool; SortOrder int }`.
2. Methods on `*sqliteDB`:
   - `ListDocumentationPages() ([]DocumentationPage, error)` — all rows, ordered
     by `sort_order, title`.
   - `GetDocumentationPage(id, lang string) (*DocumentationPage, error)` — nil
     when not found.
   - `ListDocumentationPagesByID(id string) ([]DocumentationPage, error)` — all
     langs for one slug.
   - `CreateDocumentationPage(*DocumentationPage, changedBy string) error` —
     rejects duplicate `(id, lang)`.
   - `UpdateDocumentationPage(*DocumentationPage, changedBy string) error` —
     refuses id/lang change; refuses on `is_system` rows; returns `ErrNotFound`
     when missing.
   - `DeleteDocumentationPage(id, lang, changedBy string) error` — refuses
     `is_system`; returns `ErrNotFound` when missing.
   - `ToggleDocumentationPage(id, lang, changedBy string) error` — flips
     `enabled`.
   - All mutations `appendAudit(tx, "documentation_pages", id+"@"+lang, ...)`.
3. Extend the `database.DB` interface with these methods.
4. Add `DocumentationPages []DocumentationPage` to `database.AppSettings`;
   populate in `LoadAppSettings` (ordered by `sort_order, title`).
5. Tests (table-driven, `_test` package): CRUD round-trip; duplicate
   `(id, lang)` rejected; `is_system` protected from delete and id/lang change;
   lang fallback query returns en when fr missing; `LoadAppSettings` includes
   docs.

---

## Task 2 — Seed built-in docs from embed

**Files:**

- New: `backend/database/docs_seed.go`
- New: `backend/database/docs_seed_test.go`
- Keep: `backend/docs/*.md` in place as embed sources
- Modify: `backend/main.go` (`initializeApp`)

**Steps:**

1. `//go:embed docs/*.md` (or move files to `backend/database/seed/docs/` and
   embed there) into a `map[filename]bytes` or named embed vars.
2. `SeedDocumentationPages(db DB) error` — idempotent: for each built-in
   `(id, lang)` missing from the table, insert with `is_system=1`, audience
   mapping (`user`, `cloud-init` → `user`; `admin`, `proxmox-permissions` →
   `admin`), `sort_order` per a fixed order (`user`=0, `cloud-init`=1,
   `admin`=2, `proxmox-permissions`=3). Skip rows that already exist (so admin
   edits are never clobbered).
3. `SeedDocumentationPagesIntoSettings(s *state.AppSettings)` — same map, used
   in offline mode (no DB) so the 4 built-in docs are still available.
4. Call `SeedDocumentationPages(db)` after `RunMigrations` in `initializeApp`
   (DB mode); call the settings seeder in the no-DB/offline path.
5. Tests: seeding is idempotent (run twice → no duplicates, no clobber of an
   edited title); offline settings seeder yields 4 enabled system pages.

---

## Task 3 — State cache + mapping

**Files:**

- Modify: `backend/state/settings.go` (`AppSettings` + `defaultSettings` + copy
  helpers)
- Modify: `backend/state/manager_db.go` (mapping)
- Modify: `backend/state/manager_settings.go` (deep copy)

**Steps:**

1. Add `DocumentationPages []DocumentationPage` to `state.AppSettings`; init
   `[]DocumentationPage{}` in `defaultSettings`.
2. Define `state.DocumentationPage` (mirror of DB type) +
   `mapDocumentationPagesFromDB` in `manager_db.go`; assign in the
   `LoadSettingsFromDB` path.
3. Add `copyDocumentationPages` to `manager_settings.go` and wire into the
   settings deep-copy.
4. Add helpers on `*AppSettings`:
   - `GetEnabledDocumentationPages() []DocumentationPage`
   - `GetDocumentationPage(id, lang string) (*DocumentationPage, bool)` with en
     fallback
   - `AddOrUpdateDocumentationPage(page DocumentationPage)`
   - `RemoveDocumentationPage(id, lang string) bool`
5. Tests in `manager_db_test.go` mirroring the cloudinit template round-trip
   (create → GetSettings has it → update → delete → gone).

---

## Task 4 — Extract shared Markdown rendering

**Files:**

- Modify: `backend/api/v1/docs.go`
- Optional new: `backend/api/v1/markdown.go`

**Steps:**

1. Extract the `gomarkdown` parser/renderer + `addExternalLinkAttrs` into
   `renderMarkdownToHTML(md string) string` (package-level func).
2. Refactor `DocsAPIHandler.renderDoc` to read from
   `state.AppSettings.DocumentationPages` via the handler's `state` instead of
   the filesystem: resolve `(type, lang)` → page →
   `renderMarkdownToHTML(page.BodyMD)`. Drop `docsDir`, `findFile`,
   `findAPIDocsDir`, the `os.ReadFile` path.
3. Keep `parseClaims` and the audience gate; replace the hardcoded
   `allowed`/`adminOnlyTypes` maps with: page must exist + be `enabled`;
   `audience == "admin"` requires valid JWT + `IsAdmin`; `audience == "user"`
   is public.
4. Rendered-HTML cache keyed `(id, lang)`; invalidate helper called by admin
   mutations (Task 5). Since handlers are separate structs, expose invalidation
   via a small package-level cache or a method on `DocsAPIHandler` wired into
   the admin handler (thread the `*DocsAPIHandler` into `AdminMutationsHandler`
   or use a shared invalidator).
5. Tests: `renderMarkdownToHTML` adds `target="_blank"` to external links only;
   `GetDoc` returns 404 for unknown id, 403 for non-admin on admin page,
   en-fallback when fr absent.

---

## Task 5 — Public read + admin CRUD API

**Files:**

- New: `backend/api/v1/admin_docs.go`
- New: `backend/api/v1/admin_docs_test.go`
- New: `backend/api/v1/docs_list.go`
- Modify: `backend/api/v1/admin_types.go`
- Modify: `backend/api/v1/validation.go`
- Modify: `backend/api/v1/router.go`
- Modify: `backend/api/v1/admin_db.go` (audit allowlist)

**Routes** (registered in `router.go`):

- `GET /api/v1/docs` (public list, audience-filtered by JWT) — new
  `docs_list.go`.
- `GET /api/v1/docs/:type` — existing, now DB-backed (Task 4).
- `GET /api/v1/admin/docs` → list all (all langs, enabled+disabled).
- `POST /api/v1/admin/docs` → create.
- `PUT /api/v1/admin/docs/:id/:lang` → update.
- `DELETE /api/v1/admin/docs/:id/:lang` → delete (refuse `is_system`).
- `POST /api/v1/admin/docs/:id/:lang/toggle` → toggle enabled.

> httprouter note: `GET /api/v1/docs` (collection) and `GET /api/v1/docs/:type`
> (item) coexist at different depths — no conflict. A `/api/v1/docs/pages`
> static segment would conflict with `:type`, so the list lives at
> `/api/v1/docs`.

**Request/response types** (`admin_types.go`): `AdminDocResponse`,
`AdminDocListResponse`, `CreateDocRequest`, `UpdateDocRequest`, `DocSummary`
(id, lang, title, category, audience, enabled, sort_order, is_system) for the
list (no body). Render endpoint keeps `{html}`.

**Validation** (`validation.go`): `id` matches `^[a-z0-9-]{1,60}$` (required on
create, immutable); `lang` ∈ {en, fr}; `title` 1–120 chars; `audience` ∈
{user, admin}; `body_md` non-empty, ≤ 256 KB; `category` optional ≤ 40 chars.
Reuse `generateDocSlug(title)` (clone of `generateCloudInitID`) for auto-slug.
`lang` defaults to `en` when omitted.

**Offline mode:** `HasDB()` branch — mutations update in-memory `AppSettings`
(mirror `CreateCloudInit`'s offline path). In offline mode, refuse creating a
doc with an id colliding with a system doc.

**Audit:** add `"documentation_pages": true` to `validTables` in
`admin_db.go`.

**Tests:** validation rejects bad slugs/langs/audience/oversized body;
`is_system` protected from delete + id/lang change; toggle flips enabled;
offline-mode handlers behave like cloudinit offline handlers; list endpoint
hides `admin`-audience pages from non-admin callers.

---

## Task 6 — Frontend user-facing docs

**Files:**

- Modify: `frontend/src/routes/docs/+page.svelte` (index)
- Modify: `frontend/src/routes/docs/[type]/+page.svelte` (viewer)
- Modify: `frontend/src/lib/components/layout/Navbar.svelte`
- New: `frontend/src/lib/api/docs.ts`
- Modify: `frontend/src/lib/i18n/en.json`, `frontend/src/lib/i18n/fr.json`

**Steps:**

1. `src/lib/api/docs.ts`: `listDocs()` → `GET /api/v1/docs`; `getDocHtml(id,
   lang)` → `GET /api/v1/docs/:id?lang=` (typed; mirrors existing `api.get`
   usage).
2. `routes/docs/+page.svelte`: replace the redirect-to-`/docs/user` with a docs
   index — fetch `listDocs()`, group by `category`, render cards linking to
   `/docs/[id]`, with a lang selector (en/fr) that filters to available langs
   (en fallback). Show admin-audience pages only when `auth.isAdmin`.
3. `routes/docs/[type]/+page.svelte`: relax the hardcoded `ALLOWED`/
   `ADMIN_ONLY` arrays — instead validate `:type` against the fetched list;
   keep the TOC/search/reading-time/DOMPurify/IntersectionObserver UX intact.
   Keep the `api.get('/api/v1/docs/${type}?lang=...')` call (URL unchanged).
   Gate admin-audience pages on `auth.isAdmin` (replace
   `ADMIN_ONLY.includes(docType)` with a lookup from the list).
4. `Navbar.svelte`: point "Documentation" at `/docs` (index) instead of
   `/docs/user`; ensure the link is visible to both users and admins (the
   current filter hides non-adminOnly links from admins — adjust so the docs
   index link shows for everyone). Keep an "Admin Documentation" shortcut to
   `/docs/admin` for admins.
5. i18n: add `nav.docs` / reuse `nav.documentation`; add `docs.indexTitle`,
   `docs.availableDocs`, category labels, and `docs.empty` (no pages) keys to
   en.json + fr.json.

---

## Task 7 — Frontend admin docs editor

**Files:**

- New: `frontend/src/routes/admin/docs/+page.svelte`
- New: `frontend/src/lib/api/admin/docs.ts`
- Modify: `frontend/src/lib/components/layout/AdminSidebar.svelte`
- Modify: i18n files

**Steps:**

1. `src/lib/api/admin/docs.ts`: typed `listAdminDocs`, `createDoc`, `updateDoc`,
   `deleteDoc`, `toggleDoc` (mirror `admin/cloudinit.ts`).
2. `routes/admin/docs/+page.svelte`: list all pages (all langs) in a table with
   title, category, audience badge, lang, enabled toggle, edit, delete (delete
   disabled for `is_system`). "New page" form + edit modal: fields title, slug
   (auto-generated from title, editable), lang (en/fr), category, audience
   (user/admin), enabled, body Markdown textarea. "Save" → create/update;
   "Save & View" → save then `goto('/docs/:id')`. Mirror the structure of
   `admin/cloudinit/+page.svelte` (loading skeleton, error banner, toast).
3. `AdminSidebar.svelte`: add `{ href: '/admin/docs', icon: BookOpen, label:
   $t('nav.docs') }` (import `BookOpen` from phosphor-svelte), placed near
   `profiles`/`settings`.
4. i18n: add `nav.docs`, `admin.docs.*` (title, newPage, editPage, fields,
   audienceUser, audienceAdmin, confirmDelete, systemProtected) to en.json +
   fr.json.

---

## Task 8 — Migration & retirement cleanup

**Files:**

- Modify: `backend/api/v1/docs.go` (remove dead FS code)
- Modify: `backend/database/migrations_test.go`
- Verify: `backend/docs/*.md` remain only as embed sources

**Steps:**

1. After Task 4, remove `findAPIDocsDir`, `findFile`, `docsDir` field, and the
   `os.ReadFile` branch from `docs.go`. Keep `backend/docs/*.md` as embed
   sources (Task 2).
2. Add a migration test in `backend/database/migrations_test.go`: load v1 → run
   to v4 → assert `documentation_pages` exists, composite PK `(id, lang)`, and
   the 8 seeded rows present with correct audiences + `is_system=1`.
3. Update `CLAUDE.md` "Configuration"/"Architecture" notes: docs are now
   DB-backed and admin-managed; `backend/docs/*.md` are embed seeds.
4. Run `make qualif` (fmt → lint → test → dev) and fix anything that surfaces.
   Confirm `make test-offline` green and the offline docs still render.

---

## Verification

- `make test-offline` (CI standard) passes; new DB/api/state tests included.
- `make go-lint` clean (watch `interfacebloat` on the growing `database.DB`
  interface — already `//nolint:interfacebloat`).
- Manual: `make dev`, log in as admin → `/admin/docs` → create a "VM creation
  guidelines" page (audience user) → log in as user → `/docs` sees the new card
  → `/docs/<slug>` renders with TOC. Edit a built-in doc body → change
  persists, delete button disabled. Non-admin hitting an `admin`-audience page
  → 403. `?lang=fr` on a page with only en → falls back to en.
- Offline: `PVMSS_OFFLINE=true` → 4 built-in docs still listed and renderable;
  admin create persists in-memory for the session.

---

## Open notes

- **Schema version** depends on landing order vs. issue #54 (see "Version
  coordination" under Database schema).
- **`admin` doc tightening** (any-user → admin-only) is the one intentional
  behavior change; called out in Backwards compatibility.
