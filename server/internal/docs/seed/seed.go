// Package seed holds the built-in documentation pages embedded at build time
// (issue #53). SeedDocumentationPages idempotently inserts any missing
// (id, lang) rows with is_system=1 so admin edits to seeded pages are never
// clobbered on restart.
package seed

import (
	"context"
	_ "embed"
	"fmt"
	"pvmss/server/internal/store"
	"time"
)

//go:embed getting-started.md
var gettingStartedMD string

//go:embed vm-creation-guidelines.md
var vmCreationGuidelinesMD string

//go:embed cloud-init-howto.md
var cloudInitHowtoMD string

//go:embed admin.md
var adminMD string

//go:embed getting-started.fr.md
var gettingStartedFR string

//go:embed vm-creation-guidelines.fr.md
var vmCreationGuidelinesFR string

//go:embed cloud-init-howto.fr.md
var cloudInitHowtoFR string

//go:embed recovered/admin-guide.en.md
var adminGuideEN string

//go:embed recovered/admin-guide.fr.md
var adminGuideFR string

//go:embed recovered/user-guide.en.md
var userGuideEN string

//go:embed recovered/user-guide.fr.md
var userGuideFR string

//go:embed recovered/cloud-init-setup.en.md
var cloudInitSetupEN string

//go:embed recovered/cloud-init-setup.fr.md
var cloudInitSetupFR string

//go:embed recovered/proxmox-permissions.en.md
var proxmoxPermissionsEN string

//go:embed recovered/proxmox-permissions.fr.md
var proxmoxPermissionsFR string

// seedPage describes one built-in page to insert.
type seedPage struct {
	id        string
	title     string
	category  string
	bodyMD    string
	audience  string
	sortOrder int
}

// builtInPages is the fixed set of system pages. The audience maps directly:
// user docs are public, the admin doc is admin-only. Recovered v0.3 guides
// (admin/user/cloud-init/permissions) were reintegrated and rewritten for the
// v0.4 app, then seeded as system pages so they survive restarts (issue #53).
var builtInPages = []seedPage{
	{id: "getting-started", title: "Getting started", category: "Getting started", bodyMD: gettingStartedMD, audience: "user", sortOrder: 1},
	{id: "vm-creation-guidelines", title: "VM creation guidelines", category: "Creating VMs", bodyMD: vmCreationGuidelinesMD, audience: "user", sortOrder: 2},
	{id: "cloud-init-howto", title: "Cloud-init how-to", category: "Creating VMs", bodyMD: cloudInitHowtoMD, audience: "user", sortOrder: 3},
	{id: "admin", title: "Admin guide", category: "Administration", bodyMD: adminMD, audience: "admin", sortOrder: 100},
	{id: "user-guide", title: "User guide", category: "Guides", bodyMD: userGuideEN, audience: "user", sortOrder: 4},
	{id: "admin-guide", title: "Administrator guide", category: "Guides", bodyMD: adminGuideEN, audience: "admin", sortOrder: 101},
	{id: "cloud-init-setup", title: "Cloud-init setup (admin)", category: "Guides", bodyMD: cloudInitSetupEN, audience: "admin", sortOrder: 102},
	{id: "proxmox-permissions", title: "Proxmox permissions for PVMSS", category: "Guides", bodyMD: proxmoxPermissionsEN, audience: "admin", sortOrder: 103},
}

// SeedDocumentationPages inserts every built-in page that does not already
// exist (by (id, lang)). Existing rows — including admin-edited system pages —
// are left untouched, so the seed is safe to run on every startup.
//
//nolint:revive // the Seed verb documents the seeding action explicitly
func SeedDocumentationPages(ctx context.Context, st *store.Store) error {
	stamp := time.Now().UTC().Format(time.RFC3339Nano)

	for _, p := range builtInPages {
		// Pages that ship in both languages are seeded variant by variant.
		// insertSeedRow is itself idempotent per (id, lang), so re-running
		// the seed never clobbers an existing or admin-edited row.
		if v := frenchVariants(p.id); v != nil {
			for _, variant := range v {
				if err := insertSeedRow(ctx, st, variant.id, variant.lang, variant.title, variant.category, variant.bodyMD, variant.audience, variant.sortOrder, stamp); err != nil {
					return err
				}
			}
			continue
		}

		// English-only pages: seed once if the (id, en) row is missing.
		exists, err := st.DocumentationPageExists(ctx, p.id, "en")
		if err != nil {
			return fmt.Errorf("check seed page %q: %w", p.id, err)
		}
		if exists {
			continue
		}
		if err := insertSeedRow(ctx, st, p.id, "en", p.title, p.category, p.bodyMD, p.audience, p.sortOrder, stamp); err != nil {
			return err
		}
	}

	return nil
}

// seedVariant is one language variant of a bilingual recovered guide.
type seedVariant struct {
	id        string
	lang      string
	title     string
	category  string
	bodyMD    string
	audience  string
	sortOrder int
}

// frenchVariants returns the en + fr rows for a built-in page that ships in
// both languages. It returns nil for pages that are English-only.
func frenchVariants(id string) []seedVariant {
	switch id {
	case "getting-started":
		return []seedVariant{
			{id: "getting-started", lang: "en", title: "Getting started", category: "Getting started", bodyMD: gettingStartedMD, audience: "user", sortOrder: 1},
			{id: "getting-started", lang: "fr", title: "Premiers pas", category: "Premiers pas", bodyMD: gettingStartedFR, audience: "user", sortOrder: 1},
		}
	case "vm-creation-guidelines":
		return []seedVariant{
			{id: "vm-creation-guidelines", lang: "en", title: "VM creation guidelines", category: "Creating VMs", bodyMD: vmCreationGuidelinesMD, audience: "user", sortOrder: 2},
			{id: "vm-creation-guidelines", lang: "fr", title: "Recommandations de création de VM", category: "Création de VMs", bodyMD: vmCreationGuidelinesFR, audience: "user", sortOrder: 2},
		}
	case "cloud-init-howto":
		return []seedVariant{
			{id: "cloud-init-howto", lang: "en", title: "Cloud-init how-to", category: "Creating VMs", bodyMD: cloudInitHowtoMD, audience: "user", sortOrder: 3},
			{id: "cloud-init-howto", lang: "fr", title: "Guide cloud-init", category: "Création de VMs", bodyMD: cloudInitHowtoFR, audience: "user", sortOrder: 3},
		}
	case "user-guide":
		return []seedVariant{
			{id: "user-guide", lang: "en", title: "User guide", category: "Guides", bodyMD: userGuideEN, audience: "user", sortOrder: 4},
			{id: "user-guide", lang: "fr", title: "Guide de l'utilisateur", category: "Guides", bodyMD: userGuideFR, audience: "user", sortOrder: 4},
		}
	case "admin-guide":
		return []seedVariant{
			{id: "admin-guide", lang: "en", title: "Administrator guide", category: "Guides", bodyMD: adminGuideEN, audience: "admin", sortOrder: 101},
			{id: "admin-guide", lang: "fr", title: "Guide de l'administrateur", category: "Guides", bodyMD: adminGuideFR, audience: "admin", sortOrder: 101},
		}
	case "cloud-init-setup":
		return []seedVariant{
			{id: "cloud-init-setup", lang: "en", title: "Cloud-init setup (admin)", category: "Guides", bodyMD: cloudInitSetupEN, audience: "admin", sortOrder: 102},
			{id: "cloud-init-setup", lang: "fr", title: "Configuration cloud-init (administrateur)", category: "Guides", bodyMD: cloudInitSetupFR, audience: "admin", sortOrder: 102},
		}
	case "proxmox-permissions":
		return []seedVariant{
			{id: "proxmox-permissions", lang: "en", title: "Proxmox permissions for PVMSS", category: "Guides", bodyMD: proxmoxPermissionsEN, audience: "admin", sortOrder: 103},
			{id: "proxmox-permissions", lang: "fr", title: "Permissions Proxmox pour PVMSS", category: "Guides", bodyMD: proxmoxPermissionsFR, audience: "admin", sortOrder: 103},
		}
	}
	return nil
}

// insertSeedRow idempotently inserts one (id, lang) system page.
func insertSeedRow(ctx context.Context, st *store.Store, id, lang, title, category, bodyMD, audience string, sortOrder int, stamp string) error {
	exists, err := st.DocumentationPageExists(ctx, id, lang)
	if err != nil {
		return fmt.Errorf("check seed page %q/%s: %w", id, lang, err)
	}
	if exists {
		return nil
	}

	row := store.DocumentationPageRow{
		ID: id, Lang: lang, Title: title, Category: category, BodyMD: bodyMD,
		Audience: audience, Enabled: true, IsSystem: true, SortOrder: sortOrder,
		CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := st.InsertDocumentationPage(ctx, row); err != nil {
		return fmt.Errorf("insert seed page %q/%s: %w", id, lang, err)
	}
	return nil
}
