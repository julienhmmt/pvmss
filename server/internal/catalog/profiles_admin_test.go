package catalog_test

import (
	"context"
	"errors"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"testing"
)

const (
	testProfileBus   = "scsi"
	testProfileLabel = "Test"
	testProfileID    = "small"
)

// TestDeriveProfileID checks slug derivation from labels.
func TestDeriveProfileID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		label string
		want  string
	}{
		{"Small (1 vCPU, 2 GB, 20 GB)", "small-1-vcpu-2-gb-20-gb"},
		{"X-Large (8 vCPU, 16 GB, 160 GB)", "x-large-8-vcpu-16-gb-160-gb"},
		{"My Profile", "my-profile"},
		{"  spaced  ", "spaced"},
		{"!!!", "profile"},
		{"UPPERCASE", "uppercase"},
	}
	for _, tc := range tests {
		got := catalog.DeriveProfileID(tc.label)
		if got != tc.want {
			t.Errorf("DeriveProfileID(%q) = %q, want %q", tc.label, got, tc.want)
		}
	}
}

// TestCreateProfile_SlugCollision — creating a profile whose label derives to
// an existing slug returns ErrDuplicateProfile.
func TestCreateProfile_SlugCollision(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	// "small" already exists from T06's seed.
	_, err := catalog.CreateProfile(ctx, st, "default", "Small", 1, 2048, 20, testProfileBus)
	if !errors.Is(err, catalog.ErrDuplicateProfile) {
		t.Fatalf("CreateProfile with existing slug: got %v, want ErrDuplicateProfile", err)
	}
}

// TestCreateProfile_InvalidFields — out-of-range fields return ErrInvalidProfile.
func TestCreateProfile_InvalidFields(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	tests := []struct {
		name   string
		label  string
		cores  int
		memMB  int
		diskGB int
		bus    string
	}{
		{"empty label", "", 1, 2048, 20, testProfileBus},
		{"zero cores", testProfileLabel, 0, 2048, 20, testProfileBus},
		{"low memory", testProfileLabel, 1, 64, 20, testProfileBus},
		{"zero disk", testProfileLabel, 1, 2048, 0, testProfileBus},
		{"empty bus", testProfileLabel, 1, 2048, 20, ""},
	}
	for _, tc := range tests {
		_, err := catalog.CreateProfile(ctx, st, "default", tc.label, tc.cores, tc.memMB, tc.diskGB, tc.bus)
		if !errors.Is(err, catalog.ErrInvalidProfile) {
			t.Errorf("CreateProfile(%s): got %v, want ErrInvalidProfile", tc.name, err)
		}
	}
}

// TestCreateProfile_Success — a new profile is created enabled by default and
// appears in the admin list.
func TestCreateProfile_Success(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	profile, err := catalog.CreateProfile(ctx, st, "default", "X-Large (8 vCPU, 16 GB, 160 GB)", 8, 16384, 160, testProfileBus)
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	if profile.ID != "x-large-8-vcpu-16-gb-160-gb" {
		t.Errorf("profile.ID = %q, want %q", profile.ID, "x-large-8-vcpu-16-gb-160-gb")
	}

	if !profile.Enabled {
		t.Error("new profile should be enabled by default")
	}

	list, err := catalog.ListAdminProfiles(ctx, st, "default")
	if err != nil {
		t.Fatalf("ListAdminProfiles: %v", err)
	}

	found := false

	for _, p := range list {
		if p.ID == profile.ID {
			found = true

			if !p.Enabled {
				t.Error("profile should be enabled in list")
			}
		}
	}

	if !found {
		t.Error("created profile not found in admin list")
	}
}

// TestUpdateProfile_NotFound — updating a non-existent profile returns
// ErrProfileNotFound.
func TestUpdateProfile_NotFound(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	_, err := catalog.UpdateProfile(ctx, st, "default", "nonexistent", testProfileLabel, 1, 2048, 20, testProfileBus)
	if !errors.Is(err, catalog.ErrProfileNotFound) {
		t.Fatalf("UpdateProfile nonexistent: got %v, want ErrProfileNotFound", err)
	}
}

// TestUpdateProfile_Success — updating an existing profile changes its values.
func TestUpdateProfile_Success(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	updated, err := catalog.UpdateProfile(ctx, st, "default", testProfileID, "Small (2 vCPU, 4 GB, 40 GB)", 2, 4096, 40, testProfileBus)
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	if updated.Label != "Small (2 vCPU, 4 GB, 40 GB)" {
		t.Errorf("updated.Label = %q", updated.Label)
	}

	if updated.CPUCores != 2 {
		t.Errorf("updated.CPUCores = %d, want 2", updated.CPUCores)
	}
}

// TestDeleteProfile_NotFound — deleting a non-existent profile returns
// ErrProfileNotFound.
func TestDeleteProfile_NotFound(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	err := catalog.DeleteProfile(ctx, st, "default", "nonexistent")
	if !errors.Is(err, catalog.ErrProfileNotFound) {
		t.Fatalf("DeleteProfile nonexistent: got %v, want ErrProfileNotFound", err)
	}
}

// TestDeleteProfile_Success — deleting an existing profile removes it from the
// list. No cascade (FR-011).
func TestDeleteProfile_Success(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	err := catalog.DeleteProfile(ctx, st, "default", testProfileID)
	if err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}

	list, err := catalog.ListAdminProfiles(ctx, st, "default")
	if err != nil {
		t.Fatalf("ListAdminProfiles: %v", err)
	}

	for _, p := range list {
		if p.ID == testProfileID {
			t.Error("deleted profile still in list")
		}
	}
}

// TestSetProfileEnabled_Toggle — disabling a profile excludes it from
// catalog.Profiles (T06's view) while keeping it in the admin list.
func TestSetProfileEnabled_Toggle(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	err := catalog.SetProfileEnabled(ctx, st, "default", testProfileID, false)
	if err != nil {
		t.Fatalf("SetProfileEnabled: %v", err)
	}

	// Admin list should still include it (disabled).
	adminList, err := catalog.ListAdminProfiles(ctx, st, "default")
	if err != nil {
		t.Fatalf("ListAdminProfiles: %v", err)
	}

	adminP, ok := findAdminProfileByID(adminList, testProfileID)
	if !ok {
		t.Fatal("disabled profile missing from admin list")
	}

	if adminP.Enabled {
		t.Error("profile should be disabled in admin list")
	}

	// T06's Profiles should exclude it.
	t06Profiles, err := catalog.Profiles(ctx, st, "default")
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}

	if _, ok := findProfileByID(t06Profiles, testProfileID); ok {
		t.Error("disabled profile should not appear in T06's Profiles")
	}

	// Re-enable.
	err = catalog.SetProfileEnabled(ctx, st, "default", testProfileID, true)
	if err != nil {
		t.Fatalf("SetProfileEnabled re-enable: %v", err)
	}

	t06Profiles, err = catalog.Profiles(ctx, st, "default")
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}

	if _, ok := findProfileByID(t06Profiles, testProfileID); !ok {
		t.Error("re-enabled profile should appear in T06's Profiles")
	}
}

// findAdminProfileByID returns the admin profile with the given id.
func findAdminProfileByID(profiles []catalog.AdminProfile, id string) (catalog.AdminProfile, bool) {
	for _, p := range profiles {
		if p.ID == id {
			return p, true
		}
	}

	return catalog.AdminProfile{}, false
}

// findProfileByID returns the profile with the given id.
func findProfileByID(profiles []catalog.Profile, id string) (catalog.Profile, bool) {
	for _, p := range profiles {
		if p.ID == id {
			return p, true
		}
	}

	return catalog.Profile{}, false
}

// TestSetProfileEnabled_NotFound — toggling a non-existent profile returns
// ErrProfileNotFound.
func TestSetProfileEnabled_NotFound(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	err := catalog.SetProfileEnabled(ctx, st, "default", "nonexistent", false)
	if !errors.Is(err, catalog.ErrProfileNotFound) {
		t.Fatalf("SetProfileEnabled nonexistent: got %v, want ErrProfileNotFound", err)
	}
}

// --- Tags tests ---

// buildTagProjection creates a projection from the fake cluster snapshot for
// tag VM count tests.
func buildTagProjection(t *testing.T) *inventory.Projection {
	t.Helper()

	snap, err := cluster.Fake{}.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("fake Snapshot: %v", err)
	}

	idx := inventory.BuildIndex(snap)

	return inventory.NewProjectionFromIndex(&idx)
}

// TestListTags_PvmssSeeded — the pvmss tag is present after migration and
// marked protected, with a live VM count > 0.
func TestListTags_PvmssSeeded(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()
	proj := buildTagProjection(t)

	tags, err := catalog.ListTags(ctx, st, proj, "default")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	found := false

	for _, tag := range tags {
		if tag.Name == "pvmss" {
			found = true

			if !tag.Protected {
				t.Error("pvmss tag should be protected")
			}

			if tag.VMCount == 0 {
				t.Error("pvmss tag should have VM count > 0 from fake dataset")
			}
		}
	}

	if !found {
		t.Error("pvmss tag not found in list")
	}
}

// TestCreateTag_Success — creating a new tag succeeds and appears in the list.
func TestCreateTag_Success(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()
	proj := buildTagProjection(t)

	tag, err := catalog.CreateTag(ctx, st, "default", "teamweb", "#16a34a")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	if tag.Name != "teamweb" {
		t.Errorf("tag.Name = %q", tag.Name)
	}

	if tag.Color != "#16a34a" {
		t.Errorf("tag.Color = %q", tag.Color)
	}

	if tag.VMCount != 0 {
		t.Errorf("new tag VMCount = %d, want 0", tag.VMCount)
	}

	if tag.Protected {
		t.Error("new tag should not be protected")
	}

	tags, err := catalog.ListTags(ctx, st, proj, "default")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	found := false

	for _, t2 := range tags {
		if t2.Name == "teamweb" {
			found = true
		}
	}

	if !found {
		t.Error("created tag not in list")
	}
}

// TestCreateTag_Duplicate — creating a tag with an existing name returns
// ErrDuplicateTag.
func TestCreateTag_Duplicate(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	_, err := catalog.CreateTag(ctx, st, "default", "pvmss", "#000000")
	if !errors.Is(err, catalog.ErrDuplicateTag) {
		t.Fatalf("CreateTag duplicate pvmss: got %v, want ErrDuplicateTag", err)
	}
}

// TestCreateTag_InvalidName — names with non-alphanumeric characters or wrong
// length are rejected.
func TestCreateTag_InvalidName(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	tests := []string{
		"",         // too short
		"team web", // space
		"team_web", // underscore
		"team-web", // hyphen
		"this-tag-name-is-way-too-long-to-be-valid-yes", // 51 chars
	}
	for _, name := range tests {
		_, err := catalog.CreateTag(ctx, st, "default", name, "#000000")
		if !errors.Is(err, catalog.ErrInvalidTagName) {
			t.Errorf("CreateTag(%q): got %v, want ErrInvalidTagName", name, err)
		}
	}
}

// TestCreateTag_DefaultColor — an empty color falls back to the indigo default
// (#4f46e5) rather than being rejected (FR-013).
func TestCreateTag_DefaultColor(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	tag, err := catalog.CreateTag(ctx, st, "default", "defaultcolor", "")
	if err != nil {
		t.Fatalf("CreateTag empty color: %v", err)
	}

	if tag.Color != "#4f46e5" {
		t.Errorf("default color = %q, want #4f46e5", tag.Color)
	}

	// Read it back through ListTags to confirm the persisted row carries the
	// default, not an empty string.
	tags, err := catalog.ListTags(ctx, st, nil, "default")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	for _, t2 := range tags {
		if t2.Name == "defaultcolor" && t2.Color != "#4f46e5" {
			t.Errorf("persisted default color = %q, want #4f46e5", t2.Color)
		}
	}
}

// TestSetTagColor_Success — updating a tag's color works, including for pvmss.
func TestSetTagColor_Success(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	// Change pvmss color (allowed — protection is delete-only).
	tag, err := catalog.SetTagColor(ctx, st, "default", "pvmss", "#dc2626")
	if err != nil {
		t.Fatalf("SetTagColor pvmss: %v", err)
	}

	if tag.Color != "#dc2626" {
		t.Errorf("tag.Color = %q, want #dc2626", tag.Color)
	}
}

// TestSetTagColor_NotFound — updating a non-existent tag returns
// ErrTagNotFound.
func TestSetTagColor_NotFound(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	_, err := catalog.SetTagColor(ctx, st, "default", "nonexistent", "#000000")
	if !errors.Is(err, catalog.ErrTagNotFound) {
		t.Fatalf("SetTagColor nonexistent: got %v, want ErrTagNotFound", err)
	}
}

// TestDeleteTag_Protected — deleting pvmss returns ErrProtectedTag.
func TestDeleteTag_Protected(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	err := catalog.DeleteTag(ctx, st, "default", "pvmss")
	if !errors.Is(err, catalog.ErrProtectedTag) {
		t.Fatalf("DeleteTag pvmss: got %v, want ErrProtectedTag", err)
	}
}

// TestDeleteTag_NotFound — deleting a non-existent tag returns ErrTagNotFound.
func TestDeleteTag_NotFound(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	err := catalog.DeleteTag(ctx, st, "default", "nonexistent")
	if !errors.Is(err, catalog.ErrTagNotFound) {
		t.Fatalf("DeleteTag nonexistent: got %v, want ErrTagNotFound", err)
	}
}

// TestDeleteTag_Success — deleting a non-pvmss tag removes it from the list.
func TestDeleteTag_Success(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()
	proj := buildTagProjection(t)

	_, err := catalog.CreateTag(ctx, st, "default", "temp", "#000000")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	err = catalog.DeleteTag(ctx, st, "default", "temp")
	if err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}

	tags, err := catalog.ListTags(ctx, st, proj, "default")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	for _, tag := range tags {
		if tag.Name == "temp" {
			t.Error("deleted tag still in list")
		}
	}
}
