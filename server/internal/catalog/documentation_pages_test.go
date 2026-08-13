//nolint:wsl_v5 // test scaffolding keeps setup and assertions adjacent
package catalog_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

const (
	testDocTitle     = "A title"
	testDocBody      = "# body"
	testAudienceUser = "user"
)

func openCatalogDocsStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "catalog-docs.db"),
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
func TestDocumentationPages_CatalogRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := openCatalogDocsStore(t)

	testDocCreateAndDuplicate(ctx, t, st)
	testDocEnFallback(ctx, t, st)
	testDocUpdate(ctx, t, st)
	testDocToggleAndDelete(ctx, t, st)
}

// testDocCreateAndDuplicate verifies slug derivation, the enabled default, the
// duplicate-slang rejection, and same-slug/different-lang coexistence.
func testDocCreateAndDuplicate(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	// Create derives the slug and inserts enabled.
	page, err := catalog.CreateDocumentationPage(ctx, st, "Getting started", "en", "Intro", "# Hello", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if page.ID != "getting-started" || !page.Enabled || page.IsSystem {
		t.Fatalf("created page = %+v", page)
	}

	// Duplicate slug for the same lang is rejected.
	if _, err := catalog.CreateDocumentationPage(ctx, st, "Getting started", "en", "", "# x", "user"); !errors.Is(err, catalog.ErrDuplicateDocumentationPage) {
		t.Fatalf("duplicate err = %v, want ErrDuplicateDocumentationPage", err)
	}

	// Same slug, different lang, coexists.
	if _, err := catalog.CreateDocumentationPage(ctx, st, "Getting started", "fr", "Intro", "# Bonjour", "user"); err != nil {
		t.Fatalf("create fr: %v", err)
	}
}

// testDocEnFallback verifies lang resolution: fr returns fr, de falls back to
// en, and an unknown id is not found.
func testDocEnFallback(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	fr, err := catalog.GetDocumentationPage(ctx, st, "getting-started", "fr")
	if err != nil || fr.Lang != "fr" || !strings.Contains(fr.BodyMD, "Bonjour") {
		t.Fatalf("get fr = %+v, err %v", fr, err)
	}

	fallback, err := catalog.GetDocumentationPage(ctx, st, "getting-started", "de")
	if err != nil || fallback.Lang != "en" {
		t.Fatalf("get de fallback = %+v, err %v", fallback, err)
	}

	// Unknown id → not found.
	if _, err := catalog.GetDocumentationPage(ctx, st, "nope", "en"); !errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		t.Fatalf("get unknown err = %v, want ErrDocumentationPageNotFound", err)
	}
}

// testDocUpdate verifies mutable-field updates and the not-found path.
func testDocUpdate(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	updated, err := catalog.UpdateDocumentationPage(ctx, st, "getting-started", "en", catalog.DocumentationPageUpdate{
		Title: "Getting Started", Category: "Intro", BodyMD: "# Hello world", Audience: "user", Enabled: true, SortOrder: 5,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Title != "Getting Started" || updated.SortOrder != 5 {
		t.Fatalf("updated = %+v", updated)
	}

	// Update not-found.
	if _, err := catalog.UpdateDocumentationPage(ctx, st, "nope", "en", catalog.DocumentationPageUpdate{
		Title: "x", BodyMD: "# x", Audience: "user", Enabled: true,
	}); !errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		t.Fatalf("update unknown err = %v, want ErrDocumentationPageNotFound", err)
	}
}

// testDocToggleAndDelete verifies enabled toggling, the enabled-only list
// filter, non-system deletion, and the delete not-found path.
func testDocToggleAndDelete(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	// Toggle flips enabled.
	if err := catalog.SetDocumentationPageEnabled(ctx, st, "getting-started", "en", false); err != nil {
		t.Fatalf("toggle: %v", err)
	}

	enabled, err := catalog.EnabledDocumentationPages(ctx, st)
	if err != nil {
		t.Fatalf("enabled list: %v", err)
	}

	for _, p := range enabled {
		if p.ID == "getting-started" && p.Lang == "en" {
			t.Fatal("disabled en page should not appear in enabled list")
		}
	}

	// Delete removes a non-system page.
	if err := catalog.DeleteDocumentationPage(ctx, st, "getting-started", "fr"); err != nil {
		t.Fatalf("delete fr: %v", err)
	}

	// Delete unknown → not found.
	if err := catalog.DeleteDocumentationPage(ctx, st, "nope", "en"); !errors.Is(err, catalog.ErrDocumentationPageNotFound) {
		t.Fatalf("delete unknown err = %v, want ErrDocumentationPageNotFound", err)
	}
}

//nolint:paralleltest // each case owns a temporary SQLite database
func TestDocumentationPages_Validation(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name     string
		title    string
		lang     string
		category string
		body     string
		audience string
	}{
		{"empty title", "  ", "en", "", testDocBody, testAudienceUser},
		{"bad lang", testDocTitle, "de", "", testDocBody, testAudienceUser},
		{"empty body", testDocTitle, "en", "", "  ", testAudienceUser},
		{"bad audience", testDocTitle, "en", "", testDocBody, "guest"},
		{"oversized title", string(repeatRune('a', 121)), "en", "", testDocBody, testAudienceUser},
		{"oversized category", testDocTitle, "en", string(repeatRune('a', 41)), testDocBody, testAudienceUser},
		{"oversized body", testDocTitle, "en", "", string(repeatRune('a', 256*1024+1)), testAudienceUser},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := openCatalogDocsStore(t)
			_, err := catalog.CreateDocumentationPage(ctx, st, tc.title, tc.lang, tc.category, tc.body, tc.audience)
			if !errors.Is(err, catalog.ErrInvalidDocumentationPage) {
				t.Fatalf("err = %v, want ErrInvalidDocumentationPage", err)
			}
		})
	}
}

//nolint:paralleltest // system-page protection owns a shared SQLite fixture
func TestDocumentationPages_SystemProtection(t *testing.T) {
	ctx := context.Background()
	st := openCatalogDocsStore(t)

	stamp := "2026-01-01T00:00:00Z"
	sys := store.DocumentationPageRow{
		ID: "admin", Lang: "en", Title: "Admin", BodyMD: "# Admin", Audience: "admin",
		Enabled: true, IsSystem: true, CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := st.InsertDocumentationPage(ctx, sys); err != nil {
		t.Fatalf("insert system: %v", err)
	}

	// Delete a system page → refused.
	if err := catalog.DeleteDocumentationPage(ctx, st, "admin", "en"); !errors.Is(err, catalog.ErrSystemDocumentationPage) {
		t.Fatalf("delete system err = %v, want ErrSystemDocumentationPage", err)
	}

	// Edit a system page → allowed (content/title change).
	edited, err := catalog.UpdateDocumentationPage(ctx, st, "admin", "en", catalog.DocumentationPageUpdate{
		Title: "Admin guide", Category: "Administration", BodyMD: "# Admin guide", Audience: "admin", Enabled: true, SortOrder: 10,
	})
	if err != nil {
		t.Fatalf("update system: %v", err)
	}

	if edited.Title != "Admin guide" || !edited.IsSystem {
		t.Fatalf("edited system page = %+v", edited)
	}
}

func repeatRune(r rune, n int) []rune {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return out
}
