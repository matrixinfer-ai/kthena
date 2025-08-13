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
	"sync"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/klog/v2"

	networkingv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/filters/tokenizer"
)

type RateLimitExceededError struct{}

func (e RateLimitExceededError) Error() string {
	return "rate limit exceeded"
}

type InputRateLimitExceededError struct{}

func (e InputRateLimitExceededError) Error() string {
	return "input token rate limit exceeded"
}

type OutputRateLimitExceededError struct{}

func (e OutputRateLimitExceededError) Error() string {
	return "output token rate limit exceeded"
}

type TokenRateLimiter struct {
	mutex sync.RWMutex
	// ratelimiter by model
	inputLimiter  map[string]*rate.Limiter
	outputLimiter map[string]*rate.Limiter
	tokenizer     tokenizer.Tokenizer
}

func NewRateLimiter() *TokenRateLimiter {
	return &TokenRateLimiter{
		inputLimiter:  make(map[string]*rate.Limiter),
		outputLimiter: make(map[string]*rate.Limiter),
		tokenizer:     tokenizer.NewSimpleEstimateTokenizer(),
	}
}

func (r *TokenRateLimiter) RateLimit(model, prompt string) error {
	r.mutex.RLock()
	inputLimiter, hasInputLimit := r.inputLimiter[model]
	outputLimiter, hasOutputLimit := r.outputLimiter[model]
	r.mutex.RUnlock()

	// Check input token rate limit
	if hasInputLimit {
		size, err := r.tokenizer.CalculateTokenNum(prompt)
		if err != nil {
			return err
		}
		if !inputLimiter.AllowN(time.Now(), size) {
			return &InputRateLimitExceededError{}
		}
	}

	// Check if output rate limit has any capacity left
	// We check if there are enough tokens for a typical output (assume at least 1 token needed)
	// This is conservative - we only block if there's no capacity at all
	if hasOutputLimit && outputLimiter.Tokens() < 1.0 {
		return &OutputRateLimitExceededError{}
	}

	return nil
}

// RecordOutputTokens records the actual output tokens consumed after response generation
// This should be called after getting the response with completion_tokens
func (r *TokenRateLimiter) RecordOutputTokens(model string, tokenCount int) {
	if tokenCount <= 0 {
		return
	}

	r.mutex.RLock()
	limiter, exists := r.outputLimiter[model]
	r.mutex.RUnlock()

	if !exists {
		return
	}

	// Consume the actual tokens that were used
	limiter.AllowN(time.Now(), tokenCount)
}

func (r *TokenRateLimiter) AddOrUpdateLimiter(model string, ratelimit *networkingv1alpha1.RateLimit) {
	if model == "" {
		return
	}

	// Calculate time unit factor
	var unit float64
	switch ratelimit.Unit {
	case networkingv1alpha1.Second:
		unit = 1
	case networkingv1alpha1.Minute:
		unit = 60
	case networkingv1alpha1.Hour:
		unit = 3600
	case networkingv1alpha1.Day:
		unit = 24 * 3600
	case networkingv1alpha1.Month:
		unit = 30 * 24 * 3600 // Approximate a month as 30 days
	default:
		klog.Errorf("unknown rate limit unit: %s", ratelimit.Unit)
		return
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Handle input token rate limiting
	if ratelimit == nil || ratelimit.InputTokensPerUnit == nil || *ratelimit.InputTokensPerUnit <= 0 {
		delete(r.inputLimiter, model)
	} else {
		inputLimiter := rate.NewLimiter(rate.Limit(*ratelimit.InputTokensPerUnit)/rate.Limit(unit), int(*ratelimit.InputTokensPerUnit))
		r.inputLimiter[model] = inputLimiter
	}

	// Handle output token rate limiting
	if ratelimit == nil || ratelimit.OutputTokensPerUnit == nil || *ratelimit.OutputTokensPerUnit <= 0 {
		delete(r.outputLimiter, model)
	} else {
		outputLimiter := rate.NewLimiter(rate.Limit(*ratelimit.OutputTokensPerUnit)/rate.Limit(unit), int(*ratelimit.OutputTokensPerUnit))
		r.outputLimiter[model] = outputLimiter
	}
}

func (r *TokenRateLimiter) DeleteLimiter(model string) {
	if model == "" {
		return
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.inputLimiter, model)
	delete(r.outputLimiter, model)
}
