package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pvmss/database"
)

func newProfile(id string) *database.VMProfile {
	return &database.VMProfile{
		ID:          id,
		Name:        "Profile " + id,
		Description: "a test profile",
		Config:      `{"cores":2,"memory":4096,"disk_gb":24}`,
		Enabled:     true,
	}
}

func TestVMProfile_CreateAndList(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateVMProfile(newProfile("p1"), "admin"))
	require.NoError(t, db.CreateVMProfile(newProfile("p2"), "admin"))

	list, err := db.ListVMProfiles()
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestVMProfile_GetByID(t *testing.T) {
	db := openTestDB(t)
	want := newProfile("g1")
	require.NoError(t, db.CreateVMProfile(want, "admin"))

	got, err := db.GetVMProfile("g1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.Name, got.Name)
	assert.Equal(t, want.Config, got.Config)
	assert.True(t, got.Enabled)
}

func TestVMProfile_GetByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetVMProfile("missing")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestVMProfile_Update(t *testing.T) {
	db := openTestDB(t)
	p := newProfile("u1")
	require.NoError(t, db.CreateVMProfile(p, "admin"))

	p.Name = "Renamed"
	p.Enabled = false
	require.NoError(t, db.UpdateVMProfile(p, "admin"))

	got, err := db.GetVMProfile("u1")
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Name)
	assert.False(t, got.Enabled)
}

func TestVMProfile_Delete(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateVMProfile(newProfile("d1"), "admin"))
	require.NoError(t, db.DeleteVMProfile("d1", "admin"))

	got, err := db.GetVMProfile("d1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestVMProfile_CreateCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateVMProfile(newProfile("a1"), "frank"))

	entries, err := db.ListAuditLog("vm_profiles", 5, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "create", entries[0].Action)
	assert.Equal(t, "frank", entries[0].ChangedBy)
}

func TestVMProfile_UpdateCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	p := newProfile("b1")
	require.NoError(t, db.CreateVMProfile(p, "admin"))
	p.Name = "Updated"
	require.NoError(t, db.UpdateVMProfile(p, "admin"))

	entries, err := db.ListAuditLog("vm_profiles", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)
}

func TestVMProfile_CreateDuplicate_ReturnsError(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateVMProfile(newProfile("dup"), "admin"))
	err := db.CreateVMProfile(newProfile("dup"), "admin")
	assert.Error(t, err)
}

func TestVMProfile_DeleteCreatesAuditEntry(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.CreateVMProfile(newProfile("c1"), "admin"))
	require.NoError(t, db.DeleteVMProfile("c1", "admin"))

	entries, err := db.ListAuditLog("vm_profiles", 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "delete", entries[0].Action)
}

func TestVMProfile_UpdateNonExistent_ReturnsErrNotFound(t *testing.T) {
	db := openTestDB(t)
	err := db.UpdateVMProfile(newProfile("ghost"), "admin")
	assert.ErrorIs(t, err, database.ErrNotFound)
}

func TestVMProfile_DeleteNonExistent_ReturnsErrNotFound(t *testing.T) {
	db := openTestDB(t)
	err := db.DeleteVMProfile("ghost", "admin")
	assert.ErrorIs(t, err, database.ErrNotFound)
}
