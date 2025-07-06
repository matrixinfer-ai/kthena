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

	networkingv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/filters/tokenizer"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
)

var (
	log = logger.NewLogger("ratelimit")
)

type RateLimitExceededError struct{}

func (e RateLimitExceededError) Error() string {
	return "rate limit exceeded"
}

type TokenRateLimiter struct {
	mutex sync.RWMutex
	// ratelimiter by model
	inputLimiter map[string]*rate.Limiter
	tokenizer    tokenizer.Tokenizer
}

func NewRateLimiter() *TokenRateLimiter {
	return &TokenRateLimiter{
		inputLimiter: make(map[string]*rate.Limiter),
		tokenizer:    tokenizer.NewSimpleEstimateTokenizer(),
	}
}

func (r *TokenRateLimiter) RateLimit(model, prompt string) (int, error) {
	r.mutex.RLock()
	limiter, exists := r.inputLimiter[model]
	r.mutex.RUnlock()
	if !exists {
		return 0,nil
	}

	size, err := r.tokenizer.CalculateTokenNum(prompt)
	if err != nil {
		return 0, err
	}
	if limiter.AllowN(time.Now(), size) {
		return size, nil
	}
	return size, &RateLimitExceededError{}
}

func (r *TokenRateLimiter) AddOrUpdateLimiter(model string, ratelimit *networkingv1alpha1.RateLimit) {
	if model == "" {
		return
	}

	// TODO: only handle input tokens first, add output tokens later
	if ratelimit == nil || ratelimit.InputTokensPerUnit == nil || *ratelimit.InputTokensPerUnit <= 0 {
		r.mutex.Lock()
		delete(r.inputLimiter, model)
		r.mutex.Unlock()
		return
	}
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
		log.Errorf("unknown rate limit unit: %s", ratelimit.Unit)
		return
	}

	limiter := rate.NewLimiter(rate.Limit(*ratelimit.InputTokensPerUnit)/rate.Limit(unit), int(*ratelimit.InputTokensPerUnit))
	r.mutex.Lock()
	r.inputLimiter[model] = limiter
	r.mutex.Unlock()
}

func (r *TokenRateLimiter) DeleteLimiter(model string) {
	if model == "" {
		return
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.inputLimiter, model)
}
