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

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

// hashModelKey represents a composite key combining hash and model name
type hashModelKey struct {
	hash  uint64
	model string
}

const (
	// numShards is the number of shards to use for the modelHashes map.
	// Using a power of 2 can be slightly more efficient for the modulo operation.
	numShards = 32
)

// modelHashesShard holds a shard of the hashes for a specific model.
type modelHashesShard struct {
	mu     sync.RWMutex
	hashes map[uint64]sets.Set[types.NamespacedName]
}

// modelHashes holds the sharded hashes for a specific model.
type modelHashes struct {
	shards [numShards]*modelHashesShard
}

// newModelHashes creates a new sharded modelHashes struct.
func newModelHashes() *modelHashes {
	mh := &modelHashes{}
	for i := 0; i < numShards; i++ {
		mh.shards[i] = &modelHashesShard{
			hashes: make(map[uint64]sets.Set[types.NamespacedName]),
		}
	}
	return mh
}

// getShard returns the appropriate shard for a given hash.
func (mh *modelHashes) getShard(hash uint64) *modelHashesShard {
	return mh.shards[hash%numShards]
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
	podLRU, exists := s.podHashes[data.Pod]
	if exists {
		delete(s.podHashes, data.Pod)
	}
	s.podHashesMu.Unlock()

	if exists {
		hashByModel := make(map[string][]uint64)
		for _, key := range podLRU.Keys() {
			hashByModel[key.model] = append(hashByModel[key.model], key.hash)
		}
		for model, hashSlice := range hashByModel {
			s.onHashEvicted(model, hashSlice, data.Pod)
		}
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

	// Track processed pods to avoid duplicates
	processedPods := sets.New[types.NamespacedName]()

	// Start matching from the end of hashes
	// This works because each hash depends on the previous hash in hashPrompt
	for i := len(hashes) - 1; i >= 0; i-- {
		hash := hashes[i]
		shard := modelCache.getShard(hash)
		shard.mu.RLock()
		podSet, exists := shard.hashes[hash]
		if exists {
			// Note: we are iterating over a copy of the set, so we don't need to hold the lock.
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
					shard.mu.RUnlock()
					return matches
				}
			}
		}
		shard.mu.RUnlock()
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
		podLRU, _ = NewLRUCache(s.hashCapacity, func(key hashModelKey, value struct{}) {
			// onEvict callback need to acquire `modelCache.mu.Lock()` as well, so start a goroutine to run it async.
			go s.onHashEvicted(key.model, []uint64{key.hash}, nsName)
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

	// Add hashes from the end to the beginning to avoid
	// the situation where a long prefix can be matched but a shorter prefix cannot.
	for i := len(hashes) - 1; i >= 0; i-- {
		hash := hashes[i]
		shard := modelCache.getShard(hash)
		shard.mu.Lock()
		if _, exists := shard.hashes[hash]; !exists {
			shard.hashes[hash] = sets.New[types.NamespacedName]()
		}
		shard.hashes[hash].Insert(nsName)
		shard.mu.Unlock()

		// Here we protect podLRU and modelCache within a same lock, becasue we should make sure modelCache
		// must be deleted when pod delete or LRU evict
		podLRU.Add(hashModelKey{hash: hashes[i], model: model}, struct{}{})
	}
}

// onHashEvicted handles the eviction of a hash from a pod's LRU cache
func (s *ModelPrefixStore) onHashEvicted(model string, hashSlice []uint64, nsName types.NamespacedName) {
	s.entriesMu.RLock()
	modelCache, exists := s.entries[model]
	s.entriesMu.RUnlock()
	if !exists {
		return
	}

	for _, hash := range hashSlice {
		shard := modelCache.getShard(hash)
		shard.mu.Lock()
		if podSet, exists := shard.hashes[hash]; exists {
			podSet.Delete(nsName)
			if podSet.Len() == 0 {
				delete(shard.hashes, hash)
			}
		}
		shard.mu.Unlock()
	}
}
