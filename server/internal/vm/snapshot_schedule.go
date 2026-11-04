package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"sort"
	"strings"
	"time"
)

// Schedule frequencies accepted by the API. "off" removes the automatic run.
const (
	ScheduleOff    = "off"
	ScheduleDaily  = "daily"
	ScheduleWeekly = "weekly"
)

// AutoSnapshotPrefix marks snapshots taken by the scheduler. Retention only
// ever counts and prunes names with this prefix; manual snapshots never match.
const AutoSnapshotPrefix = "pvmss-auto-"

// Audit actions for the scheduler. Actor is SystemActor for sweeper rows.
const (
	AuditActionScheduleSet     = "vm_snapshot_schedule_set"
	AuditActionScheduleClear   = "vm_snapshot_schedule_clear"
	AuditActionSnapshotAuto    = "vm_snapshot_auto"
	AuditActionSnapshotPrune   = "vm_snapshot_auto_prune"
	AuditActionScheduleBlocked = "vm_snapshot_schedule_blocked"
	// SystemActor is the audit identity of background work.
	SystemActor = "system"
)

const (
	autoSnapshotTimeFormat  = "20060102-1504"
	autoSnapshotDescription = "scheduled by PVMSS"
)

var (
	// ErrInvalidSnapshotSchedule reports an unknown frequency or a retain below 1.
	ErrInvalidSnapshotSchedule = errors.New("invalid snapshot schedule")
	// ErrScheduleRetainExceedsMax reports a retain that would fill the snapshot cap.
	ErrScheduleRetainExceedsMax = errors.New("snapshot retention must stay below the snapshot limit")
)

// SnapshotScheduleStore is the persistence the scheduler needs. *store.Store
// satisfies it.
type SnapshotScheduleStore interface {
	UpsertSnapshotSchedule(ctx context.Context, cluster string, vmid int, pool, frequency string, retain int) error
	DeleteSnapshotSchedule(ctx context.Context, cluster string, vmid int) error
	ListSnapshotSchedules(ctx context.Context) ([]store.SnapshotSchedule, error)
	MarkSnapshotScheduleRun(ctx context.Context, cluster string, vmid int, at time.Time) error
	MarkSnapshotScheduleFailure(ctx context.Context, cluster string, vmid int, failure string, at time.Time) error
	RecordAction(ctx context.Context, actor, cluster string, vmid int, action string) error
}

// ScheduleDependencies carries what a user-facing schedule write needs.
type ScheduleDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Policy      *policy.Policy
	Store       SnapshotScheduleStore
}

// IsSnapshotDue reports whether a schedule should fire at now. A schedule that
// never ran is due at once; otherwise the last run plus the interval must be
// in the past.
func IsSnapshotDue(schedule store.SnapshotSchedule, now time.Time) bool {
	interval, ok := scheduleInterval(schedule.Frequency)
	if !ok {
		return false
	}

	if schedule.LastRunAt.IsZero() {
		return true
	}

	return schedule.LastRunAt.Add(interval).Before(now)
}

func scheduleInterval(frequency string) (time.Duration, bool) {
	switch frequency {
	case ScheduleDaily:
		return 24 * time.Hour, true
	case ScheduleWeekly:
		return 7 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

// AutoSnapshotName is the name of the snapshot taken at now, e.g.
// pvmss-auto-20260101-0300. It is a valid snapshot name (under 40 characters).
func AutoSnapshotName(now time.Time) string {
	return AutoSnapshotPrefix + now.UTC().Format(autoSnapshotTimeFormat)
}

// IsAutoSnapshot reports whether a snapshot was taken by the scheduler.
func IsAutoSnapshot(name string) bool {
	return strings.HasPrefix(name, AutoSnapshotPrefix)
}

// AutoSnapshotsToPrune returns the scheduler-owned snapshots beyond the newest
// keep, oldest first. Manual snapshots are never returned.
func AutoSnapshotsToPrune(snapshots []Snapshot, keep int) []Snapshot {
	auto := make([]Snapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if IsAutoSnapshot(snapshot.Name) {
			auto = append(auto, snapshot)
		}
	}

	sort.SliceStable(auto, func(i, j int) bool { return auto[i].CreatedAt.Before(auto[j].CreatedAt) })

	excess := len(auto) - max(keep, 0)
	if excess <= 0 {
		return nil
	}

	return auto[:excess]
}

// ValidateSnapshotSchedule checks a frequency and retain against the snapshot
// cap. A maxSnapshots of 0 means no cap.
func ValidateSnapshotSchedule(frequency string, retain, maxSnapshots int) error {
	switch frequency {
	case ScheduleOff:
		return nil
	case ScheduleDaily, ScheduleWeekly:
	default:
		return fmt.Errorf("%w: unknown frequency %q", ErrInvalidSnapshotSchedule, frequency)
	}

	if retain < 1 {
		return fmt.Errorf("%w: retain must be at least 1", ErrInvalidSnapshotSchedule)
	}

	if maxSnapshots > 0 && retain >= maxSnapshots {
		return fmt.Errorf("%w: retain %d must be below the limit of %d", ErrScheduleRetainExceedsMax, retain, maxSnapshots)
	}

	return nil
}

// SetSnapshotSchedule validates and stores a VM's automatic snapshot policy.
// Off stores retain 0.
func SetSnapshotSchedule(ctx context.Context, deps ScheduleDependencies, frequency string, retain int) error {
	entity, err := resolveScheduleTarget(deps)
	if err != nil {
		return err
	}

	if deps.Policy == nil {
		return policy.ErrUnavailable
	}

	gabarit, err := deps.Policy.Gabarit(ctx, deps.ClusterName)
	if err != nil {
		return err
	}

	if frequency == ScheduleOff {
		retain = 0
	}

	if err := ValidateSnapshotSchedule(frequency, retain, gabarit.MaxSnapshots); err != nil {
		return err
	}

	if err := deps.Store.UpsertSnapshotSchedule(ctx, deps.ClusterName, deps.VMID, entity.Pool, frequency, retain); err != nil {
		return fmt.Errorf("store snapshot schedule: %w", err)
	}

	return recordScheduleAction(ctx, deps, AuditActionScheduleSet)
}

// ClearSnapshotSchedule removes a VM's automatic snapshot policy. Existing
// auto snapshots are kept.
func ClearSnapshotSchedule(ctx context.Context, deps ScheduleDependencies) error {
	if _, err := resolveScheduleTarget(deps); err != nil {
		return err
	}

	if err := deps.Store.DeleteSnapshotSchedule(ctx, deps.ClusterName, deps.VMID); err != nil {
		return fmt.Errorf("delete snapshot schedule: %w", err)
	}

	return recordScheduleAction(ctx, deps, AuditActionScheduleClear)
}

func resolveScheduleTarget(deps ScheduleDependencies) (Entity, error) {
	if deps.Index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
}

func recordScheduleAction(ctx context.Context, deps ScheduleDependencies, action string) error {
	if err := deps.Store.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, action); err != nil {
		return fmt.Errorf("record snapshot schedule audit: %w", err)
	}

	return nil
}
