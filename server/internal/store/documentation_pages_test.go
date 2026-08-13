//nolint:wsl_v5 // test scaffolding keeps setup and assertions adjacent
package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
)

const adminDocID = "admin"

func openDocsStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "docs.db"),
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

//nolint:gocyclo,paralleltest // round trip owns shared SQLite fixture and covers each boundary
func TestDocumentationPages_RoundTripAndStates(t *testing.T) {
	ctx := context.Background()
	st := openDocsStore(t)

	all, err := st.DocumentationPagesAll(ctx)
	if err != nil {
		t.Fatalf("all: %v", err)
	}

	if len(all) != 0 {
		t.Fatalf("fresh store has %d pages, want 0", len(all))
	}

	stamp := "2026-01-01T00:00:00Z"
	page := store.DocumentationPageRow{
		ID: "getting-started", Lang: "en", Title: "Getting started", Category: "Intro",
		BodyMD: "# Hello", Audience: "user", Enabled: true, IsSystem: false, SortOrder: 1,
		CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := st.InsertDocumentationPage(ctx, page); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Duplicate (id, lang) → ErrDuplicate.
	if err := st.InsertDocumentationPage(ctx, page); !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("duplicate insert err = %v, want ErrDuplicate", err)
	}

	// A second language for the same id coexists.
	page.Lang = "fr"
	page.Title = "Guide de depart"
	if err := st.InsertDocumentationPage(ctx, page); err != nil {
		t.Fatalf("insert fr: %v", err)
	}

	got, err := st.GetDocumentationPage(ctx, "getting-started", "en")
	if err != nil {
		t.Fatalf("get en: %v", err)
	}

	if got.Title != "Getting started" || got.Audience != "user" || !got.Enabled {
		t.Fatalf("en row = %+v", got)
	}

	_, err = st.GetDocumentationPage(ctx, "getting-started", "de")
	if err == nil {
		t.Fatal("expected error for missing de row, got nil")
	}

	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get de err = %v, want sql.ErrNoRows", err)
	}

	// Enabled-only reader filters disabled rows.
	if err := st.SetDocumentationPageEnabled(ctx, "getting-started", "en", false, stamp); err != nil {
		t.Fatalf("disable: %v", err)
	}

	enabled, err := st.DocumentationPagesEnabled(ctx)
	if err != nil {
		t.Fatalf("enabled: %v", err)
	}

	if len(enabled) != 1 || enabled[0].Lang != "fr" {
		t.Fatalf("enabled rows = %+v, want only fr", enabled)
	}

	all, err = st.DocumentationPagesAll(ctx)
	if err != nil {
		t.Fatalf("all after disable: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("all rows = %d, want 2", len(all))
	}

	// Delete refuses system pages at the storage layer.
	sys := store.DocumentationPageRow{
		ID: adminDocID, Lang: "en", Title: "Admin", BodyMD: "# Admin", Audience: "admin",
		Enabled: true, IsSystem: true, CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := st.InsertDocumentationPage(ctx, sys); err != nil {
		t.Fatalf("insert system: %v", err)
	}

	if err := st.DeleteDocumentationPage(ctx, adminDocID, "en"); err == nil {
		t.Fatal("delete system page: expected error (no rows affected), got nil")
	} else if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("delete system err = %v, want sql.ErrNoRows", err)
	}

	if err := st.DeleteDocumentationPage(ctx, "getting-started", "en"); err != nil {
		t.Fatalf("delete non-system en: %v", err)
	}

	exists, err := st.DocumentationPageExists(ctx, "getting-started", "en")
	if err != nil {
		t.Fatalf("exists check: %v", err)
	}

	if exists {
		t.Fatal("en row should be gone after delete")
	}
}
