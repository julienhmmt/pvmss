// Package utils provides generic utility functions for the PVMSS application.
// These utilities leverage Go generics for type-safe operations on collections,
// caching, and common patterns.
package utils

import (
	"sync"
	"time"
)

// Optional represents a value that may or may not be present.
type Optional[T any] struct {
	value   T
	present bool
}

// Some creates an Optional with a present value.
func Some[T any](value T) Optional[T] {
	return Optional[T]{value: value, present: true}
}

// None creates an Optional with no value.
func None[T any]() Optional[T] {
	return Optional[T]{present: false}
}

// IsPresent returns true if the value is present.
func (o Optional[T]) IsPresent() bool {
	return o.present
}

// Get returns the value and a boolean indicating if it was present.
func (o Optional[T]) Get() (T, bool) {
	return o.value, o.present
}

// GetOrDefault returns the value if present, otherwise returns the default.
func (o Optional[T]) GetOrDefault(defaultValue T) T {
	if o.present {
		return o.value
	}
	return defaultValue
}

// GetOrElse returns the value if present, otherwise calls the function.
func (o Optional[T]) GetOrElse(fn func() T) T {
	if o.present {
		return o.value
	}
	return fn()
}

// Map transforms the value if present.
func Map[T, U any](o Optional[T], fn func(T) U) Optional[U] {
	if !o.present {
		return None[U]()
	}
	return Some(fn(o.value))
}

// Result represents the result of an operation that may fail.
type Result[T any] struct {
	value T
	err   error
}

// Ok creates a successful Result.
func Ok[T any](value T) Result[T] {
	return Result[T]{value: value}
}

// Err creates a failed Result.
func Err[T any](err error) Result[T] {
	return Result[T]{err: err}
}

// IsOk returns true if the result is successful.
func (r Result[T]) IsOk() bool {
	return r.err == nil
}

// IsErr returns true if the result is an error.
func (r Result[T]) IsErr() bool {
	return r.err != nil
}

// Unwrap returns the value and error.
func (r Result[T]) Unwrap() (T, error) {
	return r.value, r.err
}

// UnwrapOr returns the value if successful, otherwise returns the default.
func (r Result[T]) UnwrapOr(defaultValue T) T {
	if r.err != nil {
		return defaultValue
	}
	return r.value
}

// Cache is a generic thread-safe cache with TTL support.
type Cache[K comparable, V any] struct {
	mu      sync.RWMutex
	items   map[K]cacheItem[V]
	ttl     time.Duration
	maxSize int
}

type cacheItem[V any] struct {
	value     V
	expiresAt time.Time
}

// CacheWith creates a cache with the specified TTL and max size.
func CacheWith[K comparable, V any](ttl time.Duration, maxSize int) *Cache[K, V] {
	return &Cache[K, V]{
		items:   make(map[K]cacheItem[V]),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Get retrieves a value from the cache.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	if time.Now().After(item.expiresAt) {
		var zero V
		return zero, false
	}
	return item.value, true
}

// Set stores a value in the cache.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		c.evictOldest()
	}
	c.items[key] = cacheItem[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// SetWithTTL stores a value with a custom TTL.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		c.evictOldest()
	}
	c.items[key] = cacheItem[V]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a value from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear removes all values from the cache.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]cacheItem[V])
}

// Size returns the number of items in the cache.
func (c *Cache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// evictOldest removes the oldest item from the cache.
func (c *Cache[K, V]) evictOldest() {
	var oldestKey K
	var oldestTime time.Time
	first := true
	for k, v := range c.items {
		if first || v.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.expiresAt
			first = false
		}
	}
	if !first {
		delete(c.items, oldestKey)
	}
}

// GetOrSet retrieves a value from the cache, or sets it using the provided function.
func (c *Cache[K, V]) GetOrSet(key K, fn func() V) V {
	if value, ok := c.Get(key); ok {
		return value
	}
	value := fn()
	c.Set(key, value)
	return value
}

// Slice utilities for generic slices.

// Filter returns a new slice containing only elements that satisfy the predicate.
func Filter[T any](slice []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

// MapSlice transforms each element of a slice using the provided function.
func MapSlice[T, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, item := range slice {
		result[i] = fn(item)
	}
	return result
}

// Reduce reduces a slice to a single value using the provided function.
func Reduce[T, U any](slice []T, initial U, fn func(U, T) U) U {
	result := initial
	for _, item := range slice {
		result = fn(result, item)
	}
	return result
}

// Find returns the first element that satisfies the predicate.
func Find[T any](slice []T, predicate func(T) bool) Optional[T] {
	for _, item := range slice {
		if predicate(item) {
			return Some(item)
		}
	}
	return None[T]()
}

// Contains checks if a slice contains an element.
func Contains[T comparable](slice []T, element T) bool {
	for _, item := range slice {
		if item == element {
			return true
		}
	}
	return false
}

// Unique returns a new slice with duplicate elements removed.
func Unique[T comparable](slice []T) []T {
	seen := make(map[T]struct{})
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// GroupBy groups elements by a key function.
func GroupBy[T any, K comparable](slice []T, keyFn func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, item := range slice {
		key := keyFn(item)
		result[key] = append(result[key], item)
	}
	return result
}

// First returns the first element of a slice, or None if empty.
func First[T any](slice []T) Optional[T] {
	if len(slice) == 0 {
		return None[T]()
	}
	return Some(slice[0])
}

// Last returns the last element of a slice, or None if empty.
func Last[T any](slice []T) Optional[T] {
	if len(slice) == 0 {
		return None[T]()
	}
	return Some(slice[len(slice)-1])
}

// Keys returns the keys of a map.
func Keys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns the values of a map.
func Values[K comparable, V any](m map[K]V) []V {
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T {
	return &v
}

// Deref returns the value pointed to, or the zero value if nil.
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// DerefOr returns the value pointed to, or the default if nil.
func DerefOr[T any](p *T, defaultValue T) T {
	if p == nil {
		return defaultValue
	}
	return *p
}

// Coalesce returns the first non-zero value.
func Coalesce[T comparable](values ...T) T {
	var zero T
	for _, v := range values {
		if v != zero {
			return v
		}
	}
	return zero
}

// MustValue panics if err is not nil, otherwise returns the value.
// Use sparingly, only for initialization code.
func MustValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
