//nolint:wsl_v5 // test scaffolding keeps setup and assertions adjacent
package seed_test

import (
	"context"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/docs/seed"
	"pvmss/server/internal/store"
	"testing"
)

func openSeedStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "seed-docs.db"),
		LogLevel:  "info",
		LogFormat: "json",
		LogOutput: "stdout",
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

//nolint:paralleltest // seed idempotency owns a shared SQLite fixture
func TestSeedDocumentationPages_Idempotent(t *testing.T) {
	ctx := context.Background()
	st := openSeedStore(t)

	if err := seed.SeedDocumentationPages(ctx, st); err != nil {
		t.Fatalf("first seed: %v", err)
	}

	all, err := st.DocumentationPagesAll(ctx)
	if err != nil {
		t.Fatalf("list after first seed: %v", err)
	}

	firstCount := len(all)
	if firstCount == 0 {
		t.Fatal("expected seeded pages, got 0")
	}

	// Every seeded row is a system page.
	for _, p := range all {
		if !p.IsSystem {
			t.Fatalf("seeded page %q is not is_system", p.ID)
		}
	}

	assertRecoveredBilingual(t, st)
	assertBuiltInAdminBilingual(t, st)

	// Edit one seeded page's title, then re-seed — the edit must survive.
	const id, lang = "getting-started", "en"
	edited, err := storeRow(st, id, lang)
	if err != nil {
		t.Fatalf("read seeded page: %v", err)
	}

	if err := st.UpdateSystemDocumentationPage(ctx, store.DocumentationPageUpdate{
		ID: id, Lang: lang, Title: "Edited title", Category: edited.Category, BodyMD: edited.BodyMD,
		Audience: edited.Audience, Enabled: edited.Enabled, SortOrder: edited.SortOrder, UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("edit seeded page: %v", err)
	}

	if err := seed.SeedDocumentationPages(ctx, st); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	all, err = st.DocumentationPagesAll(ctx)
	if err != nil {
		t.Fatalf("list after second seed: %v", err)
	}

	if len(all) != firstCount {
		t.Fatalf("after re-seed: %d rows, want %d (no duplicates)", len(all), firstCount)
	}

	after, err := storeRow(st, id, lang)
	if err != nil {
		t.Fatalf("read after re-seed: %v", err)
	}

	if after.Title != "Edited title" {
		t.Fatalf("re-seed clobbered edit: title = %q, want %q", after.Title, "Edited title")
	}
}

// assertRecoveredBilingual checks that the reintegrated v0.3 guides are present
// in both languages with the correct audience and a non-trivial body.
func assertRecoveredBilingual(t *testing.T, st *store.Store) {
	t.Helper()

	const audienceAdmin = "admin"

	type pageKey struct {
		id   string
		lang string
	}

	wantRecovered := map[pageKey]struct {
		lang     string
		audience string
	}{
		{"user-guide", "en"}:          {"en", "user"},
		{"user-guide", "fr"}:          {"fr", "user"},
		{"admin-guide", "en"}:         {"en", audienceAdmin},
		{"admin-guide", "fr"}:         {"fr", audienceAdmin},
		{"cloud-init-setup", "en"}:    {"en", audienceAdmin},
		{"cloud-init-setup", "fr"}:    {"fr", audienceAdmin},
		{"proxmox-permissions", "en"}: {"en", audienceAdmin},
		{"proxmox-permissions", "fr"}: {"fr", audienceAdmin},
	}

	for key, want := range wantRecovered {
		row, err := storeRow(st, key.id, key.lang)
		if err != nil {
			t.Fatalf("recovered page %q/%s missing after seed: %v", key.id, key.lang, err)
		}
		if row.Lang != want.lang {
			t.Fatalf("recovered page %q/%s lang = %q, want %q", key.id, key.lang, row.Lang, want.lang)
		}
		if row.Audience != want.audience {
			t.Fatalf("recovered page %q/%s audience = %q, want %q", key.id, key.lang, row.Audience, want.audience)
		}
		if len(row.BodyMD) < 500 {
			t.Fatalf("recovered page %q/%s body too short (%d bytes)", key.id, key.lang, len(row.BodyMD))
		}
	}
}

// assertBuiltInAdminBilingual checks that the built-in admin overview page is
// seeded in both English and French.
func assertBuiltInAdminBilingual(t *testing.T, st *store.Store) {
	t.Helper()

	const audienceAdmin = "admin"

	for _, lang := range []string{"en", "fr"} {
		row, err := storeRow(st, "admin", lang)
		if err != nil {
			t.Fatalf("built-in admin page %q missing after seed: %v", lang, err)
		}
		if row.Audience != audienceAdmin {
			t.Fatalf("built-in admin page %q audience = %q, want %q", lang, row.Audience, audienceAdmin)
		}
		if len(row.BodyMD) < 500 {
			t.Fatalf("built-in admin page %q body too short (%d bytes)", lang, len(row.BodyMD))
		}
	}
}

// storeRow reads one row via the store (helper to avoid pulling catalog here).
func storeRow(st *store.Store, id, lang string) (store.DocumentationPageRow, error) {
	return st.GetDocumentationPage(context.Background(), id, lang)
}
