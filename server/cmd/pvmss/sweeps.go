package main

import (
	"context"
	"log/slog"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"time"
)

// auditPruneInterval is how often the daily prune tick fires.
const auditPruneInterval = 24 * time.Hour

// snapshotSweepInterval is how often the scheduled-snapshot sweep runs. A
// schedule is due once its interval (daily or weekly) has elapsed since its
// last run, so a 15-minute tick adds at most that much lateness.
const snapshotSweepInterval = 15 * time.Minute

// runSnapshotSweep runs vm.SweepSnapshots on a fixed tick until ctx ends. It
// does not sweep at startup: inventory is still warming up, and the sweeper
// would otherwise see an unfresh index on every schedule.
func runSnapshotSweep(ctx context.Context, deps vm.SweepDependencies) {
	ticker := time.NewTicker(snapshotSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			vm.SweepSnapshots(ctx, deps)
		}
	}
}

// runAuditPrune deletes audit_log rows older than the configured retention,
// once at startup then every auditPruneInterval. It logs the deleted count at
// info level so retention activity is visible in aggregated logs. A prune
// failure is logged but never stops the tick - the next tick retries.
func runAuditPrune(ctx context.Context, st *store.Store, log *slog.Logger) {
	prune := func() {
		cfg, err := st.GetAuditConfig(ctx)
		if err != nil {
			log.ErrorContext(ctx, "audit prune: get config failed", "component", "audit", "error", err)
			return
		}

		n, err := st.PruneAuditLog(ctx, cfg.RetentionDays)
		if err != nil {
			log.ErrorContext(ctx, "audit prune failed", "component", "audit", "error", err)
			return
		}

		if n > 0 {
			log.InfoContext(ctx, "audit prune completed", "component", "audit", "deleted", n, "retentionDays", cfg.RetentionDays)
		}
	}

	log.DebugContext(ctx, "audit prune loop started", "component", "audit", "intervalMs", auditPruneInterval.Milliseconds())
	defer log.DebugContext(ctx, "audit prune loop stopped", "component", "audit")

	prune()

	ticker := time.NewTicker(auditPruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}
