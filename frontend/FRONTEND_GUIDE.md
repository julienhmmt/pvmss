# Frontend Development Guide

This guide documents the frontend architecture and conventions for the PVMSS application.

## 1. Template Architecture

### 1.1 Layout Hierarchy

```text
layout.html (base shell)
├── <head> (meta, CSS, favicon)
├── navbar.html (navigation)
├── Proxmox status banner (conditional)
├── <main> ← Page content inserted here
├── <footer>
└── Global JS (accessibility.js, etc.)
```

### 1.2 Creating a New Page

1. **Define the page template** with `{{define "page_name"}}`:

```html
{{define "my_page"}}
<section class="section" aria-labelledby="main-title" role="main">
    <div class="container">
        {{template "page_header" (dict 
            "Title" (T "MyPage.Title")
            "Icon" "fas fa-icon"
        )}}
        
        <!-- Page content here -->
    </div>
</section>
{{end}}
```

1. **Register the route** in the backend router

1. **Render via layout** - The handler calls `layout.html` which includes the page template

### 1.3 Page Structure Pattern

Every user-facing page should follow this structure:

```html
<section class="section">
    <div class="container">
        <!-- 1. Page Header (required) -->
        {{template "page_header" (dict ...)}}
        
        <!-- 2. Notifications (optional, from server) -->
        {{if .Error}}
        {{template "notification" (dict "Type" "danger" "Message" .Error ...)}}
        {{end}}
        
        <!-- 3. Main Content -->
        <div class="card">
            <!-- Content -->
        </div>
    </div>
</section>
```

---

## 2. Core Components

### 2.1 `page_header`

Standardized page header with title, icon, and action buttons.

```html
{{template "page_header" (dict 
    "Title" (T "Page.Title")
    "Subtitle" (T "Page.Subtitle")
    "Icon" "fas fa-server"
    "Actions" (dict 
        "Right" (slice 
            (dict "Link" "/path" "Text" "Button" "Icon" "fas fa-plus" "Type" "primary")
        )
    )
)}}
```

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `Title` | string | Page title (becomes `<h1>`) |
| `Subtitle` | string | Optional subtitle |
| `Icon` | string | Font Awesome icon class |
| `Actions.Left` | slice | Left-aligned action buttons |
| `Actions.Right` | slice | Right-aligned action buttons |

**Action Button Properties:**

- `Link` - URL
- `Text` - Button text
- `Icon` - Font Awesome icon
- `Type` - Bulma button type (primary, light, danger, etc.)
- `ID` - Optional element ID
- `DataAttrs` - Optional data attributes string

### 2.2 `notification`

Rich notification component for alerts and messages.

```html
{{template "notification" (dict 
    "Type" "success"
    "Title" "Success!"
    "Message" "Operation completed"
    "Icon" "fas fa-check"
    "Dismissible" true
    "AutoDismiss" true
    "AutoDismissDelay" 5000
)}}
```

**Types:** `success`, `danger`, `warning`, `info`

### 2.3 `status_badge`

VM status indicator with icon and color.

```html
{{template "status_badge" (dict "Status" .VM.Status)}}
```

### 2.4 `card` / `card_admin`

Standardized card containers.

```html
{{template "card" (dict 
    "Title" "Card Title"
    "Icon" "fas fa-cog"
    "Content" "Card content here"
)}}
```

### 2.5 `empty_state`

Display when no data is available.

```html
{{template "empty_state" (dict 
    "Icon" "fas fa-inbox"
    "Title" (T "NoData.Title")
    "Message" (T "NoData.Message")
    "Action" (dict "Link" "/create" "Text" "Create" "Icon" "fas fa-plus")
)}}
```

### 2.6 `error_handler`

Standardized error display with retry action.

```html
{{template "error_handler" (dict 
    "Error" .Error
    "Type" "danger"
    "Title" (T "Common.Error")
    "RetryAction" "/retry"
    "Dismissible" true
)}}
```

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `Error` | string | Error message to display |
| `Type` | string | `danger`, `warning`, `info`, `success` |
| `Title` | string | Error title (defaults to `Common.Error`) |
| `RetryAction` | string | URL for retry button (optional) |
| `Context` | string | Additional context text (optional) |
| `Dismissible` | bool | Show dismiss button (optional) |

### 2.7 `loading_state`

Standardized loading indicator for async operations.

```html
{{template "loading_state" (dict 
    "Message" (T "Common.Loading")
    "Size" "medium"
)}}
```

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `Message` | string | Loading message (defaults to `Common.Loading`) |
| `Size` | string | `small`, `medium`, `large` (defaults to `medium`) |

### 2.8 Tables

#### Standard Table Pattern

All tables should follow this pattern for consistency:

```html
<div class="table-container">
    <table class="table is-fullwidth is-striped">
        <thead>
            <tr class="has-background-light">
                <th>Column 1</th>
                <th>Column 2</th>
            </tr>
        </thead>
        <tbody>
            <tr>
                <td>Data 1</td>
                <td>Data 2</td>
            </tr>
        </tbody>
    </table>
</div>
```

#### Using `table_standard` Component

For simple tables, use the component:

```html
{{template "table_standard" .TableContent}}
```

#### Table Styling Classes

| Class | Description |
|-------|-------------|
| `.table-container` | Responsive wrapper (horizontal scroll on mobile) |
| `.table` | Base Bulma table |
| `.is-fullwidth` | Full width table |
| `.is-striped` | Alternating row colors |
| `.is-hoverable` | Highlight row on hover |
| `.has-background-light` | Light background for headers |
| `.table.modern` | Admin-style table with enhanced styling |

---

## 3. Form Patterns

### 3.0 Admin Forms Migration Guide

When updating admin forms (`admin_limits.html`, `admin_storage.html`, `admin_vmbr.html`), progressively adopt the canonical patterns:

**Current State**: Most admin forms use Bulma-basic patterns (`.label`, `.control`, `.input`)

**Migration Path**:

1. For important fields, add `.form-label` with `.form-label-icon`:

   ```html
   <label class="form-label" for="fieldId">
       <span class="form-label-icon">
           <span class="icon"><i class="fas fa-cog"></i></span>
       </span>
       <span class="form-label-text">Field Label</span>
   </label>
   ```

2. Keep existing `.control` and `.input` classes
3. Add tooltips only for complex fields requiring explanation

**Priority**: Low - existing admin forms work well. Migrate only when actively editing a form.

### 3.1 Pattern A: Full Form (Complex)

Use for VM creation, settings forms:

```html
<div class="field">
    <label class="form-label" for="fieldId">
        <span class="form-label-icon">
            <span class="icon"><i class="fas fa-icon"></i></span>
        </span>
        <span class="form-label-text">Field Label</span>
        <span class="form-label-help">
            <span class="form-tooltip">
                <button type="button" class="form-tooltip-trigger" aria-label="Help">
                    <i class="fas fa-question"></i>
                </button>
                <span class="form-tooltip-content">Help text</span>
            </span>
        </span>
    </label>
    <div class="control">
        <input class="input" type="text" id="fieldId" name="fieldName" />
    </div>
</div>
```

### 3.2 Pattern B: Compact Form (Simple)

Use for login, filters:

```html
<div class="field">
    <label class="form-label" for="fieldId">
        <span class="form-label-icon">
            <span class="icon"><i class="fas fa-icon"></i></span>
        </span>
        <span class="form-label-text">Field Label</span>
    </label>
    <div class="control">
        <input class="input" type="text" id="fieldId" name="fieldName" />
    </div>
</div>
```

---

## 4. JavaScript Architecture

### 4.1 Module Pattern

Page-specific JS goes in `/js/` directory:

- `vm-utils.js` - Shared utilities (status, formatting)
- `profile.js` - Profile page logic
- `search-page.js` - Search page logic
- `accessibility.js` - Global enhancements

### 4.2 Using VMUtils

```javascript
// Include vm-utils.js before your page script
<script src="/js/vm-utils.js" defer></script>
<script src="/js/my-page.js" defer></script>

// In your module:
if (typeof VMUtils === 'undefined') {
    console.error('VMUtils not loaded');
    return;
}

const badge = VMUtils.createStatusBadge('running', translations);
const escaped = VMUtils.escapeHtml(userInput);
```

### 4.3 Passing Translations to JS

Use hidden data elements:

```html
<div id="my-translations" class="is-hidden" aria-hidden="true"
     data-loading="{{T "Common.Loading"}}"
     data-error="{{T "Common.Error"}}">
</div>
```

```javascript
const el = document.getElementById('my-translations');
const t = el ? el.dataset : {};
console.log(t.loading); // "Loading..."
```

---

## 5. CSS Architecture

### 5.1 Load Order

1. `tokens.css` - Design tokens (colors, shadows, radii)
2. `base.css` - Core layout, typography
3. `components.css` - UI components
4. `forms.css` - Form system
5. `glass.css` - Glassmorphism effects
6. `admin.css` - Admin-specific overrides
7. `utilities.css` - Helper classes

### 5.2 Design Tokens

Always use CSS custom properties:

```css
.my-element {
    background: var(--glass-bg);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
    color: var(--text-primary);
}
```

### 5.3 Key Tokens

| Token | Usage |
|-------|-------|
| `--primary` | Brand color |
| `--glass-bg` | Translucent background |
| `--radius-sm/md/lg` | Border radius |
| `--shadow-sm/md/lg` | Box shadows |
| `--text-primary/secondary` | Text colors |

---

## 6. Accessibility Checklist

- [ ] Page has exactly one `<h1>` (via `page_header`)
- [ ] Proper heading hierarchy (h1 → h2 → h3)
- [ ] Interactive elements have `aria-label` or visible text
- [ ] Decorative icons have `aria-hidden="true"`
- [ ] Form inputs have associated labels
- [ ] Color is not the only indicator (use icons too)
- [ ] Focus states are visible
- [ ] Skip link if content is long

---

## 7. Internationalization

All user-facing strings must use translation keys:

```html
{{T "Namespace.Key"}}
```

For dynamic JS content, pass translations via data attributes (see 4.3).

---

## 8. Admin Pages

Admin pages use `admin_base.html` which provides:

- Side navigation menu
- Admin header
- Notification area
- Content section

Admin-specific CSS uses `.admin-*` scoped selectors.

```html
{{define "admin_my_page"}}
    {{template "admin_base" .}}
{{end}}

{{define "admin_my_page_section"}}
    <!-- Content here -->
{{end}}
```
