# Vue Migration Guide: Go templ to Vue

## Current State

### Architecture Overview

- **Backend**: Go with [templ](https://templ.gd/) library for server-side rendering
- **Frontend**: Hybrid approach with:
  - Vue 3 + Pinia (partially implemented in `frontend/src/`)
  - Alpine.js (legacy interactivity)
  - Bulma CSS framework
- **API**: REST API at `/api/v1/*` with JWT authentication

### Existing Vue Setup

```text
frontend/
├── src/
│   ├── main.js          # Vue app entry (mounts on #vue-app)
│   ├── App.js           # Root component (render function style)
│   ├── api/
│   │   ├── client.js    # Axios instance
│   │   ├── auth.js      # Auth endpoints
│   │   └── vms.js       # VM endpoints
│   ├── stores/
│   │   ├── auth.js      # Pinia auth store
│   │   └── vms.js       # Pinia VM store
│   └── components/
│       ├── VmCard.js    # VM card component
│       └── VmActionButtons.js
└── vendor/              # Vendored Vue, Pinia, Axios (no npm needed)
```

### How Vue Currently Works

1. Mounts on `<div id="vue-app">` element
2. Auth state bootstrapped from data attributes + `/api/v1/auth/me`
3. Uses render functions (`h()`) instead of `.vue` templates

### Go templ Structure

```text
backend/components/
├── home.templ           # Home page component
├── vm_card.templ        # VM card component  
├── layout.templ         # Base layout
├── vm_create.templ      # VM creation form
├── admin_*.templ       # Admin pages
└── *_templ.go          # Generated Go code
```

---

## Migration Strategy

### Phase 1: Foundation (Start Here)

1.1 Switch from Render Functions to `.vue` Single File Components

Current (`VmCard.js`):

```javascript
import { defineComponent, h } from 'vue';
export const VmCard = defineComponent({
  setup(props) {
    return () => h('div', { class: 'vm-card' }, ...);
  }
});
```

Migrate to:

```vue
<!-- VmCard.vue -->
<script setup>
defineProps({ vm: Object });
defineEmits(['action']);
</script>

<template>
  <div class="vm-card">
    ...
  </div>
</template>
```

1.2 Add Vue Build Step

The project currently uses vendored ESM modules. Consider adding a build step:

- Option A: Keep no-build approach (works but limited)
- Option B: Add Vite for `.vue` SFC support + hot reload

Add to `package.json`:

```json
{
  "scripts": {
    "dev": "vite --port 3001",
    "build": "vite build"
  },
  "devDependencies": {
    "vite": "^5.x",
    "@vitejs/plugin-vue": "^5.x"
  }
}
```

Create `vite.config.js`:

```javascript
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  root: 'frontend',
  build: { outDir: '../dist' }
});
```

### Phase 2: Page-by-Page Migration

**Recommended Order:**

1. **Home (/)** - Simple, low risk
2. **VM List** - Already partially Vue (`VmCard.js`)
3. **VM Details** - Medium complexity
4. **Create VM** - Complex form, save for later
5. **Admin Pages** - Progressive migration

#### 2.1 Home Page Migration

Current `backend/components/home.templ`:

- Server-renders welcome cards
- Uses `HomeData` struct with Username, CSRF, IsAdmin, etc.

Vue approach:

- Keep layout in Go (navbar, footer)
- Replace content section with Vue mount point
- Fetch data from API instead of server-side

```vue
<!-- frontend/src/pages/Home.vue -->
<script setup>
import { useVMStore } from '../stores/vms.js';
const vmStore = useVMStore();
</script>

<template>
  <section class="section">
    <!-- Welcome cards -->
  </section>
</template>
```

#### 2.2 Replace Go Data with API Calls

Current (Go templ):

```go
type HomeData struct {
  Username  string
  IsAdmin   bool
  CSRFToken string
}
templ HomePage(data HomeData, T TranslationFunc) {
  <div>{ data.Username }</div>
}
```

Migrate to Vue:

```javascript
// frontend/src/api/me.js
export function getProfile() {
  return api.get('/auth/me');
}

// frontend/src/stores/auth.js
async function init() {
  const { data } = await me();
  username.value = data.username;
}
```

### Phase 3: Component Mapping

| Go templ Component | Vue Component | Status |
| ------------------ | ------------- | ------ |
| `home.templ` | `Home.vue` | Todo |
| `vm_card.templ` | `VmCard.js` → `VmCard.vue` | Todo |
| `vm_action_buttons.templ` | `VmActionButtons.js` → `Vue` | Todo |
| `vm_create.templ` | `VmCreate.vue` | Todo |
| `navbar.templ` | Keep as Go | - |
| `layout.templ` | Keep as Go | - |

---

## Key Migration Tips

### 1. CSRF Handling

Go templ embeds CSRF in forms. Vue needs to:

- Get CSRF from cookie or API response
- Include in request headers

```javascript
// In api/client.js
api.interceptors.request.use(config => {
  const csrf = document.querySelector('[name="csrf_token"]')?.value;
  if (csrf) config.headers['X-CSRF-Token'] = csrf;
  return config;
});
```

### 2. Auth State

Current: Server-side session + JWT
Vue: Client-side state in Pinia store

```javascript
// Bootstrap from HTML data attributes (server-rendered)
const mountEl = document.getElementById('vue-app');
const username = mountEl?.dataset.username;

// Then sync with API
const { data } = await api.get('/auth/me');
```

### 3. i18n (Internationalization)

Current: Go `T()` function in templates
Vue: Use `vue-i18n` or simple JSON files

```javascript
// frontend/src/i18n/index.js
import { createI18n } from 'vue-i18n';
import en from './en.json';
import fr from './fr.json';

export const i18n = createI18n({
  locale: document.documentElement.lang || 'en',
  messages: { en, fr }
});
```

### 4. Keeping Hybrid Approach

You don't need to migrate everything at once:

```html
<!-- Hybrid: Server layout + Vue content -->
<html>
  <body>
    <!-- Server-rendered navbar -->
    @Navbar()
    
    <!-- Vue mount point -->
    <div id="vue-app" data-username="{{ .Username }}">
      <VMList />
    </div>
    
    <!-- Server-rendered footer -->
    @Footer()
  </body>
</html>
```

### 5. API Compatibility

The `/api/v1/*` endpoints are already Vue-friendly (JSON, JWT auth). No backend changes needed for data fetching.

---

## Quick Start Checklist

- [ ] Add Vite build system
- [ ] Convert `VmCard.js` to `VmCard.vue`
- [ ] Convert `App.js` to use `<template>` instead of render functions
- [ ] Create `Home.vue` page
- [ ] Add vue-i18n for translations
- [ ] Test CSRF token handling
- [ ] Migrate VM List page first
- [ ] Then VM Details
- [ ] Then Create VM form

---

## File Locations

| Purpose | Path |
| ------- | ---- |
| Vue entry | `frontend/src/main.js` |
| Vue app | `frontend/src/App.js` |
| API client | `frontend/src/api/client.js` |
| Auth store | `frontend/src/stores/auth.js` |
| VM store | `frontend/src/stores/vms.js` |
| Go templates | `backend/components/*.templ` |
| Generated Go | `backend/components/*_templ.go` |
| API routes | `backend/api/v1/` |
| i18n (Go) | `backend/i18n/*.toml` |
