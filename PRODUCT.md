# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Two audiences share one portal:

**End users** - developers and team members who need VMs without Proxmox access. They log in with their Proxmox credentials, pick a profile, and get a running machine. Their context: they want a VM fast, they don't want to understand Proxmox, and they want to manage their own machines (start, stop, snapshot, console) without filing tickets. Primary task: create and manage my VMs.

**Admins** - infrastructure operators who configure the portal itself. They approve nodes, storages, bridges, ISOs, and VM templates from the Proxmox cluster. They set quotas, gabarit limits, and per-node capacity. They manage hardware profiles, tags, pools, documentation, the audit log, and multi-cluster connections. Their context: they know Proxmox, they want control without logging into Proxmox for every change, and they need to enforce policy. Primary task: keep the catalog and policy in sync with the cluster.

## Product Purpose

PVMSS (Proxmox VM Self-Service) is a lightweight web portal that gives users VM self-service without direct Proxmox access. It exists to remove the bottleneck of admin tickets for routine VM operations while enforcing organizational policy (quotas, catalog enforcement, gabarit limits).

Success looks like: a user creates a VM in under a minute, an admin approves a new node in two clicks, and neither party needs to touch the Proxmox UI.

## Positioning

The differentiator is warmth. Infrastructure tooling is cold by default - Proxmox's own UI, and the admin panels built around it, read as machinery. PVMSS offers the same capability with warm paper surfaces, a friendly voice and empty states that teach. A competitor can copy the feature list, but it is harder to copy a tool that behaves like a competent colleague.

The plumbing is hidden to deliver that experience: users see profiles and options, never nodes, storages, or API calls. Hiding Proxmox makes that warmth possible.

## Operating Context

PVMSS is self-hosted by a single organization for its own teams - one operator, one portal, many clusters.

- **Deployment**: Docker image (distroless, non-root) or Kubernetes via the Helm chart; configuration is environment variables only, validated at startup.
- **Clusters**: one or more Proxmox clusters behind a single portal, selectable per user; `PVMSS_CLUSTER_SOURCE` chooses the live `proxmox` source or the `fake` demo source.
- **Admin rituals**: approving catalog items (nodes, storages, bridges, ISOs, templates), setting quotas and gabarit limits, curating hardware profiles, and deciding user quota requests.
- **User rituals**: sign in with Proxmox credentials, create from a starting point, connect over SSH or the in-browser console, manage power and snapshots.
- **Locale**: English, French, German and Italian across every screen and state; in-app docs in English and French.

## Capabilities and Constraints

**Capabilities** - VM lifecycle for end users: create (ISO or Proxmox template clone), list and search (on the sign-in cluster), live status, bulk power actions, snapshot, in-browser console, activity and audit history. Admin: cluster connections, approved catalog, hardware profiles, quotas and per-node capacity, pools, tags, documentation, audit log.

**Constraints**:

- Users never get direct Proxmox access; ownership is enforced server-side, not by list filtering.
- PVMSS never writes on the nodes.
- The server is deliberately dependency-light: stdlib routing, SQLite for persistence, no CGO.
- Cloud-init was removed on 2026-10-31 and is not a current capability (see `docs/cloud-init.md`).

**Terminology** is defined in `CONTEXT.md` (projection, live status, UPID, lock); feature status lives in `docs/FEATURES.md`, workflows in `WORKFLOWS.md`.

## Brand Commitments

**Name**: PVMSS (Proxmox VM Self-Service).

**Voice**: warm, clear, dependable. Direct and friendly, never technical for its own sake. A competent colleague who helps you get things done, not a gatekeeper who demands technical knowledge.

**Identity**: the warm paper background and orange accent are intentional and binding - they signal a tool made by people, not a cold corporate dashboard.

**Anti-references** - the look must not drift toward:

- **Generic SaaS templates** - indigo/violet gradients, Inter font on gray-50, identical card grids, hero-metric layouts. PVMSS should not look like it was generated from a template.
- **Cluttered enterprise dashboards** - too many panels, gauges, widgets, and tabs fighting for attention. Density is fine when it serves the task; decoration is not.
- **Overly playful consumer apps** - excessive animation, illustrations, pastel colors, mascots. This is infrastructure; warmth doesn't mean childish.

## Evidence on Hand

The live application and screenshots of it. There are no testimonials, case studies, press mentions, customer logos, or benchmarks. Future work must not fabricate any of them.

## Product Principles

1. **Show the task, hide the plumbing.** Users see profiles and options, not Proxmox API calls. Admins see catalog items and policy, not raw cluster responses. Implementation details surface only when the user explicitly asks (e.g. the review step's JSON preview).

2. **A friendly voice.** Every surface should feel made by a person - friendly microcopy, empty states that teach rather than scold.

3. **The same on every page.** The same vocabulary, structure, and loading pattern across every page. Users navigate faster when structure is predictable.

4. **Safety nets for destructive actions.** Delete confirmations, undo toasts. The interface should never let a user lose data through a misclick. High-stakes moments (delete, reset, revoke) get design interventions.

5. **Keyboard-first for power users.** Global shortcuts, focus-visible rings, tab navigation, `Cmd+Enter` to submit. The app should be fully usable without a mouse, and shortcuts should be discoverable.

## Accessibility & Inclusion

Target: WCAG 2.1 AA compliance.

- Keyboard navigation across all surfaces, with visible focus states
- Screen reader support via ARIA landmarks, live regions, and semantic HTML
- Reduced-motion support (all animations respect `prefers-reduced-motion`)
- Color contrast meeting AA ratios (4.5:1 body, 3:1 large text)
- Skip-to-content link for bypassing navigation
- Interface in English, French, German and Italian with proper `lang` attribute
