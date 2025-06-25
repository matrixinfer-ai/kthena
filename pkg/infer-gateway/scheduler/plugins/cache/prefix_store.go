package cache

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/lru"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

// ModelPrefixStore manages a three-level map structure for model inference requests
type ModelPrefixStore struct {
	// Mutex to protect entries access
	// TODO: use finer-grained locks.
	mu sync.RWMutex
	// Three-level map: model -> hash -> pod namespaced name -> pod
	entries map[string]map[uint64]map[types.NamespacedName]*datastore.PodInfo

	podHashes    map[types.NamespacedName]Cache[uint64, string] // Map of pod to its hash LRU
	topK         int                                            // Each match returns at most topK pods.
	hashCapacity int                                            // Capacity for each pod's hash LRU
}

// MatchResult represents a matching pod and its match length
type MatchResult struct {
	Pod      *datastore.PodInfo
	MatchLen int
}

// NewModelPrefixStore creates a new ModelPrefixStore with the specified capacity and topK
func NewModelPrefixStore(store datastore.Store, hashCapacity, topK int) *ModelPrefixStore {
	s := &ModelPrefixStore{
		entries:      make(map[string]map[uint64]map[types.NamespacedName]*datastore.PodInfo),
		podHashes:    make(map[types.NamespacedName]Cache[uint64, string]),
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

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove pod's hash LRU
	if hashLRU, exists := s.podHashes[data.Pod]; exists {
		delete(s.podHashes, data.Pod)

		// NOTE: The Clear operation may trigger eviction, so must unlock first.
		hashLRU.Clear()
	}
}

// FindTopMatches finds the topK pods with the longest matching prefixes for given model and hashes
func (s *ModelPrefixStore) FindTopMatches(model string, hashes []uint64, pods []*datastore.PodInfo) []MatchResult {
	matches := make([]MatchResult, 0, s.topK)

	// Only check entries for the requested model
	s.mu.RLock()
	defer s.mu.RUnlock()
	modelEntries, exists := s.entries[model]
	if !exists {
		return nil
	}

	// Track processed pods to avoid duplicates
	processedPods := make(map[*datastore.PodInfo]bool)

	// Start matching from the end of hashes
	// This works because each hash depends on the previous hash in hashPrompt
	for i := len(hashes) - 1; i >= 0; i-- {
		hash := hashes[i]
		if podMap, exists := modelEntries[hash]; exists {
			for _, pod := range podMap {
				// Skip if pod is not in the candidate set or already processed
				if processedPods[pod] {
					continue
				}
				processedPods[pod] = true

				// If we found a match at position i, we know all previous hashes must match
				// because each hash depends on the previous one in hashPrompt
				matchLen := i + 1

				matches = append(matches, MatchResult{
					Pod:      pod,
					MatchLen: matchLen,
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

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create hash LRU for this pod
	var hashLRU Cache[uint64, string]
	if existingLRU, exists := s.podHashes[nsName]; exists {
		hashLRU = existingLRU
	} else {
		// Create new hash LRU for this pod
		newHashLRU, _ := NewLRUCache[uint64, string](s.hashCapacity, func(key lru.Key, value interface{}) {
			// NOTE: The eviction callback does not need to be locked.
			// Because it's triggered by other operations, Add or Clear, and we should have locked at that time.

			// Convert key and value to hash and model
			hash := key.(uint64)
			model := value.(string)

			// Remove pod from the hash's pod map in the model
			if modelEntries, exists := s.entries[model]; exists {
				if podMap, exists := modelEntries[hash]; exists {
					delete(podMap, nsName)

					// If no more pods for this hash, remove the hash
					if len(podMap) == 0 {
						delete(modelEntries, hash)

						// If no more hashes for this model, remove the model
						if len(modelEntries) == 0 {
							delete(s.entries, model)
						}
					}
				}
			}
		}) // Using hashCapacity for hash LRU
		hashLRU = newHashLRU
		s.podHashes[nsName] = hashLRU
	}

	// Initialize model map if it doesn't exist
	if _, exists := s.entries[model]; !exists {
		s.entries[model] = make(map[uint64]map[types.NamespacedName]*datastore.PodInfo)
	}

	// Add pod to each hash's pod map
	// Add hashes from the end to the beginning to avoid
	// the situation where a long prefix can be matched but a shorter prefix cannot.
	for i := len(hashes) - 1; i >= 0; i-- {
		hash := hashes[i]
		if _, exists := s.entries[model][hash]; !exists {
			s.entries[model][hash] = make(map[types.NamespacedName]*datastore.PodInfo)
		}
		s.entries[model][hash][nsName] = pod

		// Add hash to pod's hash LRU
		// The Add operation may trigger eviction, so must unlock first.
		hashLRU.Add(hash, model)
	}
}
