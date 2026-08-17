# Product

## Register

product

## Users

Two audiences share one portal:

**End users** — developers and team members who need VMs without Proxmox access. They log in with their Proxmox credentials, pick a profile, and get a running machine. Their context: they want a VM fast, they don't want to understand Proxmox, and they want to manage their own machines (start, stop, snapshot, console) without filing tickets. Primary task: create and manage my VMs.

**Admins** — infrastructure operators who configure the portal itself. They approve nodes, storages, bridges, and ISOs from the Proxmox cluster. They set quotas and gabarit limits. They manage cloud-init templates, tags, pools, documentation, and multi-cluster connections. Their context: they know Proxmox, they want control without logging into Proxmox for every change, and they need to enforce policy. Primary task: keep the catalog and policy in sync with the cluster.

## Product Purpose

PVMSS (Proxmox VM Self-Service) is a lightweight web portal that gives users VM self-service without direct Proxmox access. It exists to remove the bottleneck of admin tickets for routine VM operations while enforcing organizational policy (quotas, catalog enforcement, gabarit limits).

Success looks like: a user creates a VM in under a minute, an admin approves a new node in two clicks, and neither party needs to touch the Proxmox UI.

## Brand Personality

Warm, human, approachable. Infrastructure is intimidating; PVMSS should make it feel manageable. The interface should feel like a competent colleague who helps you get things done, not a gatekeeper who demands technical knowledge.

Three words: warm, clear, dependable.

The warm paper background and orange accent are intentional — they signal that this is a tool made by people, not a cold corporate dashboard. The tone in microcopy should be direct and friendly, never technical-for-its-own-sake.

## Anti-references

- **Generic SaaS templates** — indigo/violet gradients, Inter font on gray-50, identical card grids, hero-metric layouts. PVMSS should not look like it was generated from a template.
- **Cluttered enterprise dashboards** — too many panels, gauges, widgets, and tabs fighting for attention. Density is fine when it serves the task; decoration is not.
- **Overly playful consumer apps** — excessive animation, illustrations, pastel colors, mascots. This is infrastructure; warmth doesn't mean childish.

## Design Principles

1. **Show the task, hide the plumbing.** Users see profiles and options, not Proxmox API calls. Admins see catalog items and policy, not raw cluster responses. The interface should never expose implementation details unless the user explicitly asks (e.g., the review step's JSON preview).

2. **Warmth over coldness.** Every surface should feel made-by-a-person: the warm paper background, the orange accent, the friendly microcopy, the clear empty states that teach rather than scold. This is the differentiator from generic admin panels.

3. **Consistency is the feature.** The same button shape, the same form vocabulary, the same loading skeleton, the same empty state pattern across every page. Users navigate faster when structure is predictable. Inconsistency is a bug.

4. **Safety nets for destructive actions.** Delete confirmations, undo toasts, draft auto-save. The interface should never let a user lose data through a misclick. High-stakes moments (delete, reset, revoke) get design interventions.

5. **Keyboard-first for power users.** Global shortcuts, focus-visible rings, tab navigation, `Cmd+Enter` to submit. The app should be fully usable without a mouse, and shortcuts should be discoverable.

## Accessibility & Inclusion

Target: WCAG 2.1 AA compliance.

- Keyboard navigation across all surfaces, with visible focus states
- Screen reader support via ARIA landmarks, live regions, and semantic HTML
- Reduced-motion support (all animations respect `prefers-reduced-motion`)
- Color contrast meeting AA ratios (4.5:1 body, 3:1 large text)
- Skip-to-content link for bypassing navigation
- Bilingual interface (English + French) with proper `lang` attribute
