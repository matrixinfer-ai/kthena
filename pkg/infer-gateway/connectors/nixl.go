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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

// NIXLConnector implements high-performance distributed in-memory KV cache using NIXL
type NIXLConnector struct {
	prefillRequest    *http.Request
	decodeRequestBody map[string]interface{}
}

// NewNIXLConnector creates a new NIXL connector
func NewNIXLConnector() KVConnector {
	return &NIXLConnector{}
}

// Name returns the connector type name
func (n *NIXLConnector) Name() string {
	return "nixl"
}

// Proxy executes the complete prefill-decode flow using NIXL for high-performance KV transfer
func (n *NIXLConnector) Proxy(c *gin.Context, reqBody map[string]interface{}, prefillAddr, decodeAddr string) error {
	req := c.Request
	if n.prefillRequest == nil {
		prefillBody := cloneReqBody(reqBody)
		n.prefillRequest = n.buildPrefillRequest(req, prefillBody)
	}
	if n.decodeRequestBody == nil {
		n.decodeRequestBody = setDecodeRequestBody(c, reqBody)
	}

	// 1. send prefill request
	kvTransferParams, err := n.executePrefillRequest(n.prefillRequest, prefillAddr)
	if err != nil {
		return err
	}
	// 2. send decode request
	decodeReq := n.buildDecodeRequest(c, n.decodeRequestBody, kvTransferParams)
	return n.executeDecodeRequest(c, decodeReq, decodeAddr)
}

// executePrefillRequest builds and executes the prefill request, returns kv_transfer_params
func (n *NIXLConnector) executePrefillRequest(req *http.Request, prefillAddr string) (interface{}, error) {
	req.URL.Host = prefillAddr
	klog.V(4).Infof("NIXL prefill: sending to %s", prefillAddr)

	// Send prefill request
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("prefill request failed with status %d", resp.StatusCode)
	}

	// Parse prefill response
	var buf strings.Builder
	_, err = io.Copy(&buf, resp.Body)
	if err != nil {
		return nil, err
	}
	var prefillerResponse map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &prefillerResponse); err != nil {
		return nil, err
	}
	kvTransferParams, ok := prefillerResponse["kv_transfer_params"]
	if !ok {
		klog.Warning("NIXL: missing 'kv_transfer_params' in prefill response")
	}
	return kvTransferParams, nil
}

func setDecodeRequestBody(c *gin.Context, reqBody map[string]interface{}) map[string]interface{} {
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

func (n *NIXLConnector) buildDecodeRequest(c *gin.Context, reqBody map[string]interface{}, kvTransferParams interface{}) *http.Request {
	reqBody["kv_transfer_params"] = kvTransferParams
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil
	}

	// build request
	reqCopy := c.Request.Clone(c.Request.Context())
	reqCopy.URL.Scheme = "http"
	reqCopy.Body = io.NopCloser(bytes.NewBuffer(body))
	reqCopy.ContentLength = int64(len(body))

	return reqCopy
}

// executeDecodeRequest builds and executes the decode request with streaming response
func (n *NIXLConnector) executeDecodeRequest(c *gin.Context, req *http.Request, decodeAddr string) error {
	// Set kv_transfer_params from prefill response
	req.URL.Host = decodeAddr

	klog.V(4).Infof("NIXL decode: sending to %s", decodeAddr)

	// Use decoderProxy to handle the decode response with proper streaming
	return decoderProxy(c, req)
}

func cloneReqBody(reqBody map[string]interface{}) map[string]interface{} {
	// Create a deep copy of the request body
	clone := make(map[string]interface{})
	maps.Copy(clone, reqBody)
	return clone
}

func (n *NIXLConnector) buildPrefillRequest(req *http.Request, reqBody map[string]interface{}) *http.Request {
	// In PD disaggregated mode, we need to send a prefill request to the prefill pod with non stream mode.
	delete(reqBody, "stream")
	delete(reqBody, "stream_options")

	reqBody["max_tokens"] = 1
	if reqBody["max_completion_tokens"] != nil {
		reqBody["max_completion_tokens"] = 1
	}

	// Prepare prefill request
	reqBody["kv_transfer_params"] = map[string]interface{}{
		"do_remote_decode":  true,
		"do_remote_prefill": false,
		"remote_engine_id":  nil,
		"remote_block_ids":  nil,
		"remote_host":       nil,
		"remote_port":       nil,
	}

	body, err := json.Marshal(reqBody)
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
