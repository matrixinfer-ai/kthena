/*
Copyright MatrixInfer-AI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	lru "github.com/hashicorp/golang-lru/v2"
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
	// Keys returns a slice of the keys in the cache, from oldest to newest.
	Keys() []K
}

// LRUCache is an implementation of Cache using hashicorp/golang-lru
type LRUCache[K comparable, V any] struct {
	cache *lru.Cache[K, V]
}

// NewLRUCache creates a new LRU cache with the specified size
func NewLRUCache[K comparable, V any](size int, onEvict func(key K, value V)) (*LRUCache[K, V], error) {
	cache, err := lru.NewWithEvict(size, onEvict)
	if err != nil {
		return nil, err
	}
	return &LRUCache[K, V]{cache: cache}, nil
}

// Add adds a value to the cache
func (c *LRUCache[K, V]) Add(key K, value V) {
	c.cache.Add(key, value)
}

// Get retrieves a value from the cache
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	return c.cache.Get(key)
}

// Remove removes a value from the cache
func (c *LRUCache[K, V]) Remove(key K) {
	c.cache.Remove(key)
}

// Contains checks if a key exists in the cache
func (c *LRUCache[K, V]) Contains(key K) bool {
	return c.cache.Contains(key)
}

// Len returns the number of items in the cache
func (c *LRUCache[K, V]) Len() int {
	return c.cache.Len()
}

func (c *LRUCache[K, V]) Clear() {
	c.cache.Purge()
}

// Keys returns a slice of the keys in the cache, from oldest to newest.
func (c *LRUCache[K, V]) Keys() []K {
	return c.cache.Keys()
}
