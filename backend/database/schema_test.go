package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchema_AllTablesAccessible verifies that after OpenMemory all expected
// tables are present by exercising a read operation on each one.
func TestSchema_AllTablesAccessible(t *testing.T) {
	db := openTestDB(t)

	t.Run("vm_limits", func(t *testing.T) {
		_, err := db.GetVMLimits()
		require.NoError(t, err)
	})

	t.Run("node_limits", func(t *testing.T) {
		_, err := db.GetNodeLimits()
		require.NoError(t, err)
	})

	t.Run("enabled_nodes", func(t *testing.T) {
		_, err := db.GetEnabledNodes()
		require.NoError(t, err)
	})

	t.Run("enabled_storages", func(t *testing.T) {
		_, err := db.GetEnabledStorages()
		require.NoError(t, err)
	})

	t.Run("enabled_isos", func(t *testing.T) {
		_, err := db.GetEnabledISOs()
		require.NoError(t, err)
	})

	t.Run("enabled_vmbrs", func(t *testing.T) {
		_, err := db.GetEnabledVMBRs()
		require.NoError(t, err)
	})

	t.Run("tags", func(t *testing.T) {
		_, err := db.GetTags()
		require.NoError(t, err)
	})

	t.Run("cloudinit_templates", func(t *testing.T) {
		_, err := db.ListCloudInitTemplates()
		require.NoError(t, err)
	})

	t.Run("vm_profiles", func(t *testing.T) {
		_, err := db.ListVMProfiles()
		require.NoError(t, err)
	})

	t.Run("sftp_config", func(t *testing.T) {
		_, err := db.GetSFTPConfig()
		require.NoError(t, err)
	})

	t.Run("audit_log", func(t *testing.T) {
		_, err := db.ListAuditLog("", 10, 0)
		require.NoError(t, err)
	})

	t.Run("bootstrap", func(t *testing.T) {
		done, err := db.IsBootstrapComplete()
		require.NoError(t, err)
		assert.False(t, done)
	})
}

// TestSchema_EmptyListsReturnSlices ensures read methods return non-nil empty
// slices instead of nil on a fresh database (safe for range loops).
func TestSchema_EmptyListsReturnSlices(t *testing.T) {
	db := openTestDB(t)

	tests := []struct {
		name string
		fn   func() ([]string, error)
	}{
		{"enabled_nodes", db.GetEnabledNodes},
		{"enabled_storages", db.GetEnabledStorages},
		{"enabled_isos", db.GetEnabledISOs},
		{"enabled_vmbrs", db.GetEnabledVMBRs},
		{"tags", db.GetTags},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.fn()
			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}
