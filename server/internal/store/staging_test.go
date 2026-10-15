package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

// newStagingStore opens a fully-migrated Store for staging tests.
func newStagingStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "staging.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// TestImportStaging_DistinctTokensForTwoStages - two concurrent stages
// get distinct tokens.
//
//nolint:paralleltest // serial: shared staging map
func TestImportStaging_DistinctTokensForTwoStages(t *testing.T) {
	st := newStagingStore(t)
	s := st.Staging()

	tok1 := s.Stage(filepath.Join(t.TempDir(), "a.db"), store.ImportPreview{Tables: []store.TablePreview{{Name: tblCatalogNodes, RowCount: 1}}})
	tok2 := s.Stage(filepath.Join(t.TempDir(), "b.db"), store.ImportPreview{Tables: []store.TablePreview{{Name: tblCatalogTags, RowCount: 2}}})

	if tok1 == "" || tok2 == "" {
		t.Fatalf("tokens empty: %q %q", tok1, tok2)
	}

	if tok1 == tok2 {
		t.Fatalf("two stages returned the same token: %q", tok1)
	}
}

// TestImportStaging_ExpiresAfterTTL - a staged entry expires after its
// TTL and is unreachable by ConfirmImport afterward.
//
//nolint:paralleltest // serial: shared staging map
func TestImportStaging_ExpiresAfterTTL(t *testing.T) {
	st := newStagingStore(t)
	s := st.Staging()

	tok := s.Stage(filepath.Join(t.TempDir(), "expired.db"), store.ImportPreview{Tables: []store.TablePreview{{Name: tblCatalogNodes, RowCount: 1}}})

	// Force expiry by advancing the staging clock past the TTL.
	s.AdvanceTime(6 * time.Minute)

	entry, err := s.Lookup(tok)
	if err == nil {
		t.Fatalf("Lookup after expiry returned %+v, want error", entry)
	}

	if !store.IsExpired(err) {
		t.Errorf("error after TTL = %v, want an ExpiredError", err)
	}
}

// An expired upload is a full database copy: expiry must remove the file,
// both when the token is looked up and when a later Stage purges it.
//
//nolint:paralleltest // serial: shared staging map
func TestImportStaging_ExpiryRemovesTempFile(t *testing.T) {
	st := newStagingStore(t)
	s := st.Staging()

	looked := filepath.Join(t.TempDir(), "looked.db")
	purged := filepath.Join(t.TempDir(), "purged.db")

	for _, path := range []string{looked, purged} {
		if err := os.WriteFile(path, []byte("db"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	lookedToken := s.Stage(looked, store.ImportPreview{})
	s.Stage(purged, store.ImportPreview{})
	s.AdvanceTime(6 * time.Minute)

	if _, err := s.Lookup(lookedToken); !store.IsExpired(err) {
		t.Fatalf("Lookup err = %v, want expired", err)
	}

	s.Stage(filepath.Join(t.TempDir(), "fresh.db"), store.ImportPreview{})

	for _, path := range []string{looked, purged} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still on disk after expiry (stat err = %v)", filepath.Base(path), err)
		}
	}
}

// TestImportStaging_UnknownTokenReturnsNotFound - a token that was
// never staged returns the not-found sentinel.
//
//nolint:paralleltest // serial: shared staging map
func TestImportStaging_UnknownTokenReturnsNotFound(t *testing.T) {
	st := newStagingStore(t)
	s := st.Staging()

	_, err := s.Lookup("never-staged")
	if err == nil {
		t.Fatal("Lookup unknown token returned nil error")
	}

	if !store.IsNotFound(err) {
		t.Errorf("error for unknown token = %v, want a NotFoundError", err)
	}
}

// TestImportStaging_RemoveDeletesEntry - removing a token makes it
// unreachable as not-found (not expired).
//
//nolint:paralleltest // serial: shared staging map
func TestImportStaging_RemoveDeletesEntry(t *testing.T) {
	st := newStagingStore(t)
	s := st.Staging()

	tok := s.Stage(filepath.Join(t.TempDir(), "removable.db"), store.ImportPreview{Tables: []store.TablePreview{{Name: tblCatalogNodes, RowCount: 1}}})
	s.Remove(tok)

	_, err := s.Lookup(tok)
	if err == nil {
		t.Fatal("Lookup after Remove returned nil error")
	}

	if !store.IsNotFound(err) {
		t.Errorf("error after Remove = %v, want NotFoundError", err)
	}
}

// TestImportStaging_LookupReturnsPreview - a valid token returns the staged
// preview and temp path.
//
//nolint:paralleltest // serial: shared staging map
func TestImportStaging_LookupReturnsPreview(t *testing.T) {
	st := newStagingStore(t)
	s := st.Staging()

	wantPath := filepath.Join(t.TempDir(), "valid.db")
	wantPreview := store.ImportPreview{Tables: []store.TablePreview{{Name: tblCatalogNodes, RowCount: 3}}}
	tok := s.Stage(wantPath, wantPreview)

	entry, err := s.Lookup(tok)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	if entry.TempPath != wantPath {
		t.Errorf("temp path = %q, want %q", entry.TempPath, wantPath)
	}

	if len(entry.Preview.Tables) != 1 || entry.Preview.Tables[0].Name != tblCatalogNodes {
		t.Errorf("preview = %+v, want catalog_nodes", entry.Preview)
	}
}

// ensure Staging is exercised via the Store facade too - a smoke test that
// the wiring is in place.
//
//nolint:paralleltest // serial: shared staging map
func TestStore_StagingFacadeIsWired(t *testing.T) {
	st := newStagingStore(t)
	if st.Staging() == nil {
		t.Fatal("Store.Staging() returned nil")
	}

	_ = context.Background()
}
