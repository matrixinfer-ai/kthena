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
	"sync"

	"istio.io/istio/pkg/util/sets"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/lru"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

// hashModelKey represents a composite key combining hash and model name
type hashModelKey struct {
	hash  uint64
	model string
}

// modelHashes holds the hashes for a specific model and a mutex to protect access.
type modelHashes struct {
	mu     sync.RWMutex
	hashes map[uint64]sets.Set[types.NamespacedName]
}

// newModelHashes creates a new modelHashes struct.
func newModelHashes() *modelHashes {
	return &modelHashes{
		hashes: make(map[uint64]sets.Set[types.NamespacedName]),
	}
}

// ModelPrefixStore manages a three-level map structure for model inference requests
type ModelPrefixStore struct {
	// Mutex to protect the entries map itself
	entriesMu sync.RWMutex
	// map: model -> modelHashes
	entries map[string]*modelHashes

	// Mutex to protect podHashes access
	podHashesMu  sync.RWMutex
	podHashes    map[types.NamespacedName]Cache[hashModelKey, struct{}] // Map of pod to its hash LRU
	topK         int                                                    // Each match returns at most topK pods.
	hashCapacity int                                                    // Capacity for each pod's hash LRU
}

// MatchResult represents a matching pod and its match length
type MatchResult struct {
	NamespacedName types.NamespacedName
	MatchLen       int
}

// NewModelPrefixStore creates a new ModelPrefixStore with the specified capacity and topK
func NewModelPrefixStore(store datastore.Store, hashCapacity, topK int) *ModelPrefixStore {
	s := &ModelPrefixStore{
		entries:      make(map[string]*modelHashes),
		podHashes:    make(map[types.NamespacedName]Cache[hashModelKey, struct{}]),
		topK:         topK,
		hashCapacity: hashCapacity,
	}

	// Register callback for pod deletion
	store.RegisterCallback("Pod", s.onPodDeleted)

	return s
}

// onPodDeleted is called when a pod is deleted
func (s *ModelPrefixStore) onPodDeleted(data datastore.EventData) {
	if data.EventType != datastore.EventDelete {
		return
	}

	s.podHashesMu.Lock()
	hashLRU, exists := s.podHashes[data.Pod]
	if exists {
		delete(s.podHashes, data.Pod)
	}
	s.podHashesMu.Unlock()

	if exists {
		// The Clear operation will trigger eviction for all items in the LRU.
		// The onEvict callback will attempt to acquire locks.
		hashLRU.Clear()
	}
}

// FindTopMatches finds the topK pods with the longest matching prefixes for given model and hashes
func (s *ModelPrefixStore) FindTopMatches(model string, hashes []uint64, pods []*datastore.PodInfo) []MatchResult {
	matches := make([]MatchResult, 0, s.topK)

	s.entriesMu.RLock()
	modelCache, exists := s.entries[model]
	s.entriesMu.RUnlock()

	if !exists {
		return nil
	}

	modelCache.mu.RLock()
	defer modelCache.mu.RUnlock()

	// Track processed pods to avoid duplicates
	processedPods := sets.New[types.NamespacedName]()

	// Start matching from the end of hashes
	// This works because each hash depends on the previous hash in hashPrompt
	for i := len(hashes) - 1; i >= 0; i-- {
		hash := hashes[i]
		if podSet, exists := modelCache.hashes[hash]; exists {
			for pod := range podSet {
				// Skip if pod is not in the candidate set or already processed
				if processedPods.Contains(pod) {
					continue
				}
				processedPods.Insert(pod)

				// If we found a match at position i, we know all previous hashes must match
				// because each hash depends on the previous one in hashPrompt
				matchLen := i + 1

				matches = append(matches, MatchResult{
					NamespacedName: pod,
					MatchLen:       matchLen,
				})

				// Return if we have enough matches
				if len(matches) >= s.topK {
					return matches
				}
			}
		}
	}

	return matches
}

// Add adds new hash->pod mappings to cache, using LRU for eviction
func (s *ModelPrefixStore) Add(model string, hashes []uint64, pod *datastore.PodInfo) {
	nsName := types.NamespacedName{
		Namespace: pod.Pod.Namespace,
		Name:      pod.Pod.Name,
	}

	s.podHashesMu.Lock()
	podLRU, exists := s.podHashes[nsName]
	if !exists {
		podLRU, _ = NewLRUCache[hashModelKey, struct{}](s.hashCapacity, func(key lru.Key, value interface{}) {
			// The cb is protected by lru cache's lock, so we should start a new goroutine
			// to avoid blocking the Add below.
			go func() {
				// onEvict callback
				s.onHashEvicted(key.(hashModelKey), nsName)
			}()
		})
		s.podHashes[nsName] = podLRU
	}
	s.podHashesMu.Unlock()

	// Note there could a be case where Add and Evict happen concurrently.
	// The modelHash could be deleted, that does not matter much, since the prefix cache is an approximate cache.
	s.entriesMu.Lock()
	modelCache, exists := s.entries[model]
	if !exists {
		modelCache = newModelHashes()
		s.entries[model] = modelCache
	}
	s.entriesMu.Unlock()

	modelCache.mu.Lock()
	// Add pod to each hash's pod map
	// Add hashes from the end to the beginning to avoid
	// the situation where a long prefix can be matched but a shorter prefix cannot.
	for i := len(hashes) - 1; i >= 0; i-- {
		hash := hashes[i]
		if _, exists := modelCache.hashes[hash]; !exists {
			modelCache.hashes[hash] = sets.New[types.NamespacedName]()
		}
		modelCache.hashes[hash].Insert(nsName)
		podLRU.Add(hashModelKey{hash: hash, model: model}, struct{}{})
	}
	modelCache.mu.Unlock()
}

// onHashEvicted handles the eviction of a hash from a pod's LRU cache
func (s *ModelPrefixStore) onHashEvicted(key hashModelKey, nsName types.NamespacedName) {
	s.entriesMu.RLock()
	modelCache, exists := s.entries[key.model]
	s.entriesMu.RUnlock()
	if !exists {
		return
	}

	modelCache.mu.Lock()
	defer modelCache.mu.Unlock()
	if podSet, exists := modelCache.hashes[key.hash]; exists {
		podSet.Delete(nsName)
		if podSet.Len() == 0 {
			delete(modelCache.hashes, key.hash)
			if len(modelCache.hashes) == 0 {
				// If no hashes left, we can remove the modelCache from entries
				s.entriesMu.Lock()
				delete(s.entries, key.model)
				s.entriesMu.Unlock()
			}
		}
	}
}
