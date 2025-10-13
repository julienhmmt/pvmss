package proxmox

import (
	"sync"
	"testing"
	"time"
)

func TestLRUCache_BasicOperations(t *testing.T) {
	cache := NewLRUCache(3, 5*time.Second)

	// Test Set and Get
	cache.Set("key1", []byte("value1"))
	if val := cache.Get("key1"); string(val) != "value1" {
		t.Errorf("Expected 'value1', got '%s'", string(val))
	}

	// Test Get non-existent key
	if val := cache.Get("nonexistent"); val != nil {
		t.Errorf("Expected nil for non-existent key, got %v", val)
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	cache := NewLRUCache(2, 5*time.Second)

	cache.Set("key1", []byte("value1"))
	cache.Set("key2", []byte("value2"))
	cache.Set("key3", []byte("value3")) // Should evict key1

	if val := cache.Get("key1"); val != nil {
		t.Error("Expected key1 to be evicted")
	}
	if val := cache.Get("key2"); string(val) != "value2" {
		t.Error("Expected key2 to still exist")
	}
	if val := cache.Get("key3"); string(val) != "value3" {
		t.Error("Expected key3 to exist")
	}
}

func TestLRUCache_LRUOrdering(t *testing.T) {
	cache := NewLRUCache(2, 5*time.Second)

	cache.Set("key1", []byte("value1"))
	cache.Set("key2", []byte("value2"))

	// Access key1 to make it most recently used
	cache.Get("key1")

	// Add key3, should evict key2 (least recently used)
	cache.Set("key3", []byte("value3"))

	if val := cache.Get("key1"); val == nil {
		t.Error("Expected key1 to still exist (was recently accessed)")
	}
	if val := cache.Get("key2"); val != nil {
		t.Error("Expected key2 to be evicted (least recently used)")
	}
}

func TestLRUCache_TTL(t *testing.T) {
	cache := NewLRUCache(10, 100*time.Millisecond)

	cache.Set("key1", []byte("value1"))

	// Should exist immediately
	if val := cache.Get("key1"); val == nil {
		t.Error("Expected key1 to exist")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	if val := cache.Get("key1"); val != nil {
		t.Error("Expected key1 to be expired")
	}
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(10, 5*time.Second)

	cache.Set("key1", []byte("value1"))
	cache.Delete("key1")

	if val := cache.Get("key1"); val != nil {
		t.Error("Expected key1 to be deleted")
	}
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(10, 5*time.Second)

	cache.Set("key1", []byte("value1"))
	cache.Set("key2", []byte("value2"))

	if cache.Len() != 2 {
		t.Errorf("Expected cache length 2, got %d", cache.Len())
	}

	cache.Clear()

	if cache.Len() != 0 {
		t.Errorf("Expected cache length 0 after clear, got %d", cache.Len())
	}
}

func TestLRUCache_CleanExpired(t *testing.T) {
	cache := NewLRUCache(10, 100*time.Millisecond)

	cache.Set("key1", []byte("value1"))
	cache.Set("key2", []byte("value2"))

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Add a fresh key
	cache.Set("key3", []byte("value3"))

	// Clean expired entries
	cleaned := cache.CleanExpired()

	if cleaned != 2 {
		t.Errorf("Expected 2 entries to be cleaned, got %d", cleaned)
	}
	if cache.Len() != 1 {
		t.Errorf("Expected 1 entry remaining, got %d", cache.Len())
	}
}

func TestLRUCache_Concurrency(t *testing.T) {
	cache := NewLRUCache(100, 5*time.Second)
	var wg sync.WaitGroup

	// Multiple goroutines writing
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				key := string(rune('a' + id))
				cache.Set(key, []byte(key))
			}
		}(i)
	}

	// Multiple goroutines reading
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				key := string(rune('a' + id))
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Test should not panic
	if cache.Len() > 100 {
		t.Errorf("Cache exceeded max size: %d", cache.Len())
	}
}

func TestLRUCache_UpdateExisting(t *testing.T) {
	cache := NewLRUCache(10, 5*time.Second)

	cache.Set("key1", []byte("value1"))
	cache.Set("key1", []byte("value2")) // Update

	if val := cache.Get("key1"); string(val) != "value2" {
		t.Errorf("Expected 'value2', got '%s'", string(val))
	}

	if cache.Len() != 1 {
		t.Errorf("Expected cache length 1, got %d", cache.Len())
	}
}
