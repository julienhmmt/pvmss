package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrLeaseNotFound reports that a VM has no lease row (never leased, or
// exempted by an administrator).
var ErrLeaseNotFound = errors.New("vm lease not found")

// leaseTimeFormat is RFC3339 in UTC at second precision. A fixed width and
// zone make the stored string sort chronologically, which the sweeper's
// expires_at < now query relies on.
const leaseTimeFormat = time.RFC3339

// VMLease is one VM's expiry. A row exists only while the VM is leased.
// Pool is the owner pool the lease was written for: a recycled VMID must not
// expire a VM that belongs to another pool.
type VMLease struct {
	Cluster   string
	VMID      int
	ExpiresAt time.Time
	Pool      string
	// Identity is the VM's Proxmox creation identity, bound after the create
	// task finishes. Empty until bound; the sweeper never deletes an unbound lease.
	Identity string
}

// GetVMLease returns the lease of one VM, or ErrLeaseNotFound.
func (s *Store) GetVMLease(ctx context.Context, cluster string, vmid int) (VMLease, error) {
	var (
		lease   VMLease
		expires string
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT cluster, vmid, expires_at, pool, identity FROM vm_leases WHERE cluster = ? AND vmid = ?`,
		cluster, vmid).Scan(&lease.Cluster, &lease.VMID, &expires, &lease.Pool, &lease.Identity)
	if errors.Is(err, sql.ErrNoRows) {
		return VMLease{}, ErrLeaseNotFound
	}

	if err != nil {
		return VMLease{}, fmt.Errorf("query vm lease: %w", err)
	}

	lease.ExpiresAt, err = time.Parse(leaseTimeFormat, expires)
	if err != nil {
		return VMLease{}, fmt.Errorf("parse vm lease expiry: %w", err)
	}

	return lease, nil
}

// SetVMLease writes a VM's expiry, inserting the lease or replacing its expiry.
func (s *Store) SetVMLease(ctx context.Context, lease VMLease) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO vm_leases (cluster, vmid, expires_at, pool, identity) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(cluster, vmid) DO UPDATE SET expires_at = excluded.expires_at, pool = excluded.pool, identity = excluded.identity,
			notified_at = NULL`,
		lease.Cluster, lease.VMID, lease.ExpiresAt.UTC().Format(leaseTimeFormat), lease.Pool, lease.Identity)
	if err != nil {
		return fmt.Errorf("upsert vm lease: %w", err)
	}

	return nil
}

// DeleteVMLease removes a VM's lease. Deleting an absent lease is not an error.
func (s *Store) DeleteVMLease(ctx context.Context, cluster string, vmid int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM vm_leases WHERE cluster = ? AND vmid = ?`, cluster, vmid)
	if err != nil {
		return fmt.Errorf("delete vm lease: %w", err)
	}

	return nil
}

// ExtendVMLease moves an existing lease's expiry and re-arms its warning. It
// never touches the identity or pool. A missing row is ErrLeaseNotFound.
func (s *Store) ExtendVMLease(ctx context.Context, cluster string, vmid int, expires time.Time) error {
	res, err := s.db.ExecContext(ctx, `UPDATE vm_leases SET expires_at = ?, notified_at = NULL WHERE cluster = ? AND vmid = ?`,
		expires.UTC().Format(leaseTimeFormat), cluster, vmid)
	if err != nil {
		return fmt.Errorf("extend vm lease: %w", err)
	}

	if n, _ := res.RowsAffected(); n == 0 {
		return ErrLeaseNotFound
	}

	return nil
}

// SetLeaseIdentity binds a lease to the VM identity read from the live config.
// A missing row is ErrLeaseNotFound.
func (s *Store) SetLeaseIdentity(ctx context.Context, cluster string, vmid int, identity string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE vm_leases SET identity = ? WHERE cluster = ? AND vmid = ?`, identity, cluster, vmid)
	if err != nil {
		return fmt.Errorf("bind vm lease identity: %w", err)
	}

	if n, _ := res.RowsAffected(); n == 0 {
		return ErrLeaseNotFound
	}

	return nil
}

// ListExpiredVMLeases returns the cluster's leases whose expiry is strictly
// before now.
func (s *Store) ListExpiredVMLeases(ctx context.Context, cluster string, now time.Time) ([]VMLease, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cluster, vmid, expires_at, pool, identity FROM vm_leases WHERE cluster = ? AND expires_at < ? ORDER BY vmid`,
		cluster, now.UTC().Format(leaseTimeFormat))
	if err != nil {
		return nil, fmt.Errorf("query expired vm leases: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var leases []VMLease

	for rows.Next() {
		var (
			lease   VMLease
			expires string
		)

		if err := rows.Scan(&lease.Cluster, &lease.VMID, &expires, &lease.Pool, &lease.Identity); err != nil {
			return nil, fmt.Errorf("scan expired vm lease: %w", err)
		}

		lease.ExpiresAt, err = time.Parse(leaseTimeFormat, expires)
		if err != nil {
			return nil, fmt.Errorf("parse vm lease expiry: %w", err)
		}

		leases = append(leases, lease)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired vm leases: %w", err)
	}

	return leases, nil
}
