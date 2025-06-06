package ratelimit

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/filters/tokenizer"
)

type RateLimit struct {
	token *tokenizer.StringsTokenizer
}

func NewRateLimit() RateLimit {
	return RateLimit{}
}

func (r *RateLimit) SingleNodeRateLimit(prompt string) error {
	size, err := r.token.CalculateTokenNum(prompt)
	if err != nil {
		return err
	}

	limit := rate.Every(100 * time.Millisecond)
	limiter := rate.NewLimiter(limit, 1000000)

	if limiter.AllowN(time.Now(), size) {
		fmt.Println("rate limit allow")
	} else {
		fmt.Println("rate limit disallow")
		// set 1 min wait timeout
		cxt, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := limiter.Wait(cxt)
		if err != nil {
			limiter.SetLimit(20)
			return err
		}
	}
	return nil
}
