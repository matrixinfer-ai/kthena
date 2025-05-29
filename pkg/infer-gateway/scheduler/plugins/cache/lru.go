package cache

import (
	"k8s.io/utils/lru"
)

// Cache defines the interface for a generic cache implementation
type Cache[K comparable, V any] interface {
	// Add adds a value to the cache
	Add(key K, value V)
	// Get retrieves a value from the cache
	Get(key K) (V, bool)
	// Remove removes a value from the cache
	Remove(key K)
	// Contains checks if a key exists in the cache
	Contains(key K) bool
	// Len returns the number of items in the cache
	Len() int
	// Clear clears the cache
	Clear()
}

// LRUCache is an implementation of Cache using hashicorp/golang-lru
type LRUCache[K comparable, V any] struct {
	cache *lru.Cache
}

// NewLRUCache creates a new LRU cache with the specified size
func NewLRUCache[K comparable, V any](size int, onEvict lru.EvictionFunc) (*LRUCache[K, V], error) {
	cache := lru.NewWithEvictionFunc(size, onEvict)
	return &LRUCache[K, V]{cache: cache}, nil
}

// Add adds a value to the cache
func (c *LRUCache[K, V]) Add(key K, value V) {
	c.cache.Add(key, value)
}

// Get retrieves a value from the cache
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	if val, ok := c.cache.Get(key); ok {
		return val.(V), true
	}
	var zero V
	return zero, false
}

// Remove removes a value from the cache
func (c *LRUCache[K, V]) Remove(key K) {
	c.cache.Remove(key)
}

// Contains checks if a key exists in the cache
func (c *LRUCache[K, V]) Contains(key K) bool {
	_, ok := c.cache.Get(key)
	return ok
}

// Len returns the number of items in the cache
func (c *LRUCache[K, V]) Len() int {
	return c.cache.Len()
}

func (c *LRUCache[K, V]) Clear() {
	c.cache.Clear()
}
