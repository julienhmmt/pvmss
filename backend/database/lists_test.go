package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── enabled_nodes ─────────────────────────────────────────────────────────────

func TestEnabledNodes_EmptyOnFreshDB(t *testing.T) {
	db := openTestDB(t)
	nodes, err := db.GetEnabledNodes()
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestEnabledNodes_SetAndGet(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetEnabledNodes([]string{"pve1", "pve2"}, "admin"))

	got, err := db.GetEnabledNodes()
	require.NoError(t, err)
	assert.Equal(t, []string{"pve1", "pve2"}, got)
}

func TestEnabledNodes_Replace(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetEnabledNodes([]string{"pve1", "pve2"}, "admin"))
	require.NoError(t, db.SetEnabledNodes([]string{"pve3"}, "admin"))

	got, err := db.GetEnabledNodes()
	require.NoError(t, err)
	assert.Equal(t, []string{"pve3"}, got)
}

func TestEnabledNodes_SetEmpty_ClearsAll(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetEnabledNodes([]string{"pve1"}, "admin"))
	require.NoError(t, db.SetEnabledNodes([]string{}, "admin"))

	got, err := db.GetEnabledNodes()
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestEnabledNodes_SetCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetEnabledNodes([]string{"pve1"}, "admin"))

	entries, err := db.ListAuditLog("enabled_nodes", 5, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "update", entries[0].Action)
}

// ── enabled_storages ──────────────────────────────────────────────────────────

func TestEnabledStorages_SetAndGet(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetEnabledStorages([]string{"local", "ceph"}, "admin"))

	got, err := db.GetEnabledStorages()
	require.NoError(t, err)
	assert.Equal(t, []string{"ceph", "local"}, got) // sorted
}

func TestEnabledStorages_Replace(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetEnabledStorages([]string{"local"}, "admin"))
	require.NoError(t, db.SetEnabledStorages([]string{"zfs"}, "admin"))

	got, err := db.GetEnabledStorages()
	require.NoError(t, err)
	assert.Equal(t, []string{"zfs"}, got)
}

// ── enabled_isos ──────────────────────────────────────────────────────────────

func TestEnabledISOs_SetAndGet(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetEnabledISOs([]string{"ubuntu.iso", "debian.iso"}, "admin"))

	got, err := db.GetEnabledISOs()
	require.NoError(t, err)
	assert.Equal(t, []string{"debian.iso", "ubuntu.iso"}, got) // sorted
}

// ── enabled_vmbrs ─────────────────────────────────────────────────────────────

func TestEnabledVMBRs_SetAndGet(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetEnabledVMBRs([]string{"vmbr0", "vmbr1"}, "admin"))

	got, err := db.GetEnabledVMBRs()
	require.NoError(t, err)
	assert.Equal(t, []string{"vmbr0", "vmbr1"}, got)
}

// ── tags ──────────────────────────────────────────────────────────────────────

func TestTags_SetAndGet(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetTags([]string{"pvmss", "prod"}, "admin"))

	got, err := db.GetTags()
	require.NoError(t, err)
	assert.Equal(t, []string{"prod", "pvmss"}, got) // sorted
}

func TestTags_Replace(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetTags([]string{"a", "b"}, "admin"))
	require.NoError(t, db.SetTags([]string{"c"}, "admin"))

	got, err := db.GetTags()
	require.NoError(t, err)
	assert.Equal(t, []string{"c"}, got)
}

func TestTags_SetCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetTags([]string{"pvmss"}, "dave"))

	entries, err := db.ListAuditLog("tags", 5, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "dave", entries[0].ChangedBy)
}
