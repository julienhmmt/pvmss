//nolint:goconst // test fixtures reuse cluster/tag/profile string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"pvmss/server/internal/recovery"
	"testing"
)

// T008: tags → catalog_tags, default palette assignment, pvmss row upsert no-op.
func TestMapTags_DirectCopy(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Tags: []string{"pvmss", "prod", "dev", "test"},
	})

	tags, err := recovery.MapTags(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapTags: %v", err)
	}

	if len(tags) != 4 {
		t.Fatalf("len(tags) = %d, want 4", len(tags))
	}
	// Tags are ordered alphabetically — dev, prod, pvmss, test
	if tags[0].Name != "dev" {
		t.Errorf("tags[0].Name = %q, want %q (alphabetical)", tags[0].Name, "dev")
	}
	// Every tag should have a non-empty color
	for _, tag := range tags {
		if tag.Color == "" {
			t.Errorf("tag %q has empty color", tag.Name)
		}

		if tag.CreatedAt == "" {
			t.Errorf("tag %q has empty created_at", tag.Name)
		}
	}
}

func TestMapTags_DeterministicColors(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Tags: []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota"},
	})

	tags, err := recovery.MapTags(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapTags: %v", err)
	}
	// First 8 tags get unique palette colors; 9th wraps around to palette[0]
	if tags[0].Color != tags[8].Color {
		t.Errorf("color wrapping: tags[0]=%q tags[8]=%q, want same (palette wraps)", tags[0].Color, tags[8].Color)
	}
	// Same input → same output (deterministic)
	tags2, err := recovery.MapTags(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("second MapTags: %v", err)
	}

	for i := range tags {
		if tags[i].Color != tags2[i].Color {
			t.Errorf("non-deterministic: tag %d color %q vs %q", i, tags[i].Color, tags2[i].Color)
		}
	}
}

func TestMapTags_EmptyTable(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{})

	tags, err := recovery.MapTags(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapTags: %v", err)
	}

	if len(tags) != 0 {
		t.Fatalf("len(tags) = %d, want 0", len(tags))
	}
}

// T008: pvmss row upsert is a no-op when it already exists (v0.4 seeds it).
func TestUpsertTag_PvmssNoop(t *testing.T) {
	t.Parallel()

	v04DB := openV04DB(t)
	ctx := context.Background()

	// v0.4 already has the pvmss seed row — verify it exists
	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_tags WHERE cluster = ? AND name = ?`, "default", "pvmss"); count != 1 {
		t.Fatalf("pvmss seed row missing: count = %d", count)
	}

	// Upsert pvmss with a different color — should update, not duplicate
	r := recovery.TagRow{Name: "pvmss", Color: "#ff0000", CreatedAt: "2026-08-12T12:00:00Z"}
	if err := recovery.UpsertTagForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("UpsertTag: %v", err)
	}

	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_tags WHERE cluster = ? AND name = ?`, "default", "pvmss"); count != 1 {
		t.Errorf("pvmss row count = %d, want 1 (no duplicate)", count)
	}
}

func TestUpsertTag_NewTag(t *testing.T) {
	t.Parallel()

	v04DB := openV04DB(t)
	ctx := context.Background()

	r := recovery.TagRow{Name: "prod", Color: "#3b82f6", CreatedAt: "2026-08-12T12:00:00Z"}
	if err := recovery.UpsertTagForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("UpsertTag: %v", err)
	}

	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_tags WHERE cluster = ? AND name = ?`, "default", "prod"); count != 1 {
		t.Errorf("prod row count = %d, want 1", count)
	}
}
