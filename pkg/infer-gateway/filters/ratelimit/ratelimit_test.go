package ratelimit

import (
	"testing"
	"time"

	networkingv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
)

func TestTokenRateLimiter_Basic(t *testing.T) {
	rl := NewRateLimiter()
	model := "test-model"
	prompt := "hello world" // 3 tokens
	tokens := uint32(10)
	unit := networkingv1alpha1.Second

	rl.AddOrUpdateLimiter(model, &networkingv1alpha1.RateLimit{
		InputTokensPerUnit: &tokens,
		Unit:               unit,
	})

	// Should allow up to 10 tokens immediately
	for i := 0; i < 3; i++ {
		err := rl.RateLimit(model, prompt)
		if err != nil {
			t.Fatalf("unexpected error on allowed request: %v, %d", err, i)
		}
	}

	// 4th request should be rate limited
	err := rl.RateLimit(model, prompt)
	if err == nil {
		t.Fatalf("expected rate limit error, got nil")
	}
}

func TestTokenRateLimiter_NoLimiter(t *testing.T) {
	rl := NewRateLimiter()
	// No limiter added, should always allow
	err := rl.RateLimit("unknown-model", "test")
	if err != nil {
		t.Fatalf("expected nil error for unknown model, got %v", err)
	}
}

func TestTokenRateLimiter_ResetAfterTime(t *testing.T) {
	rl := NewRateLimiter()
	model := "test-model"
	prompt := "hello world"
	tokens := uint32(10)
	unit := networkingv1alpha1.Second

	rl.AddOrUpdateLimiter(model, &networkingv1alpha1.RateLimit{
		InputTokensPerUnit: &tokens,
		Unit:               unit,
	})

	// Use up tokens
	for i := 0; i < 3; i++ {
		err := rl.RateLimit(model, prompt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	// Should be rate limited now
	err := rl.RateLimit(model, prompt)
	if err == nil {
		t.Fatalf("expected rate limit error, got nil")
	}

	// Wait for refill
	time.Sleep(1100 * time.Millisecond)
	err = rl.RateLimit(model, prompt)
	if err != nil {
		t.Fatalf("expected nil after refill, got %v", err)
	}
}
