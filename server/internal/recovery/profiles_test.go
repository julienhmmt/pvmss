//nolint:goconst // test fixtures reuse cluster/tag/profile string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"pvmss/server/internal/recovery"
	"testing"
)

// T009: vm_profiles (valid JSON, malformed JSON skip case) → catalog_profiles.
func TestMapProfiles_ValidJSON(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Profiles: []legacyProfile{
			{
				id:      "small",
				name:    "Small",
				config:  `{"sockets":1,"cores":2,"ram_gb":4,"disk_gb":20,"disk_bus":"virtio"}`,
				enabled: true,
			},
			{
				id:      "large",
				name:    "Large",
				config:  `{"sockets":2,"cores":4,"ram_gb":8,"disk_gb":50,"disk_bus":"scsi"}`,
				enabled: true,
			},
		},
	})

	profiles, skips, err := recovery.MapProfiles(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapProfiles: %v", err)
	}

	if len(profiles) != 2 {
		t.Fatalf("len(profiles) = %d, want 2", len(profiles))
	}

	if len(skips) != 0 {
		t.Fatalf("len(skips) = %d, want 0", len(skips))
	}

	// Verify JSON field mapping: ram_gb → memory_mb (×1024)
	// Profiles are ordered by id — "large" comes before "small" alphabetically
	var smallProfile *recovery.ProfileRow

	for i := range profiles {
		if profiles[i].ID == "small" {
			smallProfile = &profiles[i]
			break
		}
	}

	if smallProfile == nil {
		t.Fatal("small profile not found")
	}

	if smallProfile.MemoryMB != 4*1024 {
		t.Errorf("small.MemoryMB = %d, want %d", smallProfile.MemoryMB, 4*1024)
	}

	if smallProfile.Bus != "virtio" {
		t.Errorf("small.Bus = %q, want %q", smallProfile.Bus, "virtio")
	}
}

func TestMapProfiles_MalformedJSON_Skipped(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Profiles: []legacyProfile{
			{
				id:      "good",
				name:    "Good",
				config:  `{"sockets":1,"cores":2,"ram_gb":4,"disk_gb":20,"disk_bus":"virtio"}`,
				enabled: true,
			},
			{
				id:      "bad",
				name:    "Bad",
				config:  `{not valid json`,
				enabled: true,
			},
		},
	})

	profiles, skips, err := recovery.MapProfiles(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapProfiles: %v", err)
	}

	if len(profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1 (bad JSON skipped)", len(profiles))
	}

	if profiles[0].ID != "good" {
		t.Errorf("profiles[0].ID = %q, want %q", profiles[0].ID, "good")
	}

	if len(skips) != 1 {
		t.Fatalf("len(skips) = %d, want 1", len(skips))
	}

	if skips[0].Row != "bad" {
		t.Errorf("skips[0].Row = %q, want %q", skips[0].Row, "bad")
	}
}

//nolint:dupl // test body mirrors ZeroResources but with different fixture data
func TestMapProfiles_MissingDiskBus_Skipped(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Profiles: []legacyProfile{
			{
				id:      "no-bus",
				name:    "NoBus",
				config:  `{"sockets":1,"cores":2,"ram_gb":4,"disk_gb":20}`,
				enabled: true,
			},
		},
	})

	profiles, skips, err := recovery.MapProfiles(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapProfiles: %v", err)
	}

	if len(profiles) != 0 {
		t.Fatalf("len(profiles) = %d, want 0", len(profiles))
	}

	if len(skips) != 1 {
		t.Fatalf("len(skips) = %d, want 1", len(skips))
	}
}

//nolint:dupl // test body mirrors MissingDiskBus but with different fixture data
func TestMapProfiles_ZeroResources_Skipped(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Profiles: []legacyProfile{
			{
				id:      "zero-ram",
				name:    "ZeroRAM",
				config:  `{"sockets":1,"cores":2,"ram_gb":0,"disk_gb":20,"disk_bus":"virtio"}`,
				enabled: true,
			},
		},
	})

	profiles, skips, err := recovery.MapProfiles(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapProfiles: %v", err)
	}

	if len(profiles) != 0 {
		t.Fatalf("len(profiles) = %d, want 0", len(profiles))
	}

	if len(skips) != 1 {
		t.Fatalf("len(skips) = %d, want 1", len(skips))
	}
}

func TestUpsertProfile_WritesAndIsIdempotent(t *testing.T) {
	t.Parallel()

	v04DB := openV04DB(t)
	ctx := context.Background()

	r := recovery.ProfileRow{
		ID: "small", Label: "Small", CPUCores: 2, MemoryMB: 4096, DiskGB: 20, Bus: "virtio", Enabled: true,
	}
	if err := recovery.UpsertProfileForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	if err := recovery.UpsertProfileForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_profiles WHERE cluster = ? AND id = ?`, "default", "small"); count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}
