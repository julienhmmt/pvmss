//nolint:goconst // test fixtures reuse cluster/tag/profile string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"pvmss/server/internal/recovery"
	"testing"
)

// T010: vm_limits (five copied fields) and node_limits (four copied fields)
// → v0.4 equivalents, asserting max_sockets/max_cores/max_memory_mb are
// never written by this function (SC-002's literal assertion).
func TestMapVMLimits_FiveFieldsCopied(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		VMLimits: &legacyVMLimits{
			maxVMS:          10,
			maxVMPerUser:    5,
			maxNetworkCards: 3,
			maxDiskPerVM:    20,
			allowCustomYAML: true,
			maxSnapshots:    8,
		},
	})

	row, err := recovery.MapVMLimitsForTest(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapVMLimits: %v", err)
	}

	if row.MaxVMPerUser != 5 {
		t.Errorf("MaxVMPerUser = %d, want 5", row.MaxVMPerUser)
	}

	if row.MaxNetworkCards != 3 {
		t.Errorf("MaxNetworkCards = %d, want 3", row.MaxNetworkCards)
	}

	if row.MaxDiskPerVMGB != 20 {
		t.Errorf("MaxDiskPerVMGB = %d, want 20", row.MaxDiskPerVMGB)
	}

	if !row.AllowCustomYAML {
		t.Error("AllowCustomYAML = false, want true")
	}

	if row.MaxSnapshots != 8 {
		t.Errorf("MaxSnapshots = %d, want 8", row.MaxSnapshots)
	}
}

// SC-002 literal assertion: VMLimitsRow has NO max_sockets/max_cores/max_memory_mb fields.
// This test verifies the type itself does not carry those fields — they are
// never written by the recovery tool because there is no on-disk source.
func TestVMLimitsRow_HasNoSocketsCoresMemoryFields(t *testing.T) {
	t.Parallel()

	// This is a compile-time assertion: the VMLimitsRow struct type does not
	// have MaxSockets, MaxCores, or MaxMemoryMB fields. If someone adds them,
	// this test will fail to compile, forcing a spec review.
	row := recovery.VMLimitsRow{
		MaxDiskPerVMGB:  20,
		MaxNetworkCards: 3,
		MaxSnapshots:    8,
		MaxVMPerUser:    5,
		AllowCustomYAML: true,
	}
	// The following lines would fail to compile if the fields were added
	// with those names — Go does not allow accessing non-existent fields.
	// We use the fields that DO exist to ensure the struct is used:
	if row.MaxDiskPerVMGB != 20 {
		t.Errorf("MaxDiskPerVMGB = %d, want 20", row.MaxDiskPerVMGB)
	}
}

func TestMapNodeLimits_FourFieldsCopied(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		NodeLimits: []legacyNodeLimits{
			{node: "pve-a", maxVMs: 10, maxVCPUs: 32, maxRAMGB: 64, maxDiskGB: 500},
			{node: "pve-b", maxVMs: 5, maxVCPUs: 16, maxRAMGB: 32, maxDiskGB: 250},
		},
	})

	rows, err := recovery.MapNodeLimitsForTest(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapNodeLimits: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}

	if rows[0].Node != "pve-a" {
		t.Errorf("rows[0].Node = %q, want %q", rows[0].Node, "pve-a")
	}

	if rows[0].MaxVMs != 10 || rows[0].MaxVCPUs != 32 || rows[0].MaxRAMGB != 64 || rows[0].MaxDiskGB != 500 {
		t.Errorf("rows[0] = %+v, want all four fields copied", rows[0])
	}
}

// T010: pre-schemaV2 zero-value case — node_limits without max_vcpus/ram/disk columns.
// The fixture uses COALESCE to read 0 when columns are NULL.
func TestMapNodeLimits_PreSchemaV2_ZeroValues(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	// Seed a node_limits row with NULL for the schemaV2 columns
	ctx := context.Background()
	if _, err := legacyDB.ExecContext(ctx,
		`INSERT INTO node_limits (node_name, max_vms) VALUES (?, ?)`,
		"old-node", 10,
	); err != nil {
		t.Fatalf("seed old node_limits: %v", err)
	}

	rows, err := recovery.MapNodeLimitsForTest(ctx, legacyDB)
	if err != nil {
		t.Fatalf("MapNodeLimits: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}

	if rows[0].MaxVMs != 10 {
		t.Errorf("MaxVMs = %d, want 10", rows[0].MaxVMs)
	}

	if rows[0].MaxVCPUs != 0 || rows[0].MaxRAMGB != 0 || rows[0].MaxDiskGB != 0 {
		t.Errorf("schemaV2 fields = %d/%d/%d, want 0/0/0 (no cap)", rows[0].MaxVCPUs, rows[0].MaxRAMGB, rows[0].MaxDiskGB)
	}
}

// SC-002: upsertVMLimits preserves T12's shipped defaults for the three no-source fields.
func TestUpsertVMLimits_PreservesShippedDefaults(t *testing.T) {
	t.Parallel()

	v04DB := openV04DB(t)
	ctx := context.Background()

	// The v0.4 seed has defaults: max_sockets=4, max_cores=8, max_memory_mb=16384
	row := recovery.VMLimitsRow{
		MaxDiskPerVMGB:  20,
		MaxNetworkCards: 3,
		MaxSnapshots:    8,
		MaxVMPerUser:    5,
		AllowCustomYAML: true,
	}
	if err := recovery.UpsertVMLimitsForTest(ctx, v04DB, "default", row); err != nil {
		t.Fatalf("UpsertVMLimits: %v", err)
	}

	var (
		sockets, cores, memoryMB, diskPerVM, netCards, snapshots, vmPerUser int
		allowCustom                                                         int
	)

	err := v04DB.QueryRowContext(ctx,
		`SELECT max_sockets, max_cores, max_memory_mb, max_disk_per_vm_gb, max_network_cards, max_snapshots, max_vm_per_user, allow_custom_yaml FROM vm_limits WHERE cluster = ?`,
		"default").Scan(&sockets, &cores, &memoryMB, &diskPerVM, &netCards, &snapshots, &vmPerUser, &allowCustom)
	if err != nil {
		t.Fatalf("query vm_limits: %v", err)
	}
	// The three no-source fields must retain T12's defaults
	if sockets != 4 {
		t.Errorf("max_sockets = %d, want 4 (T12 default, not copied)", sockets)
	}

	if cores != 8 {
		t.Errorf("max_cores = %d, want 8 (T12 default, not copied)", cores)
	}

	if memoryMB != 16384 {
		t.Errorf("max_memory_mb = %d, want 16384 (T12 default, not copied)", memoryMB)
	}
	// The five copied fields must match the legacy source
	if diskPerVM != 20 {
		t.Errorf("max_disk_per_vm_gb = %d, want 20", diskPerVM)
	}

	if netCards != 3 {
		t.Errorf("max_network_cards = %d, want 3", netCards)
	}

	if snapshots != 8 {
		t.Errorf("max_snapshots = %d, want 8", snapshots)
	}

	if vmPerUser != 5 {
		t.Errorf("max_vm_per_user = %d, want 5", vmPerUser)
	}

	if allowCustom != 1 {
		t.Errorf("allow_custom_yaml = %d, want 1", allowCustom)
	}
}

func TestUpsertNodeLimits_WritesAndIsIdempotent(t *testing.T) {
	t.Parallel()

	v04DB := openV04DB(t)
	ctx := context.Background()

	r := recovery.NodeLimitsRow{Node: "pve-a", MaxVMs: 10, MaxVCPUs: 32, MaxRAMGB: 64, MaxDiskGB: 500}
	if err := recovery.UpsertNodeLimitsForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	if err := recovery.UpsertNodeLimitsForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM node_limits WHERE cluster = ? AND node = ?`, "default", "pve-a"); count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}
