//nolint:wsl_v5 // sweep steps are ordered guards, one per skip rule
package vm

import (
	"context"
	"fmt"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"slices"
	"time"
)

// ClusterIndexSource gives the sweeper each cluster's inventory and freshness.
// *inventory.Registry satisfies it.
type ClusterIndexSource interface {
	Index(name string) (*inventory.Index, error)
	IsIndexFresh(name string) bool
}

// GabaritSource reads a cluster's snapshot cap. *policy.Policy satisfies it.
type GabaritSource interface {
	Gabarit(ctx context.Context, clusterName string) (policy.Gabarit, error)
}

// SweepDependencies are the collaborators of one snapshot sweep pass.
type SweepDependencies struct {
	Store   SnapshotScheduleStore
	Indexes ClusterIndexSource
	Clients cluster.ClientProvider
	Policy  GabaritSource
	Log     *slog.Logger
	Now     func() time.Time
}

// sweepTarget is a VM the sweeper may snapshot or prune right now.
type sweepTarget struct {
	node   string
	reader cluster.SnapshotReader
	writer cluster.SnapshotWriter
	tasks  TaskReader
	max    int
}

// Failure codes stored on a schedule; the web UI translates them.
const (
	ScheduleFailureCapReached   = "cap_reached"
	ScheduleFailureCreateFailed = "create_failed"
	// ScheduleFailureUnavailable: a due snapshot could not be attempted
	// because the cluster or its data could not be read.
	ScheduleFailureUnavailable = "cluster_unavailable"
	ScheduleFailurePruneFailed = "prune_failed"
)

// ownership is the verdict of checkOwnership for one schedule.
type ownership int

const (
	// ownershipOK: the schedule belongs to the VM it names.
	ownershipOK ownership = iota
	// ownershipSkip: nothing is decided this pass (stale inventory, untagged VM).
	ownershipSkip
	// ownershipDrop: the schedule belongs to a VM that is gone or was replaced.
	ownershipDrop
)

// SweepSnapshots runs one pass over every schedule. For each valid schedule it
// takes the snapshot when due, then prunes auto snapshots to retain, on every
// pass, so a failed prune is retried in 15 minutes rather than a day. Each
// schedule is handled on its own; a failure skips that VM only. It never
// returns an error; problems are logged.
func SweepSnapshots(ctx context.Context, deps SweepDependencies) {
	schedules, err := deps.Store.ListSnapshotSchedules(ctx)
	if err != nil {
		deps.Log.ErrorContext(ctx, "snapshot sweep: list schedules failed", "component", "snapshots", "error", err)

		return
	}

	now := deps.Now()
	for _, schedule := range schedules {
		if ctx.Err() != nil {
			return
		}

		// Ownership is checked on every pass, due or not, so a reused VMID
		// never receives a snapshot or a prune from a stale schedule.
		machine, verdict := checkOwnership(ctx, deps, schedule)
		if verdict == ownershipDrop {
			pruneOrphanSchedule(ctx, deps, schedule)
		}

		// "off" keeps the snapshots already taken: no prune, since its
		// retain is 0 and pruning would delete every auto snapshot.
		if verdict != ownershipOK || schedule.Frequency == ScheduleOff {
			continue
		}

		sweepSchedule(ctx, deps, schedule, machine, now)
	}
}

// checkOwnership confirms the schedule still belongs to its VM: the index is
// fresh, the VMID exists, the VM is in the pool recorded when the schedule was
// set, and it carries the pvmss tag. A pool mismatch means the VMID was reused
// (drop); an untagged VM is left alone (skip).
func checkOwnership(ctx context.Context, deps SweepDependencies, schedule store.SnapshotSchedule) (cluster.VM, ownership) {
	log := deps.Log.With("component", "snapshots", "cluster", schedule.Cluster, "vmid", schedule.VMID)

	// An index that was never refreshed reports no VMs; treating that as
	// "VM gone" would drop every schedule after a restart.
	if !deps.Indexes.IsIndexFresh(schedule.Cluster) {
		log.WarnContext(ctx, "snapshot sweep: cluster inventory not fresh, skipping", "event", "snapshot_sweep_skipped")
		markDueSkipped(ctx, deps, schedule, log)

		return cluster.VM{}, ownershipSkip
	}

	index, err := deps.Indexes.Index(schedule.Cluster)
	if err != nil || index == nil {
		log.WarnContext(ctx, "snapshot sweep: cluster index unavailable, skipping", "event", "snapshot_sweep_skipped", "error", err)
		markDueSkipped(ctx, deps, schedule, log)

		return cluster.VM{}, ownershipSkip
	}

	machine, ok := index.ByVMID[schedule.VMID]
	if !ok {
		return cluster.VM{}, ownershipDrop
	}

	if machine.Pool != schedule.Pool {
		log.WarnContext(ctx, "snapshot sweep: vmid now in another pool, dropping schedule", "event", "snapshot_schedule_pruned")

		return cluster.VM{}, ownershipDrop
	}

	if !slices.Contains(machine.Tags, pvmssTag) {
		log.WarnContext(ctx, "snapshot sweep: vm not managed by pvmss, skipping", "event", "snapshot_sweep_skipped")

		return cluster.VM{}, ownershipSkip
	}

	return machine, ownershipOK
}

// sweepSchedule takes the due snapshot (awaited), then prunes auto snapshots
// to retain counted over the current list, the new snapshot included.
func sweepSchedule(ctx context.Context, deps SweepDependencies, schedule store.SnapshotSchedule, machine cluster.VM, now time.Time) {
	log := deps.Log.With("component", "snapshots", "cluster", schedule.Cluster, "vmid", schedule.VMID)

	target, ok := resolveSweepTarget(ctx, deps, schedule, machine, log)
	if !ok {
		return
	}

	snapshots, err := target.reader.ListSnapshots(ctx, target.node, schedule.VMID)
	if err != nil {
		log.WarnContext(ctx, "snapshot sweep: list snapshots failed, skipping", "event", "snapshot_sweep_skipped", "error", err)
		markDueSkipped(ctx, deps, schedule, log)

		return
	}

	snapshots = visibleSnapshots(snapshots)

	due := IsSnapshotDue(schedule, now)
	taken := due && takeScheduledSnapshot(ctx, deps, schedule, target, snapshots, now, log)
	if taken {
		// The snapshot is awaited, so it is in the list now.
		snapshots, err = target.reader.ListSnapshots(ctx, target.node, schedule.VMID)
		if err != nil {
			log.WarnContext(ctx, "snapshot sweep: relist after create failed, prune deferred", "event", "snapshot_sweep_prune_failed", "error", err)

			return
		}

		snapshots = visibleSnapshots(snapshots)
	}

	pruned := pruneAutoSnapshots(ctx, deps, schedule, target, snapshots, log)

	// A due snapshot that failed keeps its own, more useful, failure.
	switch {
	case !pruned && (!due || taken):
		markSweepFailure(ctx, deps, schedule, ScheduleFailurePruneFailed, now, log)
	case pruned && !due && schedule.LastFailure != "":
		// Nothing is due and nothing failed: an older failure no longer applies.
		if err := deps.Store.ClearSnapshotScheduleFailure(ctx, schedule.Cluster, schedule.VMID); err != nil {
			log.ErrorContext(ctx, "snapshot sweep: clear failure failed", "event", "snapshot_sweep_failed", "error", err)
		}
	}
}

// takeScheduledSnapshot creates the due snapshot and waits for its task. It
// returns true only when the snapshot was created. A full cap is audited and
// blocks the create; a failed task leaves last_run_at untouched, so the next
// pass retries.
func takeScheduledSnapshot(ctx context.Context, deps SweepDependencies, schedule store.SnapshotSchedule, target sweepTarget, snapshots []Snapshot, now time.Time, log *slog.Logger) bool {
	if target.max > 0 && len(snapshots) >= target.max {
		// Never overflow the cap: manual snapshots may have filled it.
		log.WarnContext(ctx, "snapshot sweep: snapshot cap reached, scheduled snapshot blocked", "event", "snapshot_sweep_blocked")
		recordSweepAction(ctx, deps, schedule, AuditActionScheduleBlocked, log)
		markSweepFailure(ctx, deps, schedule, ScheduleFailureCapReached, now, log)

		return false
	}

	upid, err := target.writer.CreateSnapshot(ctx, target.node, schedule.VMID, AutoSnapshotName(now), autoSnapshotDescription, false)
	if err == nil {
		err = awaitSweepTask(ctx, target.tasks, upid)
	}
	if err != nil {
		log.WarnContext(ctx, "snapshot sweep: create snapshot failed, retry next pass", "event", "snapshot_sweep_failed", "error", err)
		markSweepFailure(ctx, deps, schedule, ScheduleFailureCreateFailed, now, log)

		return false
	}

	if err := deps.Store.MarkSnapshotScheduleRun(ctx, schedule.Cluster, schedule.VMID, now); err != nil {
		log.ErrorContext(ctx, "snapshot sweep: mark run failed", "event", "snapshot_sweep_failed", "error", err)
	}

	recordSweepAction(ctx, deps, schedule, AuditActionSnapshotAuto, log)

	return true
}

// resolveSweepTarget decides whether the VM can be touched now: a client with
// snapshot and task capability, a readable cap, and no task holding a lock.
func resolveSweepTarget(ctx context.Context, deps SweepDependencies, schedule store.SnapshotSchedule, machine cluster.VM, log *slog.Logger) (sweepTarget, bool) {
	client, err := deps.Clients.Client(schedule.Cluster)
	if err != nil {
		log.WarnContext(ctx, "snapshot sweep: cluster client unavailable, skipping", "event", "snapshot_sweep_skipped", "error", err)
		markDueSkipped(ctx, deps, schedule, log)

		return sweepTarget{}, false
	}

	reader, readerOK := client.(cluster.SnapshotReader)
	writer, writerOK := client.(cluster.SnapshotWriter)
	statusReader, statusOK := client.(cluster.VMStatusReader)
	tasks, tasksOK := client.(TaskReader)
	if !readerOK || !writerOK || !statusOK || !tasksOK {
		log.WarnContext(ctx, "snapshot sweep: cluster lacks snapshot capability, skipping", "event", "snapshot_sweep_skipped")
		markDueSkipped(ctx, deps, schedule, log)

		return sweepTarget{}, false
	}

	gabarit, err := deps.Policy.Gabarit(ctx, schedule.Cluster)
	if err != nil {
		log.WarnContext(ctx, "snapshot sweep: snapshot cap unavailable, skipping", "event", "snapshot_sweep_skipped", "error", err)
		markDueSkipped(ctx, deps, schedule, log)

		return sweepTarget{}, false
	}

	live, err := statusReader.VMStatus(ctx, machine.Node, schedule.VMID)
	if err != nil {
		log.WarnContext(ctx, "snapshot sweep: vm status read failed, skipping", "event", "snapshot_sweep_skipped", "error", err)
		markDueSkipped(ctx, deps, schedule, log)

		return sweepTarget{}, false
	}

	// A running task (backup, snapshot, migrate...) holds a lock; retry on the
	// next pass rather than fight it. Stopped VMs are snapshotted too.
	if live.Lock != "" {
		log.DebugContext(ctx, "snapshot sweep: vm locked, retry next pass", "event", "snapshot_sweep_skipped", "lock", live.Lock)

		return sweepTarget{}, false
	}

	return sweepTarget{node: machine.Node, reader: reader, writer: writer, tasks: tasks, max: gabarit.MaxSnapshots}, true
}

// awaitSweepTask waits for a cluster task to finish. A task that fails, does
// not settle within MaxCreateTaskWait, or cannot be read is an error.
func awaitSweepTask(ctx context.Context, tasks TaskReader, upid string) error {
	waitCtx, cancel := context.WithTimeout(ctx, MaxCreateTaskWait)
	defer cancel()

	status, settled, readErr := AwaitTask(waitCtx, tasks, upid)
	if readErr != nil {
		return fmt.Errorf("read task %s: %w", upid, readErr)
	}

	if !settled {
		return fmt.Errorf("task %s did not finish within %s", upid, MaxCreateTaskWait)
	}

	if status.State != cluster.TaskOK {
		return fmt.Errorf("task %s failed: %s", upid, status.ExitMessage)
	}

	return nil
}

// pruneOrphanSchedule drops a schedule whose VMID is gone or was reused in
// another pool. It never touches snapshots.
func pruneOrphanSchedule(ctx context.Context, deps SweepDependencies, schedule store.SnapshotSchedule) {
	log := deps.Log.With("component", "snapshots", "cluster", schedule.Cluster, "vmid", schedule.VMID)

	if err := deps.Store.DeleteSnapshotSchedule(ctx, schedule.Cluster, schedule.VMID); err != nil {
		log.ErrorContext(ctx, "snapshot sweep: prune orphan schedule failed", "event", "snapshot_schedule_pruned", "error", err)

		return
	}

	log.InfoContext(ctx, "snapshot sweep: schedule pruned, vm gone or reused", "event", "snapshot_schedule_pruned")
}

// pruneAutoSnapshots deletes the oldest auto snapshots beyond retain. Each
// delete is awaited. A failed delete is logged and left in the list, so the
// next pass sees it again. It returns false when any delete failed.
func pruneAutoSnapshots(ctx context.Context, deps SweepDependencies, schedule store.SnapshotSchedule, target sweepTarget, snapshots []Snapshot, log *slog.Logger) bool {
	ok := true

	for _, old := range AutoSnapshotsToPrune(snapshots, schedule.Retain) {
		upid, err := target.writer.DeleteSnapshot(ctx, target.node, schedule.VMID, old.Name)
		if err == nil {
			err = awaitSweepTask(ctx, target.tasks, upid)
		}
		if err != nil {
			log.WarnContext(ctx, "snapshot sweep: prune failed, retry next pass", "event", "snapshot_sweep_prune_failed", "snapshot", old.Name, "error", err)

			ok = false

			continue
		}

		recordSweepAction(ctx, deps, schedule, AuditActionSnapshotPrune, log)
	}

	return ok
}

// markDueSkipped records a skip only when a snapshot was due: an outage between
// two scheduled runs costs the user nothing, so it raises no alert.
func markDueSkipped(ctx context.Context, deps SweepDependencies, schedule store.SnapshotSchedule, log *slog.Logger) {
	now := deps.Now()
	if IsSnapshotDue(schedule, now) {
		markSweepFailure(ctx, deps, schedule, ScheduleFailureUnavailable, now, log)
	}
}

func markSweepFailure(ctx context.Context, deps SweepDependencies, schedule store.SnapshotSchedule, failure string, now time.Time, log *slog.Logger) {
	if err := deps.Store.MarkSnapshotScheduleFailure(ctx, schedule.Cluster, schedule.VMID, failure, now); err != nil {
		log.ErrorContext(ctx, "snapshot sweep: mark failure failed", "event", "snapshot_sweep_failed", "error", err)
	}
}

func recordSweepAction(ctx context.Context, deps SweepDependencies, schedule store.SnapshotSchedule, action string, log *slog.Logger) {
	if err := deps.Store.RecordAction(ctx, SystemActor, schedule.Cluster, schedule.VMID, action); err != nil {
		log.ErrorContext(ctx, "snapshot sweep: audit row failed", "event", "snapshot_sweep_audit_failed", "action", action, "error", err)
	}
}
