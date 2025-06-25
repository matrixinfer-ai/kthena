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

func (r *TokenRateLimiter) RateLimit(model, prompt string) error {
	r.mutex.RLock()
	limiter, exists := r.inputLimiter[model]
	r.mutex.RUnlock()
	if !exists {
		return nil
	}

	size, err := r.tokenizer.CalculateTokenNum(prompt)
	if err != nil {
		return err
	}

	if limiter.AllowN(time.Now(), size) {
		return nil
	}
	return &RateLimitExceededError{}
}

func (r *TokenRateLimiter) AddOrUpdateLimiter(model string, ratelimit *networkingv1alpha1.RateLimit) {
	if model == "" || ratelimit == nil {
		return
	}

	// TODO: only handle input tokens first, add output tokens later
	if ratelimit.InputTokensPerUnit == nil || *ratelimit.InputTokensPerUnit <= 0 {
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

// GetLimiter returns the rate limiter for the given model, or nil if it doesn't exist
