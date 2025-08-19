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

package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"k8s.io/klog/v2"

	networkingv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
)

// GlobalRateLimiter implements Limiter interface using Redis
type GlobalRateLimiter struct {
	client    *redis.Client
	keyPrefix string
	modelName string
	tokenType string
	limit     uint32
	unit      networkingv1alpha1.RateLimitUnit
	burst     int
}

// NewGlobalRateLimiter creates a new GlobalRateLimiter instance
func NewGlobalRateLimiter(client *redis.Client, keyPrefix, modelName, tokenType string, limit uint32, unit networkingv1alpha1.RateLimitUnit) *GlobalRateLimiter {
	return &GlobalRateLimiter{
		client:    client,
		keyPrefix: keyPrefix,
		modelName: modelName,
		tokenType: tokenType,
		limit:     limit,
		unit:      unit,
		burst:     int(limit),
	}
}

// AllowN implements Limiter interface using token bucket algorithm
func (g *GlobalRateLimiter) AllowN(now time.Time, n int) bool {
	key := fmt.Sprintf("%s:%s:%s", g.keyPrefix, g.modelName, g.tokenType)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use Redis Lua script for atomic token bucket operations
	luaScript := `
		local key = KEYS[1]
		local requested_tokens = tonumber(ARGV[1])
		local capacity = tonumber(ARGV[2])
		local refill_rate = tonumber(ARGV[3])
		local current_time = tonumber(ARGV[4])
		local expire_seconds = tonumber(ARGV[5])
		
		-- Get current state
		local bucket_data = redis.call('hmget', key, 'tokens', 'last_update')
		local current_tokens = tonumber(bucket_data[1]) or capacity
		local last_update = tonumber(bucket_data[2]) or current_time
		
		-- Calculate tokens to add based on time elapsed
		local time_passed = math.max(0, current_time - last_update)
		local tokens_to_add = time_passed * refill_rate
		
		-- Update current tokens (capped at capacity)
		current_tokens = math.min(capacity, current_tokens + tokens_to_add)
		
		-- Check if we have enough tokens
		if current_tokens >= requested_tokens then
			-- Consume tokens
			current_tokens = current_tokens - requested_tokens
			
			-- Update Redis state
			redis.call('hmset', key, 'tokens', current_tokens, 'last_update', current_time)
			redis.call('expire', key, expire_seconds)
			
			return 1 -- Success
		else
			-- Not enough tokens, but still update the bucket state
			redis.call('hmset', key, 'tokens', current_tokens, 'last_update', current_time)
			redis.call('expire', key, expire_seconds)
			
			return 0 -- Failed
		end
	`

	// Calculate refill rate (tokens per second) and expire time
	refillRate := g.getRefillRate()
	expireSeconds := g.getExpireSeconds()
	currentTime := float64(now.Unix()) + float64(now.Nanosecond())/1e9

	result := g.client.Eval(ctx, luaScript, []string{key}, n, g.burst, refillRate, currentTime, expireSeconds)

	if result.Err() != nil {
		klog.Errorf("failed to execute token bucket lua script: %v", result.Err())
		return false
	}

	allowed, ok := result.Val().(int64)
	if !ok {
		klog.Errorf("unexpected result type from lua script: %T", result.Val())
		return false
	}

	return allowed == 1
}

// getRefillRate calculates the token refill rate per second
func (g *GlobalRateLimiter) getRefillRate() float64 {
	duration := getTimeUnitDuration(g.unit)
	return float64(g.limit) / duration.Seconds()
}

// getExpireSeconds calculates appropriate expire time based on rate limit unit
func (g *GlobalRateLimiter) getExpireSeconds() int {
	duration := getTimeUnitDuration(g.unit)
	// Set expire time to 3x the rate limit unit duration, with reasonable bounds
	expireSeconds := int(duration.Seconds() * 3)

	// Set reasonable bounds
	if expireSeconds < 600 { // Minimum 10 minutes
		expireSeconds = 600
	} else if expireSeconds > 7776000 { // Maximum 90 days
		expireSeconds = 7776000
	}

	return expireSeconds
}

// Tokens returns the estimated number of tokens currently available
func (g *GlobalRateLimiter) Tokens() float64 {
	key := fmt.Sprintf("%s:%s:%s", g.keyPrefix, g.modelName, g.tokenType)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Lua script to get current available tokens without consuming any
	luaScript := `
		local key = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local refill_rate = tonumber(ARGV[2])
		local current_time = tonumber(ARGV[3])
		local expire_seconds = tonumber(ARGV[4])
		
		-- Get current state
		local bucket_data = redis.call('hmget', key, 'tokens', 'last_update')
		local current_tokens = tonumber(bucket_data[1]) or capacity
		local last_update = tonumber(bucket_data[2]) or current_time
		
		-- Calculate tokens to add based on time elapsed
		local time_passed = math.max(0, current_time - last_update)
		local tokens_to_add = time_passed * refill_rate
		
		-- Calculate current available tokens (capped at capacity)
		local available_tokens = math.min(capacity, current_tokens + tokens_to_add)
		
		-- Update bucket state for next time (even though we're just checking)
		redis.call('hmset', key, 'tokens', available_tokens, 'last_update', current_time)
		redis.call('expire', key, expire_seconds)
		
		return available_tokens
	`

	refillRate := g.getRefillRate()
	expireSeconds := g.getExpireSeconds()
	currentTime := float64(time.Now().Unix()) + float64(time.Now().Nanosecond())/1e9

	result := g.client.Eval(ctx, luaScript, []string{key}, g.burst, refillRate, currentTime, expireSeconds)

	if result.Err() != nil {
		klog.Errorf("failed to execute tokens check lua script: %v", result.Err())
		return 0
	}

	tokens, ok := result.Val().(float64)
	if !ok {
		// Try to convert from int64 to float64
		if tokensInt, ok := result.Val().(int64); ok {
			return float64(tokensInt)
		}
		klog.Errorf("unexpected result type from tokens lua script: %T", result.Val())
		return 0
	}

	return tokens
}
