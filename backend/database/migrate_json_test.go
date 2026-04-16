package database_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pvmss/database"
)

// ── Fixtures ──────────────────────────────────────────────────────────────────

func fullJSONSettings() *database.JSONSettings {
	return &database.JSONSettings{
		EnabledNodes:    []string{"pve1", "pve2"},
		EnabledStorages: []string{"local-lvm", "ceph-pool"},
		ISOs:            []string{"debian-12.iso", "ubuntu-24.04.iso"},
		VMBRs:           []string{"vmbr0", "vmbr1"},
		Tags:            []string{"pvmss", "dev", "prod"},
		Limits: database.JSONLimitsConfig{
			MaxSnapshots: 5,
		},
		MaxNetworkCards: 2,
		MaxDiskPerVM:    4,
		MaxVMPerUser:    3,
		AllowCustomYAML: true,
		CloudInitTemplates: []database.JSONCloudInitTemplate{
			{
				ID:          "debian-base",
				Name:        "Debian Base",
				Description: "Minimal Debian 12",
				Storage:     "local",
				Filename:    "pvmss-debian-base.yml",
				YAMLContent: "#cloud-config\npackages: [curl]",
				Enabled:     true,
			},
		},
		VMProfiles: []database.JSONVMProfileConfig{
			{
				ID:          "web-server",
				Name:        "Web Server",
				Description: "Lightweight web host",
				Sockets:     1,
				Cores:       2,
				RAMGB:       2,
				DiskGB:      24,
				DiskBus:     "virtio",
				Icon:        "Globe",
				Color:       "blue",
				Enabled:     true,
			},
		},
		CloudInitSFTP: database.JSONSFTPConfig{
			Enabled:        true,
			Host:           "192.168.1.10",
			Port:           22,
			Username:       "pvmss-snippets",
			PrivateKeyPath: "/app/key",
			SnippetBaseDir: "/var/lib/vz/snippets",
		},
	}
}

// ── ReadJSONSettings ──────────────────────────────────────────────────────────

func TestReadJSONSettings_ValidFile(t *testing.T) {
	settings := fullJSONSettings()
	data, err := json.MarshalIndent(settings, "", "  ")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(path, data, 0600))

	got, err := database.ReadJSONSettings(path)
	require.NoError(t, err)
	assert.Equal(t, settings.EnabledNodes, got.EnabledNodes)
	assert.Equal(t, settings.Tags, got.Tags)
	assert.Equal(t, settings.Limits.MaxSnapshots, got.Limits.MaxSnapshots)
	assert.Equal(t, settings.CloudInitSFTP.SnippetBaseDir, got.CloudInitSFTP.SnippetBaseDir)
}

func TestReadJSONSettings_MissingFile_ReturnsError(t *testing.T) {
	_, err := database.ReadJSONSettings("/nonexistent/settings.json")
	assert.Error(t, err)
}

func TestReadJSONSettings_InvalidJSON_ReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json"), 0600))

	_, err := database.ReadJSONSettings(path)
	assert.Error(t, err)
}

// ── MigrateFromJSON – happy path ──────────────────────────────────────────────

func TestMigrateFromJSON_FullSettings_SummaryCountsMatch(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()

	summary, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	assert.Equal(t, 2, summary.NodesCount)
	assert.Equal(t, 2, summary.StoragesCount)
	assert.Equal(t, 2, summary.ISOsCount)
	assert.Equal(t, 2, summary.VMBRsCount)
	assert.Equal(t, 3, summary.TagsCount)
	assert.Equal(t, 1, summary.CloudInitCount)
	assert.Equal(t, 1, summary.VMProfilesCount)
}

func TestMigrateFromJSON_DataReadableAfterMigration(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()
	_, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	nodes, err := db.GetEnabledNodes()
	require.NoError(t, err)
	assert.ElementsMatch(t, src.EnabledNodes, nodes)

	storages, err := db.GetEnabledStorages()
	require.NoError(t, err)
	assert.ElementsMatch(t, src.EnabledStorages, storages)

	isos, err := db.GetEnabledISOs()
	require.NoError(t, err)
	assert.ElementsMatch(t, src.ISOs, isos)

	vmbrs, err := db.GetEnabledVMBRs()
	require.NoError(t, err)
	assert.ElementsMatch(t, src.VMBRs, vmbrs)

	tags, err := db.GetTags()
	require.NoError(t, err)
	assert.ElementsMatch(t, src.Tags, tags)
}

func TestMigrateFromJSON_VMLimitsReadableAfterMigration(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()
	_, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	lim, err := db.GetVMLimits()
	require.NoError(t, err)
	assert.Equal(t, src.MaxVMPerUser, lim.MaxVMPerUser)
	assert.Equal(t, src.MaxNetworkCards, lim.MaxNetworkCards)
	assert.Equal(t, src.MaxDiskPerVM, lim.MaxDiskPerVM)
	assert.Equal(t, src.AllowCustomYAML, lim.AllowCustomYAML)
	assert.Equal(t, src.Limits.MaxSnapshots, lim.MaxSnapshots)
}

func TestMigrateFromJSON_CloudInitTemplatesReadableAfterMigration(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()
	_, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	templates, err := db.ListCloudInitTemplates()
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, src.CloudInitTemplates[0].ID, templates[0].ID)
	assert.Equal(t, src.CloudInitTemplates[0].YAMLContent, templates[0].YAMLContent)
	assert.Equal(t, src.CloudInitTemplates[0].Enabled, templates[0].Enabled)
}

func TestMigrateFromJSON_VMProfilesReadableAfterMigration(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()
	_, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	profiles, err := db.ListVMProfiles()
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	assert.Equal(t, src.VMProfiles[0].ID, profiles[0].ID)
	assert.Equal(t, src.VMProfiles[0].Name, profiles[0].Name)
	assert.True(t, profiles[0].Enabled)

	var cfg map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(profiles[0].Config), &cfg))
	assert.Equal(t, "virtio", cfg["disk_bus"])
	assert.Equal(t, "Globe", cfg["icon"])
}

func TestMigrateFromJSON_SFTPConfigReadableAfterMigration(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()
	_, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	sftp, err := db.GetSFTPConfig()
	require.NoError(t, err)
	assert.True(t, sftp.Enabled)
	assert.Equal(t, src.CloudInitSFTP.Host, sftp.Host)
	assert.Equal(t, src.CloudInitSFTP.Port, sftp.Port)
	assert.Equal(t, src.CloudInitSFTP.SnippetBaseDir, sftp.RemotePath)
}

// ── Bootstrap ─────────────────────────────────────────────────────────────────

func TestMigrateFromJSON_MarksBootstrapComplete(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()

	before, err := db.IsBootstrapComplete()
	require.NoError(t, err)
	assert.False(t, before)

	_, err = database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	after, err := db.IsBootstrapComplete()
	require.NoError(t, err)
	assert.True(t, after)
}

// ── Audit log ────────────────────────────────────────────────────────────────

func TestMigrateFromJSON_AppendsAuditEntry(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()

	_, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	entries, err := db.ListAuditLog("migration", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "migration:settings.json", entries[0].ChangedBy)
	assert.Equal(t, "create", entries[0].Action)
	assert.Equal(t, "settings.json", entries[0].RecordID)
}

// ── Rollback on error ────────────────────────────────────────────────────────

func TestMigrateFromJSON_DuplicateCloudInitID_RollsBack(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()
	// Two templates with the same ID triggers UNIQUE constraint violation on 2nd INSERT.
	dup := src.CloudInitTemplates[0]
	src.CloudInitTemplates = append(src.CloudInitTemplates, dup)

	_, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.Error(t, err)

	// After rollback db should have zero templates and zero nodes.
	templates, listErr := db.ListCloudInitTemplates()
	require.NoError(t, listErr)
	assert.Empty(t, templates, "transaction must have been rolled back")

	nodes, listErr := db.GetEnabledNodes()
	require.NoError(t, listErr)
	assert.Empty(t, nodes, "transaction must have been rolled back")
}

func TestMigrateFromJSON_DuplicateVMProfileID_RollsBack(t *testing.T) {
	db := openTestDB(t)
	src := fullJSONSettings()
	dup := src.VMProfiles[0]
	src.VMProfiles = append(src.VMProfiles, dup)

	_, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.Error(t, err)

	profiles, listErr := db.ListVMProfiles()
	require.NoError(t, listErr)
	assert.Empty(t, profiles)
}

// ── Edge cases ────────────────────────────────────────────────────────────────

func TestMigrateFromJSON_NilSlices_Succeed(t *testing.T) {
	db := openTestDB(t)
	src := &database.JSONSettings{
		EnabledNodes:    nil,
		EnabledStorages: nil,
		ISOs:            nil,
		VMBRs:           nil,
		Tags:            nil,
	}

	summary, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)
	assert.Equal(t, 0, summary.NodesCount)
	assert.Equal(t, 0, summary.StoragesCount)
	assert.Equal(t, 0, summary.ISOsCount)
	assert.Equal(t, 0, summary.VMBRsCount)
	assert.Equal(t, 0, summary.TagsCount)
}

func TestMigrateFromJSON_EmptyArrays_Succeed(t *testing.T) {
	db := openTestDB(t)
	src := &database.JSONSettings{
		EnabledNodes:       []string{},
		EnabledStorages:    []string{},
		ISOs:               []string{},
		VMBRs:              []string{},
		Tags:               []string{},
		CloudInitTemplates: []database.JSONCloudInitTemplate{},
		VMProfiles:         []database.JSONVMProfileConfig{},
	}

	summary, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)
	assert.Equal(t, 0, summary.CloudInitCount)
	assert.Equal(t, 0, summary.VMProfilesCount)
}

func TestMigrateFromJSON_ZeroLimits_DefaultsPreserved(t *testing.T) {
	db := openTestDB(t)
	src := &database.JSONSettings{} // all zero values

	_, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	lim, err := db.GetVMLimits()
	require.NoError(t, err)
	// MaxVMs uses the hard-coded default (0 = no global cap).
	assert.Equal(t, 0, lim.MaxVMs)
}

func TestMigrateFromJSON_EmptyStringItemsSkipped(t *testing.T) {
	db := openTestDB(t)
	src := &database.JSONSettings{
		// One real node, one empty string that should be skipped.
		EnabledNodes: []string{"pve1", ""},
	}

	summary, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)
	// Empty string was skipped so only 1 node inserted.
	assert.Equal(t, 1, summary.NodesCount)

	nodes, err := db.GetEnabledNodes()
	require.NoError(t, err)
	assert.Equal(t, []string{"pve1"}, nodes)
}

// ── Real settings.json fixture ────────────────────────────────────────────────

// TestMigrateFromJSON_RealSettingsFixture exercises the full migration with a
// settings.json fixture that matches the project's example.env defaults.
func TestMigrateFromJSON_RealSettingsFixture(t *testing.T) {
	fixture := `{
		"enabled_nodes": ["pve"],
		"enabled_storages": ["local-lvm"],
		"isos": ["debian-12.0.0-amd64-netinst.iso"],
		"vmbrs": ["vmbr0"],
		"tags": ["pvmss"],
		"limits": {
			"max_snapshots": 8,
			"vm": {
				"sockets": {"min": 1, "max": 1},
				"cores":   {"min": 1, "max": 2},
				"ram":     {"min": 1, "max": 4},
				"disk":    {"min": 1, "max": 10}
			}
		},
		"max_network_cards": 1,
		"max_disk_per_vm": 4,
		"max_vm_per_user": 5,
		"allow_custom_yaml": true,
		"cloudinit_sftp": {
			"enabled": false,
			"host": "",
			"port": 22,
			"username": "pvmss-snippets",
			"private_key_path": "/app/pvmss_snippets_ed25519",
			"snippet_base_dir": "/var/lib/vz/snippets"
		}
	}`

	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(fixture), 0600))

	src, err := database.ReadJSONSettings(path)
	require.NoError(t, err)

	db := openTestDB(t)
	summary, err := database.MigrateFromJSON(db, src, "migration:settings.json")
	require.NoError(t, err)

	assert.Equal(t, 1, summary.NodesCount)
	assert.Equal(t, 1, summary.StoragesCount)
	assert.Equal(t, 1, summary.ISOsCount)
	assert.Equal(t, 1, summary.VMBRsCount)
	assert.Equal(t, 1, summary.TagsCount)

	lim, err := db.GetVMLimits()
	require.NoError(t, err)
	assert.Equal(t, 8, lim.MaxSnapshots)
	assert.Equal(t, 5, lim.MaxVMPerUser)

	complete, err := db.IsBootstrapComplete()
	require.NoError(t, err)
	assert.True(t, complete)
}
