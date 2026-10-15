package policy

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"slices"
	"time"
)

var (
	// ErrInventoryNotReady - the projection is wired but has not loaded yet,
	// so pool usage and names cannot be counted.
	ErrInventoryNotReady = errors.New("inventory not loaded yet")
	// ErrNameReserved - the name is used by a VM in the pool or by a create
	// still in flight.
	ErrNameReserved = errors.New("name already used in pool")
)

// createReservationTTL bounds how long an in-flight create counts before the
// projection shows it. Longer than vm.MaxCreateTaskWait plus refresh backoff.
const createReservationTTL = 15 * time.Minute

type pendingCreate struct {
	id    uint64
	pool  string
	name  string
	until time.Time
}

// ReserveCreate checks the pool quota and name uniqueness, counting creates
// that were dispatched but are not in the projection yet, and reserves one
// slot for name. Call release when the create fails; on success the slot
// clears itself once the projection shows the VM, or after the TTL.
//
// shortcut: per-process state, valid for the single-replica deployment;
// move to SQLite if replicas > 1 become supported.
func (service *Policy) ReserveCreate(ctx context.Context, clusterName string, actor auth.Identity, name string) (func(), error) {
	if service.projection != nil && service.projection.Load() == nil {
		return nil, ErrInventoryNotReady
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	inflight := service.inflightLocked(actor.Pool)

	if err := service.checkQuota(ctx, clusterName, actor, len(inflight)); err != nil {
		return nil, err
	}

	if slices.Contains(inflight, name) || service.PoolHasName(actor.Pool, name) {
		return nil, ErrNameReserved
	}

	service.nextID++
	id := service.nextID
	service.pending = append(service.pending, pendingCreate{id: id, pool: actor.Pool, name: name, until: time.Now().Add(createReservationTTL)})

	return func() {
		service.mu.Lock()
		defer service.mu.Unlock()

		service.pending = slices.DeleteFunc(service.pending, func(p pendingCreate) bool { return p.id == id })
	}, nil
}

// inflightLocked drops expired reservations and those the projection already
// shows, then returns the names still in flight for pool. Caller holds mu.
func (service *Policy) inflightLocked(pool string) []string {
	now := time.Now()
	service.pending = slices.DeleteFunc(service.pending, func(p pendingCreate) bool {
		return now.After(p.until) || service.PoolHasName(p.pool, p.name)
	})

	var names []string

	for _, p := range service.pending {
		if p.pool == pool {
			names = append(names, p.name)
		}
	}

	return names
}
