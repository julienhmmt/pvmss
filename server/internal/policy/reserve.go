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
	id      uint64
	cluster string
	pool    string
	name    string
	until   time.Time
}

// ReserveCreate checks the pool quota and name uniqueness, counting creates
// that were dispatched but are not in the projection yet, and reserves one
// slot for name. Call release when the create fails; on success the slot
// clears itself once the projection shows the VM, or after the TTL.
//
// shortcut: per-process state, valid for the single-replica deployment;
// move to SQLite if replicas > 1 become supported.
func (service *Policy) ReserveCreate(ctx context.Context, clusterName string, actor auth.Identity, name string) (func(), error) {
	index, err := service.indexFor(clusterName)
	if err != nil {
		return nil, err
	}

	if service.inventoryExpected() && index == nil {
		return nil, ErrInventoryNotReady
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	inflight, err := service.inflightLocked(clusterName, actor.Pool)
	if err != nil {
		return nil, err
	}

	if err := service.checkQuota(ctx, clusterName, actor, len(inflight)); err != nil {
		return nil, err
	}

	named, err := service.PoolHasName(clusterName, actor.Pool, name)
	if err != nil {
		return nil, err
	}

	if slices.Contains(inflight, name) || named {
		return nil, ErrNameReserved
	}

	service.nextID++
	id := service.nextID
	service.pending = append(service.pending, pendingCreate{id: id, cluster: clusterName, pool: actor.Pool, name: name, until: time.Now().Add(createReservationTTL)})

	return func() {
		service.mu.Lock()
		defer service.mu.Unlock()

		service.pending = slices.DeleteFunc(service.pending, func(p pendingCreate) bool { return p.id == id })
	}, nil
}

// ForgetCreate drops the reservation for a VM that was just deleted. Reservations
// are only swept during ReserveCreate, so one can outlive its VM.
//
// shortcut: a VM deleted or renamed outside this path keeps its name reserved
// until the TTL; sweep on each inventory refresh if that ever matters.
func (service *Policy) ForgetCreate(clusterName, pool, name string) {
	service.mu.Lock()
	defer service.mu.Unlock()

	service.pending = slices.DeleteFunc(service.pending, func(p pendingCreate) bool {
		return p.cluster == clusterName && p.pool == pool && p.name == name
	})
}

// inventoryExpected reports whether a projection source is configured. A
// deployment without one is the degraded unit-test mode that tolerates an
// unloaded inventory instead of refusing every create.
func (service *Policy) inventoryExpected() bool {
	return service.projections != nil
}

// inflightLocked drops expired reservations and those the projection already
// shows, then returns the names still in flight for clusterName's pool. Caller
// holds mu.
func (service *Policy) inflightLocked(clusterName, pool string) ([]string, error) {
	now := time.Now()
	kept := make([]pendingCreate, 0, len(service.pending))
	names := make([]string, 0, len(service.pending))

	for _, pending := range service.pending {
		if now.After(pending.until) {
			continue
		}

		named, err := service.PoolHasName(pending.cluster, pending.pool, pending.name)
		if err != nil && pending.cluster == clusterName {
			return nil, err
		}

		// A removed or renamed other cluster must not block this one; its TTL drops the entry.
		if err == nil && named {
			continue
		}

		kept = append(kept, pending)

		if pending.cluster == clusterName && pending.pool == pool {
			names = append(names, pending.name)
		}
	}

	service.pending = kept

	return names, nil
}
