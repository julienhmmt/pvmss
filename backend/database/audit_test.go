package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pvmss/database"
)

func TestAudit_EmptyOnFreshDB(t *testing.T) {
	db := openTestDB(t)
	entries, err := db.ListAuditLog("", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestAudit_WritesCreateEntries(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 5}, "admin"))
	require.NoError(t, db.SetEnabledNodes([]string{"pve1"}, "admin"))

	entries, err := db.ListAuditLog("", 10, 0)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestAudit_FilterByTable(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 5}, "admin"))
	require.NoError(t, db.SetEnabledNodes([]string{"pve1"}, "admin"))

	entries, err := db.ListAuditLog("vm_limits", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "vm_limits", entries[0].TableName)
}

func TestAudit_LimitAndOffset(t *testing.T) {
	db := openTestDB(t)
	for i := range 5 {
		require.NoError(t, db.SetNodeLimit(database.NodeLimit{NodeName: "pve1", MaxVMs: i + 1}, "admin"))
	}

	page1, err := db.ListAuditLog("node_limits", 2, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := db.ListAuditLog("node_limits", 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	all, err := db.ListAuditLog("node_limits", 0, 0)
	require.NoError(t, err)
	assert.Len(t, all, 5)
}

func TestAudit_OrderedMostRecentFirst(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetNodeLimit(database.NodeLimit{NodeName: "pve1", MaxVMs: 1}, "first"))
	require.NoError(t, db.SetNodeLimit(database.NodeLimit{NodeName: "pve1", MaxVMs: 2}, "second"))

	entries, err := db.ListAuditLog("node_limits", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "second", entries[0].ChangedBy)
	assert.Equal(t, "first", entries[1].ChangedBy)
}

func TestAudit_OldAndNewValueStored(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 10}, "admin"))
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 20}, "admin"))

	entries, err := db.ListAuditLog("vm_limits", 2, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	latestEntry := entries[0]
	assert.NotEmpty(t, latestEntry.OldValue)
	assert.NotEmpty(t, latestEntry.NewValue)
	assert.Contains(t, latestEntry.NewValue, "20")
}

func TestAudit_AuditEntry_Fields(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.SetVMLimits(&database.VMLimits{MaxVMs: 5}, "testuser"))

	entries, err := db.ListAuditLog("", 1, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	e := entries[0]
	assert.Positive(t, e.ID)
	assert.Equal(t, "vm_limits", e.TableName)
	assert.Equal(t, "1", e.RecordID)
	assert.Equal(t, "create", e.Action) // first write to a singleton table is "create"
	assert.Equal(t, "testuser", e.ChangedBy)
	assert.NotEmpty(t, e.ChangedAt)
}
