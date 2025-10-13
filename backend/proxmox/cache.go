package proxmox

import (
	"container/list"
	"sync"
	"time"
)

// LRUCache implements a Least Recently Used cache with TTL support
type LRUCache struct {
	maxEntries int
	ttl        time.Duration
	mu         sync.RWMutex
	cache      map[string]*list.Element
	lru        *list.List
}

// entry represents a cache entry with its key, value, and timestamp
type entry struct {
	key       string
	value     []byte
	timestamp time.Time
}

// NewLRUCache creates a new LRU cache with the specified maximum entries and TTL
func NewLRUCache(maxEntries int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		maxEntries: maxEntries,
		ttl:        ttl,
		cache:      make(map[string]*list.Element),
		lru:        list.New(),
	}
}

// Get retrieves a value from the cache, returns nil if not found or expired
func (c *LRUCache) Get(key string) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.cache[key]
	if !ok {
		return nil
	}

	ent := elem.Value.(*entry)

	// Check if entry has expired
	if time.Since(ent.timestamp) > c.ttl {
		c.removeElement(elem)
		return nil
	}

	// Move to front (most recently used)
	c.lru.MoveToFront(elem)
	return ent.value
}

// Set adds or updates a value in the cache
func (c *LRUCache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key exists, update it and move to front
	if elem, ok := c.cache[key]; ok {
		c.lru.MoveToFront(elem)
		ent := elem.Value.(*entry)
		ent.value = value
		ent.timestamp = time.Now()
		return
	}

	// Add new entry
	ent := &entry{
		key:       key,
		value:     value,
		timestamp: time.Now(),
	}
	elem := c.lru.PushFront(ent)
	c.cache[key] = elem

	// Evict oldest entry if cache is full
	if c.lru.Len() > c.maxEntries {
		oldest := c.lru.Back()
		if oldest != nil {
			c.removeElement(oldest)
		}
	}
}

// Delete removes a specific entry from the cache
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.removeElement(elem)
	}
}

// Clear removes all entries from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*list.Element)
	c.lru.Init()
}

// Len returns the current number of entries in the cache
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

// removeElement removes an element from the cache (caller must hold lock)
func (c *LRUCache) removeElement(elem *list.Element) {
	c.lru.Remove(elem)
	ent := elem.Value.(*entry)
	delete(c.cache, ent.key)
}

// CleanExpired removes all expired entries from the cache
func (c *LRUCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toRemove []*list.Element
	now := time.Now()

	// Iterate through all entries to find expired ones
	for elem := c.lru.Back(); elem != nil; elem = elem.Prev() {
		ent := elem.Value.(*entry)
		if now.Sub(ent.timestamp) > c.ttl {
			toRemove = append(toRemove, elem)
		}
	}

	// Remove expired entries
	for _, elem := range toRemove {
		c.removeElement(elem)
	}

	return len(toRemove)
}
