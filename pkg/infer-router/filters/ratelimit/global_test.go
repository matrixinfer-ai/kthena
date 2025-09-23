/*
Copyright The Volcano Authors.

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

package ratelimit

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkingv1alpha1 "github.com/volcano-sh/kthena/pkg/apis/networking/v1alpha1"
)

func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *networkingv1alpha1.RedisConfig) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	config := &networkingv1alpha1.RedisConfig{
		Address: mr.Addr(),
	}

	return mr, config
}

func TestTokenRateLimiter_Global(t *testing.T) {
	mr, redisConfig := setupMiniRedis(t)
	defer mr.Close()

	rl := NewTokenRateLimiter()
	model := "test-model"
	prompt := "hello world" // Should be ~3 tokens
	tokens := uint32(10)
	unit := networkingv1alpha1.Second

	// Configure global rate limiting
	globalConfig := &networkingv1alpha1.RateLimit{
		InputTokensPerUnit: &tokens,
		Unit:               unit,
		Global: &networkingv1alpha1.GlobalRateLimit{
			Redis: redisConfig,
		},
	}

	err := rl.AddOrUpdateLimiter(model, globalConfig)
	require.NoError(t, err)

	// Should allow multiple requests within limit
	for i := 0; i < 3; i++ {
		err := rl.RateLimit(model, prompt)
		assert.NoError(t, err, "Request %d should be allowed", i)
	}

	// Should be rate limited after exceeding limit
	err = rl.RateLimit(model, prompt)
	assert.Error(t, err, "Should be rate limited after exceeding limit")
	assert.IsType(t, &InputRateLimitExceededError{}, err)
}

func TestTokenRateLimiter_LocalVsGlobal(t *testing.T) {
	mr, redisConfig := setupMiniRedis(t)
	defer mr.Close()

	rl := NewTokenRateLimiter()
	localModel := "local-model"
	globalModel := "global-model"
	prompt := "test prompt"
	tokens := uint32(5)
	unit := networkingv1alpha1.Second

	// Configure local rate limiting
	localConfig := &networkingv1alpha1.RateLimit{
		InputTokensPerUnit: &tokens,
		Unit:               unit,
		// No Global field = local rate limiting
	}

	// Configure global rate limiting
	globalConfig := &networkingv1alpha1.RateLimit{
		InputTokensPerUnit: &tokens,
		Unit:               unit,
		Global: &networkingv1alpha1.GlobalRateLimit{
			Redis: redisConfig,
		},
	}

	err := rl.AddOrUpdateLimiter(localModel, localConfig)
	require.NoError(t, err)

	err = rl.AddOrUpdateLimiter(globalModel, globalConfig)
	require.NoError(t, err)

	// Both should allow initial requests
	err = rl.RateLimit(localModel, prompt)
	assert.NoError(t, err)

	err = rl.RateLimit(globalModel, prompt)
	assert.NoError(t, err)

	// Use up local tokens
	err = rl.RateLimit(localModel, prompt)
	assert.Error(t, err, "Local model should be rate limited")

	// Use up global tokens
	err = rl.RateLimit(globalModel, prompt)
	assert.Error(t, err, "Global model should be rate limited")
}

func TestTokenRateLimiter_OutputTokens(t *testing.T) {
	mr, redisConfig := setupMiniRedis(t)
	defer mr.Close()

	rl := NewTokenRateLimiter()
	model := "test-model"
	outputTokens := uint32(50)
	unit := networkingv1alpha1.Second

	// Configure output token limiting
	config := &networkingv1alpha1.RateLimit{
		OutputTokensPerUnit: &outputTokens,
		Unit:                unit,
		Global: &networkingv1alpha1.GlobalRateLimit{
			Redis: redisConfig,
		},
	}

	err := rl.AddOrUpdateLimiter(model, config)
	require.NoError(t, err)

	// Record output tokens (should not block since it's async)
	rl.RecordOutputTokens(model, 25)
	rl.RecordOutputTokens(model, 30) // Total: 55, over limit

	// Give some time for async recording
	time.Sleep(100 * time.Millisecond)

	// Verify the tokens were recorded in Redis
	key := "kthena:ratelimit:test-model:output"
	exists := mr.Exists(key)
	assert.True(t, exists)
}

func TestTokenRateLimiter_GlobalDeleteLimiter(t *testing.T) {
	mr, redisConfig := setupMiniRedis(t)
	defer mr.Close()

	rl := NewTokenRateLimiter()
	model := "test-model"
	tokens := uint32(10)
	unit := networkingv1alpha1.Second

	// Add a rate limiter
	config := &networkingv1alpha1.RateLimit{
		InputTokensPerUnit: &tokens,
		Unit:               unit,
		Global: &networkingv1alpha1.GlobalRateLimit{
			Redis: redisConfig,
		},
	}

	err := rl.AddOrUpdateLimiter(model, config)
	require.NoError(t, err)

	// Verify it works
	err = rl.RateLimit(model, "test")
	assert.NoError(t, err)

	// Delete the limiter
	rl.DeleteLimiter(model)

	// Should now allow unlimited requests (no limiter configured)
	for i := 0; i < 10; i++ {
		err = rl.RateLimit(model, "test")
		assert.NoError(t, err, "Request %d should be allowed after deletion", i)
	}
}

func TestTokenRateLimiter_RedisConnectionFailure(t *testing.T) {
	rl := NewTokenRateLimiter()
	model := "test-model"
	tokens := uint32(10)
	unit := networkingv1alpha1.Second

	// Configure with non-existent Redis
	config := &networkingv1alpha1.RateLimit{
		InputTokensPerUnit: &tokens,
		Unit:               unit,
		Global: &networkingv1alpha1.GlobalRateLimit{
			Redis: &networkingv1alpha1.RedisConfig{
				Address: "localhost:9999", // Non-existent Redis
			},
		},
	}

	err := rl.AddOrUpdateLimiter(model, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to redis")
}
