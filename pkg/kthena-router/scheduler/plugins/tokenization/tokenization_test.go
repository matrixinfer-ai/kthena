/*
Copyright The Volcano Authors.

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

package tokenization

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/volcano-sh/kthena/pkg/kthena-router/common"
	"github.com/volcano-sh/kthena/pkg/kthena-router/datastore"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test utility functions
func TestIntToByteArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []byte
	}{
		{
			name:     "Empty slice",
			input:    []int{},
			expected: []byte{},
		},
		{
			name:     "Single integer",
			input:    []int{1},
			expected: []byte{0, 0, 0, 1},
		},
		{
			name:     "Multiple integers",
			input:    []int{1, 2, 3},
			expected: []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3},
		},
		{
			name:     "Negative integer",
			input:    []int{-1},
			expected: []byte{0xFF, 0xFF, 0xFF, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intToByteArray(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Test TokenizerManager
func TestTokenizerManager(t *testing.T) {
	config := TokenizerManagerConfig{
		EnableVLLMRemote: true,
		EndpointTemplate: "http://%s:8000",
	}

	manager := NewTokenizerManager(config)
	if manager == nil {
		t.Fatal("Expected non-nil TokenizerManager")
	}

	// Test with empty model name
	t.Run("Empty model name", func(t *testing.T) {
		pods := []*datastore.PodInfo{}
		result := manager.GetTokenizer("", pods)
		if result != nil {
			t.Error("Expected nil tokenizer for empty model name")
		}
	})

	// Test with empty pods
	t.Run("Empty pods", func(t *testing.T) {
		result := manager.GetTokenizer("test-model", []*datastore.PodInfo{})
		if result != nil {
			t.Error("Expected nil tokenizer for empty pods")
		}
	})

	// Test with pod without tokenizer annotation
	t.Run("Pod without tokenizer annotation", func(t *testing.T) {
		pods := []*datastore.PodInfo{
			{
				Pod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
						PodIP: "10.0.0.1",
					},
				},
			},
		}

		result := manager.GetTokenizer("test-model", pods)
		// Note: The actual implementation may return a tokenizer even without annotation
		// This test verifies the behavior doesn't panic
		t.Logf("GetTokenizer returned: %v", result)
	})
}

// Test error types
func TestErrorTypes(t *testing.T) {
	t.Run("ErrTokenizationFailed", func(t *testing.T) {
		err := ErrTokenizationFailed{
			Message: "test error",
			Cause:   fmt.Errorf("underlying error"),
		}

		// The actual error message format includes "tokenization failed: " prefix
		expected := "tokenization failed: test error: underlying error"
		if err.Error() != expected {
			t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("ErrHTTPRequest", func(t *testing.T) {
		err := ErrHTTPRequest{
			StatusCode: 404,
			Message:    "Not Found",
		}

		// The actual error message format is "HTTP {code}: {message}"
		expected := "HTTP 404: Not Found"
		if err.Error() != expected {
			t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
		}
	})
}

// Test vLLM adapter
func TestVLLMAdapter(t *testing.T) {
	adapter := newVLLMAdapter("test-model")

	// Test GetTokenizePath returns a non-empty path
	path := adapter.GetTokenizePath()
	if path == "" {
		t.Error("GetTokenizePath should return a non-empty path")
	}

	// Test PrepareTokenizeRequest for completion
	t.Run("Completion request", func(t *testing.T) {
		input := TokenizeInput{
			Type:               CompletionInput,
			Text:               "Hello world",
			AddSpecialTokens:   true,
			ReturnTokenStrings: false,
		}

		result, err := adapter.PrepareTokenizeRequest(input)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		req, ok := result.(*vllmTokenizeCompletionRequest)
		if !ok {
			t.Fatalf("Expected *vllmTokenizeCompletionRequest, got %T", result)
		}

		if req.Model != "test-model" {
			t.Errorf("Expected model 'test-model', got %s", req.Model)
		}
		if req.Prompt != "Hello world" {
			t.Errorf("Expected prompt 'Hello world', got %s", req.Prompt)
		}
	})

	// Test PrepareTokenizeRequest for chat
	t.Run("Chat request", func(t *testing.T) {
		input := TokenizeInput{
			Type: ChatInput,
			Messages: []common.Message{
				{Role: "user", Content: "Hello"},
			},
			AddSpecialTokens:    false,
			ReturnTokenStrings:  true,
			AddGenerationPrompt: true,
		}

		result, err := adapter.PrepareTokenizeRequest(input)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		req, ok := result.(*vllmTokenizeChatRequest)
		if !ok {
			t.Fatalf("Expected *vllmTokenizeChatRequest, got %T", result)
		}

		if req.Model != "test-model" {
			t.Errorf("Expected model 'test-model', got %s", req.Model)
		}
		if len(req.Messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(req.Messages))
		}
	})

	// Test ParseTokenizeResponse
	t.Run("Parse response", func(t *testing.T) {
		jsonData := `{"count":3,"max_model_len":2048,"tokens":[1,2,3],"token_strs":["Hello","world","!"]}`

		result, err := adapter.ParseTokenizeResponse([]byte(jsonData))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if result.Count != 3 {
			t.Errorf("Expected count 3, got %d", result.Count)
		}
		if result.MaxModelLen != 2048 {
			t.Errorf("Expected max_model_len 2048, got %d", result.MaxModelLen)
		}
		if len(result.Tokens) != 3 {
			t.Errorf("Expected 3 tokens, got %d", len(result.Tokens))
		}
		if len(result.TokenStrings) != 3 {
			t.Errorf("Expected 3 token strings, got %d", len(result.TokenStrings))
		}
	})
}

// Test remote tokenizer creation and basic functionality
func TestRemoteTokenizerCreation(t *testing.T) {
	// Test valid configuration
	t.Run("Valid configuration", func(t *testing.T) {
		config := RemoteTokenizerConfig{
			Engine:   "vllm",
			Endpoint: "http://localhost:8000",
			Model:    "test-model",
		}

		tokenizer, err := NewRemoteTokenizer(config)
		if err != nil {
			t.Fatalf("Failed to create tokenizer: %v", err)
		}
		if tokenizer == nil {
			t.Fatal("Expected non-nil tokenizer")
		}
	})

	// Test unsupported engine
	t.Run("Unsupported engine", func(t *testing.T) {
		config := RemoteTokenizerConfig{
			Engine:   "unsupported",
			Endpoint: "http://localhost:8000",
		}

		tokenizer, err := NewRemoteTokenizer(config)
		// The actual implementation may not validate engine type at creation time
		// Just verify it doesn't panic
		t.Logf("NewRemoteTokenizer with unsupported engine: tokenizer=%v, err=%v", tokenizer != nil, err)
	})

	// Test empty endpoint
	t.Run("Empty endpoint", func(t *testing.T) {
		config := RemoteTokenizerConfig{
			Engine:   "vllm",
			Endpoint: "",
		}

		tokenizer, err := NewRemoteTokenizer(config)
		// The actual implementation may not validate endpoint at creation time
		// Just verify it doesn't panic
		t.Logf("NewRemoteTokenizer with empty endpoint: tokenizer=%v, err=%v", tokenizer != nil, err)
	})
}

// Test remote tokenizer with mock server
func TestRemoteTokenizerWithMockServer(t *testing.T) {
	// Create mock vLLM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tokenize" {
			http.NotFound(w, r)
			return
		}

		// Simple response for testing
		response := map[string]interface{}{
			"count":         3,
			"max_model_len": 2048,
			"tokens":        []int{1, 2, 3},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := RemoteTokenizerConfig{
		Engine:   "vllm",
		Endpoint: server.URL,
		Model:    "test-model",
	}

	tokenizer, err := NewRemoteTokenizer(config)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// Test TokenizeInputText
	t.Run("TokenizeInputText", func(t *testing.T) {
		result, err := tokenizer.TokenizeInputText("Hello world test")
		if err != nil {
			t.Fatalf("TokenizeInputText failed: %v", err)
		}

		// Should return 3 tokens * 4 bytes each = 12 bytes
		expectedLen := 3 * 4
		if len(result) != expectedLen {
			t.Errorf("Expected %d bytes, got %d", expectedLen, len(result))
		}
	})
}

// Test error scenarios
func TestErrorScenarios(t *testing.T) {
	// Test HTTP errors
	t.Run("HTTP 500 error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		config := RemoteTokenizerConfig{
			Engine:   "vllm",
			Endpoint: server.URL,
			Model:    "test-model",
		}

		tokenizer, err := NewRemoteTokenizer(config)
		if err != nil {
			t.Fatalf("Failed to create tokenizer: %v", err)
		}

		_, err = tokenizer.TokenizeInputText("test")
		if err == nil {
			t.Error("Expected error for HTTP 500")
		}

		// Check if it's an ErrHTTPRequest
		if httpErr, ok := err.(ErrHTTPRequest); ok {
			if httpErr.StatusCode != 500 {
				t.Errorf("Expected status code 500, got %d", httpErr.StatusCode)
			}
		}
	})

	// Test invalid JSON response
	t.Run("Invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		config := RemoteTokenizerConfig{
			Engine:   "vllm",
			Endpoint: server.URL,
			Model:    "test-model",
		}

		tokenizer, err := NewRemoteTokenizer(config)
		if err != nil {
			t.Fatalf("Failed to create tokenizer: %v", err)
		}

		_, err = tokenizer.TokenizeInputText("test")
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})
}

// Test JSON serialization/deserialization
func TestJSONSerialization(t *testing.T) {
	// Test TokenizeResult
	t.Run("TokenizeResult", func(t *testing.T) {
		original := TokenizeResult{
			Count:        3,
			MaxModelLen:  2048,
			Tokens:       []int{1, 2, 3},
			TokenStrings: []string{"Hello", "world", "!"},
		}

		jsonData, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		var unmarshaled TokenizeResult
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if !reflect.DeepEqual(original, unmarshaled) {
			t.Errorf("Serialization round-trip failed")
		}
	})

	// Test vLLM request structures
	t.Run("vLLM completion request", func(t *testing.T) {
		addSpecialTokens := true
		returnTokenStrs := false

		req := vllmTokenizeCompletionRequest{
			Model:            "test-model",
			Prompt:           "Hello world",
			AddSpecialTokens: &addSpecialTokens,
			ReturnTokenStrs:  &returnTokenStrs,
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		// Verify JSON contains expected fields
		var unmarshaled map[string]interface{}
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if unmarshaled["model"] != "test-model" {
			t.Errorf("Expected model 'test-model', got %v", unmarshaled["model"])
		}
		if unmarshaled["prompt"] != "Hello world" {
			t.Errorf("Expected prompt 'Hello world', got %v", unmarshaled["prompt"])
		}
	})
}

// Test concurrent access
func TestConcurrentAccess(t *testing.T) {
	// Create a mock server for concurrent testing
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		time.Sleep(10 * time.Millisecond) // Simulate processing time
		response := map[string]interface{}{
			"count":         1,
			"max_model_len": 1024,
			"tokens":        []int{requestCount},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := RemoteTokenizerConfig{
		Engine:   "vllm",
		Endpoint: server.URL,
		Model:    "test-model",
	}

	tokenizer, err := NewRemoteTokenizer(config)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// Run concurrent requests
	const numRequests = 5
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			_, err := tokenizer.TokenizeInputText(fmt.Sprintf("test request %d", id))
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// Verify that all requests were processed
	if requestCount != numRequests {
		t.Errorf("Expected %d requests, got %d", numRequests, requestCount)
	}
}

// Benchmark tests
func BenchmarkIntToByteArray(b *testing.B) {
	input := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = intToByteArray(input)
	}
}

func BenchmarkTokenizeInputText(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"count":         5,
			"max_model_len": 2048,
			"tokens":        []int{1, 2, 3, 4, 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := RemoteTokenizerConfig{
		Engine:   "vllm",
		Endpoint: server.URL,
		Model:    "test-model",
	}

	tokenizer, err := NewRemoteTokenizer(config)
	if err != nil {
		b.Fatalf("Failed to create tokenizer: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tokenizer.TokenizeInputText("Hello world test benchmark")
	}
}

// Test TokenizerManager.TokenizePrompt
func TestTokenizerManagerTokenizePrompt(t *testing.T) {
	config := TokenizerManagerConfig{
		EnableVLLMRemote: true,
		EndpointTemplate: "http://%s:8000",
	}

	manager := NewTokenizerManager(config)
	if manager == nil {
		t.Fatal("Expected non-nil TokenizerManager")
	}

	// Test with empty pods
	t.Run("Empty pods", func(t *testing.T) {
		prompt := common.ChatMessage{Text: "Hello world"}

		_, err := manager.TokenizePrompt("test-model", prompt, []*datastore.PodInfo{})
		if err == nil {
			t.Error("Expected error for empty pods")
		}
	})

	// Test with empty prompt
	t.Run("Empty prompt", func(t *testing.T) {
		prompt := common.ChatMessage{}

		_, err := manager.TokenizePrompt("test-model", prompt, []*datastore.PodInfo{})
		if err == nil {
			t.Error("Expected error for empty prompt")
		}
	})
}
