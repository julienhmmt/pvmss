// Package pools owns pool provisioning and cascade deletion orchestration.
//
//nolint:wsl_v5 // domain steps are kept adjacent for readable orchestration
package pools

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"strings"
)

// ErrProjectionNotReady means the inventory has not completed its first refresh.
var ErrProjectionNotReady = errors.New("inventory projection not ready")

// PoolSummary is the API-independent pool list row.
type PoolSummary struct {
	Name    string
	Comment string
	Total   int
	Running int
	Stopped int
}

// List returns discovered pools joined with the current inventory breakdown.
func List(ctx context.Context, client cluster.Client, projection *inventory.Projection, search string) ([]PoolSummary, error) {
	pools, err := client.ListPools(ctx)
	if err != nil {
		return nil, err
	}
	index := projection.Load()
	if index == nil {
		return nil, ErrProjectionNotReady
	}
	needle := strings.ToLower(strings.TrimSpace(search))
	rows := make([]PoolSummary, 0, len(pools))
	for _, pool := range pools {
		if needle != "" && !strings.Contains(strings.ToLower(pool.Name), needle) {
			continue
		}
		rows = append(rows, summarize(pool, index.ByPool[pool.Name]))
	}
	return rows, nil
}

func summarize(pool cluster.Pool, members []cluster.VM) PoolSummary {
	row := PoolSummary{Name: pool.Name, Comment: pool.Comment, Total: len(members)}
	for _, member := range members {
		if member.Status == cluster.VMRunning {
			row.Running++
		}
	}
	row.Stopped = row.Total - row.Running
	return row
}
