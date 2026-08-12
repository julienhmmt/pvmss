package recovery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// MapPolicy reads the legacy vm_limits singleton and the node_limits table
// and returns rows for the v0.4 equivalents.
//
// vm_limits: only the five fields legacy actually persisted are copied
// (FR-003). max_sockets/max_cores/max_memory_mb are intentionally NOT
// read or returned — there is no on-disk source for them (SC-002). The
// caller must leave them at T12's shipped defaults.
//
// node_limits: all four fields are copied. A legacy database that predates
// schemaV2 has no max_vcpus/max_ram_gb/max_disk_gb columns; the fixture
// includes them as nullable and they read as 0 ("no cap") when absent.
func MapPolicy(ctx context.Context, legacyDB *sql.DB) (VMLimitsRow, []NodeLimitsRow, error) {
	vmLimits, err := mapVMLimits(ctx, legacyDB)
	if err != nil {
		return VMLimitsRow{}, nil, fmt.Errorf("map vm_limits: %w", err)
	}

	nodeLimits, err := mapNodeLimits(ctx, legacyDB)
	if err != nil {
		return VMLimitsRow{}, nil, fmt.Errorf("map node_limits: %w", err)
	}

	return vmLimits, nodeLimits, nil
}

func mapVMLimits(ctx context.Context, legacyDB *sql.DB) (VMLimitsRow, error) {
	var (
		row             VMLimitsRow
		allowCustomYAML int
	)

	err := legacyDB.QueryRowContext(ctx, `
		SELECT max_vm_per_user, max_network_cards, max_disk_per_vm, allow_custom_yaml, max_snapshots
		FROM vm_limits WHERE id = 1`).Scan(
		&row.MaxVMPerUser, &row.MaxNetworkCards, &row.MaxDiskPerVMGB,
		&allowCustomYAML, &row.MaxSnapshots,
	)
	if err != nil {
		return VMLimitsRow{}, fmt.Errorf("query vm_limits: %w", err)
	}

	row.AllowCustomYAML = allowCustomYAML != 0

	return row, nil
}

func mapNodeLimits(ctx context.Context, legacyDB *sql.DB) ([]NodeLimitsRow, error) {
	rows, err := legacyDB.QueryContext(ctx, `
		SELECT node_name, COALESCE(max_vms, 0), COALESCE(max_vcpus, 0), COALESCE(max_ram_gb, 0), COALESCE(max_disk_gb, 0)
		FROM node_limits ORDER BY node_name`)
	if err != nil {
		return nil, fmt.Errorf("query node_limits: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []NodeLimitsRow

	for rows.Next() {
		var r NodeLimitsRow
		if err := rows.Scan(&r.Node, &r.MaxVMs, &r.MaxVCPUs, &r.MaxRAMGB, &r.MaxDiskGB); err != nil {
			return nil, fmt.Errorf("scan node_limits: %w", err)
		}

		out = append(out, r)
	}

	return out, rows.Err()
}

// upsertVMLimits writes the five copied fields into the v0.4 vm_limits row,
// preserving the existing max_sockets/max_cores/max_memory_mb values
// (T12's shipped defaults) by reading them first and re-inserting.
// This is the literal implementation of SC-002: the three no-source fields
// are never written by this function.
func upsertVMLimits(ctx context.Context, v04DB *sql.DB, cluster string, row VMLimitsRow) error {
	var existingSockets, existingCores, existingMemoryMB int

	err := v04DB.QueryRowContext(ctx,
		`SELECT max_sockets, max_cores, max_memory_mb FROM vm_limits WHERE cluster = ?`,
		cluster).Scan(&existingSockets, &existingCores, &existingMemoryMB)

	switch {
	case err == nil:
		// Row exists — update only the five copied fields, preserve the rest.
		_, err = v04DB.ExecContext(ctx, `
			UPDATE vm_limits SET
				max_disk_per_vm_gb = ?,
				max_network_cards = ?,
				max_snapshots = ?,
				max_vm_per_user = ?,
				allow_custom_yaml = ?
			WHERE cluster = ?`,
			row.MaxDiskPerVMGB, row.MaxNetworkCards, row.MaxSnapshots,
			row.MaxVMPerUser, row.AllowCustomYAML, cluster)
		if err != nil {
			return fmt.Errorf("update vm_limits: %w", err)
		}

		return nil

	case errIsNoRows(err):
		// No row yet — insert with T12's defaults for the three no-source fields.
		_, err = v04DB.ExecContext(ctx, `
			INSERT INTO vm_limits (
				cluster, max_sockets, max_cores, max_memory_mb,
				max_disk_per_vm_gb, max_network_cards, max_snapshots,
				max_vm_per_user, allow_custom_yaml
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			cluster, defaultMaxSockets, defaultMaxCores, defaultMaxMemoryMB,
			row.MaxDiskPerVMGB, row.MaxNetworkCards, row.MaxSnapshots,
			row.MaxVMPerUser, row.AllowCustomYAML)
		if err != nil {
			return fmt.Errorf("insert vm_limits: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("query existing vm_limits: %w", err)
	}
}

// T12's shipped defaults for the three fields that have no legacy source.
const (
	defaultMaxSockets  = 4
	defaultMaxCores    = 8
	defaultMaxMemoryMB = 16384
)

func upsertNodeLimits(ctx context.Context, v04DB *sql.DB, cluster string, row NodeLimitsRow) error {
	_, err := v04DB.ExecContext(ctx, `
		INSERT INTO node_limits (cluster, node, max_vms, max_vcpus, max_ram_gb, max_disk_gb)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster, node) DO UPDATE SET
			max_vms = excluded.max_vms,
			max_vcpus = excluded.max_vcpus,
			max_ram_gb = excluded.max_ram_gb,
			max_disk_gb = excluded.max_disk_gb`,
		cluster, row.Node, row.MaxVMs, row.MaxVCPUs, row.MaxRAMGB, row.MaxDiskGB)
	if err != nil {
		return fmt.Errorf("upsert node_limits: %w", err)
	}

	return nil
}

// errIsNoRows checks if an error is sql.ErrNoRows without importing database/sql
// in the helper (the package already imports it above, but this keeps the
// check local and readable).
func errIsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
