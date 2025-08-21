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
	if _, ok := err.(*InputRateLimitExceededError); !ok {
		t.Fatalf("expected InputRateLimitExceededError, got %T: %v", err, err)
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
	if _, ok := err.(*InputRateLimitExceededError); !ok {
		t.Fatalf("expected InputRateLimitExceededError, got %T: %v", err, err)
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
	tokens := uint32(5) // Small limit
	unit := networkingv1alpha1.Second

	rl.AddOrUpdateLimiter(model, &networkingv1alpha1.RateLimit{
		OutputTokensPerUnit: &tokens,
		Unit:                unit,
	})

	// Consume exactly 5 tokens to exhaust the bucket
	err := rl.RateLimit(model, "test prompt")
	if err != nil {
		t.Fatalf("unexpected error on first request: %v", err)
	}
	rl.RecordOutputTokens(model, 5) // Consume all 5 tokens

	// Next request should be rate limited (no tokens left)
	err = rl.RateLimit(model, "test prompt")
	if err == nil {
		t.Fatalf("expected output rate limit error after exhausting tokens, got nil")
	}
	if _, ok := err.(*OutputRateLimitExceededError); !ok {
		t.Fatalf("expected OutputRateLimitExceededError, got %T: %v", err, err)
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

	// Should allow 2 combined requests (limited by output tokens: 2*2=4)
	for i := 0; i < 2; i++ {
		// This checks both input tokens (consumes 2) and output capacity (just checks, doesn't consume)
		err := rl.RateLimit(model, prompt)
		if err != nil {
			t.Fatalf("unexpected error on allowed combined request: %v, %d", err, i)
		}
		// Record actual output tokens used (consumes 2 output tokens)
		rl.RecordOutputTokens(model, outputTokenCount)
	}

	// 3rd request should be rate limited due to output token limit (4 used, next needs 2 more = 6 > 4)
	err := rl.RateLimit(model, prompt)
	if err == nil {
		t.Fatalf("expected rate limit error, got nil")
	}
	if _, ok := err.(*OutputRateLimitExceededError); !ok {
		t.Fatalf("expected OutputRateLimitExceededError, got %T: %v", err, err)
	}
}

func TestTokenRateLimiter_OutputNoLimiter(t *testing.T) {
	rl := NewRateLimiter()
	// No limiter added, should always allow
	rl.RecordOutputTokens("unknown-model", 100)
	// RecordOutputTokens doesn't return error, just silently does nothing
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
	// Record some output tokens to verify output limiter works
	rl.RecordOutputTokens(model, 1)

	// Delete limiters
	rl.DeleteLimiter(model)

	// Both should now be unrestricted
	err = rl.RateLimit(model, "test")
	if err != nil {
		t.Fatalf("expected nil after deletion, got %v", err)
	}
	// Recording output tokens should work without error (no-op for deleted limiter)
	rl.RecordOutputTokens(model, 1)
}

func TestTokenRateLimiter_DifferentErrorTypes(t *testing.T) {
	rl := NewRateLimiter()
	model := "test-model"
	prompt := "hello world"   // 3 tokens
	inputTokens := uint32(5)  // Allow only 1 input request (3 tokens)
	outputTokens := uint32(2) // Allow only 2 output tokens total
	unit := networkingv1alpha1.Second

	// Test input rate limit error
	rl.AddOrUpdateLimiter(model+"-input", &networkingv1alpha1.RateLimit{
		InputTokensPerUnit: &inputTokens,
		Unit:               unit,
	})

	// First request should pass
	err := rl.RateLimit(model+"-input", prompt)
	if err != nil {
		t.Fatalf("first request should pass, got error: %v", err)
	}

	// Second request should fail with input rate limit error
	err = rl.RateLimit(model+"-input", prompt)
	if err == nil {
		t.Fatalf("expected input rate limit error, got nil")
	}
	if _, ok := err.(*InputRateLimitExceededError); !ok {
		t.Fatalf("expected InputRateLimitExceededError, got %T: %v", err, err)
	}

	// Test output rate limit error
	rl.AddOrUpdateLimiter(model+"-output", &networkingv1alpha1.RateLimit{
		OutputTokensPerUnit: &outputTokens,
		Unit:                unit,
	})

	// First request should pass
	err = rl.RateLimit(model+"-output", prompt)
	if err != nil {
		t.Fatalf("first request should pass, got error: %v", err)
	}
	// Record 2 tokens used (reaching the limit)
	rl.RecordOutputTokens(model+"-output", 2)

	// Second request should fail with output rate limit error (2 tokens used, no capacity for more)
	err = rl.RateLimit(model+"-output", prompt)
	if err == nil {
		t.Fatalf("expected output rate limit error, got nil")
	}
	if _, ok := err.(*OutputRateLimitExceededError); !ok {
		t.Fatalf("expected OutputRateLimitExceededError, got %T: %v", err, err)
	}
}
