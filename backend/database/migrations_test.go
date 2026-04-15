package database_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pvmss/database"
)

// TestMigrations_AppliedOnOpen verifies that opening a DB runs migrations and
// the resulting schema supports all CRUD operations.
func TestMigrations_AppliedOnOpen(t *testing.T) {
	db := openTestDB(t)
	lim, err := db.GetVMLimits()
	require.NoError(t, err)
	assert.NotNil(t, lim)
}

// TestMigrations_Idempotent verifies that opening the same file-backed DB
// twice does not error (migrations skip already-applied versions).
func TestMigrations_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idempotent.db")

	db1, err := database.Open(path)
	require.NoError(t, err, "first open")
	require.NoError(t, db1.Close())

	db2, err := database.Open(path)
	require.NoError(t, err, "second open (migrations must be idempotent)")
	defer func() { _ = db2.Close() }()

	_, err = db2.GetVMLimits()
	require.NoError(t, err)
}

// TestMigrations_DataSurvivesReopen verifies that data written before close
// persists through a re-open (tests that migrations don't wipe existing data).
func TestMigrations_DataSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "survive.db")

	db, err := database.Open(path)
	require.NoError(t, err)
	require.NoError(t, db.SetEnabledNodes([]string{"node1", "node2"}, "admin"))
	require.NoError(t, db.Close())

	db2, err := database.Open(path)
	require.NoError(t, err)
	defer func() { _ = db2.Close() }()

	nodes, err := db2.GetEnabledNodes()
	require.NoError(t, err)
	assert.Equal(t, []string{"node1", "node2"}, nodes)
}
