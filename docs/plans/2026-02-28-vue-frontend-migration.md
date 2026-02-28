# Vue 3 Frontend Migration: Replace Alpine.js

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace every Alpine.js interactive component with a Vue 3 equivalent, add the new `/api/v1/` endpoints those components need, set up Vue Router for client-side navigation, and remove Alpine.js entirely once all pages are migrated.

**Architecture:** The Go backend continues to serve page shells (nav + footer + empty `<main>`) for all user-facing routes. Vue Router reads `window.location` and renders the correct page component inside `#vue-app`. Admin routes remain fully server-rendered (no Alpine either — they're mostly static). Alpine.js is removed from `layout.templ` once every interactive component has a Vue equivalent. All data flows through `/api/v1/` JWT endpoints. Existing Bulma CSS classes, Font Awesome icons, and Go `i18n` string keys are reused — the visual design does not change.

**Tech Stack:** Vue 3 (vendored ESM, already in `/vendor/`), Pinia (vendored), Axios (vendored), vue-router 4 (to vendor), plain JavaScript ES modules in `frontend/src/` (no build step, airgap-compatible). Go backend in `backend/api/v1/`.

---

## What exists today (reuse these)

- `frontend/src/components/AppButton.js` — button with variant/loading
- `frontend/src/components/VmCard.js` — VM card with status badge
- `frontend/src/components/VmActionButtons.js` — start/stop/shutdown/reboot
- `frontend/src/stores/auth.js` — username, isAdmin, init()
- `frontend/src/stores/vms.js` — vms[], fetchVMs(), doAction()
- `frontend/src/api/client.js` — Axios instance for `/api/v1`
- `frontend/src/api/auth.js` — login, logout, me
- `frontend/src/api/vms.js` — listVMs, getVM, vmAction
- `backend/api/v1/` — auth, vm list, vm action endpoints

## Alpine.js components being replaced

| Alpine component | File | Replacement |
|---|---|---|
| `Alpine.store('notifications')` | `alpine-init.js` | Pinia `useNotificationsStore` + `<AppToast>` |
| `Alpine.store('loading')` | `alpine-init.js` | per-feature store state |
| `dropdown()` | `alpine-init.js` | `<AppDropdown>` component |
| `dismissible()` | `alpine-init.js` | `<AppNotification>` component |
| `tabs(default)` | `alpine-init.js` | `<AppTabs>` component |
| `modal()` | `alpine-init.js` | `<AppModal>` component |
| `memoryConverter()` | `alpine-init.js` | `useMemoryConverter` composable |
| `loadingButton()` | `alpine-init.js` | `AppButton` (already done) |
| `vmSearch()` | `alpine-init.js` | Pinia `useSearchStore` + `<SearchPage>` |
| `autoRefresh(url, interval)` | `alpine-init.js` | `useAutoRefresh` composable |
| `networkToggle(...)` | `alpine-init.js` | `<NetworkToggle>` component |
| `adminLoginTabs()` | `alpine-init.js` | `<LoginPage>` tabs |
| `vmDetails` | `vm-details-alpine.js` | Pinia `useVmDetailsStore` + `<VmDetailsPage>` |
| profile page | `profile-alpine.js` | Pinia `useProfileStore` + `<ProfilePage>` |

---

## Task 1: Vendor vue-router and add to importmap

**Files:**

- Create: `frontend/vendor/vue-router.esm-browser.js`
- Modify: `backend/components/layout.templ`
- Modify: `backend/components/layout_templ.go` (auto-generated, run `make go-template`)

**Step 1: Download vue-router ESM build**

```bash
curl -fsSL "https://cdn.jsdelivr.net/npm/vue-router@4/dist/vue-router.esm-browser.js" \
  -o frontend/vendor/vue-router.esm-browser.js
echo "Downloaded: $(wc -c < frontend/vendor/vue-router.esm-browser.js) bytes"
```

Expected: ~90 kB file.

**Step 2: Add to importmap in `layout.templ`**

In `backend/components/layout.templ`, update the importmap:

```html
<script type="importmap">
    {
        "imports": {
            "vue":        "/vendor/vue.esm-browser.prod.js",
            "pinia":      "/vendor/pinia.esm-browser.prod.js",
            "axios":      "/vendor/axios.min.mjs",
            "vue-router": "/vendor/vue-router.esm-browser.js"
        }
    }
</script>
```

**Step 3: Regenerate templ**

```bash
make go-template
```

Expected: `layout_templ.go` regenerated, no errors.

**Step 4: Build**

```bash
cd backend && go build ./...
```

Expected: no errors.

**Step 5: Commit**

```bash
git add frontend/vendor/vue-router.esm-browser.js backend/components/layout.templ backend/components/layout_templ.go
git commit -m "chore(frontend): vendor vue-router 4 ESM and add to importmap"
```

---

## Task 2: Add `GET /api/v1/search/vms` endpoint

**Files:**

- Create: `backend/api/v1/search.go`
- Create: `backend/api/v1/search_test.go`
- Modify: `backend/api/v1/router.go`

The current `/api/search/vms` route is session-protected, not JWT-protected. We need a JWT equivalent.

**Step 1: Write the failing test**

```go
// backend/api/v1/search_test.go
package apiv1

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestSearchVMs_OfflineMode(t *testing.T) {
    sm := newTestSM("testsecretthatis32byteslongexact!!")
    sm.offline = true
    h := NewSearchHandler(sm)

    req := httptest.NewRequest(http.MethodGet, "/api/v1/search/vms", nil)
    signed := signToken(t, "testsecretthatis32byteslongexact!!", "testuser", false, accessTokenTTL)
    req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})
    rr := httptest.NewRecorder()

    JWTMiddleware(sm, http.HandlerFunc(h.SearchVMs)).ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
    }
}

func TestSearchVMs_FiltersVMID(t *testing.T) {
    sm := newTestSM("testsecretthatis32byteslongexact!!")
    sm.offline = true
    h := NewSearchHandler(sm)

    req := httptest.NewRequest(http.MethodGet, "/api/v1/search/vms?vmid=100", nil)
    signed := signToken(t, "testsecretthatis32byteslongexact!!", "testuser", false, accessTokenTTL)
    req.AddCookie(&http.Cookie{Name: accessTokenCookie, Value: signed})
    rr := httptest.NewRecorder()

    JWTMiddleware(sm, http.HandlerFunc(h.SearchVMs)).ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
    }
}
```

**Step 2: Run to confirm it fails (compile error)**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test ./api/v1/... -run TestSearchVMs 2>&1 | head -5
```

Expected: `undefined: NewSearchHandler`

**Step 3: Implement `search.go`**

```go
// backend/api/v1/search.go
package apiv1

import (
    "net/http"
    "strconv"
    "strings"

    "pvmss/proxmox"
    "pvmss/state"
)

// SearchResult is a VM summary enriched with description for search display.
type SearchResult struct {
    VMSummary
    Description string `json:"description"`
}

// SearchResponse wraps search results.
type SearchResponse struct {
    Results []SearchResult `json:"results"`
    Total   int            `json:"total"`
}

// SearchHandler handles VM search requests.
type SearchHandler struct {
    state state.StateManager
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(s state.StateManager) *SearchHandler {
    return &SearchHandler{state: s}
}

// SearchVMs handles GET /api/v1/search/vms?vmid=&name=&tags=&limit=
func (h *SearchHandler) SearchVMs(w http.ResponseWriter, r *http.Request) {
    if h.state != nil && h.state.IsOfflineMode() {
        writeJSON(w, SearchResponse{Results: []SearchResult{}, Total: 0})
        return
    }

    client := h.state.GetRestyClient()
    if client == nil {
        errNotConfigured(w, "proxmox client")
        return
    }

    q := r.URL.Query()
    filterVMID := strings.TrimSpace(q.Get("vmid"))
    filterName := strings.TrimSpace(strings.ToLower(q.Get("name")))
    filterTags := strings.TrimSpace(strings.ToLower(q.Get("tags")))
    limit := 25
    if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 500 {
        limit = l
    }

    vms, err := proxmox.GetVMsResty(r.Context(), client)
    if err != nil {
        errInternal(w, "failed to fetch VMs")
        return
    }

    results := make([]SearchResult, 0, len(vms))
    for _, vm := range vms {
        if filterVMID != "" && !strings.Contains(strconv.Itoa(vm.VMID), filterVMID) {
            continue
        }
        if filterName != "" && !strings.Contains(strings.ToLower(vm.Name), filterName) {
            continue
        }
        if filterTags != "" && !strings.Contains(strings.ToLower(vm.Tags), filterTags) {
            continue
        }
        results = append(results, SearchResult{
            VMSummary:   vmToSummary(vm),
            Description: vm.Description,
        })
        if len(results) >= limit {
            break
        }
    }

    writeJSON(w, SearchResponse{Results: results, Total: len(results)})
}
```

**Step 4: Register route in `router.go`**

```go
searchHandler := NewSearchHandler(s)
router.GET("/api/v1/search/vms", jwtWrap(s, searchHandler.SearchVMs))
```

**Step 5: Run tests**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run TestSearchVMs ./api/v1/...
```

Expected: PASS

**Step 6: Commit**

```bash
git add backend/api/v1/search.go backend/api/v1/search_test.go backend/api/v1/router.go
git commit -m "feat(api/v1): add GET /api/v1/search/vms endpoint"
```

---

## Task 3: Add `GET /api/v1/profile/vms` and `GET /api/v1/vms/:id/metrics`

**Files:**

- Create: `backend/api/v1/profile.go`
- Create: `backend/api/v1/profile_test.go`
- Modify: `backend/api/v1/vms.go` (add metrics endpoint)
- Modify: `backend/api/v1/router.go`

**Step 1: Implement `profile.go`**

```go
// backend/api/v1/profile.go
package apiv1

import (
    "net/http"

    "pvmss/proxmox"
    "pvmss/state"
)

// ProfileHandler serves per-user VM data.
type ProfileHandler struct {
    state state.StateManager
}

func NewProfileHandler(s state.StateManager) *ProfileHandler {
    return &ProfileHandler{state: s}
}

// ListMyVMs handles GET /api/v1/profile/vms — returns VMs owned by the requesting user.
// Ownership is determined by tags matching the username (same logic as existing profile handler).
func (h *ProfileHandler) ListMyVMs(w http.ResponseWriter, r *http.Request) {
    if h.state != nil && h.state.IsOfflineMode() {
        writeJSON(w, VMListResponse{VMs: []VMSummary{}})
        return
    }

    client := h.state.GetRestyClient()
    if client == nil {
        errNotConfigured(w, "proxmox client")
        return
    }

    username := usernameFromCtx(r)
    vms, err := proxmox.GetVMsResty(r.Context(), client)
    if err != nil {
        errInternal(w, "failed to fetch VMs")
        return
    }

    owned := make([]VMSummary, 0)
    for _, vm := range vms {
        if proxmox.VMBelongsToUser(vm, username) {
            owned = append(owned, vmToSummary(vm))
        }
    }

    writeJSON(w, VMListResponse{VMs: owned})
}
```

Note: `proxmox.VMBelongsToUser` may not exist yet. Check `backend/proxmox/` or `backend/handlers/` for the existing user-VM ownership logic and extract it. If no helper exists, inline the tag-check logic from `backend/handlers/profile.go`.

**Step 2: Add metrics endpoint to `vms.go`**

```go
// MetricsResponse wraps VM resource usage metrics.
type MetricsResponse struct {
    VMID    int     `json:"vmid"`
    CPUPct  float64 `json:"cpu_pct"`   // 0.0–100.0
    MemMB   int64   `json:"mem_mb"`
    MaxMemMB int64  `json:"max_mem_mb"`
    DiskReadBps  int64 `json:"disk_read_bps"`
    DiskWriteBps int64 `json:"disk_write_bps"`
    NetInBps  int64 `json:"net_in_bps"`
    NetOutBps int64 `json:"net_out_bps"`
}

// GetVMMetrics handles GET /api/v1/vms/:id/metrics
func (h *VMHandler) GetVMMetrics(w http.ResponseWriter, r *http.Request) {
    ps := httprouter.ParamsFromContext(r.Context())
    vmid, err := strconv.Atoi(ps.ByName("id"))
    if err != nil || vmid <= 0 {
        errBadRequest(w, "invalid vm id")
        return
    }

    if h.state.IsOfflineMode() {
        errOffline(w)
        return
    }

    client := h.state.GetRestyClient()
    if client == nil {
        errNotConfigured(w, "proxmox client")
        return
    }

    // Use existing Resty client to fetch current VM status (contains metrics)
    vms, err := proxmox.GetVMsResty(r.Context(), client)
    if err != nil {
        errInternal(w, "failed to fetch VM metrics")
        return
    }

    for _, vm := range vms {
        if vm.VMID == vmid {
            writeJSON(w, MetricsResponse{
                VMID:         vmid,
                CPUPct:       vm.CPU * 100,
                MemMB:        vm.Mem / 1024 / 1024,
                MaxMemMB:     vm.MaxMem / 1024 / 1024,
                DiskReadBps:  vm.DiskRead,
                DiskWriteBps: vm.DiskWrite,
                NetInBps:     vm.NetIn,
                NetOutBps:    vm.NetOut,
            })
            return
        }
    }
    writeError(w, http.StatusNotFound, "not_found", "VM not found")
}
```

**Step 3: Register routes**

```go
profileHandler := NewProfileHandler(s)
router.GET("/api/v1/profile/vms", jwtWrap(s, profileHandler.ListMyVMs))
router.GET("/api/v1/vms/:id/metrics", jwtWrap(s, vmHandler.GetVMMetrics))
```

**Step 4: Write tests, run, commit**

```bash
# Write profile_test.go with TestListMyVMs_OfflineMode (expects 200 + empty list)
# Run:
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run "TestListMyVMs|TestGetVMMetrics" ./api/v1/...
```

```bash
git add backend/api/v1/profile.go backend/api/v1/profile_test.go backend/api/v1/vms.go backend/api/v1/router.go
git commit -m "feat(api/v1): add profile/vms and vms/:id/metrics endpoints"
```

---

## Task 4: Global notification store + `<AppToast>` component

**Files:**

- Create: `frontend/src/stores/notifications.js`
- Create: `frontend/src/components/AppToast.js`
- Modify: `frontend/src/App.js`

This replaces `Alpine.store('notifications')` in `alpine-init.js`.

**Step 1: Create Pinia notifications store**

```js
// frontend/src/stores/notifications.js
import { defineStore } from 'pinia';
import { ref } from 'vue';

export const useNotificationsStore = defineStore('notifications', () => {
  const items = ref([]);
  let counter = 0;

  function add({ type = 'info', message = '', title = '', duration = 5000, dismissible = true }) {
    const id = ++counter;
    items.value.push({ id, type, message, title, duration, dismissible });
    if (duration > 0) {
      setTimeout(() => remove(id), duration);
    }
    return id;
  }

  function remove(id) {
    const i = items.value.findIndex(n => n.id === id);
    if (i > -1) items.value.splice(i, 1);
  }

  function clear() { items.value = []; }

  return { items, add, remove, clear };
});
```

**Step 2: Create `<AppToast>` component**

```js
// frontend/src/components/AppToast.js
import { defineComponent, h, TransitionGroup } from 'vue';
import { useNotificationsStore } from '../stores/notifications.js';

const TYPE_CLASS = {
  success: 'is-success',
  danger:  'is-danger',
  warning: 'is-warning',
  info:    'is-info',
};
const TYPE_ICON = {
  success: 'fa-check-circle',
  danger:  'fa-exclamation-circle',
  warning: 'fa-exclamation-triangle',
  info:    'fa-info-circle',
};

export const AppToast = defineComponent({
  name: 'AppToast',
  setup() {
    const store = useNotificationsStore();
    return () => h('div', { class: 'app-toast-container', style: 'position:fixed;top:1rem;right:1rem;z-index:9999;width:320px;' },
      store.items.map(n =>
        h('div', {
          key: n.id,
          class: ['notification', TYPE_CLASS[n.type] || 'is-info'],
          style: 'margin-bottom:0.5rem;',
        }, [
          n.dismissible && h('button', { class: 'delete', onClick: () => store.remove(n.id) }),
          h('span', { class: 'icon' }, [h('i', { class: ['fas', TYPE_ICON[n.type] || 'fa-info-circle'] })]),
          n.title && h('strong', null, ` ${n.title} `),
          h('span', null, n.message),
        ])
      )
    );
  },
});
```

**Step 3: Mount `<AppToast>` in `App.js`**

```js
// In App.js, add AppToast alongside the vm-list output:
import { AppToast } from './components/AppToast.js';

// In the render function, wrap with a fragment:
return () => h('div', null, [
  h(AppToast),
  // ... existing vm list render ...
]);
```

**Step 4: Run app manually to verify toast renders**

No automated test for pure rendering — visual check: open a page, run `document.querySelector('#vue-app')` in devtools, confirm `AppToast` is present.

**Step 5: Commit**

```bash
git add frontend/src/stores/notifications.js frontend/src/components/AppToast.js frontend/src/App.js
git commit -m "feat(frontend): add Pinia notifications store and AppToast component"
```

---

## Task 5: Shared UI primitives — `<AppModal>`, `<AppTabs>`, `<AppDropdown>`

**Files:**

- Create: `frontend/src/components/AppModal.js`
- Create: `frontend/src/components/AppTabs.js`
- Create: `frontend/src/components/AppDropdown.js`

These replace Alpine's `modal()`, `tabs()`, and `dropdown()` — used by VM Details, Login, and Navbar.

**Step 1: `AppModal.js`**

```js
// frontend/src/components/AppModal.js
import { defineComponent, h, ref } from 'vue';

export const AppModal = defineComponent({
  name: 'AppModal',
  props: {
    title: { type: String, default: '' },
    modelValue: { type: Boolean, default: false },
  },
  emits: ['update:modelValue'],
  setup(props, { slots, emit }) {
    function close() {
      emit('update:modelValue', false);
      document.body.style.overflow = '';
    }
    return () => props.modelValue
      ? h('div', { class: 'modal is-active' }, [
          h('div', { class: 'modal-background', onClick: close }),
          h('div', { class: 'modal-card' }, [
            h('header', { class: 'modal-card-head' }, [
              h('p', { class: 'modal-card-title' }, props.title),
              h('button', { class: 'delete', onClick: close }),
            ]),
            h('section', { class: 'modal-card-body' }, slots.default?.()),
            slots.footer && h('footer', { class: 'modal-card-foot' }, slots.footer()),
          ]),
        ])
      : null;
  },
});
```

**Step 2: `AppTabs.js`**

```js
// frontend/src/components/AppTabs.js
import { defineComponent, h, ref } from 'vue';

export const AppTabs = defineComponent({
  name: 'AppTabs',
  props: {
    tabs: { type: Array, required: true }, // [{ key, label }]
    modelValue: { type: String, required: true },
  },
  emits: ['update:modelValue'],
  setup(props, { slots, emit }) {
    return () => h('div', null, [
      h('div', { class: 'tabs' },
        h('ul', null, props.tabs.map(tab =>
          h('li', { class: props.modelValue === tab.key ? 'is-active' : '' },
            h('a', { onClick: () => emit('update:modelValue', tab.key) }, tab.label)
          )
        ))
      ),
      slots[props.modelValue]?.(),
    ]);
  },
});
```

**Step 3: `AppDropdown.js`**

```js
// frontend/src/components/AppDropdown.js
import { defineComponent, h, ref, onMounted, onUnmounted } from 'vue';

export const AppDropdown = defineComponent({
  name: 'AppDropdown',
  setup(_, { slots }) {
    const open = ref(false);
    function close(e) {
      if (!e.target.closest('.dropdown')) open.value = false;
    }
    onMounted(() => document.addEventListener('click', close));
    onUnmounted(() => document.removeEventListener('click', close));
    return () => h('div', { class: ['dropdown', open.value ? 'is-active' : ''] }, [
      h('div', { class: 'dropdown-trigger' },
        h('div', { onClick: () => open.value = !open.value }, slots.trigger?.())
      ),
      h('div', { class: 'dropdown-menu' },
        h('div', { class: 'dropdown-content' }, slots.default?.())
      ),
    ]);
  },
});
```

**Step 4: Commit**

```bash
git add frontend/src/components/AppModal.js frontend/src/components/AppTabs.js frontend/src/components/AppDropdown.js
git commit -m "feat(frontend): add AppModal, AppTabs, AppDropdown components (replace Alpine equivalents)"
```

---

## Task 6: Set up Vue Router and SPA routing

**Files:**

- Modify: `frontend/src/main.js`
- Create: `frontend/src/router.js`
- Create: `frontend/src/pages/VmListPage.js`
- Create: `frontend/src/pages/SearchPage.js` (stub — filled in Task 7)
- Create: `frontend/src/pages/ProfilePage.js` (stub — filled in Task 8)
- Create: `frontend/src/pages/VmDetailsPage.js` (stub — filled in Task 9)
- Create: `frontend/src/pages/LoginPage.js` (stub — filled in Task 10)

**Step 1: Create `frontend/src/pages/` directory and `VmListPage.js`**

`VmListPage.js` already exists as `App.js` logic — extract it:

```js
// frontend/src/pages/VmListPage.js
import { defineComponent, h, onMounted } from 'vue';
import { useVMStore } from '../stores/vms.js';
import { VmCard } from '../components/VmCard.js';

export const VmListPage = defineComponent({
  name: 'VmListPage',
  setup() {
    const vmStore = useVMStore();
    onMounted(() => vmStore.fetchVMs());
    return () => {
      if (vmStore.loading) return h('div', { class: 'has-text-centered p-6' }, 'Loading VMs…');
      if (vmStore.error) return h('div', { class: 'notification is-danger' }, vmStore.error);
      if (!vmStore.vms.length) return h('div', { class: 'notification is-warning' }, 'No VMs found.');
      return h('div', { class: 'vm-list columns is-multiline' },
        vmStore.vms.map(vm => h('div', { key: vm.vmid, class: 'column is-12-mobile is-6-tablet is-4-desktop' },
          h(VmCard, { vm, onAction: ({ vmid, action, node }) => vmStore.doAction(vmid, action, node) })
        ))
      );
    };
  },
});
```

**Step 2: Create stub pages**

```js
// frontend/src/pages/SearchPage.js — stub
import { defineComponent, h } from 'vue';
export const SearchPage = defineComponent({ name: 'SearchPage', setup: () => () => h('div', null, 'Search (coming soon)') });

// frontend/src/pages/ProfilePage.js — stub
import { defineComponent, h } from 'vue';
export const ProfilePage = defineComponent({ name: 'ProfilePage', setup: () => () => h('div', null, 'Profile (coming soon)') });

// frontend/src/pages/VmDetailsPage.js — stub
import { defineComponent, h } from 'vue';
export const VmDetailsPage = defineComponent({ name: 'VmDetailsPage', setup: () => () => h('div', null, 'VM Details (coming soon)') });

// frontend/src/pages/LoginPage.js — stub
import { defineComponent, h } from 'vue';
export const LoginPage = defineComponent({ name: 'LoginPage', setup: () => () => h('div', null, 'Login (coming soon)') });
```

**Step 3: Create `frontend/src/router.js`**

```js
// frontend/src/router.js
import { createRouter, createWebHistory } from 'vue-router';
import { VmListPage }    from './pages/VmListPage.js';
import { SearchPage }    from './pages/SearchPage.js';
import { ProfilePage }   from './pages/ProfilePage.js';
import { VmDetailsPage } from './pages/VmDetailsPage.js';
import { LoginPage }     from './pages/LoginPage.js';

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/',                  component: VmListPage },
    { path: '/search',            component: SearchPage },
    { path: '/profile',           component: ProfilePage },
    { path: '/vm/details/:vmid',  component: VmDetailsPage },
    { path: '/login',             component: LoginPage },
    // admin routes remain server-rendered — no Vue route needed
  ],
});
```

**Step 4: Update `frontend/src/main.js`**

```js
import { createApp } from 'vue';
import { createPinia } from 'pinia';
import { router } from './router.js';
import { AppToast } from './components/AppToast.js';
import { RouterView } from 'vue-router';
import { defineComponent, h } from 'vue';
import { useAuthStore } from './stores/auth.js';

// Root app: router-view + global toast
const RootApp = defineComponent({
  name: 'RootApp',
  setup: () => () => h('div', null, [
    h(AppToast),
    h(RouterView),
  ]),
});

const mountEl = document.getElementById('vue-app');
if (mountEl) {
  const app = createApp(RootApp);
  const pinia = createPinia();
  app.use(pinia);
  app.use(router);

  const authStore = useAuthStore();
  authStore.init(mountEl).then(() => {
    app.mount(mountEl);
  });
}
```

**Step 5: Update `App.js` to re-export `VmListPage` for backward compat**

Delete the old `App.js` or redirect it to `VmListPage.js`. The router now handles what used to be in `App.js`.

**Step 6: Build and manual test**

```bash
cd backend && go build ./...
```

Open <http://localhost:50000> — the `#vue-app` div should load the VM list via Vue Router. Check browser console for errors.

**Step 7: Commit**

```bash
git add frontend/src/router.js frontend/src/main.js frontend/src/pages/ frontend/src/App.js
git commit -m "feat(frontend): set up Vue Router with page stubs for each route"
```

---

## Task 7: Migrate Search page — replace `vmSearch()` Alpine component

**Files:**

- Modify: `frontend/src/pages/SearchPage.js`
- Create: `frontend/src/stores/search.js`

**Step 1: Create Pinia search store**

```js
// frontend/src/stores/search.js
import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '../api/client.js';

export const useSearchStore = defineStore('search', () => {
  const results = ref([]);
  const loading = ref(false);
  const error = ref('');
  const hasSearched = ref(false);

  async function search({ vmid = '', name = '', tags = '', limit = 25 } = {}) {
    loading.value = true;
    error.value = '';
    hasSearched.value = true;
    try {
      const { data } = await api.get('/search/vms', { params: { vmid, name, tags, limit } });
      results.value = data.results || [];
    } catch (err) {
      error.value = err.response?.data?.message || 'Search failed';
      results.value = [];
    } finally {
      loading.value = false;
    }
  }

  function clear() {
    results.value = [];
    hasSearched.value = false;
    error.value = '';
  }

  return { results, loading, error, hasSearched, search, clear };
});
```

**Step 2: Implement `SearchPage.js`**

Replicate the Alpine template structure using Vue render functions. Keep all Bulma CSS classes identical to the existing `search.templ` for visual consistency:

```js
// frontend/src/pages/SearchPage.js
import { defineComponent, h, ref } from 'vue';
import { useSearchStore } from '../stores/search.js';

function statusClass(s) { return { running: 'is-success', stopped: 'is-danger', paused: 'is-warning' }[s] || 'is-light'; }
function statusIcon(s)  { return { running: 'fa-play', stopped: 'fa-stop', paused: 'fa-pause' }[s] || 'fa-question'; }
function parseTags(str) { return str ? str.split(';').map(t => t.trim()).filter(Boolean) : []; }

export const SearchPage = defineComponent({
  name: 'SearchPage',
  setup() {
    const store = useSearchStore();
    const vmid = ref('');
    const name = ref('');
    const tags = ref('');
    const limit = ref(25);
    let debounceTimer;

    function debounceSearch() {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        if (vmid.value || name.value || tags.value) {
          store.search({ vmid: vmid.value, name: name.value, tags: tags.value, limit: limit.value });
        } else {
          store.clear();
        }
      }, 400);
    }

    function handleSearch() {
      store.search({ vmid: vmid.value, name: name.value, tags: tags.value, limit: limit.value });
    }

    function handleClear() {
      vmid.value = ''; name.value = ''; tags.value = ''; limit.value = 25;
      store.clear();
    }

    function renderInput(label, model, placeholder, icon) {
      return h('div', { class: 'column' }, h('div', { class: 'field' }, [
        h('label', { class: 'label' }, label),
        h('div', { class: 'control has-icons-left' }, [
          h('input', { class: 'input', type: 'text', value: model.value, placeholder,
            onInput: e => { model.value = e.target.value; debounceSearch(); },
            onKeydown: e => { if (e.key === 'Enter') { e.preventDefault(); handleSearch(); } },
          }),
          h('span', { class: 'icon is-small is-left' }, h('i', { class: `fas ${icon}` })),
        ]),
      ]));
    }

    function renderResults() {
      if (!store.hasSearched) return null;
      const rows = store.results.map(vm =>
        h('tr', { key: vm.vmid }, [
          h('td', { class: 'has-text-centered' }, h('span', { class: 'tag is-light is-medium has-text-weight-bold' }, vm.vmid)),
          h('td', h('span', { class: 'has-text-weight-semibold' }, vm.name || '-')),
          h('td', vm.description || '-'),
          h('td', { class: 'has-text-centered' }, h('span', { class: 'tag is-medium' }, vm.node)),
          h('td', { class: 'has-text-centered' }, h('span', { class: `tag is-medium ${statusClass(vm.status)}` }, [
            h('span', { class: 'icon is-small' }, h('i', { class: `fas ${statusIcon(vm.status)}` })), vm.status,
          ])),
          h('td', { class: 'has-text-centered' }, parseTags(vm.tags).map(tag =>
            h('span', { key: tag, class: 'tag is-small is-info is-light mr-1' }, tag)
          )),
          h('td', { class: 'has-text-centered' },
            h('a', { href: `/vm/details/${vm.vmid}`, class: 'button is-primary is-small' }, 'Details')
          ),
        ])
      );

      return h('div', { class: 'card' }, [
        h('header', { class: 'card-header brand-header' },
          h('p', { class: 'card-header-title' }, `Results (${store.results.length})`)
        ),
        h('div', { class: 'card-content' },
          store.results.length === 0 && !store.loading
            ? h('div', { class: 'notification is-warning' }, 'No results found.')
            : h('div', { class: 'table-container' },
                h('table', { class: 'table is-fullwidth is-hoverable' }, [
                  h('thead', h('tr', ['VMID','Name','Description','Node','Status','Tags','Actions'].map(col =>
                    h('th', { class: 'has-text-centered' }, col)
                  ))),
                  h('tbody', rows),
                ])
              )
        ),
      ]);
    }

    return () => h('section', { class: 'section' },
      h('div', { class: 'container' }, [
        h('div', { class: 'card mb-5' }, [
          h('header', { class: 'card-header brand-header' },
            h('p', { class: 'card-header-title' }, 'Search VMs')
          ),
          h('div', { class: 'card-content' }, [
            h('div', { class: 'columns is-variable is-4' }, [
              renderInput('VM ID', vmid, 'e.g. 100', 'fa-hashtag'),
              renderInput('Name', name, 'e.g. web-server', 'fa-server'),
              renderInput('Tags', tags, 'e.g. pvmss', 'fa-tags'),
            ]),
            h('div', { class: 'buttons' }, [
              h('button', { class: 'button is-primary', disabled: store.loading, onClick: handleSearch },
                store.loading ? 'Searching…' : 'Search'),
              h('button', { class: 'button is-light', disabled: store.loading, onClick: handleClear }, 'Clear'),
            ]),
            store.error && h('div', { class: 'notification is-danger mt-3' }, store.error),
          ]),
        ]),
        renderResults(),
      ])
    );
  },
});
```

**Step 3: Run tests (backend search endpoint tests) + manual test**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test ./... | grep -E "^(ok|FAIL)"
```

**Step 4: Commit**

```bash
git add frontend/src/stores/search.js frontend/src/pages/SearchPage.js
git commit -m "feat(frontend): migrate Search page from Alpine vmSearch() to Vue + Pinia"
```

---

## Task 8: Migrate Profile page — replace `profile-alpine.js`

**Files:**

- Modify: `frontend/src/pages/ProfilePage.js`
- Create: `frontend/src/stores/profile.js`
- Add: `frontend/src/composables/useAutoRefresh.js`

**Step 1: Create `useAutoRefresh` composable**

```js
// frontend/src/composables/useAutoRefresh.js
import { ref, onMounted, onUnmounted } from 'vue';

export function useAutoRefresh(fetchFn, intervalMs = 30000) {
  const loading = ref(false);
  const error = ref('');
  let timer = null;

  async function refresh() {
    if (document.hidden) return;
    loading.value = true;
    error.value = '';
    try {
      await fetchFn();
    } catch (err) {
      error.value = err.message;
    } finally {
      loading.value = false;
    }
  }

  function start() {
    refresh();
    timer = setInterval(refresh, intervalMs);
    document.addEventListener('visibilitychange', handleVisibility);
  }

  function stop() {
    clearInterval(timer);
    document.removeEventListener('visibilitychange', handleVisibility);
  }

  function handleVisibility() {
    if (document.hidden) stop();
    else start();
  }

  onMounted(start);
  onUnmounted(stop);

  return { loading, error, refresh };
}
```

**Step 2: Create `stores/profile.js`**

```js
// frontend/src/stores/profile.js
import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '../api/client.js';

export const useProfileStore = defineStore('profile', () => {
  const vms = ref([]);

  async function fetchMyVMs() {
    const { data } = await api.get('/profile/vms');
    vms.value = data.vms || [];
  }

  async function doAction(vmid, action, node) {
    await api.post(`/vms/${vmid}/action`, { action, node });
    await fetchMyVMs();
  }

  return { vms, fetchMyVMs, doAction };
});
```

**Step 3: Implement `ProfilePage.js`**

Port the existing profile Alpine template to Vue. The page shows the user's VMs with auto-refresh every 30 seconds (same as `profile-alpine.js`). Reuse `VmCard` and `VmActionButtons`:

```js
// frontend/src/pages/ProfilePage.js
import { defineComponent, h } from 'vue';
import { useProfileStore } from '../stores/profile.js';
import { useAuthStore } from '../stores/auth.js';
import { useAutoRefresh } from '../composables/useAutoRefresh.js';
import { VmCard } from '../components/VmCard.js';

export const ProfilePage = defineComponent({
  name: 'ProfilePage',
  setup() {
    const store = useProfileStore();
    const auth = useAuthStore();
    const { loading, error } = useAutoRefresh(() => store.fetchMyVMs(), 30000);

    return () => h('section', { class: 'section' },
      h('div', { class: 'container' }, [
        h('h1', { class: 'title' }, `My VMs — ${auth.username}`),
        loading.value && h('p', null, 'Loading…'),
        error.value && h('div', { class: 'notification is-danger' }, error.value),
        !loading.value && store.vms.length === 0
          ? h('div', { class: 'notification is-warning' }, 'No VMs found.')
          : h('div', { class: 'columns is-multiline' },
              store.vms.map(vm =>
                h('div', { key: vm.vmid, class: 'column is-12-mobile is-6-tablet is-4-desktop' },
                  h(VmCard, {
                    vm,
                    onAction: ({ vmid, action, node }) => store.doAction(vmid, action, node),
                  })
                )
              )
            ),
      ])
    );
  },
});
```

**Step 4: Commit**

```bash
git add frontend/src/composables/useAutoRefresh.js frontend/src/stores/profile.js frontend/src/pages/ProfilePage.js
git commit -m "feat(frontend): migrate Profile page from profile-alpine.js to Vue + Pinia with auto-refresh"
```

---

## Task 9: Migrate VM Details page — replace `vm-details-alpine.js`

**Files:**

- Modify: `frontend/src/pages/VmDetailsPage.js`
- Create: `frontend/src/stores/vmDetails.js`

This is the most complex page: metrics auto-refresh, network card toggles, modals for editing description/tags/resources, action buttons, celebration banner.

**Step 1: Create `stores/vmDetails.js`**

```js
// frontend/src/stores/vmDetails.js
import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '../api/client.js';

export const useVmDetailsStore = defineStore('vmDetails', () => {
  const vm = ref(null);
  const metrics = ref(null);
  const loading = ref(false);
  const error = ref('');

  async function fetchVM(vmid) {
    loading.value = true;
    try {
      const { data } = await api.get(`/vms/${vmid}`);
      vm.value = data;
    } catch (err) {
      error.value = err.response?.data?.message || 'Failed to load VM';
    } finally {
      loading.value = false;
    }
  }

  async function fetchMetrics(vmid) {
    try {
      const { data } = await api.get(`/vms/${vmid}/metrics`);
      metrics.value = data;
    } catch (_) { /* metrics are non-critical */ }
  }

  async function doAction(vmid, action, node) {
    await api.post(`/vms/${vmid}/action`, { action, node });
    await fetchVM(vmid);
  }

  return { vm, metrics, loading, error, fetchVM, fetchMetrics, doAction };
});
```

**Step 2: Implement `VmDetailsPage.js`**

Extract the VMID from the route (`useRoute().params.vmid`), load the VM, auto-refresh metrics every 30s, render:

- VM info cards (name, status, resources)
- Metrics bar (CPU%, RAM usage)
- Action buttons (reuse `VmActionButtons`)
- Edit modals (description, tags, resources) using `AppModal`
- Network toggles (replacing Alpine `networkToggle`)
- Celebration banner if `?created=1` in URL

```js
// frontend/src/pages/VmDetailsPage.js
import { defineComponent, h, ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { useVmDetailsStore } from '../stores/vmDetails.js';
import { useAutoRefresh } from '../composables/useAutoRefresh.js';
import { VmActionButtons } from '../components/VmActionButtons.js';
import { AppModal } from '../components/AppModal.js';

export const VmDetailsPage = defineComponent({
  name: 'VmDetailsPage',
  setup() {
    const route = useRoute();
    const store = useVmDetailsStore();
    const vmid = route.params.vmid;

    const showDescModal = ref(false);
    const showTagsModal = ref(false);
    const showResourcesModal = ref(false);
    const showCelebration = ref(window.location.search.includes('created=1'));

    onMounted(() => {
      store.fetchVM(vmid);
      if (showCelebration.value) setTimeout(() => { showCelebration.value = false; }, 10000);
    });

    // Auto-refresh metrics every 30s
    useAutoRefresh(() => store.fetchMetrics(vmid), 30000);

    return () => {
      if (store.loading) return h('div', { class: 'has-text-centered p-6' }, 'Loading VM…');
      if (store.error) return h('div', { class: 'notification is-danger' }, store.error);
      if (!store.vm) return null;

      const vm = store.vm;
      return h('section', { class: 'section' },
        h('div', { class: 'container' }, [
          showCelebration.value && h('div', { class: 'notification is-success' }, [
            h('button', { class: 'delete', onClick: () => { showCelebration.value = false; } }),
            '🎉 VM created successfully!',
          ]),
          h('div', { class: 'level' }, [
            h('div', { class: 'level-left' }, h('h1', { class: 'title' }, vm.name || `VM ${vmid}`)),
            h('div', { class: 'level-right' },
              h(VmActionButtons, {
                vmid: vm.vmid,
                node: vm.node,
                status: vm.status,
                onAction: ({ vmid, action, node }) => store.doAction(vmid, action, node),
              })
            ),
          ]),
          // Metrics card
          store.metrics && h('div', { class: 'card mb-4' },
            h('div', { class: 'card-content' }, [
              h('p', null, `CPU: ${(store.metrics.cpu_pct || 0).toFixed(1)}%`),
              h('p', null, `RAM: ${store.metrics.mem_mb} / ${store.metrics.max_mem_mb} MB`),
            ])
          ),
          // Edit buttons
          h('div', { class: 'buttons' }, [
            h('button', { class: 'button', onClick: () => { showDescModal.value = true; } }, 'Edit Description'),
            h('button', { class: 'button', onClick: () => { showTagsModal.value = true; } }, 'Edit Tags'),
            h('button', { class: 'button', onClick: () => { showResourcesModal.value = true; } }, 'Edit Resources'),
          ]),
          // Modals (content TBD per edit form requirements)
          h(AppModal, { title: 'Edit Description', modelValue: showDescModal.value, 'onUpdate:modelValue': v => { showDescModal.value = v; } },
            { default: () => h('p', null, 'Description editor form here') }
          ),
          h(AppModal, { title: 'Edit Tags', modelValue: showTagsModal.value, 'onUpdate:modelValue': v => { showTagsModal.value = v; } },
            { default: () => h('p', null, 'Tags editor form here') }
          ),
          h(AppModal, { title: 'Edit Resources', modelValue: showResourcesModal.value, 'onUpdate:modelValue': v => { showResourcesModal.value = v; } },
            { default: () => h('p', null, 'Resources editor form here') }
          ),
        ])
      );
    };
  },
});
```

Note: The edit modal form bodies are stubs. Each edit calls the existing Go form handlers (POST to `/vm/edit/description`, etc.) via fetch with CSRF token, or add new `/api/v1/vms/:id` PATCH endpoints in a follow-up.

**Step 3: Commit**

```bash
git add frontend/src/stores/vmDetails.js frontend/src/pages/VmDetailsPage.js
git commit -m "feat(frontend): migrate VM Details page from vm-details-alpine.js to Vue + Pinia"
```

---

## Task 10: Migrate Login page — replace Alpine `adminLoginTabs()`

**Files:**

- Modify: `frontend/src/pages/LoginPage.js`

The login page has two tabs: "Local admin" (bcrypt against `ADMIN_PASSWORD_HASH`) and "Proxmox user" (forwards to Proxmox ticket endpoint). Both call `POST /api/v1/auth/login`.

**Step 1: Implement `LoginPage.js`**

```js
// frontend/src/pages/LoginPage.js
import { defineComponent, h, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useNotificationsStore } from '../stores/notifications.js';
import { AppTabs } from '../components/AppTabs.js';
import { AppButton } from '../components/AppButton.js';
import { login } from '../api/auth.js';

export const LoginPage = defineComponent({
  name: 'LoginPage',
  setup() {
    const router = useRouter();
    const notif = useNotificationsStore();
    const activeTab = ref('local');
    const username = ref('admin');
    const password = ref('');
    const pveUsername = ref('');
    const pvePassword = ref('');
    const loading = ref(false);

    async function handleLocalLogin(e) {
      e.preventDefault();
      loading.value = true;
      try {
        await login(username.value, password.value, true);
        router.push('/');
      } catch (err) {
        notif.add({ type: 'danger', message: err.response?.data?.message || 'Login failed' });
      } finally {
        loading.value = false;
      }
    }

    async function handlePveLogin(e) {
      e.preventDefault();
      loading.value = true;
      const user = pveUsername.value.includes('@') ? pveUsername.value : pveUsername.value + '@pve';
      try {
        await login(user, pvePassword.value, false);
        router.push('/profile');
      } catch (err) {
        notif.add({ type: 'danger', message: err.response?.data?.message || 'Proxmox login failed' });
      } finally {
        loading.value = false;
      }
    }

    const localForm = () => h('form', { onSubmit: handleLocalLogin }, [
      h('div', { class: 'field' }, [
        h('label', { class: 'label' }, 'Username'),
        h('input', { class: 'input', value: username.value, onInput: e => { username.value = e.target.value; } }),
      ]),
      h('div', { class: 'field' }, [
        h('label', { class: 'label' }, 'Password'),
        h('input', { class: 'input', type: 'password', value: password.value, onInput: e => { password.value = e.target.value; } }),
      ]),
      h(AppButton, { variant: 'primary', loading: loading.value }, () => 'Login'),
    ]);

    const pveForm = () => h('form', { onSubmit: handlePveLogin }, [
      h('div', { class: 'field' }, [
        h('label', { class: 'label' }, 'Proxmox Username'),
        h('input', { class: 'input', value: pveUsername.value,
          onInput: e => { pveUsername.value = e.target.value; },
          onBlur: e => { if (e.target.value && !e.target.value.includes('@')) pveUsername.value += '@pve'; },
        }),
      ]),
      h('div', { class: 'field' }, [
        h('label', { class: 'label' }, 'Password'),
        h('input', { class: 'input', type: 'password', value: pvePassword.value, onInput: e => { pvePassword.value = e.target.value; } }),
      ]),
      h(AppButton, { variant: 'primary', loading: loading.value }, () => 'Login with Proxmox'),
    ]);

    return () => h('section', { class: 'section' },
      h('div', { class: 'container' },
        h('div', { class: 'columns is-centered' },
          h('div', { class: 'column is-half' },
            h('div', { class: 'card' }, [
              h('header', { class: 'card-header' }, h('p', { class: 'card-header-title' }, 'Sign in to PVMSS')),
              h('div', { class: 'card-content' },
                h(AppTabs, {
                  tabs: [{ key: 'local', label: 'Admin' }, { key: 'pve', label: 'Proxmox User' }],
                  modelValue: activeTab.value,
                  'onUpdate:modelValue': v => { activeTab.value = v; },
                }, { local: localForm, pve: pveForm })
              ),
            ])
          )
        )
      )
    );
  },
});
```

**Step 2: Commit**

```bash
git add frontend/src/pages/LoginPage.js
git commit -m "feat(frontend): migrate Login page tabs from Alpine adminLoginTabs() to Vue AppTabs"
```

---

## Task 11: Remove Alpine.js

**Files:**

- Modify: `backend/components/layout.templ`
- Modify: `backend/components/layout_templ.go` (run `make go-template`)
- Delete: `frontend/js/alpine-init.js`
- Delete: `frontend/js/alpinejs.3.15.3.min.js`
- Delete: `frontend/js/profile-alpine.js`
- Delete: `frontend/js/vm-details-alpine.js`
- Keep: `frontend/js/htmx.2.0.7.min.js` (used by admin pages and remaining templ forms)
- Keep: `frontend/js/accessibility.js`

**Step 1: Verify no Alpine directives remain in user-facing templ files**

```bash
grep -r "x-data\|x-bind\|x-on\|x-show\|x-if\|x-for\|x-model\|Alpine\.\|@click\|@submit\|@input" \
  backend/components/ --include="*.templ" \
  --exclude="admin_*.templ"
```

Expected: zero results. If any remain, implement their Vue equivalent before removing Alpine.

**Step 2: Remove Alpine script tags from `layout.templ`**

Remove these three lines from `layout.templ`:

```html
<script src="/js/alpine-init.js"></script>
<script src="/js/alpinejs.3.15.3.min.js" defer></script>
```

Keep htmx and accessibility.js.

**Step 3: Regenerate templ**

```bash
make go-template
```

**Step 4: Delete Alpine JS files**

```bash
git rm frontend/js/alpine-init.js
git rm frontend/js/alpinejs.3.15.3.min.js
git rm frontend/js/profile-alpine.js
git rm frontend/js/vm-details-alpine.js
```

**Step 5: Build and run full test suite**

```bash
cd backend && go build ./...
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test ./... | grep -E "^(ok|FAIL)"
```

Expected: all PASS (except pre-existing `TestDocsFilesExist`).

**Step 6: Manual smoke test**

- Open <http://localhost:50000> — VM list renders via Vue
- Navigate to `/search` — Search page works
- Navigate to `/profile` — Profile page shows user's VMs
- Navigate to `/vm/details/100` (or any VMID) — Details page loads
- Navigate to `/login` — Login form with two tabs
- Admin pages (`/admin/*`) — still server-rendered, no Alpine errors in console

**Step 7: Commit**

```bash
git add backend/components/layout.templ backend/components/layout_templ.go
git commit -m "feat(layout): remove Alpine.js — fully replaced by Vue 3"
```

---

## Out of scope for this plan (follow-up)

- **VM Create page** (`/vm/create`) — multi-step form with `memoryConverter()`, disk bus logic, cloud-init. Migrate in a separate plan once `/api/v1/vms` POST endpoint is added.
- **Network card toggles** — the `networkToggle()` Alpine component calls the existing session-based `/vm/toggle/network` route with CSRF. Add `PATCH /api/v1/vms/:id/network/:idx` in the same follow-up.
- **Admin pages** — intentionally kept as server-rendered templ; they're infrequently used and mostly static forms.
- **i18n in Vue** — currently translation strings are rendered server-side. A future plan can add a `GET /api/v1/i18n/:lang` endpoint to serve them as JSON for client-side use.
