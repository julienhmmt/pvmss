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
