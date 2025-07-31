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
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

const (
	tokenUsageKey = "token_usage"
)

// HTTPConnector implements simple HTTP-based KV transfer
// This maintains backward compatibility with current implementation
type HTTPConnector struct {
}

// NewHTTPConnector creates a new HTTP connector with default configuration
func NewHTTPConnector() *HTTPConnector {
	return &HTTPConnector{}
}

// Name returns the connector type name
func (h *HTTPConnector) Name() string {
	return "http"
}

// Prefill executes prefill request
func (h *HTTPConnector) Prefill(ctx context.Context, req *http.Request, prefillAddr string) error {
	req.URL.Host = prefillAddr
	req.URL.Scheme = "http"

	return prefillerProxy(nil, req)
}

// Decode executes decode request and streams response
func (h *HTTPConnector) Decode(ctx context.Context, c *gin.Context, req *http.Request, decodeAddr string) error {
	req.URL.Host = decodeAddr
	req.URL.Scheme = "http"

	klog.V(4).Infof("Sending decode request to %s", decodeAddr)

	return decoderProxy(c, req)
}

// Proxy executes the complete prefill-decode flow for HTTP connector
func (h *HTTPConnector) Proxy(c *gin.Context, reqBody map[string]interface{}, prefillAddr, decodeAddr string) error {
	// For HTTP connector, we don't have a unified proxy method
	// This is a fallback that should not be used - HTTP connector should use separate Prefill/Decode calls
	return fmt.Errorf("HTTP connector does not support unified Proxy method - use separate Prefill/Decode calls")
}
