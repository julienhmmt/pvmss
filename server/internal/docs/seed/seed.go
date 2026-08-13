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
// user docs are public, the admin doc is admin-only.
var builtInPages = []seedPage{
	{id: "getting-started", title: "Getting started", category: "Getting started", bodyMD: gettingStartedMD, audience: "user", sortOrder: 1},
	{id: "vm-creation-guidelines", title: "VM creation guidelines", category: "Creating VMs", bodyMD: vmCreationGuidelinesMD, audience: "user", sortOrder: 2},
	{id: "cloud-init-howto", title: "Cloud-init how-to", category: "Creating VMs", bodyMD: cloudInitHowtoMD, audience: "user", sortOrder: 3},
	{id: "admin", title: "Admin guide", category: "Administration", bodyMD: adminMD, audience: "admin", sortOrder: 100},
}

// SeedDocumentationPages inserts every built-in page that does not already
// exist (by (id, lang)). Existing rows — including admin-edited system pages —
// are left untouched, so the seed is safe to run on every startup.
//
//nolint:revive // the Seed verb documents the seeding action explicitly
func SeedDocumentationPages(ctx context.Context, st *store.Store) error {
	stamp := time.Now().UTC().Format(time.RFC3339Nano)

	for _, p := range builtInPages {
		exists, err := st.DocumentationPageExists(ctx, p.id, "en")
		if err != nil {
			return fmt.Errorf("check seed page %q: %w", p.id, err)
		}

		if exists {
			continue
		}

		row := store.DocumentationPageRow{
			ID: p.id, Lang: "en", Title: p.title, Category: p.category, BodyMD: p.bodyMD,
			Audience: p.audience, Enabled: true, IsSystem: true, SortOrder: p.sortOrder,
			CreatedAt: stamp, UpdatedAt: stamp,
		}
		if err := st.InsertDocumentationPage(ctx, row); err != nil {
			return fmt.Errorf("insert seed page %q: %w", p.id, err)
		}
	}

	return nil
}
