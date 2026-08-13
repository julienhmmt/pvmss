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

	// Edit one seeded page's title, then re-seed — the edit must survive.
	const id, lang = "getting-started", "en"
	edited, err := storeRow(st, id, lang)
	if err != nil {
		t.Fatalf("read seeded page: %v", err)
	}

	if err := st.UpdateSystemDocumentationPage(ctx, id, lang, "Edited title", edited.Category, edited.BodyMD, edited.Audience, edited.Enabled, edited.SortOrder, "2026-01-02T00:00:00Z"); err != nil {
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

// storeRow reads one row via the store (helper to avoid pulling catalog here).
func storeRow(st *store.Store, id, lang string) (store.DocumentationPageRow, error) {
	return st.GetDocumentationPage(context.Background(), id, lang)
}
