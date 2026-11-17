package cluster

import (
	"slices"
	"sync"
	"time"
)

// discoveryTTL is how long bridge and ISO listings are reused. Both change
// rarely and only in Proxmox (PVMSS never creates them), while one listing
// costs a call per node or per ISO storage.
//
// shortcut: in-process and per replica, so two replicas may disagree for up to
// discoveryTTL; move to a shared store if PVMSS ever runs several replicas.
const discoveryTTL = 30 * time.Second

var (
	bridgeCache = newTTLCache[[]Bridge](discoveryTTL, time.Now)
	isoCache    = newTTLCache[[]ISOImage](discoveryTTL, time.Now)
)

// ttlCache keeps one successful slice per key for ttl. Errors are never
// cached. Concurrent misses on one key may both fetch; the last one wins.
type ttlCache[S ~[]E, E any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]ttlEntry[S]
}

type ttlEntry[S any] struct {
	value   S
	expires time.Time
}

func newTTLCache[S ~[]E, E any](ttl time.Duration, now func() time.Time) *ttlCache[S, E] {
	return &ttlCache[S, E]{ttl: ttl, now: now, entries: map[string]ttlEntry[S]{}}
}

// get returns a copy of the cached value for key, calling fetch on a miss.
func (c *ttlCache[S, E]) get(key string, fetch func() (S, error)) (S, error) {
	c.mu.Lock()
	entry, ok := c.entries[key]
	c.mu.Unlock()

	if ok && c.now().Before(entry.expires) {
		return slices.Clone(entry.value), nil
	}

	value, err := fetch()
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[key] = ttlEntry[S]{value: value, expires: c.now().Add(c.ttl)}
	c.mu.Unlock()

	return slices.Clone(value), nil
}
