package plugins

import (
	"fmt"

	"github.com/cespare/xxhash"
	lru "github.com/hashicorp/golang-lru/v2"
	"k8s.io/apimachinery/pkg/types"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

const PrefixCachePluginName = "prefix-cache"

const (
	BlockSizeToHash  = 64
	MaxBlocksToMatch = 128
)

var _ framework.ScorePlugin = &PrefixCache{}

type PrefixCache struct {
	name string

	blockSizeToHash  int
	maxBlocksToMatch int
	store            *PrefixCacheStore
}

type PrefixCacheStore struct {
	// Three-level map: model -> hash -> pod namespaced name -> pod
	entries map[string]map[uint64]map[types.NamespacedName]*datastore.PodInfo
	lru     *lru.Cache[uint64, string]
	topK    int // Number of top matches to return
}

type matchResult struct {
	pod      *datastore.PodInfo
	matchLen int
}

func (p *PrefixCache) newStore(maxHashes, topK int) *PrefixCacheStore {
	store := &PrefixCacheStore{
		entries: make(map[string]map[uint64]map[types.NamespacedName]*datastore.PodInfo),
		topK:    topK,
	}

	// Create LRU cache with OnEvicted callback to clean up entries
	cache, _ := lru.NewWithEvict[uint64, string](maxHashes, func(key uint64, value string) {
		hash := key
		model := value

		// Remove pod from the model->hash entry
		if hashMap, exists := store.entries[model]; exists {
			delete(hashMap, hash)
			// If no more hashes for this model, remove the model entry
			if len(hashMap) == 0 {
				delete(store.entries, model)
			}
		}
	})

	store.lru = cache
	return store
}

// findTopMatches finds the pods with top K longest matching prefixes for given model and hashes
func (s *PrefixCacheStore) findTopMatches(model string, hashes []uint64, pods []*datastore.PodInfo) []matchResult {
	matches := make([]matchResult, 0, s.topK)

	// Create a set of candidate pods for quick lookup
	podSet := make(map[*datastore.PodInfo]bool)
	for _, pod := range pods {
		podSet[pod] = true
	}

	// Only check entries for the requested model
	modelEntries, exists := s.entries[model]
	if !exists {
		return matches
	}

	// Track processed pods to avoid duplicates
	processedPods := make(map[*datastore.PodInfo]bool)

	// Start matching from the end of hashes
	// This works because each hash depends on the previous hash in hashPrompt
	for i := len(hashes) - 1; i >= 0; i-- {
		hash := hashes[i]
		// Check if this hash exists in LRU cache
		if _, exists := s.lru.Get(hash); !exists {
			continue
		}
		if podMap, exists := modelEntries[hash]; exists {
			for _, pod := range podMap {
				// Skip if pod is not in the candidate set or already processed
				if !podSet[pod] || processedPods[pod] {
					continue
				}
				processedPods[pod] = true

				// If we found a match at position i, we know all previous hashes must match
				// because each hash depends on the previous one in hashPrompt
				matchLen := i + 1

				matches = append(matches, matchResult{
					pod:      pod,
					matchLen: matchLen,
				})

				// Break if we have enough matches
				if len(matches) >= s.topK {
					break
				}
			}
		}
	}

	// Sort matches by matchLen in descending order
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].matchLen < matches[j].matchLen {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}

// add adds new hash->pod mappings to cache, using LRU for eviction
func (s *PrefixCacheStore) add(model string, hashes []uint64, pod *datastore.PodInfo) {
	nsName := types.NamespacedName{
		Namespace: pod.Pod.Namespace,
		Name:      pod.Pod.Name,
	}

	// Initialize model map if it doesn't exist
	if _, exists := s.entries[model]; !exists {
		s.entries[model] = make(map[uint64]map[types.NamespacedName]*datastore.PodInfo)
	}

	// Add pod to each hash's pod map from start to end
	// This ensures that when LRU eviction happens, we keep the most important (last) hashes
	for i := 0; i < len(hashes); i++ {
		hash := hashes[i]
		if _, exists := s.entries[model][hash]; !exists {
			s.entries[model][hash] = make(map[types.NamespacedName]*datastore.PodInfo)
		}
		s.entries[model][hash][nsName] = pod

		// Add to LRU with hash as key and model as value
		s.lru.Add(hash, model)
	}
}

// Helper function to check if a pod exists in a map
func hasPod(podMap map[types.NamespacedName]*datastore.PodInfo, pod *datastore.PodInfo) bool {
	nsName := types.NamespacedName{
		Namespace: pod.Pod.Namespace,
		Name:      pod.Pod.Name,
	}
	_, exists := podMap[nsName]
	return exists
}

func NewPrefixCache() *PrefixCache {
	p := &PrefixCache{
		name: PrefixCachePluginName,

		// TODO: make these parameters configurable.
		blockSizeToHash:  BlockSizeToHash,
		maxBlocksToMatch: MaxBlocksToMatch,
	}
	// Initialize store with default values
	p.store = p.newStore(1000, 5) // TODO: make these configurable
	return p
}

func (p *PrefixCache) Name() string {
	return p.name
}

func (p *PrefixCache) Score(pods []*datastore.PodInfo, ctx *framework.Context) map[*datastore.PodInfo]int {
	scoreResults := make(map[*datastore.PodInfo]int)

	// Initialize all pods with score 0
	for _, pod := range pods {
		scoreResults[pod] = 0
	}

	// Hash the prompt
	hashes := p.hashPrompt(ctx.Model, ctx.Prompt)
	if len(hashes) == 0 {
		return scoreResults
	}

	// Store hashes in context for later use in PostSchedule
	ctx.Hashes = hashes

	// Find pods with matching prefixes
	matches := p.store.findTopMatches(ctx.Model, hashes, pods)

	// Calculate scores based on prefix match length
	totalHashes := len(hashes)
	for _, match := range matches {
		// Score is the ratio of matching hashes to total hashes, scaled to 0-100
		score := int((float64(match.matchLen) / float64(totalHashes)) * 100)
		scoreResults[match.pod] = score
	}

	return scoreResults
}

func (p *PrefixCache) PostSchedule(ctx *framework.Context) {
	if ctx.TargetPod != nil && len(ctx.Hashes) > 0 {
		// Add the selected pod and its hashes to the cache
		p.store.add(ctx.Model, ctx.Hashes, ctx.TargetPod)
	}
}

func (p *PrefixCache) hashPrompt(model string, prompt string) []uint64 {
	res := []uint64{}
	if len(prompt) == 0 {
		return res
	}

	// Initialize first block hash
	var prevHash uint64 = 0
	blockStart := 0

	// Process blocks up to maxBlocksToMatch or until we run out of prompt
	for i := 0; i < p.maxBlocksToMatch && blockStart < len(prompt); i++ {
		// Calculate end position for current block
		blockEnd := blockStart + p.blockSizeToHash
		if blockEnd > len(prompt) {
			blockEnd = len(prompt)
		}

		// Get current block content and combine with previous hash
		block := prompt[blockStart:blockEnd]
		data := []byte(fmt.Sprintf("%d%s", prevHash, block))

		// Use xxHash algorithm
		currHash := xxhash.Sum64(data)

		// Append hash to results
		res = append(res, currHash)

		// Update for next iteration
		prevHash = currHash
		blockStart = blockEnd

		// Break if we've reached the end of prompt
		if blockEnd == len(prompt) {
			break
		}
	}

	return res
}
