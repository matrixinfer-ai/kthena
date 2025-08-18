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

package connectors

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/filters/ratelimit"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/handlers"
)

const (
	tokenUsageKey = "token_usage"
)

func prefillerProxy(c *gin.Context, req *http.Request) error {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("prefill request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("prefill request failed with status %d", resp.StatusCode)
	}

	klog.V(4).Infof("Prefill request completed successfully")
	return nil
}

func decoderProxy(c *gin.Context, req *http.Request, rateLimiter *ratelimit.TokenRateLimiter, modelName string) error {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("decode request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("decode request failed with status %d", resp.StatusCode)
	}

	// Copy response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Header(k, v)
		}
	}

	c.Status(resp.StatusCode)

	// Determine if this is a streaming response
	stream := isStreamingResponse(resp)

	if stream {
		// Handle streaming response
		return handleStreamingResponse(c, resp, rateLimiter, modelName)
	} else {
		// Handle non-streaming response
		return handleNonStreamingResponse(c, resp, rateLimiter, modelName)
	}
}

// preparePrefillBody modifies a request body for a prefill request.
// It removes streaming options and sets the token counts appropriately.
func preparePrefillBody(reqBody map[string]interface{}) {
	delete(reqBody, "stream")
	delete(reqBody, "stream_options")

	reqBody["max_tokens"] = 1
	if reqBody["max_completion_tokens"] != nil {
		reqBody["max_completion_tokens"] = 1
	}
}

func buildPrefillRequest(req *http.Request, modelRequest map[string]interface{}) *http.Request {
	// In PD disaggregated mode, we need to send a prefill request to the prefill pod with non stream mode.
	preparePrefillBody(modelRequest)

	body, err := json.Marshal(modelRequest)
	if err != nil {
		return nil
	}

	// build request
	reqCopy := req.Clone(req.Context())
	reqCopy.URL.Scheme = "http"
	reqCopy.Body = io.NopCloser(bytes.NewBuffer(body))
	reqCopy.ContentLength = int64(len(body))

	return reqCopy
}

func BuildDecodeRequest(c *gin.Context, req *http.Request, modelRequest map[string]interface{}) *http.Request {
	modelRequest = addTokenUsage(c, modelRequest)
	body, err := json.Marshal(modelRequest)
	if err != nil {
		return nil
	}

	// build request
	req.URL.Scheme = "http"
	req.Body = io.NopCloser(bytes.NewBuffer(body))
	req.ContentLength = int64(len(body))

	return req
}

// addTokenUsage adds token usage to the request body if it is not already present
// should be used for decode requests or non PD disaggregated mode
func addTokenUsage(c *gin.Context, reqBody map[string]interface{}) map[string]interface{} {
	// Check if streaming is enabled
	if isStreamingRequest(reqBody) {
		if !isTokenUsageEnabled(reqBody) {
			// For streaming requests, add stream_options to include token usage
			reqBody["stream_options"] = map[string]interface{}{
				"include_usage": true,
			}
			// add stream token usage to context
			c.Set(tokenUsageKey, true)
		}
	} else {
		// For non-streaming requests, ensure we request usage information
		// Most OpenAI-compatible APIs return usage by default for non-streaming,
		// but we can be explicit about it
		reqBody["include_usage"] = true
	}
	return reqBody
}

// isStreaming checks if the given model request has streaming enabled
func isStreamingRequest(modelRequest map[string]interface{}) bool {
	if v, ok := modelRequest["stream"]; ok {
		if stream, isBool := v.(bool); isBool && stream {
			return true
		}
	}
	return false
}

func isTokenUsageEnabled(modelRequest map[string]interface{}) bool {
	// Check if token usage is enabled in the model request
	if v, ok := modelRequest["stream_options"]; ok {
		if streamOptions, isMap := v.(map[string]interface{}); isMap {
			if includeUsage, isBool := streamOptions["include_usage"].(bool); isBool && includeUsage {
				return true
			}
		}
	}
	return false
}

// isStreamingResponse checks if the response is a streaming response
func isStreamingResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return contentType == "text/event-stream" || contentType == "application/x-ndjson"
}

// handleStreamingResponse handles streaming responses
func handleStreamingResponse(c *gin.Context, resp *http.Response, rateLimiter *ratelimit.TokenRateLimiter, modelName string) error {
	reader := bufio.NewReader(resp.Body)
	c.Stream(func(w io.Writer) bool {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Try to parse usage from this line
			parsed := handlers.ParseStreamRespForUsage(string(line))
			if parsed.Usage.TotalTokens > 0 {
				klog.V(4).Infof("Parsed usage: %+v", parsed.Usage)
				// Record output tokens for rate limiting
				if rateLimiter != nil {
					rateLimiter.RecordOutputTokens(modelName, parsed.Usage.CompletionTokens)
				}
				// Check if token usage should be filtered
				if v, ok := c.Get(tokenUsageKey); ok && v.(bool) {
					return true
				}
			}
			// Forward to downstream
			_, _ = w.Write(line)
		}
		if err != nil {
			if err != io.EOF {
				klog.Errorf("error reading stream body: %v", err)
			}
			return false
		}
		return true
	})
	return nil
}

// handleNonStreamingResponse handles non-streaming responses
func handleNonStreamingResponse(c *gin.Context, resp *http.Response, rateLimiter *ratelimit.TokenRateLimiter, modelName string) error {
	var buf bytes.Buffer
	teeReader := io.TeeReader(resp.Body, &buf)

	_, err := io.Copy(c.Writer, teeReader)
	if err != nil {
		klog.Errorf("copy response to downstream failed: %v", err)
		return err
	}

	// Parse usage if present
	parsed, _ := handlers.ParseOpenAIResponseBody(buf.Bytes())
	if parsed != nil && parsed.Usage.TotalTokens > 0 {
		klog.V(4).Infof("Parsed usage: %+v", parsed.Usage)
		// Record output tokens for rate limiting
		if rateLimiter != nil {
			rateLimiter.RecordOutputTokens(modelName, parsed.Usage.CompletionTokens)
		}
	}

	return nil
}
