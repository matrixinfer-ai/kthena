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

func TestTokenRateLimiter_OutputRateLimit_TokenCount(t *testing.T) {
	rl := NewRateLimiter()
	model := "test-model"
	tokenCount := 3
	tokens := uint32(10)
	unit := networkingv1alpha1.Second

	rl.AddOrUpdateLimiter(model, &networkingv1alpha1.RateLimit{
		OutputTokensPerUnit: &tokens,
		Unit:                unit,
	})

	// Should allow up to 10 tokens immediately
	for i := 0; i < 3; i++ {
		err := rl.RateLimitOutputTokens(model, tokenCount)
		if err != nil {
			t.Fatalf("unexpected error on allowed output request: %v, %d", err, i)
		}
	}

	// 4th request should be rate limited
	err := rl.RateLimitOutputTokens(model, tokenCount)
	if err == nil {
		t.Fatalf("expected output rate limit error, got nil")
	}
}

func TestTokenRateLimiter_CombinedInputOutput(t *testing.T) {
	rl := NewRateLimiter()
	model := "test-model"
	prompt := "hello"         // 5 chars → ceil(5/4) = 2 tokens
	outputTokenCount := 2     // Direct token count for output
	inputTokens := uint32(6)  // Allow 3 input requests (3*2=6)
	outputTokens := uint32(4) // Allow 2 output requests (2*2=4)
	unit := networkingv1alpha1.Second

	rl.AddOrUpdateLimiter(model, &networkingv1alpha1.RateLimit{
		InputTokensPerUnit:  &inputTokens,
		OutputTokensPerUnit: &outputTokens,
		Unit:                unit,
	})

	// Should allow 3 input requests (3*2=6 tokens)
	for i := 0; i < 3; i++ {
		err := rl.RateLimit(model, prompt)
		if err != nil {
			t.Fatalf("unexpected error on allowed input request: %v, %d", err, i)
		}
	}

	// Should allow 2 output requests (2*2=4 tokens)
	for i := 0; i < 2; i++ {
		err := rl.RateLimitOutputTokens(model, outputTokenCount)
		if err != nil {
			t.Fatalf("unexpected error on allowed output request: %v, %d", err, i)
		}
	}

	// Both should now be rate limited
	err := rl.RateLimit(model, prompt)
	if err == nil {
		t.Fatalf("expected input rate limit error, got nil")
	}

	err = rl.RateLimitOutputTokens(model, outputTokenCount)
	if err == nil {
		t.Fatalf("expected output rate limit error, got nil")
	}
}

func TestTokenRateLimiter_OutputNoLimiter(t *testing.T) {
	rl := NewRateLimiter()
	// No limiter added, should always allow
	err := rl.RateLimitOutputTokens("unknown-model", 100)
	if err != nil {
		t.Fatalf("expected nil error for unknown model output tokens, got %v", err)
	}
}

func TestTokenRateLimiter_DeleteLimiter_BothInputOutput(t *testing.T) {
	rl := NewRateLimiter()
	model := "test-model"
	inputTokens := uint32(10)
	outputTokens := uint32(5)
	unit := networkingv1alpha1.Second

	rl.AddOrUpdateLimiter(model, &networkingv1alpha1.RateLimit{
		InputTokensPerUnit:  &inputTokens,
		OutputTokensPerUnit: &outputTokens,
		Unit:                unit,
	})

	// Verify limiters exist and work
	err := rl.RateLimit(model, "test")
	if err != nil {
		t.Fatalf("unexpected error before deletion: %v", err)
	}
	err = rl.RateLimitOutputTokens(model, 1)
	if err != nil {
		t.Fatalf("unexpected error before deletion: %v", err)
	}

	// Delete limiters
	rl.DeleteLimiter(model)

	// Both should now be unrestricted
	err = rl.RateLimit(model, "test")
	if err != nil {
		t.Fatalf("expected nil after deletion, got %v", err)
	}
	err = rl.RateLimitOutputTokens(model, 1)
	if err != nil {
		t.Fatalf("expected nil after deletion, got %v", err)
	}
}
