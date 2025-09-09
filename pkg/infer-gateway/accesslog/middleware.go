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

package accesslog

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"k8s.io/klog/v2"
)

const (
	// AccessLogContextKey is the key used to store AccessLogContext in gin.Context
	AccessLogContextKey = "access_log_context"
)

// AccessLogMiddleware returns a Gin middleware that tracks request timing and metadata
func AccessLogMiddleware(logger AccessLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip logging for health check endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready" {
			c.Next()
			return
		}

		// Generate request ID if not present
		requestID := c.Request.Header.Get("x-request-id")
		if requestID == "" {
			requestID = uuid.New().String()
			c.Request.Header.Set("x-request-id", requestID)
		}

		// Create access log context
		ctx := NewAccessLogContext(
			requestID,
			c.Request.Method,
			c.Request.URL.Path,
			c.Request.Proto,
			"", // ModelName will be set later when parsed from request body
		)

		// Store context in gin.Context for other handlers to access
		c.Set(AccessLogContextKey, ctx)

		// Process request
		c.Next()

		// Log the access entry after request completion
		statusCode := c.Writer.Status()
		entry := ctx.ToAccessLogEntry(statusCode)

		if err := logger.Log(entry); err != nil {
			klog.Errorf("Failed to write access log: %v", err)
		}
	}
}

// GetAccessLogContext retrieves the AccessLogContext from gin.Context
func GetAccessLogContext(c *gin.Context) *AccessLogContext {
	if ctx, exists := c.Get(AccessLogContextKey); exists {
		if accessCtx, ok := ctx.(*AccessLogContext); ok {
			return accessCtx
		}
	}
	return nil
}

// SetModelName sets the model name in the access log context
func SetModelName(c *gin.Context, modelName string) {
	if ctx := GetAccessLogContext(c); ctx != nil {
		ctx.ModelName = modelName
	}
}

// SetModelRouting sets model routing information in the access log context
func SetModelRouting(c *gin.Context, modelRoute, modelServer, selectedPod string) {
	if ctx := GetAccessLogContext(c); ctx != nil {
		// Parse namespace/name format for model server
		var modelServerName string
		if modelServer != "" {
			modelServerName = modelServer
		}
		ctx.SetModelRouting(modelRoute, parseNamespacedName(modelServerName), selectedPod)
	}
}

// SetTokenCounts sets token counts in the access log context
func SetTokenCounts(c *gin.Context, inputTokens, outputTokens int) {
	if ctx := GetAccessLogContext(c); ctx != nil {
		ctx.SetTokenCounts(inputTokens, outputTokens)
	}
}

// SetError sets error information in the access log context
func SetError(c *gin.Context, errorType, message string) {
	if ctx := GetAccessLogContext(c); ctx != nil {
		ctx.SetError(errorType, message)
	}
}

// MarkRequestProcessingEnd marks the end of request processing phase
func MarkRequestProcessingEnd(c *gin.Context) {
	if ctx := GetAccessLogContext(c); ctx != nil {
		ctx.MarkRequestProcessingEnd()
	}
}

// MarkUpstreamStart marks the start of upstream processing
func MarkUpstreamStart(c *gin.Context) {
	if ctx := GetAccessLogContext(c); ctx != nil {
		ctx.MarkUpstreamStart()
	}
}

// MarkUpstreamEnd marks the end of upstream processing
func MarkUpstreamEnd(c *gin.Context) {
	if ctx := GetAccessLogContext(c); ctx != nil {
		ctx.MarkUpstreamEnd()
	}
}

// MarkResponseProcessingEnd marks the end of response processing
func MarkResponseProcessingEnd(c *gin.Context) {
	if ctx := GetAccessLogContext(c); ctx != nil {
		ctx.MarkResponseProcessingEnd()
	}
}

// parseNamespacedName parses a "namespace/name" string into NamespacedName
func parseNamespacedName(nameStr string) (result struct{ Namespace, Name string }) {
	if nameStr == "" {
		return
	}

	// Simple parsing - split on first '/'
	for i, ch := range nameStr {
		if ch == '/' {
			result.Namespace = nameStr[:i]
			result.Name = nameStr[i+1:]
			return
		}
	}

	// No namespace found, treat entire string as name
	result.Name = nameStr
	return
}
