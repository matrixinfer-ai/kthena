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

// LMCacheConnector implements distributed KV cache management using vLLM's LMCache system
type LMCacheConnector struct {
}

// NewLMCacheConnector creates a new LMCache connector
func NewLMCacheConnector() *LMCacheConnector {
	return &LMCacheConnector{}
}

// Name returns the connector type name
func (l *LMCacheConnector) Name() string {
	return "lmcache"
}

// Prefill executes prefill with LMCache integration
func (l *LMCacheConnector) Prefill(ctx context.Context, req *http.Request, prefillAddr string) error {
	req.URL.Host = prefillAddr
	req.URL.Scheme = "http"

	klog.V(4).Infof("Sending prefill request to %s", prefillAddr)

	return prefillerProxy(nil, req)
}

// Decode executes decode using LMCache for KV retrieval
func (l *LMCacheConnector) Decode(ctx context.Context, c *gin.Context, req *http.Request, decodeAddr string) error {
	req.URL.Host = decodeAddr
	req.URL.Scheme = "http"

	klog.V(4).Infof("Sending decode request to %s", decodeAddr)

	return decoderProxy(c, req)
}

// Proxy executes the complete prefill-decode flow using LMCache for KV coordination
func (l *LMCacheConnector) Proxy(c *gin.Context, reqBody map[string]interface{}, prefillAddr, decodeAddr string) error {
	// TODO: Implement LMCache-specific prefill-decode coordination
	// This should use LMCache APIs to manage KV cache between prefill and decode operations
	return fmt.Errorf("LMCacheConnector.Proxy not yet implemented")
}
