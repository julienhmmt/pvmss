package vm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"time"
)

// ErrLeaseDisabled is returned when an owner tries to extend a VM in a
// cluster whose policy has no lease length (lease_days = 0).
var ErrLeaseDisabled = errors.New("leases are disabled for this cluster")

// ErrLeaseDays is returned when a requested extension is below one day.
var ErrLeaseDays = errors.New("lease extension must be at least 1 day")

// systemActor is the identity the expiry sweeper acts as. IsAdmin lets Resolve
// skip the pool check; the pvmss tag gate still applies.
var systemActor = auth.Identity{Username: "system", DisplayName: "system", IsAdmin: true}

// leaseDay is one day, the unit of lease_days and extension requests.
const leaseDay = 24 * time.Hour

// LeaseExtension is one owner request to push a VM's expiry out.
type LeaseExtension struct {
	Actor       auth.Identity
	ClusterName string
	VMID        int
	// Days is the requested extension; values above MaxDays are clamped.
	Days int
	// MaxDays is the cluster's lease length (policy lease_days).
	MaxDays int
	IP      string
}

// StartLease sets the lease of a freshly created VM, overwriting any row left
// for the same VMID by a previous VM. days <= 0 means the cluster has no
// leases: any stale row is removed so it cannot reach the new VM.
func StartLease(ctx context.Context, st *store.Store, clusterName string, vmid, days int, pool string, now time.Time) error {
	if days <= 0 {
		return st.DeleteVMLease(ctx, clusterName, vmid)
	}

	return st.SetVMLease(ctx, store.VMLease{Cluster: clusterName, VMID: vmid, ExpiresAt: now.Add(time.Duration(days) * leaseDay), Pool: pool})
}

// dropLeaseRow removes a VM's lease after the VM itself is gone or renamed
// handling needs it. A failure is logged, never returned: the VM change has
// already happened and the identity check protects the next sweep.
func dropLeaseRow(ctx context.Context, st *store.Store, log *slog.Logger, clusterName string, vmid int) {
	if st == nil {
		return
	}

	if err := st.DeleteVMLease(ctx, clusterName, vmid); err != nil && log != nil {
		log.ErrorContext(ctx, "vm lease row not removed after delete", "component", "vm", "event", "lease_drop_failed", "cluster", clusterName, "vmid", vmid, "error", err)
	}
}

// ExtendLease sets a leased VM's expiry to now + min(Days, MaxDays). It goes
// through Resolve, so only the VM's owner (or an admin) can extend it. A VM
// without a lease row is refused with store.ErrLeaseNotFound.
func ExtendLease(ctx context.Context, source inventory.LookupSource, st *store.Store, req LeaseExtension, now time.Time) (store.VMLease, error) {
	if _, err := Resolve(source, req.Actor, req.ClusterName, req.VMID, st); err != nil {
		return store.VMLease{}, err
	}

	if req.MaxDays <= 0 {
		return store.VMLease{}, ErrLeaseDisabled
	}

	if req.Days < 1 {
		return store.VMLease{}, ErrLeaseDays
	}

	days := min(req.Days, req.MaxDays)

	current, err := st.GetVMLease(ctx, req.ClusterName, req.VMID)
	if err != nil {
		return store.VMLease{}, err
	}

	next := store.VMLease{Cluster: req.ClusterName, VMID: req.VMID, ExpiresAt: extendedExpiry(current.ExpiresAt, now, days, req.MaxDays), Pool: current.Pool, Identity: current.Identity}
	// Only the expiry is written: a full-row write from this read could wipe
	// an identity bound concurrently.
	if err := st.ExtendVMLease(ctx, req.ClusterName, req.VMID, next.ExpiresAt); err != nil {
		return store.VMLease{}, fmt.Errorf("extend vm lease: %w", err)
	}

	detail := leaseDetail(
		fmt.Sprintf("lease extended by %d day(s)", days),
		map[string]any{"field": "expiresAt", "old": current.ExpiresAt.Format(time.RFC3339), "new": next.ExpiresAt.Format(time.RFC3339)},
	)
	if err := st.RecordAdminAction(ctx, req.Actor.Username, "vm.lease_extended", "vm", FormatVMRef(req.ClusterName, req.VMID), detail, req.IP); err != nil {
		return store.VMLease{}, fmt.Errorf(auditWrapFmt, err)
	}

	return next, nil
}

// RemoveLease exempts a VM from expiry by deleting its lease row. Admin-only;
// the caller enforces that. A VM with no lease is reported as ErrLeaseNotFound.
func RemoveLease(ctx context.Context, st *store.Store, actor auth.Identity, clusterName string, vmid int, ip string) error {
	if _, err := st.GetVMLease(ctx, clusterName, vmid); err != nil {
		return err
	}

	if err := st.DeleteVMLease(ctx, clusterName, vmid); err != nil {
		return err
	}

	detail := leaseDetail("lease removed by administrator", map[string]any{"field": "expiresAt", "old": "set", "new": "none"})
	if err := st.RecordAdminAction(ctx, actor.Username, "vm.lease_removed", "vm", FormatVMRef(clusterName, vmid), detail, ip); err != nil {
		return fmt.Errorf(auditWrapFmt, err)
	}

	return nil
}

// leaseDetail builds the structured audit detail the admin_audit viewer
// expects: {"summary": "...", "changes": [...]}.
func leaseDetail(summary string, change map[string]any) string {
	payload, err := json.Marshal(map[string]any{"summary": summary, "changes": []any{change}})
	if err != nil {
		return fmt.Sprintf(`{"summary":%q,"changes":[]}`, summary)
	}

	return string(payload)
}

// ErrNoVMIdentity reports that the live VM config carries no creation identity.
var ErrNoVMIdentity = errors.New("vm creation identity not found")

// CreationIdentity extracts a VM's creation identity from its flat config: the
// creation time from the meta property, or the vmgenid when that is absent.
// Empty when the config has neither.
func CreationIdentity(cfg map[string]string) string {
	fields := map[string]string{}

	for part := range strings.SplitSeq(cfg["meta"], ",") {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}

	for _, key := range []string{"creation-time", "ctime", "vmgenid"} {
		if fields[key] != "" {
			return fields[key]
		}
	}

	return cfg["vmgenid"]
}

// BindLeaseIdentity records the VM's live creation identity on its lease. A
// VM without a lease is a no-op. The identity is read from the current config.
func BindLeaseIdentity(ctx context.Context, st *store.Store, reader cluster.SnapshotConfigReader, clusterName, node string, vmid int) error {
	if _, err := st.GetVMLease(ctx, clusterName, vmid); errors.Is(err, store.ErrLeaseNotFound) {
		return nil
	} else if err != nil {
		return err
	}

	cfg, err := reader.SnapshotConfig(ctx, node, vmid, "current")
	if err != nil {
		return fmt.Errorf("read vm config: %w", err)
	}

	identity := CreationIdentity(cfg)
	if identity == "" {
		return ErrNoVMIdentity
	}

	return st.SetLeaseIdentity(ctx, clusterName, vmid, identity)
}

// extendedExpiry is the expiry after an owner extension by days. It adds the
// days to the later of the current expiry and now, so an expired lease
// restarts from now and an active one grows. The result is capped at
// now + maxDays and is never earlier than current: an extension can never
// shorten a lease.
func extendedExpiry(current, now time.Time, days, maxDays int) time.Time {
	base := current
	if now.After(base) {
		base = now
	}

	next := base.Add(time.Duration(days) * leaseDay)

	if limit := now.Add(time.Duration(maxDays) * leaseDay); next.After(limit) {
		next = limit
	}

	if next.Before(current) {
		next = current
	}

	return next.UTC().Truncate(time.Second)
}
