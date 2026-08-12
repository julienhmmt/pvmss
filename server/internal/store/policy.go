package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// PolicyRow is the persisted per-cluster gabarit and quota row.
type PolicyRow struct {
	Cluster         string
	MaxSockets      int
	MaxCores        int
	MaxMemoryMB     int
	MaxDiskPerVMGB  int
	MaxNetworkCards int
	MaxSnapshots    int
	MaxVMPerUser    int
	AllowCustomYAML bool
}

// NodePolicyRow is the persisted per-node capacité row.
type NodePolicyRow struct {
	Cluster   string
	Node      string
	MaxVMs    int
	MaxVCPUs  int
	MaxRAMGB  int
	MaxDiskGB int
}

// PolicyRow reads one cluster's persisted gabarit and quota values.
func (s *Store) PolicyRow(ctx context.Context, cluster string) (PolicyRow, error) {
	var row PolicyRow

	err := s.db.QueryRowContext(ctx, `
		SELECT cluster, max_sockets, max_cores, max_memory_mb, max_disk_per_vm_gb,
		       max_network_cards, max_snapshots, max_vm_per_user, allow_custom_yaml
		FROM vm_limits WHERE cluster = ?`, cluster).Scan(
		&row.Cluster, &row.MaxSockets, &row.MaxCores, &row.MaxMemoryMB,
		&row.MaxDiskPerVMGB, &row.MaxNetworkCards, &row.MaxSnapshots,
		&row.MaxVMPerUser, &row.AllowCustomYAML,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PolicyRow{}, fmt.Errorf("policy row for cluster %q: %w", cluster, sql.ErrNoRows)
	}

	if err != nil {
		return PolicyRow{}, fmt.Errorf("query policy row: %w", err)
	}

	return row, nil
}

// UpsertPolicyRow persists the complete gabarit and quota row atomically.
func (s *Store) UpsertPolicyRow(ctx context.Context, row PolicyRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO vm_limits (
			cluster, max_sockets, max_cores, max_memory_mb, max_disk_per_vm_gb,
			max_network_cards, max_snapshots, max_vm_per_user, allow_custom_yaml
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster) DO UPDATE SET
			max_sockets = excluded.max_sockets,
			max_cores = excluded.max_cores,
			max_memory_mb = excluded.max_memory_mb,
			max_disk_per_vm_gb = excluded.max_disk_per_vm_gb,
			max_network_cards = excluded.max_network_cards,
			max_snapshots = excluded.max_snapshots,
			max_vm_per_user = excluded.max_vm_per_user,
			allow_custom_yaml = excluded.allow_custom_yaml`,
		row.Cluster, row.MaxSockets, row.MaxCores, row.MaxMemoryMB,
		row.MaxDiskPerVMGB, row.MaxNetworkCards, row.MaxSnapshots,
		row.MaxVMPerUser, row.AllowCustomYAML,
	)
	if err != nil {
		return fmt.Errorf("upsert policy row: %w", err)
	}

	return nil
}

// NodePolicyRow reads one node's configured capacité. Missing rows are reported
// as sql.ErrNoRows so callers can apply the all-zero no-cap default.
func (s *Store) NodePolicyRow(ctx context.Context, cluster, node string) (NodePolicyRow, error) {
	var row NodePolicyRow

	err := s.db.QueryRowContext(ctx, `
		SELECT cluster, node, max_vms, max_vcpus, max_ram_gb, max_disk_gb
		FROM node_limits WHERE cluster = ? AND node = ?`, cluster, node).Scan(
		&row.Cluster, &row.Node, &row.MaxVMs, &row.MaxVCPUs, &row.MaxRAMGB, &row.MaxDiskGB,
	)
	if err != nil {
		return NodePolicyRow{}, err
	}

	return row, nil
}

// NodePolicyRows returns all configured capacité rows for a cluster.
func (s *Store) NodePolicyRows(ctx context.Context, cluster string) ([]NodePolicyRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cluster, node, max_vms, max_vcpus, max_ram_gb, max_disk_gb
		FROM node_limits WHERE cluster = ? ORDER BY node`, cluster)
	if err != nil {
		return nil, fmt.Errorf("query node policy rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]NodePolicyRow, 0)

	for rows.Next() {
		var row NodePolicyRow
		if err := rows.Scan(&row.Cluster, &row.Node, &row.MaxVMs, &row.MaxVCPUs, &row.MaxRAMGB, &row.MaxDiskGB); err != nil {
			return nil, fmt.Errorf("scan node policy row: %w", err)
		}

		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node policy rows: %w", err)
	}

	return result, nil
}

// UpsertNodePolicyRow persists one node's capacité row atomically.
func (s *Store) UpsertNodePolicyRow(ctx context.Context, row NodePolicyRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO node_limits (cluster, node, max_vms, max_vcpus, max_ram_gb, max_disk_gb)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster, node) DO UPDATE SET
			max_vms = excluded.max_vms,
			max_vcpus = excluded.max_vcpus,
			max_ram_gb = excluded.max_ram_gb,
			max_disk_gb = excluded.max_disk_gb`,
		row.Cluster, row.Node, row.MaxVMs, row.MaxVCPUs, row.MaxRAMGB, row.MaxDiskGB,
	)
	if err != nil {
		return fmt.Errorf("upsert node policy row: %w", err)
	}

	return nil
}
