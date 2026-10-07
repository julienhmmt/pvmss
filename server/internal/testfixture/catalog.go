package testfixture

import (
	"context"
	"pvmss/server/internal/store"
	"testing"
)

// SeedDemoFixtures writes the demo catalog fixture rows (approved
// nodes/storages/profiles, vm_limits gabarit, pvmss tag, one cloud image)
// for cluster "default". The baseline migration seeds no demo data; tests
// that exercise the admin catalog or create paths call this explicitly to
// reproduce the state demo mode ships with.
func SeedDemoFixtures(tb testing.TB, st *store.Store) {
	tb.Helper()

	if err := st.SeedDemoCatalog(context.Background()); err != nil {
		tb.Fatalf("seed demo fixtures: %v", err)
	}
}
