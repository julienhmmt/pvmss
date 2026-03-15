# Svelte Admin i18n Design Spec

**Date:** 2026-03-15
**Status:** Approved
**Scope:** Internationalization (EN/FR) for the SvelteKit admin SPA

## Overview

Add full EN/FR internationalization to the Svelte admin panel using `svelte-i18n` with statically imported JSON translation files. The infrastructure is structured to support future user-facing pages.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Library | `svelte-i18n` | Mature, store-based, reactive, works with static adapter |
| Loading strategy | Static imports (`addMessages`) | 2 languages, ~134 keys — no need for async loading |
| Translation source | Written from scratch | Modern, professional wording adapted to the new UI |
| Locale detection | localStorage > cookie > navigator > fallback EN | Consistent with old Vue frontend cookie, instant SPA access |
| Persistence | localStorage + cookie `pvmss_lang` | Cookie ensures cross-frontend coherence, localStorage gives sync access |
| Scope | Admin pages + infrastructure for future user pages | `user.*` namespace reserved but empty |

## File Structure

```
frontend-svelte/src/lib/i18n/
├── index.ts          # init(), locale detection, setLocale() export
├── en.json           # English translations
└── fr.json           # French translations
```

## Translation Key Namespace

Flat keys with dot notation, organized by domain:

- `common.*` — Shared labels: Save, Cancel, Delete, Edit, Enabled, Yes, No, Actions, Name, Node, Type, etc.
- `nav.*` — Navbar links and sidebar items
- `admin.dashboard.*` — Dashboard page
- `admin.nodes.*` — Nodes page
- `admin.storage.*` — Storage page
- `admin.vmbr.*` — Network bridges page
- `admin.iso.*` — ISO images page
- `admin.vms.*` — Virtual machines page
- `admin.tags.*` — Tags page
- `admin.limits.*` — Resource limits page
- `admin.cloudinit.*` — Cloud-init page
- `admin.appinfo.*` — App info page
- `admin.userpool.*` — User pools page
- `user.*` — Reserved for future user pages (empty)

### Interpolation

Dynamic values use `svelte-i18n` interpolation:

```
"admin.vms.pagination.showing": "Showing {start} to {end} of {total} VMs"
```

Usage: `$t('admin.vms.pagination.showing', { values: { start, end, total } })`

## Initialization (`lib/i18n/index.ts`)

```typescript
import { init, addMessages, getLocaleFromNavigator } from 'svelte-i18n';
import en from './en.json';
import fr from './fr.json';

function getInitialLocale(): string {
  const stored = localStorage.getItem('pvmss_lang');
  if (stored === 'fr' || stored === 'en') return stored;

  const cookie = document.cookie.match(/pvmss_lang=(en|fr)/)?.[1];
  if (cookie) return cookie;

  const nav = getLocaleFromNavigator() ?? 'en';
  return nav.startsWith('fr') ? 'fr' : 'en';
}

export function setLocale(lang: 'en' | 'fr') {
  locale.set(lang);
  localStorage.setItem('pvmss_lang', lang);
  document.cookie = `pvmss_lang=${lang};path=/;max-age=31536000;SameSite=Lax`;
}

addMessages('en', en);
addMessages('fr', fr);
init({ fallbackLocale: 'en', initialLocale: getInitialLocale() });
```

**Entry point:** Imported in root `+layout.svelte` before any rendering:

```typescript
import '$lib/i18n';
```

## Navbar Language Selector

Replace current `/set-lang` navigation links with direct `setLocale()` calls:

```svelte
<Button variant="ghost" size="sm" onclick={() => setLocale('fr')} class="px-2 text-xs">FR</Button>
<Button variant="ghost" size="sm" onclick={() => setLocale('en')} class="px-2 text-xs">EN</Button>
```

No page navigation, instant language switch.

## Component Usage Pattern

```svelte
<script lang="ts">
  import { t } from 'svelte-i18n';
</script>

<PageHeader title={$t('admin.storage.title')} icon={Database} />
<Button>{$t('common.save')}</Button>
```

## Migration Rules

1. **All user-visible strings** go through `$t()` — titles, labels, buttons, messages, placeholders, descriptions
2. **API data stays untranslated** — node names, VM names, storage names, tags, error messages from backend
3. **`common.*` keys are shared** across pages — no duplication
4. **Page-specific keys** use `admin.<page>.*` namespace

## Estimated Key Count (~134 keys)

| Namespace | Est. keys |
|-----------|-----------|
| `common.*` | ~20 |
| `nav.*` | ~15 |
| `admin.dashboard.*` | ~5 |
| `admin.nodes.*` | ~8 |
| `admin.storage.*` | ~5 |
| `admin.vmbr.*` | ~5 |
| `admin.iso.*` | ~4 |
| `admin.vms.*` | ~12 |
| `admin.tags.*` | ~8 |
| `admin.limits.*` | ~15 |
| `admin.cloudinit.*` | ~12 |
| `admin.appinfo.*` | ~15 |
| `admin.userpool.*` | ~10 |

## Files to Modify (13 pages + 2 layout components)

### New files (3):
- `src/lib/i18n/index.ts`
- `src/lib/i18n/en.json`
- `src/lib/i18n/fr.json`

### Modified files (15):
- `src/routes/+layout.svelte` — import i18n init
- `src/lib/components/layout/Navbar.svelte` — language selector + translated nav links
- `src/lib/components/layout/AdminSidebar.svelte` — translated sidebar labels
- `src/routes/(admin)/+page.svelte` — Dashboard
- `src/routes/(admin)/nodes/+page.svelte`
- `src/routes/(admin)/storage/+page.svelte`
- `src/routes/(admin)/vmbr/+page.svelte`
- `src/routes/(admin)/iso/+page.svelte`
- `src/routes/(admin)/vms/+page.svelte`
- `src/routes/(admin)/tags/+page.svelte`
- `src/routes/(admin)/limits/+page.svelte`
- `src/routes/(admin)/cloudinit/+page.svelte`
- `src/routes/(admin)/appinfo/+page.svelte`
- `src/routes/(admin)/userpool/+page.svelte`

## Tests & Success Criteria

### Tests:
- **Key parity test:** Verify `en.json` and `fr.json` have exactly the same keys
- **No hardcoded strings:** Post-migration grep for common English words in `.svelte` files
- **Functional:** Language toggle switches all visible text instantly

### Success criteria:
1. All admin pages display translated text in FR and EN
2. FR/EN selector in Navbar works (instant switch, no reload)
3. Preference persists via localStorage + cookie across sessions
4. New users see language detected from browser `navigator.language`
5. Cookie `pvmss_lang` is compatible with old Vue frontend
6. `svelte-check` passes with 0 errors
7. Both JSON files have identical key sets

## Out of Scope

- Advanced pluralization (not needed for current keys)
- RTL or non-Latin languages
- User-facing page translations (namespace reserved, keys empty)
- API error message translation (remain in English from backend)
