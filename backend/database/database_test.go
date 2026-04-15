package database_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pvmss/database"
)

// openTestDB opens a fresh in-memory DB and registers cleanup.
func openTestDB(t *testing.T) database.DB {
	t.Helper()
	db, err := database.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpen_InMemory(t *testing.T) {
	db, err := database.OpenMemory()
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, db.Close())
}

func TestOpen_FileDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(path)
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, db.Close())
}

func TestOpen_ReopensExistingDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	db, err := database.Open(path)
	require.NoError(t, err)
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 42}, "admin"))
	require.NoError(t, db.Close())

	db2, err := database.Open(path)
	require.NoError(t, err)
	defer func() { _ = db2.Close() }()
	lim, err := db2.GetVMLimits()
	require.NoError(t, err)
	assert.Equal(t, 42, lim.MaxVMs)
}

func TestClose_Idempotent(t *testing.T) {
	db, err := database.OpenMemory()
	require.NoError(t, err)
	assert.NoError(t, db.Close())
}

func TestBackup(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "src.db")
	dstPath := filepath.Join(t.TempDir(), "dst.db")

	db, err := database.Open(srcPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 7}, "admin"))
	require.NoError(t, db.Backup(dstPath))

	db2, err := database.Open(dstPath)
	require.NoError(t, err)
	defer func() { _ = db2.Close() }()

	lim, err := db2.GetVMLimits()
	require.NoError(t, err)
	assert.Equal(t, 7, lim.MaxVMs)
}
