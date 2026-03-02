# Vue Migration: Current State

> Last updated: March 2026 (phase 2 complete)

## What's Done (phase 2 branch: `feature/vue-spa-phase2-alpine-removal`)

### Backend — JWT API (`/api/v1/`)

| Endpoint | Status |
|---|---|
| `POST /api/v1/auth/login` | ✅ |
| `POST /api/v1/auth/logout` | ✅ |
| `GET /api/v1/auth/me` | ✅ |
| `POST /api/v1/auth/exchange` | ✅ session→JWT |
| `GET /api/v1/vms` | ✅ |
| `POST /api/v1/vms/:id/action` | ✅ start/stop/shutdown/reboot |
| `GET /api/v1/vms/:id/metrics` | ✅ cpu%, mem, uptime |
| `GET /api/v1/search/vms` | ✅ |
| `GET /api/v1/profile/vms` | ✅ tag-based ownership |

### Backend — Go handlers using `renderVueShell`

All user-facing routes now render the Vue SPA shell instead of Alpine templ pages:

| Route | Status |
|---|---|
| `GET /` | ✅ Vue shell |
| `GET /login` | ✅ Vue shell |
| `GET /search` | ✅ Vue shell (POST removed) |
| `GET /profile` | ✅ Vue shell |
| `GET /vm/details/:vmid` | ✅ Vue shell |

`VueShellPage` templ component places `#vue-app` inside `<main>` so Vue
content renders in the correct position in the layout.

### Frontend — Vue 3 SPA (`frontend/src/`)

```bash
frontend/src/
├── main.js              # Vue app bootstrap (router + pinia + AppToast)
├── App.js               # re-exports VmListPage
├── router.js            # vue-router: /, /search, /profile, /vm/details/:vmid, /login
├── api/
│   ├── client.js        # Axios instance with credentials
│   ├── auth.js          # login, logout, me, exchange
│   └── vms.js           # listVMs, getVM, vmAction
├── composables/
│   └── useAutoRefresh.js  # polling with visibilitychange pause
├── stores/
│   ├── auth.js          # Pinia auth store
│   ├── vms.js           # Pinia VM store (home page)
│   ├── notifications.js # toast notifications
│   ├── search.js        # search store
│   ├── profile.js       # profile VMs store
│   └── vmDetails.js     # VM details + metrics store
├── components/
│   ├── VmCard.js        # VM card
│   ├── VmActionButtons.js
│   ├── AppButton.js
│   ├── AppToast.js      # toast notification container
│   ├── AppModal.js      # v-model modal
│   ├── AppTabs.js       # tabbed interface
│   └── AppDropdown.js   # click-outside dropdown
└── pages/
    ├── VmListPage.js    # home — VM grid with actions
    ├── SearchPage.js    # search with debounce
    ├── ProfilePage.js   # user's VMs with auto-refresh
    ├── VmDetailsPage.js # VM info + metrics auto-refresh
    └── LoginPage.js     # local + pve tabs
```

### Vendored ESM libraries

| Library | Version |
|---|---|
| `vue` | 3.x |
| `pinia` | 2.x |
| `axios` | latest |
| `vue-router` | 4.x |

---

## What's NOT Done (follow-up work)

| Area | Status | Notes |
|---|---|---|
| Alpine.js removal from admin | ❌ | Admin pages still use Alpine; out of scope for this branch |
| VM Create page | ❌ | Still Go templ + Alpine |
| Full VM Details in Vue | ⚠️ | Basic info + metrics done; snapshots, cloud-init, disk editor still Alpine |
| `.vue` SFC / Vite build | ❌ | Using render functions (`h()`), no build step |
| i18n | ❌ | Vue doesn't use Go i18n; no translations in Vue yet |

### Alpine.js still used in

These files still contain Alpine directives and cannot have Alpine removed yet:

- `backend/components/navbar.templ` — dropdown menu
- `backend/components/vm_action_buttons.templ` — loading states
- `backend/components/notification.templ` — dismissible notifications
- `backend/components/vm_create_cards.templ` — OS/template selection
- `backend/components/vm_create_modal.templ` — create modal
- `backend/components/admin_*.templ` — all admin pages

Alpine is loaded in `layout.templ` and is still required for admin and VM create flows.
