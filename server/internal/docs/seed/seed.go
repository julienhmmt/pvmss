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

//go:embed admin.fr.md
var adminFR string

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

// Page ids for built-in system pages. Used both as the builtInPages id field
// and as the frenchVariants switch keys, so renaming an id is a one-line change.
const (
	idGettingStarted       = "getting-started"
	idVMCreationGuidelines = "vm-creation-guidelines"
	idCloudInitHowto       = "cloud-init-howto"
	idAdmin                = "admin"
	idUserGuide            = "user-guide"
	idAdminGuide           = "admin-guide"
	idCloudInitSetup       = "cloud-init-setup"
	idProxmoxPermissions   = "proxmox-permissions"
)

// Audience values for built-in pages. user docs are public, admin docs are
// admin-only. Extracted as constants so the audience strings are not repeated
// across builtInPages and frenchVariants.
const (
	audienceUser  = "user"
	audienceAdmin = "admin"
)

// Category labels for built-in pages. Only the categories that recur across
// multiple pages are extracted; single-use categories stay inline.
const (
	categoryGettingStarted = "Getting started"
	categoryCreatingVMs    = "Creating VMs"
	categoryAdministration = "Administration"
	categoryGuides         = "Guides"
)

// seedPage describes one built-in page to insert.
type seedPage struct {
	id        string
	title     string
	category  string
	bodyMD    string
	audience  string
	sortOrder int
}

// enVariant builds the English seedVariant for this page. The EN row is stated
// once here (in builtInPages) and reused for seeding, so it is never restated
// in frenchVariants.
func (p seedPage) enVariant() seedVariant {
	return seedVariant{
		id:        p.id,
		lang:      "en",
		title:     p.title,
		category:  p.category,
		bodyMD:    p.bodyMD,
		audience:  p.audience,
		sortOrder: p.sortOrder,
	}
}

// builtInPages is the fixed set of system pages. The audience maps directly:
// user docs are public, the admin doc is admin-only. Recovered v0.3 guides
// (admin/user/cloud-init/permissions) were reintegrated and rewritten for the
// v0.4 app, then seeded as system pages so they survive restarts (issue #53).
var builtInPages = []seedPage{
	{id: idGettingStarted, title: "Getting started", category: categoryGettingStarted, bodyMD: gettingStartedMD, audience: audienceUser, sortOrder: 1},
	{id: idVMCreationGuidelines, title: "VM creation guidelines", category: categoryCreatingVMs, bodyMD: vmCreationGuidelinesMD, audience: audienceUser, sortOrder: 2},
	{id: idCloudInitHowto, title: "Cloud-init how-to", category: categoryCreatingVMs, bodyMD: cloudInitHowtoMD, audience: audienceUser, sortOrder: 3},
	{id: idAdmin, title: "Admin guide", category: categoryAdministration, bodyMD: adminMD, audience: audienceAdmin, sortOrder: 100},
	{id: idUserGuide, title: "User guide", category: categoryGuides, bodyMD: userGuideEN, audience: audienceUser, sortOrder: 4},
	{id: idAdminGuide, title: "Administrator guide", category: categoryGuides, bodyMD: adminGuideEN, audience: audienceAdmin, sortOrder: 101},
	{id: idCloudInitSetup, title: "Cloud-init setup (admin)", category: categoryGuides, bodyMD: cloudInitSetupEN, audience: audienceAdmin, sortOrder: 102},
	{id: idProxmoxPermissions, title: "Proxmox permissions for PVMSS", category: categoryGuides, bodyMD: proxmoxPermissionsEN, audience: audienceAdmin, sortOrder: 103},
}

// SeedDocumentationPages inserts every built-in page that does not already
// exist (by (id, lang)). Existing rows — including admin-edited system pages —
// are left untouched, so the seed is safe to run on every startup.
//
//nolint:revive // the Seed verb documents the seeding action explicitly
func SeedDocumentationPages(ctx context.Context, st *store.Store) error {
	stamp := time.Now().UTC().Format(time.RFC3339Nano)

	// Every page seeds its EN row from builtInPages; bilingual pages append
	// their FR variant when one exists. insertSeedRow is itself idempotent per
	// (id, lang), so re-running the seed never clobbers an existing row.
	for _, p := range builtInPages {
		if err := insertSeedRow(ctx, st, p.enVariant(), stamp); err != nil {
			return err
		}

		if fr := frenchVariants(p.id); fr != nil {
			if err := insertSeedRow(ctx, st, *fr, stamp); err != nil {
				return err
			}
		}
	}

	return nil
}

// seedVariant is one language variant of a built-in page.
type seedVariant struct {
	id        string
	lang      string
	title     string
	category  string
	bodyMD    string
	audience  string
	sortOrder int
}

// frenchVariants returns the French variant for a built-in page that ships in
// both languages, or nil if the page is English-only. The English row is not
// restated here — it comes from builtInPages via seedPage.enVariant.
//
//nolint:misspell // French titles are intentional seed data
func frenchVariants(id string) *seedVariant {
	switch id {
	case idAdmin:
		return &seedVariant{id: idAdmin, lang: "fr", title: "Guide de l'administration", category: categoryAdministration, bodyMD: adminFR, audience: audienceAdmin, sortOrder: 100}
	case idGettingStarted:
		return &seedVariant{id: idGettingStarted, lang: "fr", title: "Premiers pas", category: "Premiers pas", bodyMD: gettingStartedFR, audience: audienceUser, sortOrder: 1}
	case idVMCreationGuidelines:
		return &seedVariant{id: idVMCreationGuidelines, lang: "fr", title: "Recommandations de création de VM", category: "Création de VMs", bodyMD: vmCreationGuidelinesFR, audience: audienceUser, sortOrder: 2}
	case idCloudInitHowto:
		return &seedVariant{id: idCloudInitHowto, lang: "fr", title: "Guide cloud-init", category: "Création de VMs", bodyMD: cloudInitHowtoFR, audience: audienceUser, sortOrder: 3}
	case idUserGuide:
		return &seedVariant{id: idUserGuide, lang: "fr", title: "Guide de l'utilisateur", category: categoryGuides, bodyMD: userGuideFR, audience: audienceUser, sortOrder: 4}
	case idAdminGuide:
		return &seedVariant{id: idAdminGuide, lang: "fr", title: "Guide de l'administrateur", category: categoryGuides, bodyMD: adminGuideFR, audience: audienceAdmin, sortOrder: 101}
	case idCloudInitSetup:
		return &seedVariant{id: idCloudInitSetup, lang: "fr", title: "Configuration cloud-init (administrateur)", category: categoryGuides, bodyMD: cloudInitSetupFR, audience: audienceAdmin, sortOrder: 102}
	case idProxmoxPermissions:
		return &seedVariant{id: idProxmoxPermissions, lang: "fr", title: "Permissions Proxmox pour PVMSS", category: categoryGuides, bodyMD: proxmoxPermissionsFR, audience: audienceAdmin, sortOrder: 103}
	}

	return nil
}

// insertSeedRow idempotently inserts one (id, lang) system page.
func insertSeedRow(ctx context.Context, st *store.Store, v seedVariant, stamp string) error {
	exists, err := st.DocumentationPageExists(ctx, v.id, v.lang)
	if err != nil {
		return fmt.Errorf("check seed page %q/%s: %w", v.id, v.lang, err)
	}

	if exists {
		return nil
	}

	row := store.DocumentationPageRow{
		ID: v.id, Lang: v.lang, Title: v.title, Category: v.category, BodyMD: v.bodyMD,
		Audience: v.audience, Enabled: true, IsSystem: true, SortOrder: v.sortOrder,
		CreatedAt: stamp, UpdatedAt: stamp,
	}

	if err := st.InsertDocumentationPage(ctx, row); err != nil {
		return fmt.Errorf("insert seed page %q/%s: %w", v.id, v.lang, err)
	}

	return nil
}
