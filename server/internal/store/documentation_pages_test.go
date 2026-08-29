//nolint:wsl_v5,goconst // test scaffolding keeps setup and assertions adjacent; log level string reused across store open sites
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

const (
	adminDocID           = "admin"
	docTestStamp         = "2026-01-01T00:00:00Z"
	testDocAudienceUser  = "user"
	testDocAudienceAdmin = "admin"
)

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

//nolint:paralleltest // round trip owns a shared SQLite fixture across ordered steps
func TestDocumentationPages_RoundTripAndStates(t *testing.T) {
	ctx := context.Background()
	st := openDocsStore(t)

	testDocStoreInsertAndDuplicate(ctx, t, st)
	testDocStoreGetAndEnabledFilter(ctx, t, st)
	testDocStoreSystemDeleteGuard(ctx, t, st)
}

// testDocStoreInsertAndDuplicate verifies the empty-store start, insert,
// duplicate (id, lang) rejection, and same-id/different-lang coexistence.
func testDocStoreInsertAndDuplicate(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	all, err := st.DocumentationPagesAll(ctx)
	if err != nil {
		t.Fatalf("all: %v", err)
	}

	if len(all) != 0 {
		t.Fatalf("fresh store has %d pages, want 0", len(all))
	}

	stamp := docTestStamp
	page := store.DocumentationPageRow{
		ID: "getting-started", Lang: "en", Title: "Getting started", Category: "Intro",
		BodyMD: "# Hello", Audience: testDocAudienceUser, Enabled: true, IsSystem: false, SortOrder: 1,
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
}

// testDocStoreGetAndEnabledFilter verifies get-by-(id,lang), the missing-row
// sql.ErrNoRows path, the enabled toggle, and the enabled-only/all readers.
func testDocStoreGetAndEnabledFilter(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	got, err := st.GetDocumentationPage(ctx, "getting-started", "en")
	if err != nil {
		t.Fatalf("get en: %v", err)
	}

	if got.Title != "Getting started" || got.Audience != testDocAudienceUser || !got.Enabled {
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
	if err := st.SetDocumentationPageEnabled(ctx, "getting-started", "en", false, docTestStamp); err != nil {
		t.Fatalf("disable: %v", err)
	}

	enabled, err := st.DocumentationPagesEnabled(ctx)
	if err != nil {
		t.Fatalf("enabled: %v", err)
	}

	if len(enabled) != 1 || enabled[0].Lang != "fr" {
		t.Fatalf("enabled rows = %+v, want only fr", enabled)
	}

	all, err := st.DocumentationPagesAll(ctx)
	if err != nil {
		t.Fatalf("all after disable: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("all rows = %d, want 2", len(all))
	}
}

// testDocStoreSystemDeleteGuard verifies the storage-layer system-page delete
// guard, non-system deletion, and the exists check after delete.
func testDocStoreSystemDeleteGuard(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	stamp := docTestStamp
	sys := store.DocumentationPageRow{
		ID: adminDocID, Lang: "en", Title: "Admin", BodyMD: "# Admin", Audience: testDocAudienceAdmin,
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

// TestDocumentationPages_UpdateMutatesFields — UpdateDocumentationPage changes
// the mutable columns and returns ErrNoRows for a missing (id, lang).
//
//nolint:paralleltest // serial: shared SQLite fixture
func TestDocumentationPages_UpdateMutatesFields(t *testing.T) {
	ctx := context.Background()
	st := openDocsStore(t)

	stamp := docTestStamp
	page := store.DocumentationPageRow{
		ID: "update-me", Lang: "en", Title: "Old", BodyMD: "# Old", Audience: testDocAudienceUser,
		Enabled: true, CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := st.InsertDocumentationPage(ctx, page); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := st.UpdateDocumentationPage(ctx, store.DocumentationPageUpdate{
		ID: "update-me", Lang: "en", Title: "New", Category: "Cat", BodyMD: "# New",
		Audience: testDocAudienceAdmin, Enabled: false, SortOrder: 7, UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := st.GetDocumentationPage(ctx, "update-me", "en")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}

	if got.Title != "New" || got.Category != "Cat" || got.BodyMD != "# New" ||
		got.Audience != testDocAudienceAdmin || got.Enabled || got.SortOrder != 7 ||
		got.UpdatedAt != "2026-01-02T00:00:00Z" {
		t.Fatalf("updated row = %+v", got)
	}

	// Missing row → ErrNoRows.
	if err := st.UpdateDocumentationPage(ctx, store.DocumentationPageUpdate{
		ID: "nope", Lang: "en", Title: "x", BodyMD: "# x", Audience: testDocAudienceUser, UpdatedAt: stamp,
	}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("update missing err = %v, want sql.ErrNoRows", err)
	}
}

// TestDocumentationPages_UpdateSystemMutatesFields — UpdateSystemDocumentationPage
// only touches is_system=1 rows; a non-system row is left unchanged (ErrNoRows).
//
//nolint:paralleltest // serial: shared SQLite fixture
func TestDocumentationPages_UpdateSystemMutatesFields(t *testing.T) {
	ctx := context.Background()
	st := openDocsStore(t)

	stamp := docTestStamp
	sys := store.DocumentationPageRow{
		ID: "sys-page", Lang: "en", Title: "Sys", BodyMD: "# Sys", Audience: testDocAudienceAdmin,
		Enabled: true, IsSystem: true, CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := st.InsertDocumentationPage(ctx, sys); err != nil {
		t.Fatalf("insert system: %v", err)
	}

	if err := st.UpdateSystemDocumentationPage(ctx, store.DocumentationPageUpdate{
		ID: "sys-page", Lang: "en", Title: "Sys edited", BodyMD: "# Sys edited",
		Audience: testDocAudienceAdmin, Enabled: false, SortOrder: 2, UpdatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("update system: %v", err)
	}

	got, err := st.GetDocumentationPage(ctx, "sys-page", "en")
	if err != nil {
		t.Fatalf("get after system update: %v", err)
	}

	if got.Title != "Sys edited" || !got.IsSystem || got.Enabled {
		t.Fatalf("updated system row = %+v", got)
	}

	// UpdateSystemDocumentationPage on a non-system row → ErrNoRows (is_system=1 guard).
	plain := store.DocumentationPageRow{
		ID: "plain-page", Lang: "en", Title: "Plain", BodyMD: "# Plain", Audience: testDocAudienceUser,
		Enabled: true, IsSystem: false, CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := st.InsertDocumentationPage(ctx, plain); err != nil {
		t.Fatalf("insert plain: %v", err)
	}

	if err := st.UpdateSystemDocumentationPage(ctx, store.DocumentationPageUpdate{
		ID: "plain-page", Lang: "en", Title: "Hijack", BodyMD: "# x", Audience: testDocAudienceUser, UpdatedAt: stamp,
	}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("update system on plain err = %v, want sql.ErrNoRows", err)
	}

	// Confirm the plain row was not modified.
	unchanged, err := st.GetDocumentationPage(ctx, "plain-page", "en")
	if err != nil {
		t.Fatalf("get plain: %v", err)
	}

	if unchanged.Title != "Plain" {
		t.Fatalf("plain row title = %q, want Plain (system update must not touch it)", unchanged.Title)
	}
}
