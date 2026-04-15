package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pvmss/database"
)

// ── VMLimits ─────────────────────────────────────────────────────────────────

func TestVMLimits_DefaultsOnFreshDB(t *testing.T) {
	db := openTestDB(t)
	lim, err := db.GetVMLimits()
	require.NoError(t, err)
	assert.Equal(t, 10, lim.MaxVMs)
	assert.Equal(t, 2, lim.MaxVMPerUser)
	assert.Equal(t, 3, lim.MaxSnapshots)
}

func TestVMLimits_SetAndGet(t *testing.T) {
	db := openTestDB(t)
	want := &database.VMLimits{
		MaxVMs:          20,
		MaxVMPerUser:    5,
		MaxNetworkCards: 4,
		MaxDiskPerVM:    8,
		AllowCustomYAML: true,
		MaxSnapshots:    10,
	}
	require.NoError(t, db.SetVMLimits(want, "admin"))

	got, err := db.GetVMLimits()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestVMLimits_SetCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 5}, "alice"))

	entries, err := db.ListAuditLog("vm_limits", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].ChangedBy)
	assert.Equal(t, "create", entries[0].Action) // first write is a create
}

func TestVMLimits_SecondSetAuditActionIsUpdate(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 5}, "admin"))
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 10}, "admin"))

	entries, err := db.ListAuditLog("vm_limits", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "update", entries[0].Action) // most recent first
	assert.Equal(t, "create", entries[1].Action)
}

func TestVMLimits_Overwrite(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 1}, "admin"))
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 99}, "admin"))

	got, err := db.GetVMLimits()
	require.NoError(t, err)
	assert.Equal(t, 99, got.MaxVMs)
}

// ── NodeLimits ────────────────────────────────────────────────────────────────

func TestNodeLimits_EmptyOnFreshDB(t *testing.T) {
	db := openTestDB(t)
	m, err := db.GetNodeLimits()
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestNodeLimits_SetAndGet(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetNodeLimit("pve1", 10, "admin"))
	require.NoError(t, db.SetNodeLimit("pve2", 5, "admin"))

	m, err := db.GetNodeLimits()
	require.NoError(t, err)
	assert.Equal(t, 10, m["pve1"])
	assert.Equal(t, 5, m["pve2"])
}

func TestNodeLimits_Update(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetNodeLimit("pve1", 10, "admin"))
	require.NoError(t, db.SetNodeLimit("pve1", 20, "admin"))

	m, err := db.GetNodeLimits()
	require.NoError(t, err)
	assert.Equal(t, 20, m["pve1"])
}

func TestNodeLimits_Delete(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetNodeLimit("pve1", 10, "admin"))
	require.NoError(t, db.DeleteNodeLimit("pve1", "admin"))

	m, err := db.GetNodeLimits()
	require.NoError(t, err)
	assert.NotContains(t, m, "pve1")
}

func TestNodeLimits_DeleteNonExistent_ReturnsErrNotFound(t *testing.T) {
	db := openTestDB(t)
	err := db.DeleteNodeLimit("ghost", "admin")
	assert.ErrorIs(t, err, database.ErrNotFound)
}

func TestNodeLimits_SetCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetNodeLimit("pve1", 3, "bob"))

	entries, err := db.ListAuditLog("node_limits", 5, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "bob", entries[0].ChangedBy)
}

// ── SFTPConfig ────────────────────────────────────────────────────────────────

func TestSFTPConfig_DefaultsOnFreshDB(t *testing.T) {
	db := openTestDB(t)
	cfg, err := db.GetSFTPConfig()
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, 22, cfg.Port)
}

func TestSFTPConfig_SetAndGet(t *testing.T) {
	db := openTestDB(t)
	want := &database.SFTPConfig{
		Enabled:        true,
		Host:           "pve.example.com",
		Port:           2222,
		Username:       "snippets",
		PrivateKeyPath: "/keys/id_ed25519",
		RemotePath:     "/var/lib/vz/snippets",
	}
	require.NoError(t, db.SetSFTPConfig(want, "admin"))

	got, err := db.GetSFTPConfig()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSFTPConfig_SetCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetSFTPConfig(&database.SFTPConfig{Port: 22}, "carol"))

	entries, err := db.ListAuditLog("sftp_config", 5, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "carol", entries[0].ChangedBy)
	assert.Equal(t, "create", entries[0].Action) // first write is a create
}

func TestSFTPConfig_SecondSetAuditActionIsUpdate(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetSFTPConfig(&database.SFTPConfig{Port: 22}, "admin"))
	require.NoError(t, db.SetSFTPConfig(&database.SFTPConfig{Port: 2222}, "admin"))

	entries, err := db.ListAuditLog("sftp_config", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "update", entries[0].Action) // most recent first
	assert.Equal(t, "create", entries[1].Action)
}

// ── LoadAppSettings ───────────────────────────────────────────────────────────

func TestLoadAppSettings_FreshDB(t *testing.T) {
	db := openTestDB(t)
	s, err := db.LoadAppSettings()
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, 10, s.Limits.MaxVMs)
	assert.Empty(t, s.EnabledNodes)
	assert.Empty(t, s.CloudInitTemplates)
	assert.Empty(t, s.VMProfiles)
}

func TestLoadAppSettings_ReflectsWrites(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetEnabledNodes([]string{"n1", "n2"}, "admin"))
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 77}, "admin"))

	s, err := db.LoadAppSettings()
	require.NoError(t, err)
	assert.Equal(t, 77, s.Limits.MaxVMs)
	assert.Equal(t, []string{"n1", "n2"}, s.EnabledNodes)
}
