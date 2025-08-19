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

// AllowN implements Limiter interface
func (g *GlobalRateLimiter) AllowN(now time.Time, n int) bool {
	key := fmt.Sprintf("%s:%s:%s", g.keyPrefix, g.modelName, g.tokenType)

	// Use sliding window algorithm with Redis
	windowStart := now.Add(-getTimeUnitDuration(g.unit))
	windowStartScore := float64(windowStart.Unix())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Remove old entries and add new entry atomically
	pipe := g.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%.0f", windowStartScore))
	pipe.ZAdd(ctx, key, &redis.Z{
		Score:  float64(now.Unix()),
		Member: fmt.Sprintf("%d:%d", now.UnixNano(), n),
	})
	pipe.Expire(ctx, key, getTimeUnitDuration(g.unit))

	_, err := pipe.Exec(ctx)
	if err != nil {
		klog.Errorf("failed to execute redis pipeline: %v", err)
		return false
	}

	// Get current usage
	totalTokens, err := g.getTotalTokensInWindow(key, windowStartScore)
	if err != nil {
		klog.Errorf("failed to get total tokens: %v", err)
		return false
	}

	return totalTokens <= int64(g.limit)
}

func (g *GlobalRateLimiter) getTotalTokensInWindow(key string, windowStart float64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := g.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%.0f", windowStart),
		Max: "+inf",
	})

	members, err := result.Result()
	if err != nil {
		return 0, err
	}

	var total int64
	for _, member := range members {
		var timestamp, tokens int64
		if _, err := fmt.Sscanf(member, "%d:%d", &timestamp, &tokens); err == nil {
			total += tokens
		}
	}

	return total, nil
}

// Tokens returns the estimated number of tokens currently available
func (g *GlobalRateLimiter) Tokens() float64 {
	// For global rate limiter, we estimate available tokens by checking current usage
	key := fmt.Sprintf("%s:%s:%s", g.keyPrefix, g.modelName, g.tokenType)
	windowStart := time.Now().Add(-getTimeUnitDuration(g.unit))
	windowStartScore := float64(windowStart.Unix())

	totalTokens, err := g.getTotalTokensInWindow(key, windowStartScore)
	if err != nil {
		klog.Errorf("failed to get total tokens for Tokens(): %v", err)
		return 0
	}

	available := int64(g.limit) - totalTokens
	if available < 0 {
		return 0
	}
	return float64(available)
}
