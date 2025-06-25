package ratelimit

import (
	"time"

	"golang.org/x/time/rate"

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

type RateLimit struct {
	limiter *rate.Limiter
	token   tokenizer.Tokenizer
}

func NewRateLimit() RateLimit {
	// TODO: replace API to get token value
	limit := rate.Every(100 * time.Millisecond)
	limiter := rate.NewLimiter(limit, 1000000)
	return RateLimit{
		limiter: limiter,
		token:   tokenizer.NewSimpleEstimateTokenizer(),
	}
}

func (r *RateLimit) RateLimit(prompt string) error {
	size, err := r.token.CalculateTokenNum(prompt)
	if err != nil {
		return err
	}

	if r.limiter.AllowN(time.Now(), size) {
		log.Info("rate limit allow")
		return nil
	}
	return &RateLimitExceededError{}
}
