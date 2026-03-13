# Refactor: Rename `New...` Constructor Functions

## Problem

The codebase uses Go's idiomatic `New...` constructor naming convention (e.g., `NewAuthHandler`, `NewRestyClient`). This is verbose and repetitive. We want cleaner, more concise names.

## Constraint

Simply removing `New` causes conflicts: `NewAuthHandler()` → `AuthHandler()` would shadow the type `AuthHandler`. The replacement must be a different prefix.

## Decision: `New` → `Make`

Replace `New` with `Make` across all constructor functions.

- `NewAuthHandler(sm)` → `MakeAuthHandler(sm)`
- `NewRestyClient(...)` → `MakeRestyClient(...)`
- `NewLRUCache(...)` → `MakeLRUCache(...)`

This is a pure mechanical rename — no logic changes, no API changes, no type renames.

## All Functions to Rename

### `backend/handlers/`

| Before | After | File |
|--------|-------|------|
| `NewErrorHelper` | `MakeErrorHelper` | `errors.go` |
| `NewFormSession` | `MakeFormSession` | `form_session.go` |
| `NewTemplateDataWithOptions` | `MakeTemplateDataWithOptions` | `template_data.go` |
| `NewMessageHelper` | `MakeMessageHelper` | `template_data.go` |
| `NewRouteHelpers` | `MakeRouteHelpers` | `route_helpers.go` |
| `NewAdminPageRoutes` | `MakeAdminPageRoutes` | `route_helpers.go` |
| `NewBaseAdminHandler` | `MakeBaseAdminHandler` | `base_handlers.go` |
| `NewBaseFormHandler` | `MakeBaseFormHandler` | `base_handlers.go` |
| `NewBaseAPIHandler` | `MakeBaseAPIHandler` | `base_handlers.go` |
| `NewAdminVMsHandler` | `MakeAdminVMsHandler` | `admin_vms.go` |
| `NewInputSanitizer` | `MakeInputSanitizer` | `sanitize.go` |
| `NewTagsHandler` | `MakeTagsHandler` | `tags.go` |
| `NewVMHandler` | `MakeVMHandler` | `vm_details_base.go` |
| `NewVMSnapshotsHandler` | `MakeVMSnapshotsHandler` | `vm_snapshots.go` |
| `NewCloudInitHandler` | `MakeCloudInitHandler` | `admin_cloudinit.go` |
| `NewProfileHandler` | `MakeProfileHandler` | `profile.go` |
| `NewHandlerContext` | `MakeHandlerContext` | `helpers.go` |
| `NewAuthHandler` | `MakeAuthHandler` | `auth.go` |
| `NewValidationHelper` | `MakeValidationHelper` | `validation.go` |
| `NewStorageHandler` | `MakeStorageHandler` | `storage.go` |
| `NewLanguageHandler` | `MakeLanguageHandler` | `language.go` |
| `NewDocsHandler` | `MakeDocsHandler` | `docs.go` |
| `NewDiskHandler` | `MakeDiskHandler` | `disks.go` |
| `NewMessageHandlers` | `MakeMessageHandlers` | `template_helpers.go` |
| `NewContextualMessageHelper` | `MakeContextualMessageHelper` | `template_helpers.go` |
| `NewSettingsHandler` | `MakeSettingsHandler` | `settings.go` |
| `NewCSSHandler` | `MakeCSSHandler` | `css.go` |
| `NewVMCreateOptimizedHandler` | `MakeVMCreateOptimizedHandler` | `vm_create.go` |
| `NewSearchOptimizedHandler` | `MakeSearchOptimizedHandler` | `search.go` |
| `NewAdminOptimizedHandler` | `MakeAdminOptimizedHandler` | `admin.go` |
| `NewUserPoolHandler` | `MakeUserPoolHandler` | `user_pool.go` |
| `NewVMBRHandler` | `MakeVMBRHandler` | `vmbr.go` |
| `NewHealthHandler` | `MakeHealthHandler` | `health.go` |

### `backend/api/v1/`

| Before | After | File |
|--------|-------|------|
| `NewVMHandler` | `MakeVMHandler` | `vms.go` |
| `NewVMActionHandler` | `MakeVMActionHandler` | `vm_actions.go` |
| `NewAuthHandler` | `MakeAuthHandler` | `auth.go` |

> Note: `NewVMHandler` exists in both `handlers/` and `api/v1/` — both become `MakeVMHandler` in their respective packages.

### `backend/proxmox/`

| Before | After | File |
|--------|-------|------|
| `NewClient` | `MakeClient` | `telmate_client.go` |
| `NewClientCookieAuth` | `MakeClientCookieAuth` | `telmate_client.go` |
| `NewRestyClientFromEnv` | `MakeRestyClientFromEnv` | `helpers.go` |
| `NewLRUCache` | `MakeLRUCache` | `cache.go` |
| `NewRestyClient` | `MakeRestyClient` | `resty_client.go` |

### `backend/middleware/`

| Before | After | File |
|--------|-------|------|
| `NewMiddlewareLogger` | `MakeMiddlewareLogger` | `util.go` |
| `NewRateLimiter` | `MakeRateLimiter` | `ratelimit.go` |

### `backend/utils/`

| Before | After | File |
|--------|-------|------|
| `NewErrorWrapper` | `MakeErrorWrapper` | `errors.go` |

### `backend/state/`

| Before | After | File |
|--------|-------|------|
| `NewAppState` | `MakeAppState` | `manager.go` |

### `backend/app/`

| Before | After | File |
|--------|-------|------|
| `NewTestApp` | `MakeTestApp` | `app.go` |

### `backend/tests/`

| Before | After | File |
|--------|-------|------|
| `NewRequest` | `MakeRequest` | `helpers.go` |

## Total

- **45 functions** renamed
- **81 files** contain references (definitions + call sites)

## Implementation Steps

1. Use `sed` or IDE find-replace to rename each function definition
2. Update all call sites in the same pass
3. Run `make go-fmt` to reformat
4. Run `make test-offline` to verify nothing broken
5. Run `make go-lint` to check for any issues

### Suggested sed commands (run from repo root)

```bash
# Rename all New -> Make in Go files (definitions + call sites)
find backend -name '*.go' | xargs sed -i '' \
  -e 's/\bNewErrorHelper\b/MakeErrorHelper/g' \
  -e 's/\bNewFormSession\b/MakeFormSession/g' \
  -e 's/\bNewTemplateDataWithOptions\b/MakeTemplateDataWithOptions/g' \
  -e 's/\bNewMessageHelper\b/MakeMessageHelper/g' \
  -e 's/\bNewRouteHelpers\b/MakeRouteHelpers/g' \
  -e 's/\bNewAdminPageRoutes\b/MakeAdminPageRoutes/g' \
  -e 's/\bNewBaseAdminHandler\b/MakeBaseAdminHandler/g' \
  -e 's/\bNewBaseFormHandler\b/MakeBaseFormHandler/g' \
  -e 's/\bNewBaseAPIHandler\b/MakeBaseAPIHandler/g' \
  -e 's/\bNewAdminVMsHandler\b/MakeAdminVMsHandler/g' \
  -e 's/\bNewInputSanitizer\b/MakeInputSanitizer/g' \
  -e 's/\bNewTagsHandler\b/MakeTagsHandler/g' \
  -e 's/\bNewVMSnapshotsHandler\b/MakeVMSnapshotsHandler/g' \
  -e 's/\bNewCloudInitHandler\b/MakeCloudInitHandler/g' \
  -e 's/\bNewProfileHandler\b/MakeProfileHandler/g' \
  -e 's/\bNewHandlerContext\b/MakeHandlerContext/g' \
  -e 's/\bNewValidationHelper\b/MakeValidationHelper/g' \
  -e 's/\bNewStorageHandler\b/MakeStorageHandler/g' \
  -e 's/\bNewLanguageHandler\b/MakeLanguageHandler/g' \
  -e 's/\bNewDocsHandler\b/MakeDocsHandler/g' \
  -e 's/\bNewDiskHandler\b/MakeDiskHandler/g' \
  -e 's/\bNewMessageHandlers\b/MakeMessageHandlers/g' \
  -e 's/\bNewContextualMessageHelper\b/MakeContextualMessageHelper/g' \
  -e 's/\bNewSettingsHandler\b/MakeSettingsHandler/g' \
  -e 's/\bNewCSSHandler\b/MakeCSSHandler/g' \
  -e 's/\bNewVMCreateOptimizedHandler\b/MakeVMCreateOptimizedHandler/g' \
  -e 's/\bNewSearchOptimizedHandler\b/MakeSearchOptimizedHandler/g' \
  -e 's/\bNewAdminOptimizedHandler\b/MakeAdminOptimizedHandler/g' \
  -e 's/\bNewUserPoolHandler\b/MakeUserPoolHandler/g' \
  -e 's/\bNewVMBRHandler\b/MakeVMBRHandler/g' \
  -e 's/\bNewHealthHandler\b/MakeHealthHandler/g' \
  -e 's/\bNewVMActionHandler\b/MakeVMActionHandler/g' \
  -e 's/\bNewClient\b/MakeClient/g' \
  -e 's/\bNewClientCookieAuth\b/MakeClientCookieAuth/g' \
  -e 's/\bNewRestyClientFromEnv\b/MakeRestyClientFromEnv/g' \
  -e 's/\bNewLRUCache\b/MakeLRUCache/g' \
  -e 's/\bNewRestyClient\b/MakeRestyClient/g' \
  -e 's/\bNewMiddlewareLogger\b/MakeMiddlewareLogger/g' \
  -e 's/\bNewRateLimiter\b/MakeRateLimiter/g' \
  -e 's/\bNewErrorWrapper\b/MakeErrorWrapper/g' \
  -e 's/\bNewAppState\b/MakeAppState/g' \
  -e 's/\bNewTestApp\b/MakeTestApp/g' \
  -e 's/\bNewRequest\b/MakeRequest/g'

# Handle the two NewVMHandler / NewAuthHandler collisions (same name, different packages, same target name — safe)
find backend -name '*.go' | xargs sed -i '' \
  -e 's/\bNewVMHandler\b/MakeVMHandler/g' \
  -e 's/\bNewAuthHandler\b/MakeAuthHandler/g'
```

## Notes

- `NewVMHandler` exists in both `handlers/` and `api/v1/` packages — both rename to `MakeVMHandler` in their own package, no conflict
- `NewAuthHandler` similarly exists in both `handlers/auth.go` and `api/v1/auth.go` — same resolution
- No type renames required; only function names change
- This refactor is purely cosmetic — no behavior changes
- Low risk: compiler will catch any missed call sites
