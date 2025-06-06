package ratelimit

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/filters/tokenizer"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
)

var (
	log = logger.NewLogger("ratelimit")
)

type RateLimit struct {
	limiter *rate.Limiter
	token   *tokenizer.StringsTokenizer
}

func NewRateLimit() RateLimit {
	limit := rate.Every(100 * time.Millisecond)
	limiter := rate.NewLimiter(limit, 1000000)
	return RateLimit{
		limiter: limiter,
	}
}

func (r *RateLimit) SingleNodeRateLimit(prompt string) error {
	size, err := r.token.CalculateTokenNum(prompt)
	if err != nil {
		return err
	}

	if r.limiter.AllowN(time.Now(), size) {
		log.Info("rate limit allow")
	} else {
		log.Error("rate limit disallow")
		// set 1 min wait timeout
		cxt, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := r.limiter.WaitN(cxt, 1)
		if err != nil {
			r.limiter.SetLimit(20)
			return err
		}
	}
	return nil
}
