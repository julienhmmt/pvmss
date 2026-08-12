//nolint:wsl_v5 // registry lifecycle operations keep synchronization adjacent
package inventory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"pvmss/server/internal/cluster"
	"sync"
	"time"
)

// ErrClusterNotFound is returned when an inventory entry is unknown or removed.
var ErrClusterNotFound = errors.New("inventory cluster not found")

// ErrDuplicateCluster is returned when an inventory entry already exists for a name.
var ErrDuplicateCluster = errors.New("duplicate inventory cluster")

// Source exposes the cluster-scoped indexes consumed by VM read paths.
type Source interface {
	All() map[string]*Index
}

// Registry owns one projection and refresh worker per active cluster.
type Registry struct {
	mu       sync.RWMutex
	provider cluster.ClientProvider
	entries  map[string]*registryEntry
	interval time.Duration
	options  []Option
	log      *slog.Logger
	started  bool
	// ctx is the parent lifecycle context for every worker goroutine. It is
	// captured at Start time so that Add-after-Start can derive a child context
	// for the new worker on the same root — late-added clusters share the
	// shutdown signal without callers having to thread the context back in.
	//nolint:containedctx // one lifecycle context owns all registry workers
	ctx context.Context
}

type registryEntry struct {
	projection *Projection
	worker     *Worker
	refresher  *Refresher
	cancel     context.CancelFunc
}

// NewRegistry builds independent inventory workers for every active client.
func NewRegistry(provider cluster.ClientProvider, interval time.Duration, log *slog.Logger, options ...Option) *Registry {
	if log == nil {
		log = slog.Default()
	}
	registry := &Registry{provider: provider, entries: make(map[string]*registryEntry), interval: interval, options: append([]Option(nil), options...), log: log}
	for _, name := range provider.List() {
		registry.addEntry(name)
	}
	return registry
}

// NewRegistryFromIndexes builds a registry around pre-populated projections,
// useful for pure domain and handler tests that do not need refresh goroutines.
func NewRegistryFromIndexes(indexes map[string]*Index) *Registry {
	registry := &Registry{entries: make(map[string]*registryEntry, len(indexes)), interval: time.Hour, log: slog.Default()}
	for name, index := range indexes {
		registry.entries[name] = &registryEntry{projection: NewProjectionFromIndex(index)}
	}
	return registry
}

// Start launches one independent refresh loop per active cluster.
func (registry *Registry) Start(ctx context.Context) {
	registry.mu.Lock()
	if registry.started {
		registry.mu.Unlock()
		return
	}
	registry.started = true
	registry.ctx = ctx
	for _, entry := range registry.entries {
		registry.startEntryLocked(entry)
	}
	registry.mu.Unlock()
}

// SetManualRefreshMinInterval updates the guard for every active cluster.
func (registry *Registry) SetManualRefreshMinInterval(interval time.Duration) {
	registry.mu.Lock()
	for _, entry := range registry.entries {
		if entry.refresher != nil {
			entry.refresher.minInterval = interval
		}
	}
	registry.mu.Unlock()
}

// Worker returns one cluster's background worker for integrations that need its lifecycle.
func (registry *Registry) Worker(name string) (*Worker, error) {
	registry.mu.RLock()
	entry, ok := registry.entries[name]
	registry.mu.RUnlock()
	if !ok || entry.worker == nil {
		return nil, ErrClusterNotFound
	}
	return entry.worker, nil
}

// Add creates one projection and worker for a newly registered active client.
func (registry *Registry) Add(name string) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.entries[name]; exists {
		return ErrDuplicateCluster
	}
	if registry.provider == nil {
		return errors.New("inventory client provider is not configured")
	}
	if _, err := registry.provider.Client(name); err != nil {
		return err
	}
	entry, err := registry.makeEntry(name)
	if err != nil {
		return err
	}
	registry.entries[name] = entry
	if registry.started {
		registry.startEntryLocked(entry)
	}
	return nil
}

// Remove stops and drops one cluster's worker and projection.
func (registry *Registry) Remove(name string) {
	registry.mu.Lock()
	entry, ok := registry.entries[name]
	if ok {
		if entry.cancel != nil {
			entry.cancel()
		}
		delete(registry.entries, name)
	}
	registry.mu.Unlock()
}

// Index returns the current immutable index for one active cluster.
func (registry *Registry) Index(name string) (*Index, error) {
	registry.mu.RLock()
	entry, ok := registry.entries[name]
	registry.mu.RUnlock()
	if !ok {
		return nil, ErrClusterNotFound
	}
	if entry.projection == nil {
		return nil, nil
	}
	return entry.projection.Load(), nil
}

// Projection returns the atomic projection for one active cluster.
func (registry *Registry) Projection(name string) (*Projection, error) {
	registry.mu.RLock()
	entry, ok := registry.entries[name]
	registry.mu.RUnlock()
	if !ok {
		return nil, ErrClusterNotFound
	}
	return entry.projection, nil
}

// Refresher returns the manual refresh guard for one active cluster.
func (registry *Registry) Refresher(name string) (*Refresher, error) {
	registry.mu.RLock()
	entry, ok := registry.entries[name]
	registry.mu.RUnlock()
	if !ok || entry.refresher == nil {
		return nil, ErrClusterNotFound
	}
	return entry.refresher, nil
}

// Refresh performs one refresh for a named cluster and leaves other projections untouched.
func (registry *Registry) Refresh(ctx context.Context, name string) (time.Time, error) {
	registry.mu.RLock()
	entry, ok := registry.entries[name]
	registry.mu.RUnlock()
	if !ok || entry.worker == nil {
		return time.Time{}, ErrClusterNotFound
	}
	return entry.worker.Refresh(ctx)
}

// StoreSnapshot publishes a successful explicit connection test into one projection.
func (registry *Registry) StoreSnapshot(name string, snapshot cluster.Snapshot) error {
	registry.mu.RLock()
	entry, ok := registry.entries[name]
	registry.mu.RUnlock()
	if !ok || entry.projection == nil {
		return ErrClusterNotFound
	}
	index := BuildIndexForCluster(name, snapshot)
	index.RefreshedAt = time.Now().UTC()
	entry.projection.store(&index)
	return nil
}

// All returns a copy of the active cluster-to-index map. A nil value means the
// cluster has not completed a successful refresh yet.
func (registry *Registry) All() map[string]*Index {
	registry.mu.RLock()
	result := make(map[string]*Index, len(registry.entries))
	for name, entry := range registry.entries {
		if entry.projection == nil {
			result[name] = nil
			continue
		}
		result[name] = entry.projection.Load()
	}
	registry.mu.RUnlock()
	return result
}

func (registry *Registry) addEntry(name string) {
	if registry.provider == nil {
		return
	}
	entry, err := registry.makeEntry(name)
	if err != nil {
		registry.log.Warn("skipping inventory entry with unavailable client", "component", "inventory", "cluster", name, "error", err)
		return
	}
	registry.entries[name] = entry
}

func (registry *Registry) makeEntry(name string) (*registryEntry, error) {
	client, err := registry.provider.Client(name)
	if err != nil {
		return nil, fmt.Errorf("inventory client for %q: %w", name, err)
	}
	projection := NewProjection()
	options := append([]Option{WithClusterName(name)}, registry.options...)
	worker := NewWorker(client, projection, registry.interval, registry.log, options...)
	return &registryEntry{projection: projection, worker: worker, refresher: NewRefresher(worker, registry.interval)}, nil
}

func (registry *Registry) startEntryLocked(entry *registryEntry) {
	if entry.worker == nil || entry.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(registry.ctx)
	entry.cancel = cancel
	go entry.worker.Run(ctx)
}
