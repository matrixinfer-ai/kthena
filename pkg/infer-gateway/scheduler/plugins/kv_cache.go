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

/*
KV Cache Plugin Design

Overview:
The KV Cache Plugin is a scoring plugin for the matrixinfer gateway scheduler that implements a token-based block matching mechanism
for model inference requests. It leverages Redis as a distributed cache to track which pods have processed specific token sequences,
enabling intelligent pod scheduling based on KV cache hit potential. The plugin supports both chat completion and text completion
requests with advanced tokenization capabilities.

Key Components:

1. KVCache
   - Main plugin struct implementing the framework.ScorePlugin interface
   - Manages distributed caching mechanism using Redis for cross-pod cache coordination
   - Integrates with tokenization system for accurate token sequence processing
   - Configurable parameters for block size and maximum blocks to match

2. TokenBlockProcessor
   - Processes token sequences into fixed-size blocks for hashing
   - Generates SHA-256 based hashes for token blocks to ensure consistency
   - Handles token chunking with configurable block sizes (default: 128 tokens)

3. Tokenization Integration
   - TokenizerManager for managing model-specific tokenizers
   - Support for vLLM remote tokenization
   - Chat template processing for ChatML format requests

4. Redis-based Distributed Cache
   - Uses Redis hash structures to store block-to-pod mappings
   - Key format: "matrix:kv:block:{model}@{hash}" -> {pod_identifiers}
   - Pipeline operations for efficient batch queries
   - Timeout handling and error recovery

Core Features:

1. Token Block Matching
   - Tokenizes input prompts using model-specific tokenizers
   - Divides token sequences into fixed-size blocks (default: 128 tokens)
   - Generates standardized SHA-256 hashes for each token block
   - Queries Redis to find pods that have cached the same token blocks

2. Chat Template Support
   - Automatic detection of chat completion requests (presence of "messages" field)
   - ChatML format processing with proper role and content extraction
   - Integration with model-specific chat templates via tokenizer pool

3. Scoring Mechanism
   - Scores pods based on consecutive token block matches starting from the beginning
   - Score calculation: (matching consecutive blocks / total blocks) * 100
   - Range: 0-100, where higher scores indicate better KV cache hit potential
   - Early termination when no pods have consecutive matches

4. Distributed Cache Management
   - Redis-based storage for cross-pod cache coordination
   - Efficient pipeline queries for batch block lookups
   - Pod identifier extraction and normalization
   - Timeout handling for Redis operations (5 seconds default)

Usage:
The plugin is used in the matrixinfer gateway scheduler framework to score pods based on their potential
for KV cache hits. It's particularly effective for:
- Chat completion workloads with similar conversation patterns
- Text completion tasks with repeated prompt prefixes
- Multi-turn conversations where context is reused
- Scenarios where token-level cache precision is important

Configuration Parameters:
- BlockSizeToHash: Number of tokens per block for hashing (default: 128)
- MaxBlocksToMatch: Maximum number of blocks to process (default: 128)
- Redis connection managed through utils.GetRedisClient()
- Tokenizer manager configuration with vLLM remote support
- Timeout settings for tokenization (10s) and Redis operations (5s)

Architecture Differences from Prefix Cache:
- Uses token-based blocks instead of byte-based blocks for better semantic alignment
- Leverages Redis for distributed caching instead of local LRU cache
- Integrates advanced tokenization with chat template support
- Designed for cross-pod cache coordination in distributed inference environments

*/

package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/common"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/tokenization"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
	"sigs.k8s.io/yaml"
)

const (
	// KVCachePluginName is the name identifier for the KV cache scoring plugin
	KVCachePluginName = "kv-cache"

	// kvCacheKeyPrefix is the Redis key prefix for storing token block mappings
	// Redis key format: "matrix:kv:block:{model}@{hash}"
	// Example: "matrix:kv:block:deepseek-ai/DeepSeek-R1-Distill-Qwen-7B@12345678901234567890"
	kvCacheKeyPrefix = "matrix:kv:block:"

	// defaultBlockSizeToHash is the default number of tokens per block for hashing
	// Each token sequence is divided into blocks of this size before generating hashes
	defaultBlockSizeToHash = 128

	// defaultMaxBlocksToMatch is the default maximum number of blocks to process for scoring
	// Limits the number of blocks to prevent excessive Redis queries and processing time
	defaultMaxBlocksToMatch = 128
)

type KVCacheArgs struct {
	BlockSizeToHash  int `yaml:"blockSizeToHash,omitempty"`
	MaxBlocksToMatch int `yaml:"maxBlocksToMatch,omitempty"`
}

type KVCache struct {
	name             string
	maxBlocksToMatch int
	keyPrefix        string
	redisClient      *redis.Client
	processor        *TokenBlockProcessor
	tokenizerManager *tokenization.TokenizerManager
}

var _ framework.ScorePlugin = &KVCache{}

type TokenBlockProcessor struct {
	blockSize int
}

// KVCacheBlock represents a token block for Redis storage
type KVCacheBlock struct {
	ModelName string // Model name (e.g., "deepseek-ai/DeepSeek-R1-Distill-Qwen-7B")
	ChunkHash uint64 // SHA-256 hash of the token block
}

// String generates the Redis key for this token block
// Format: "{prefix}{model}@{hash}"
// Example: "matrix:kv:block:deepseek-ai/DeepSeek-R1-Distill-Qwen-7B@12345678901234567890"
//
// The resulting Redis hash structure:
//
//	Key: "matrix:kv:block:deepseek-ai/DeepSeek-R1-Distill-Qwen-7B@12345678901234567890"
//	Fields: {
//	  "pod-name-1.namespace.svc.cluster.local": "1703123456",
//	  "pod-name-2.namespace.svc.cluster.local": "1703123789"
//	}
func (b KVCacheBlock) String(prefix string) string {
	return fmt.Sprintf("%s%s@%d", prefix, b.ModelName, b.ChunkHash)
}

func NewKVCache(pluginArg runtime.RawExtension) *KVCache {
	var args KVCacheArgs
	if len(pluginArg.Raw) > 0 {
		if err := yaml.Unmarshal(pluginArg.Raw, &args); err != nil {
			klog.Warningf("Failed to unmarshal KVCacheArgs: %v", err)
		}
	}

	blockSizeToHash := args.BlockSizeToHash
	if blockSizeToHash <= 0 {
		blockSizeToHash = defaultBlockSizeToHash
	}
	maxBlocksToMatch := args.MaxBlocksToMatch
	if maxBlocksToMatch <= 0 {
		maxBlocksToMatch = defaultMaxBlocksToMatch
	}

	managerConfig := tokenization.TokenizerManagerConfig{
		EnableVLLMRemote: true,
		EndpointTemplate: "http://%s:8000",
		ModelServiceMap:  make(map[string]string),
	}
	manager := tokenization.NewTokenizerManager(managerConfig)

	return &KVCache{
		name:             KVCachePluginName,
		maxBlocksToMatch: maxBlocksToMatch,
		keyPrefix:        kvCacheKeyPrefix,
		redisClient:      utils.GetRedisClient(),
		processor:        &TokenBlockProcessor{blockSize: blockSizeToHash},
		tokenizerManager: manager,
	}
}

func (t *KVCache) Name() string {
	return t.name
}

func (t *KVCache) getTokenizerForModel(ctx *framework.Context, pods []*datastore.PodInfo) tokenization.Tokenizer {
	if t.tokenizerManager == nil {
		return nil
	}
	return t.tokenizerManager.GetTokenizer(ctx.Model, pods)
}

func (t *KVCache) normalizeAndTokenizePrompt(ctx *framework.Context, pods []*datastore.PodInfo) ([]uint32, error) {
	tok := t.getTokenizerForModel(ctx, pods)
	if tok == nil {
		return nil, fmt.Errorf("no tokenizer available for model %s", ctx.Model)
	}

	// If it's a Text prompt, use TokenizeInputText directly
	if ctx.Prompt.Text != "" {
		text := ctx.Prompt.Text
		tokens, err := tok.TokenizeInputText(text)
		if err != nil {
			return nil, err
		}
		tokens32 := make([]uint32, len(tokens)/4)
		for i := 0; i < len(tokens32); i++ {
			tokens32[i] = binary.BigEndian.Uint32(tokens[i*4 : (i+1)*4])
		}
		return tokens32, nil
	}

	// For chat messages, require extended tokenizer with chat template
	if extendedTok, ok := tok.(interface {
		TokenizeWithOptions(context.Context, tokenization.TokenizeInput) (*tokenization.TokenizeResult, error)
	}); ok {
		return t.tokenizeWithChatTemplate(extendedTok, ctx.Prompt)
	}

	return nil, fmt.Errorf("the model %s does not support normalize and tokenize prompt", ctx.Model)
}

func (t *KVCache) tokenizeWithChatTemplate(
	extendedTok interface {
		TokenizeWithOptions(context.Context, tokenization.TokenizeInput) (*tokenization.TokenizeResult, error)
	},
	chatMessage common.ChatMessage,
) ([]uint32, error) {
	tokenizeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	input := tokenization.TokenizeInput{
		Type:                tokenization.ChatInput,
		Messages:            chatMessage.Messages,
		AddSpecialTokens:    false,
		AddGenerationPrompt: true,
		ReturnTokenStrings:  false,
	}

	result, err := extendedTok.TokenizeWithOptions(tokenizeCtx, input)
	if err != nil {
		return nil, fmt.Errorf("chat template tokenization failed: %w", err)
	}

	tokens32 := make([]uint32, len(result.Tokens))
	for i, token := range result.Tokens {
		tokens32[i] = uint32(token)
	}

	return tokens32, nil
}

func (t *KVCache) Score(ctx *framework.Context, pods []*datastore.PodInfo) map[*datastore.PodInfo]int {
	scoreResults := make(map[*datastore.PodInfo]int)

	for _, pod := range pods {
		scoreResults[pod] = 0
	}

	if (ctx.Prompt.Text == "" && len(ctx.Prompt.Messages) == 0) || ctx.Model == "" {
		return scoreResults
	}

	start := time.Now()
	tokens, err := t.normalizeAndTokenizePrompt(ctx, pods)
	tokenizerDuration := time.Since(start)
	klog.V(4).Infof("Tokenizer processing time: %v", tokenizerDuration)

	if err != nil || len(tokens) == 0 {
		return scoreResults
	}

	blockHashes := t.processor.TokensToBlockHashes(tokens)
	if len(blockHashes) == 0 {
		return scoreResults
	}

	if len(blockHashes) > t.maxBlocksToMatch {
		blockHashes = blockHashes[:t.maxBlocksToMatch]
	}

	blockToPods, err := t.queryRedisForBlocks(blockHashes, ctx.Model)
	if err != nil {
		return scoreResults
	}

	podScores := t.calculatePodScores(blockHashes, blockToPods)

	for _, pod := range pods {
		podName := pod.Pod.Name
		if score, exists := podScores[podName]; exists {
			scoreResults[pod] = score
		}
	}
	return scoreResults
}

// queryRedisForBlocks queries Redis to find which pods have cached the given token block hashes
// Returns a map from block hash to list of pod names that have cached that block
func (t *KVCache) queryRedisForBlocks(blockHashes []uint64, modelName string) (map[uint64][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	blockToPods := make(map[uint64][]string)

	if t.redisClient == nil {
		return blockToPods, fmt.Errorf("redis client not initialized")
	}

	pipe := t.redisClient.Pipeline()
	cmds := make([]*redis.StringSliceCmd, len(blockHashes))

	// Build pipeline commands for batch Redis query
	for i, hash := range blockHashes {
		block := KVCacheBlock{ModelName: modelName, ChunkHash: hash}
		key := block.String(t.keyPrefix)
		cmds[i] = pipe.HKeys(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	// Process results and extract pod names
	for i, cmd := range cmds {
		pods, err := cmd.Result()
		if err != nil || len(pods) == 0 {
			continue
		}

		podNames := make([]string, 0, len(pods))
		for _, pod := range pods {
			// Redis field is pod identifier (e.g., "pod-name.namespace")
			podName := extractPodNameFromIdentifier(pod)
			podNames = append(podNames, podName)
		}
		blockToPods[blockHashes[i]] = podNames
	}

	return blockToPods, nil
}

func extractPodNameFromIdentifier(podIdentifier string) string {
	parts := strings.Split(podIdentifier, ".")
	return parts[0]
}

func (t *KVCache) calculatePodScores(blockHashes []uint64, blockToPods map[uint64][]string) map[string]int {
	podScores := make(map[string]int)

	if len(blockHashes) == 0 {
		klog.Infof("KVCache: No block hashes to process")
		return podScores
	}

	firstBlockPods, exists := blockToPods[blockHashes[0]]
	if !exists || len(firstBlockPods) == 0 {
		return podScores
	}

	activePods := make(map[string]bool)
	for _, podName := range firstBlockPods {
		activePods[podName] = true
		podScores[podName] = 1
	}

	for i := 1; i < len(blockHashes); i++ {
		if len(activePods) == 0 {
			break
		}

		blockPods, exists := blockToPods[blockHashes[i]]
		if !exists || len(blockPods) == 0 {
			break
		}

		currentPods := make(map[string]bool)
		for _, podName := range blockPods {
			currentPods[podName] = true
		}

		newActivePods := make(map[string]bool)
		for podName := range activePods {
			if currentPods[podName] {
				newActivePods[podName] = true
				podScores[podName]++
			}
		}

		if len(newActivePods) == 0 {
			break
		}

		activePods = newActivePods
	}

	totalBlocks := len(blockHashes)
	for podName, matchLen := range podScores {
		score := int((float64(matchLen) / float64(totalBlocks)) * 100)
		podScores[podName] = score
		klog.V(4).Infof("KVCache Pod %s: matched %d/%d blocks, score: %d", podName, matchLen, totalBlocks, score)
	}

	return podScores
}

func (tbp *TokenBlockProcessor) TokensToBlockHashes(tokens []uint32) []uint64 {
	if len(tokens) == 0 {
		return nil
	}

	chunks := tbp.chunkTokens(tokens)
	return tbp.computeBlockHashes(chunks)
}

// computeStandardizedHash generates a consistent hash for token sequences using SHA-256
// Returns a 63-bit positive integer for Redis/database compatibility
func computeStandardizedHash(tokenIds []int) uint64 {
	if len(tokenIds) == 0 {
		return 0
	}

	h := sha256.New()
	for _, tokenId := range tokenIds {
		tokenBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(tokenBytes, uint32(tokenId))
		h.Write(tokenBytes)
	}

	hashBytes := h.Sum(nil)
	fullHash := binary.BigEndian.Uint64(hashBytes[:8])

	// Clear MSB to ensure positive value (0x7FFFFFFFFFFFFFFF masks out sign bit)
	result := fullHash & 0x7FFFFFFFFFFFFFFF
	klog.V(4).Infof("KVCache: compute standardized hash - token_ids=%v, hash=%d", tokenIds, result)
	return result
}

func (tbp *TokenBlockProcessor) chunkTokens(tokens []uint32) [][]uint32 {
	var chunks [][]uint32
	for i := 0; i < len(tokens); i += tbp.blockSize {
		end := i + tbp.blockSize
		if end > len(tokens) {
			end = len(tokens)
		}
		chunks = append(chunks, tokens[i:end])
	}
	return chunks
}

func (tbp *TokenBlockProcessor) computeBlockHashes(chunks [][]uint32) []uint64 {
	hashes := make([]uint64, len(chunks))
	for i, chunk := range chunks {
		tokenInts := make([]int, len(chunk))
		for j, token := range chunk {
			tokenInts[j] = int(token)
		}

		hashes[i] = computeStandardizedHash(tokenInts)
	}
	return hashes
}
