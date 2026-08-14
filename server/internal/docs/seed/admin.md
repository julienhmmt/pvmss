# Admin guide

This page is visible to administrators only. It documents the admin surface
and the operational tasks that keep PVMSS healthy.

## Catalog

The **Catalog** section of the admin nav lets you approve or hide the nodes,
storages, bridges, ISOs, profiles, tags, and cloud-init templates that VM
creation may reference. Discovered resources appear here automatically; toggle
the enabled switch to control what users see.

## Policy

**Limits** sets per-user quotas (max VMs, CPU, memory, disk). **Node capacity**
caps how much of a node's resources a single VM may consume. Both are enforced
server-side before any Proxmox call.

## Documentation

This page itself is managed under **Documentation** in the admin nav. Admins
can author, edit, toggle, and delete Markdown pages. Built-in pages (like this
one) are marked **system** and cannot be deleted, but their content may be
edited. Each page has an audience of `user` (public) or `admin` (admin-only).

## System

The **App Info** page shows the running version and configuration. The
**Settings** page exposes operational toggles. Use the audit log to trace
every VM write back to the acting user.
