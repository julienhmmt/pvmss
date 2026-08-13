//nolint:wsl_v5 // cascade steps are intentionally adjacent and ordered
package pools

import (
	"context"
	"log/slog"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"time"
)

const (
	deletePoll   = 50 * time.Millisecond
	cascadePause = 100 * time.Millisecond
)

var maxDeleteWait = 15 * time.Second

// DeleteResult is the stable response for a completed pool cascade.
type DeleteResult struct {
	Status      string
	UserDeleted bool
}

// Delete stops and purges pool members through T05's VM write paths, then
// removes the pool and makes a best-effort user deletion.
func Delete(ctx context.Context, actor auth.Identity, client cluster.Client, projection *inventory.Projection, clusterName, name string, writer cluster.Writer, audit vm.AuditRecorder, refresher vm.IndexRefresher) (DeleteResult, error) {
	if !actor.IsAdmin {
		return DeleteResult{}, ErrForbidden
	}
	if err := ensurePoolExists(ctx, client, name); err != nil {
		return DeleteResult{}, err
	}
	index := projection.Load()
	if index == nil {
		return DeleteResult{}, ErrProjectionNotReady
	}
	members := append([]cluster.VM(nil), index.ByPool[name]...)
	stopMembers(ctx, actor, projection, clusterName, members, writer, audit, refresher)
	pauseCascade(ctx, len(members) > 0)
	deleteMembers(ctx, actor, projection, clusterName, members, writer, audit, refresher)
	waitForEmpty(ctx, projection, name, len(members) > 0)
	if err := client.DeletePool(ctx, name); err != nil {
		return DeleteResult{}, err
	}
	userDeleted := deletePoolUser(ctx, client, name)
	return DeleteResult{Status: "deleted", UserDeleted: userDeleted}, nil
}

func ensurePoolExists(ctx context.Context, client cluster.Client, name string) error {
	pools, err := client.ListPools(ctx)
	if err != nil {
		return err
	}
	for _, pool := range pools {
		if pool.Name == name {
			return nil
		}
	}
	return ErrNotFound
}

func stopMembers(ctx context.Context, actor auth.Identity, projection *inventory.Projection, clusterName string, members []cluster.VM, writer cluster.Writer, audit vm.AuditRecorder, refresher vm.IndexRefresher) {
	for _, member := range members {
		if member.Status != cluster.VMRunning {
			continue
		}
		index := projection.Load()
		if index == nil {
			logCascadeError("stop", member, ErrProjectionNotReady)
			continue
		}
		if err := vm.Action(ctx, vm.BulkDeps{
			Actor:     actor,
			Writer:    writer,
			Audit:     audit,
			Refresher: refresher,
		}, index, clusterName, member.VMID, "stop"); err != nil {
			logCascadeError("stop", member, err)
		}
	}
}

func pauseCascade(ctx context.Context, needed bool) {
	if !needed {
		return
	}
	timer := time.NewTimer(cascadePause)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

func deleteMembers(ctx context.Context, actor auth.Identity, projection *inventory.Projection, clusterName string, members []cluster.VM, writer cluster.Writer, audit vm.AuditRecorder, refresher vm.IndexRefresher) {
	for _, member := range members {
		index := projection.Load()
		if index == nil {
			logCascadeError("delete", member, ErrProjectionNotReady)
			continue
		}
		if err := vm.Delete(ctx, index, actor, clusterName, member.VMID, writer, audit, refresher); err != nil {
			logCascadeError("delete", member, err)
		}
	}
}

func waitForEmpty(ctx context.Context, projection *inventory.Projection, name string, needed bool) {
	if !needed || poolEmpty(projection, name) {
		return
	}
	deadline := time.NewTimer(maxDeleteWait)
	defer deadline.Stop()
	ticker := time.NewTicker(deletePoll)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if poolEmpty(projection, name) {
				return
			}
		case <-deadline.C:
			slog.Default().Warn("pool cascade wait expired; deleting pool anyway", "pool", name)
			return
		case <-ctx.Done():
			slog.Default().Warn("pool cascade context ended; deleting pool anyway", "pool", name, "error", ctx.Err())
			return
		}
	}
}

func poolEmpty(projection *inventory.Projection, name string) bool {
	index := projection.Load()
	return index != nil && len(index.ByPool[name]) == 0
}

func deletePoolUser(ctx context.Context, client cluster.Client, name string) bool {
	username := name + "@pve"
	if err := client.DeleteUser(ctx, username); err != nil {
		slog.Default().Error("pool user deletion failed", "pool", name, "username", username, "error", err)
		return false
	}
	return true
}

func logCascadeError(action string, member cluster.VM, err error) {
	slog.Default().Error("pool cascade VM operation failed", "action", action, "vmid", member.VMID, "pool", member.Pool, "error", err)
}
