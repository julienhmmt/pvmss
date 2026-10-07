# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Two audiences share one portal:

**End users** - developers and team members who need VMs without Proxmox access. They log in with their Proxmox credentials, pick a profile, and get a running machine. Their context: they want a VM fast, they don't want to understand Proxmox, and they want to manage their own machines (start, stop, snapshot, console) without filing tickets. Primary task: create and manage my VMs.

**Admins** - infrastructure operators who configure the portal itself. They approve nodes, storages, bridges, ISOs, VM templates, and cloud images from the Proxmox cluster. They set quotas, gabarit limits, and per-node capacity. They manage cloud-init templates and the per-cluster cloud-init write target, hardware profiles, tags, pools, documentation, the audit log, and multi-cluster connections. Their context: they know Proxmox, they want control without logging into Proxmox for every change, and they need to enforce policy. Primary task: keep the catalog and policy in sync with the cluster.

## Product Purpose

PVMSS (Proxmox VM Self-Service) is a lightweight web portal that gives users VM self-service without direct Proxmox access. It exists to remove the bottleneck of admin tickets for routine VM operations while enforcing organizational policy (quotas, catalog enforcement, gabarit limits).

Success looks like: a user creates a VM in under a minute, an admin approves a new node in two clicks, and neither party needs to touch the Proxmox UI.

## Positioning

The differentiator is warmth. Infrastructure tooling is cold by default - Proxmox's own UI, and the admin panels built around it, read as machinery. PVMSS is the same capability wearing a human face: warm paper surfaces, a friendly voice, empty states that teach. A competitor can copy the feature list; the felt experience of a tool that behaves like a competent colleague rather than a gatekeeper is the claim that is hard to copy truthfully.

The plumbing is hidden to deliver that experience: users see profiles and options, never nodes, storages, or API calls. Hiding Proxmox is how warmth is made possible, not the promise itself.

## Operating Context

PVMSS is self-hosted by a single organization for its own teams - one operator, one portal, many clusters.

- **Deployment**: Docker image (distroless, non-root) or Kubernetes via the Helm chart; configuration is environment variables only, validated at startup.
- **Clusters**: one or more Proxmox clusters behind a single portal, selectable per user; `PVMSS_CLUSTER_SOURCE` chooses the live `proxmox` source or the `fake` demo source.
- **Admin rituals**: approving catalog items (nodes, storages, bridges, ISOs, templates, cloud images), setting quotas and gabarit limits, curating hardware profiles, authoring cloud-init templates, and running the copy-paste snippet publish command on chosen nodes (PVMSS never writes on the nodes itself).
- **User rituals**: sign in with Proxmox credentials, create from a starting point, connect over SSH or the in-browser console, manage power and snapshots.
- **Locale**: bilingual English + French across every screen and state.

## Capabilities and Constraints

**Capabilities** - VM lifecycle for end users: create (ISO, Proxmox template clone, or cloud-image import), list and search across clusters, live status, bulk power actions, snapshot, in-browser console, activity and audit history, personal API tokens (currently deactivated). Admin: cluster connections, approved catalog, hardware profiles, quotas and per-node capacity, pools, tags, cloud-init templates, documentation, audit log.

**Constraints**:

- Users never get direct Proxmox access; ownership is enforced server-side, not by list filtering.
- PVMSS never writes on the nodes. The Proxmox API cannot write snippets, so cloud-init files are published by an admin running a copy-paste command; presence is read live per node and never stored.
- The server is deliberately dependency-light: stdlib routing, SQLite for persistence, no CGO.
- Only PVMSS administrators author cloud-init YAML; end users cannot.

**Terminology** is defined in `CONTEXT.md` (projection, live status, UPID, lock); feature status lives in `docs/FEATURES.md`, workflows in `WORKFLOWS.md`.

## Brand Commitments

**Name**: PVMSS (Proxmox VM Self-Service).

**Voice**: warm, clear, dependable - three words. Direct and friendly, never technical for its own sake. A competent colleague who helps you get things done, not a gatekeeper who demands technical knowledge.

**Identity**: the warm paper background and orange accent are intentional and binding - they signal a tool made by people, not a cold corporate dashboard.

**Anti-references** - the look must not drift toward:

- **Generic SaaS templates** - indigo/violet gradients, Inter font on gray-50, identical card grids, hero-metric layouts. PVMSS should not look like it was generated from a template.
- **Cluttered enterprise dashboards** - too many panels, gauges, widgets, and tabs fighting for attention. Density is fine when it serves the task; decoration is not.
- **Overly playful consumer apps** - excessive animation, illustrations, pastel colors, mascots. This is infrastructure; warmth doesn't mean childish.

## Evidence on Hand

The live application and screenshots of it. There are no testimonials, case studies, press mentions, customer logos, or benchmarks. Future work must not fabricate any of them.

## Product Principles

1. **Show the task, hide the plumbing.** Users see profiles and options, not Proxmox API calls. Admins see catalog items and policy, not raw cluster responses. Implementation details surface only when the user explicitly asks (e.g. the review step's JSON preview).

2. **Warmth over coldness.** Every surface should feel made by a person - friendly microcopy, empty states that teach rather than scold. This is the differentiator from generic admin panels.

3. **Consistency is the feature.** The same vocabulary, structure, and loading pattern across every page. Users navigate faster when structure is predictable. Inconsistency is a bug.

4. **Safety nets for destructive actions.** Delete confirmations, undo toasts. The interface should never let a user lose data through a misclick. High-stakes moments (delete, reset, revoke) get design interventions.

5. **Keyboard-first for power users.** Global shortcuts, focus-visible rings, tab navigation, `Cmd+Enter` to submit. The app should be fully usable without a mouse, and shortcuts should be discoverable.

## Accessibility & Inclusion

Target: WCAG 2.1 AA compliance.

- Keyboard navigation across all surfaces, with visible focus states
- Screen reader support via ARIA landmarks, live regions, and semantic HTML
- Reduced-motion support (all animations respect `prefers-reduced-motion`)
- Color contrast meeting AA ratios (4.5:1 body, 3:1 large text)
- Skip-to-content link for bypassing navigation
- Bilingual interface (English + French) with proper `lang` attribute
