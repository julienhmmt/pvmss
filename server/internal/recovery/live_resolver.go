package recovery

import (
	"context"
	"pvmss/server/internal/cluster"
	"sync"
)

// liveStorageResolver adapts a cluster.Client to StorageNodeResolver.
// It calls Snapshot at most once per
// resolver instance — the result (or error) is cached and reused for every
// subsequent StorageNodes lookup, so a run with N storages still performs
// exactly one live Proxmox call.
//
// A Snapshot failure never aborts the run: it is cached like a normal
// result, and every storage row surfaces it as a per-row skip reason via
// mapStorages' existing error handling.
type liveStorageResolver struct {
	client cluster.Client

	once     sync.Once
	byName   map[string][]string
	fetchErr error
}

// newLiveStorageResolver returns a StorageNodeResolver backed by a live
// cluster.Client. Callers only construct one when Proxmox credentials are
// available; when they are not, Run passes a nil resolver instead.
func newLiveStorageResolver(client cluster.Client) *liveStorageResolver {
	return &liveStorageResolver{client: client}
}

func (r *liveStorageResolver) StorageNodes(ctx context.Context, storageName string) ([]string, error) {
	r.once.Do(func() {
		snap, err := r.client.Snapshot(ctx)
		if err != nil {
			r.fetchErr = err
			return
		}

		r.byName = make(map[string][]string, len(snap.Storages))
		for _, s := range snap.Storages {
			r.byName[s.Name] = append(r.byName[s.Name], s.Node)
		}
	})

	if r.fetchErr != nil {
		return nil, r.fetchErr
	}

	return r.byName[storageName], nil
}
