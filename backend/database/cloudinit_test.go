package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pvmss/database"
)

func newTemplate(id string) *database.CloudInitTemplate {
	return &database.CloudInitTemplate{
		ID:          id,
		Name:        "Template " + id,
		Description: "desc",
		Storage:     "local",
		Filename:    id + ".yml",
		YAMLContent: "#cloud-config\npackages:\n  - curl",
		Enabled:     true,
	}
}

func TestCloudInit_CreateAndList(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateCloudInitTemplate(newTemplate("t1"), "admin"))
	require.NoError(t, db.CreateCloudInitTemplate(newTemplate("t2"), "admin"))

	list, err := db.ListCloudInitTemplates()
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestCloudInit_GetByID(t *testing.T) {
	db := openTestDB(t)
	want := newTemplate("ci1")
	require.NoError(t, db.CreateCloudInitTemplate(want, "admin"))

	got, err := db.GetCloudInitTemplate("ci1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.Name, got.Name)
	assert.Equal(t, want.YAMLContent, got.YAMLContent)
	assert.True(t, got.Enabled)
}

func TestCloudInit_GetByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetCloudInitTemplate("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCloudInit_Update(t *testing.T) {
	db := openTestDB(t)
	tpl := newTemplate("u1")
	require.NoError(t, db.CreateCloudInitTemplate(tpl, "admin"))

	tpl.Name = "Updated Name"
	tpl.Enabled = false
	require.NoError(t, db.UpdateCloudInitTemplate(tpl, "admin"))

	got, err := db.GetCloudInitTemplate("u1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", got.Name)
	assert.False(t, got.Enabled)
}

func TestCloudInit_Delete(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateCloudInitTemplate(newTemplate("d1"), "admin"))
	require.NoError(t, db.DeleteCloudInitTemplate("d1", "admin"))

	got, err := db.GetCloudInitTemplate("d1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCloudInit_CreateCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateCloudInitTemplate(newTemplate("a1"), "eve"))

	entries, err := db.ListAuditLog("cloudinit_templates", 5, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "create", entries[0].Action)
	assert.Equal(t, "eve", entries[0].ChangedBy)
}

func TestCloudInit_UpdateCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	tpl := newTemplate("b1")
	require.NoError(t, db.CreateCloudInitTemplate(tpl, "admin"))
	tpl.Name = "New"
	require.NoError(t, db.UpdateCloudInitTemplate(tpl, "admin"))

	entries, err := db.ListAuditLog("cloudinit_templates", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	actions := []string{entries[0].Action, entries[1].Action}
	assert.Contains(t, actions, "create")
	assert.Contains(t, actions, "update")
}

func TestCloudInit_CreateDuplicate_ReturnsError(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateCloudInitTemplate(newTemplate("dup"), "admin"))
	err := db.CreateCloudInitTemplate(newTemplate("dup"), "admin")
	assert.Error(t, err)
}

func TestCloudInit_DeleteCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateCloudInitTemplate(newTemplate("c1"), "admin"))
	require.NoError(t, db.DeleteCloudInitTemplate("c1", "admin"))

	entries, err := db.ListAuditLog("cloudinit_templates", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "delete", entries[0].Action) // most recent first
}

func TestCloudInit_UpdateNonExistent_ReturnsErrNotFound(t *testing.T) {
	db := openTestDB(t)
	err := db.UpdateCloudInitTemplate(newTemplate("ghost"), "admin")
	assert.ErrorIs(t, err, database.ErrNotFound)
}

func TestCloudInit_DeleteNonExistent_ReturnsErrNotFound(t *testing.T) {
	db := openTestDB(t)
	err := db.DeleteCloudInitTemplate("ghost", "admin")
	assert.ErrorIs(t, err, database.ErrNotFound)
}
