# PVMSS CSS Architecture Guide

This guide documents the current CSS architecture after the comprehensive refactoring. Follow these guidelines to maintain clean, predictable styling across the application.

## 🎯 Core Principles

- **Single source of truth**: All colors, shadows, radii, and timing values live in `tokens.css`
- **Clear layer separation**: Each CSS file has distinct responsibilities
- **One canonical style per primitive**: Core components defined once, enhanced by theme layers
- **Cross-browser compatibility**: Safari/Firefox support via vendor prefixes

## 📁 File Responsibilities

### `tokens.css` - Design System Foundation

- **Purpose**: Single source of truth for all design tokens
- **Contains**: Brand colors, glass effects, gray scale, shadows, radii, transitions
- **Usage**: Other files consume tokens; never redefine raw hex values
- **Key tokens**: `--primary`, `--success`, `--warning`, `--danger`, `--glass-opacity-*`, `--gray-*`

### `base.css` - Structural Foundation

- **Purpose**: Default structural definitions for core components
- **Contains**: `.box`, `.card`, `.navbar`, `.notification`, `.table-container`, `.table`
- **Approach**: Structural properties (padding, margin, border-radius) + fallback visuals
- **Theme**: Neutral styling that gets enhanced by theme layers

### `glass.css` - Visual Theme Overlay

- **Purpose**: Glassmorphism visual effects for main theme
- **Scope**: Applied to non-admin pages only via `section:not(.admin-section)` and `body:not(.admin-page)`
- **Contains**: `backdrop-filter`, glass backgrounds, enhanced shadows
- **Compatibility**: Includes `-webkit-` prefixes for Safari support

### `components.css` - Behavioral Components

- **Purpose**: Higher-level UI components and layout logic
- **Contains**: Navbar layout, hero blocks, Proxmox banner, notification components
- **Approach**: Structure and behavior only, no theme-specific visuals
- **Key components**: `.navbar-*`, `.hero`, `.notification`, `.message`, `.divider-with-text`

### `forms.css` - Form System

- **Purpose**: Comprehensive form styling for complex forms
- **Contains**: `.form-label`, `.form-tooltip`, `.form-vertical`, `.field`, `.input`, `.select`
- **Features**: Rich tooltips, inline fields, scoped overflow fixes
- **Scope**: Overflow hacks scoped to form contexts only

### `admin.css` - Admin Theme

- **Purpose**: Admin-specific visual differences (solid vs glass)
- **Scope**: Applied to `.section.admin-section` contexts only
- **Contains**: `.admin-menu`, `.admin-box`, `.table.modern`, admin login styles
- **Approach**: Consumes tokens from `tokens.css`, solid styling instead of glassmorphism

### `utilities.css` - Utility Classes

- **Purpose**: Focused, additive utility classes
- **Contains**: Layout helpers (`.max-w-*`), borders (`.border-*`), shadows (`.shadow-*`)
- **Approach**: No redefinition of core primitives, only clear intent utilities

## 🎨 Theme Separation

### Main Theme (Glassmorphism)

- Applied to: All non-admin pages
- Visual: Translucent backgrounds, backdrop-filter blur, glass effects
- CSS: `glass.css` effects via `section:not(.admin-section)` selectors

### Admin Theme (Solid)

- Applied to: Admin pages only
- Visual: Solid white/gray backgrounds, modern but flat design
- CSS: `admin.css` styles via `.section.admin-section` selectors
- Body class: `admin-page` added by `{{if .IsAdminPage}}admin-page{{end}}` in layout.html

## 🔧 Adding New Styles

### New Design Tokens

```css
/* Add to tokens.css */
:root {
  --your-new-token: #hex-value;
}
```

### New Components

1. **Structure**: Add to `base.css` if it's a core component
2. **Behavior**: Add to `components.css` if it's a higher-level component
3. **Theme**: Add visual enhancements to `glass.css` (main) or `admin.css` (admin)

### New Utilities

```css
/* Add to utilities.css */
.your-utility {
  property: var(--token);
}
```

## 🚨 Important Rules

### DO

- ✅ Use tokens from `tokens.css` instead of raw hex values
- ✅ Respect layer responsibilities (structure in base, visuals in glass)
- ✅ Scope admin styles to `.section.admin-section`
- ✅ Add `-webkit-` prefixes for `backdrop-filter`
- ✅ Use semantic color tokens (`--success-dark`, `--warning-bg`)

### DON'T

- ❌ Redefine core primitives in multiple files
- ❌ Use raw hex colors without tokens
- ❌ Apply admin styles globally (scope them!)
- ❌ Override structural properties in theme layers
- ❌ Use `!important` unless absolutely necessary

## 🔍 Contrast & Accessibility

### Light Variants

Light background components (`.is-light`) use dark text for proper contrast:

- `.tag.is-success.is-light`: `--success-dark` text on `--success-light` background
- `.notification.is-warning.is-light`: `--warning-dark` text on `--warning-bg` background
- Similar pattern for `.is-danger`, `.is-info` variants

### Cross-Browser Support

- Safari: `-webkit-backdrop-filter` prefixes included
- Firefox: Native `backdrop-filter` support
- Chrome: Full support without prefixes needed

## 📋 Common Patterns

### Cards and Boxes

```css
/* Structure in base.css */
.card {
  border-radius: var(--radius-lg);
  padding: 0;
  /* fallback visuals */
}

/* Visual in glass.css (main theme) */
section:not(.admin-section) .card {
  background: var(--glass-opacity-normal);
  backdrop-filter: var(--blur-md) saturate(180%);
}

/* Visual in admin.css (admin theme) */
.admin-section .card {
  background: var(--surface);
  border: 1px solid var(--border);
}
```

### Status Colors

```css
/* Use semantic tokens */
.your-component {
  background: var(--success-bg);
  color: var(--success-dark);
  border: 1px solid var(--success);
}
```

## 🎯 Z-Index Strategy

PVMSS uses a layered z-index approach to prevent conflicts and maintain proper stacking order:

### Z-Index Scale

- **Base Layer** (`--z-base: 0`): Default content, no special stacking
- **Navbar** (`--z-navbar: 100`): Fixed navigation bar, always visible
- **Banner** (`--z-banner: 99`): Status banners (below navbar for proper layering)
- **Dropdown** (`--z-dropdown: 1000`): Dropdown menus, above content
- **Tooltip** (`--z-tooltip: 1100`): Tooltips, above dropdowns
- **Modal** (`--z-modal: 2000`): Modal dialogs, highest layer

### Usage Rules

- Always use z-index tokens from `tokens.css` instead of hardcoded values
- Never use z-index values outside the documented scale
- When adding new components that need stacking, add a new token to `tokens.css`
- Document the purpose of each z-index level in code comments

### Example

```css
.my-dropdown {
  position: absolute;
  z-index: var(--z-dropdown);
}

.my-modal {
  position: fixed;
  z-index: var(--z-modal);
}
```

## 📱 Responsive Design

PVMSS uses a mobile-first approach with three main breakpoints:

### Breakpoints

- **Mobile** (`--breakpoint-mobile: 768px`): Phones and small tablets
- **Tablet** (`--breakpoint-tablet: 1024px`): Tablets and small desktops
- **Desktop** (`--breakpoint-desktop: 1200px`): Large desktops and ultra-wide screens

### Responsive Adjustments

**Navbar Height**:

- Desktop: `3.75rem` (60px)
- Mobile: `3.25rem` (52px) - reduced for screen space

**Layout Changes**:

- Desktop: Multi-column flex layouts
- Tablet: Adjusted column widths
- Mobile: Single-column stack layouts

**Font Sizes**:

- Scale down on mobile for readability
- Use relative units (rem, em) for scalability

### Media Query Pattern

```css
/* Desktop-first approach */
.your-component {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
}

/* Tablet adjustments */
@media (width <= 1024px) {
  .your-component {
    grid-template-columns: 1fr 1fr;
  }
}

/* Mobile adjustments */
@media (width <= 768px) {
  .your-component {
    grid-template-columns: 1fr;
  }
}
```

## ✅ Form Validation States

Form validation uses semantic CSS classes to indicate field states:

### Validation Classes

- `.is-success`: Green background, success styling
- `.is-warning`: Yellow background, warning styling
- `.is-danger`: Red background, error styling
- `.is-info`: Blue background, informational styling

### Light variants

Light variants (`.is-light`) use dark text for proper contrast on light backgrounds:

```css
/* Success state */
.field.is-success input {
  border-color: var(--success);
  background: var(--success-light);
}

.field.is-success.is-light input {
  color: var(--success-dark);
  background: var(--success-light);
}
```

### Validation Messages

Use `.form-help` class for validation messages:

```html
<div class="field">
  <input class="input is-danger" type="text">
  <p class="form-help is-danger">This field is required</p>
</div>
```

### Animation on State Change

Validation states should animate smoothly:

```css
.field input {
  transition: border-color var(--transition-base),
              background-color var(--transition-base),
              box-shadow var(--transition-base);
}
```

## 💡 Light vs. Normal variants

Understanding when to use light vs. normal variants:

### Light variants (`.is-light`)

Use light variants for:

- Secondary information that doesn't need emphasis
- Less important alerts or notifications
- Background elements that support primary content
- Informational messages (not errors)

**Characteristics**:

- Light background color
- Dark text for contrast
- Subtle, non-intrusive appearance

### Normal variants

Use normal variants for:

- Primary information requiring attention
- Important alerts and errors
- Call-to-action elements
- Critical status messages

**Characteristics**:

- Darker background color
- Light text (usually white)
- Prominent, attention-grabbing appearance

### Decision Tree

```text
Is this critical information?
├─ YES → Use normal variant (.is-danger, .is-warning, etc.)
└─ NO → Is this supplementary?
    ├─ YES → Use light variant (.is-danger.is-light, etc.)
    └─ NO → Use normal variant
```

## 🧪 Testing

After CSS changes, verify these pages:

- **Main theme**: Home, login, profile, create VM, VM details
- **Admin theme**: Admin nodes, limits, tags, storage, userpool
- **Cross-browser**: Test in Chrome, Firefox, Safari
- **Contrast**: Check light variants have proper text contrast

## 📚 History

This architecture was established during the comprehensive CSS refactoring of November 2025, which:

- Eliminated duplication across CSS files
- Separated structural and visual concerns
- Established clear theme boundaries
- Implemented comprehensive token system
- Added cross-browser compatibility
- Fixed contrast issues in status components

Maintain this architecture to ensure future CSS changes remain safe and predictable.
