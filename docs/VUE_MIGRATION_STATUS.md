# Vue Migration: Current State

> Last updated: March 2026

## What's Already Done

### 1. Project Structure

```bash
frontend/src/
├── main.js              # Vue app bootstrap
├── App.js               # Root component (render functions)
├── api/
│   ├── client.js        # Axios instance with credentials
│   ├── auth.js          # login, logout, me endpoints
│   └── vms.js           # listVMs, getVM, vmAction
├── stores/
│   ├── auth.js          # Pinia auth store
│   └── vms.js           # Pinia VM store
└── components/
    ├── VmCard.js        # VM card component
    ├── VmActionButtons.js # Start/Stop/Shutdown/Reboot buttons
    └── AppButton.js     # Reusable button component
```

### 2. Integration Points

**Go templ layout** (`backend/components/layout.templ`):

- Vue 3 mounted via `<div id="vue-app">`
- Uses `x-ignore` to prevent Alpine.js interference
- Auth data passed via data attributes: `data-username`, `data-is-admin`
- ES modules loaded via importmap (no build step required)

```html
<script type="importmap">
{
  "imports": {
    "vue":   "/vendor/vue.esm-browser.prod.js",
    "pinia": "/vendor/pinia.esm-browser.prod.js",
    "axios": "/vendor/axios.min.mjs"
  }
}
</script>
```

**JWT Exchange**:

- Session cookie exchanged for JWT on page load
- `/api/v1/auth/exchange` called automatically

### 3. Vue Implementation Details

**Pinia Stores**:

```javascript
// stores/auth.js - bootstraps from HTML data attributes
const username = ref(mountEl.dataset.username || '');
const isAdmin = ref(mountEl.dataset.isAdmin === 'true');
// then syncs with /api/v1/auth/me

// stores/vms.js - fetches VM list and handles actions
const vms = ref([]);
async function fetchVMs() { /* calls /api/v1/vms */ }
async function doAction(vmid, action, node) { /* POST /api/v1/vms/:id/action */ }
```

**Components using render functions** (`h()`):

```javascript
// VmCard.js - maps over VM properties
h('div', { class: 'vm-card' }, [
  h('div', { class: 'vm-card__header' }, [
    h('span', { class: 'vm-card__name' }, name || `VM ${vmid}`),
    ...
  ])
])
```

### 4. Static Serving

Backend serves Vue source directly:

- `backend/handlers/handlers.go:238` - serves `/src/*` from `frontend/src/`
- No build step needed - uses ESM importmap approach

---

## What's NOT Done (Todo)

| Area | Status | Notes |
| ------ | ------ | ----- |
| `.vue` SFC | ❌ | Using render functions (`h()`), not `.vue` templates |
| Vite build | ❌ | No build step - limits component syntax |
| i18n | ❌ | Vue doesn't use Go i18n; no translations in Vue |
| VM Create | ❌ | Still Go templ + Alpine |
| VM Details | ❌ | Still Go templ + Alpine |
| Admin pages | ❌ | Still Go templ + Alpine |
| CSRF in Vue | ⚠️ | Meta tag exists but not wired in axios |

---

## Immediate Next Steps

1. **Add Vite build** - enables `.vue` single-file components
2. **Convert VmCard.js → VmCard.vue** - test the build system
3. **Wire CSRF** - add interceptor to axios client
4. **Add vue-i18n** - for translations

See `docs/VUE_MIGRATION.md` for detailed migration plan.
