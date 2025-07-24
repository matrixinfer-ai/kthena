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

package router

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/filters/ratelimit"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/handlers"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

type Router struct {
	scheduler       scheduler.Scheduler
	store           datastore.Store
	loadRateLimiter *ratelimit.TokenRateLimiter
}

func NewRouter(store datastore.Store) *Router {
	loadRateLimiter := ratelimit.NewRateLimiter()
	store.RegisterCallback("ModelRoute", func(data datastore.EventData) {
		if data.EventType == datastore.EventAdd || data.EventType == datastore.EventUpdate {
			if data.ModelRoute == nil || data.ModelRoute.Spec.RateLimit == nil {
				return
			}
			klog.Infof("add or update rate limit for model %s", data.ModelName)
			loadRateLimiter.AddOrUpdateLimiter(data.ModelName, data.ModelRoute.Spec.RateLimit)
		} else if data.EventType == datastore.EventDelete {
			klog.Infof("delete rate limit for model %s", data.ModelName)
			loadRateLimiter.DeleteLimiter(data.ModelName)
		}
	})

	return &Router{
		store:           store,
		scheduler:       scheduler.NewScheduler(store),
		loadRateLimiter: loadRateLimiter,
	}
}

type ModelRequest map[string]interface{}

func (r *Router) HandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Step 1: Parse and validate request
		modelRequest, err := parseModelRequest(c)
		if err != nil {
			return
		}

		// step 2: Detection of rate limit
		modelName := modelRequest["model"].(string)
		prompt, err := utils.GetPrompt(modelRequest)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, "prompt not found")
			return
		}
		if err := r.loadRateLimiter.RateLimit(modelName, prompt); err != nil {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, "token usage exceeds rate limit")
		}

		// step 3: Find pods and model server details
		modelServerName, isLora, err := r.store.MatchModelServer(modelName, c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find corresponding model server: %v", err))
			return
		}
		klog.V(4).Infof("modelServer is %v, is_lora: %v", modelServerName, isLora)

		pods, modelServer, err := r.getPodsAndServer(modelServerName)
		if err != nil || len(pods) == 0 {
			klog.Errorf("failed to get pods and model server: %v, %v", modelServerName, err)
			c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find model server: %v", modelServerName))
			return
		}
		model := modelServer.Spec.Model
		// step 4: Overwrite model.
		if model != nil && !isLora {
			modelRequest["model"] = *model
		}

		ctx := &framework.Context{
			Model:   modelName,
			Prompt:  prompt,
			PDGroup: modelServer.Spec.WorkloadSelector.PDGroup,
		}

		// step 5: call scheduler.Schedule. Get top n decode pods and perfill pods
		err = r.scheduler.Schedule(ctx, pods)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("can't schedule to target pod: %v", err))
			return
		}

		// step 6: Generate request ID at the beginning
		req := c.Request
		requestID := uuid.New().String()
		if req.Header.Get("x-request-id") == "" {
			// Add x-request-id header to prefill request
			req.Header.Set("x-request-id", requestID)
		}

		// step 7: proxy to pods
		if err := r.proxyModelEndpoint(c, req, ctx, modelRequest, modelServer.Spec.WorkloadPort.Port); err != nil {
			klog.Errorf("request failed: %v", err)
			return
		}
	}
}

func parseModelRequest(c *gin.Context) (ModelRequest, error) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return nil, err
	}
	var modelRequest ModelRequest
	if err := json.Unmarshal(bodyBytes, &modelRequest); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return nil, err
	}

	modelName, ok := modelRequest["model"].(string)
	if !ok {
		c.AbortWithStatusJSON(http.StatusNotFound, "model not found")
		return nil, fmt.Errorf("model not found")
	}
	klog.V(4).Infof("model name is %v", modelName)

	return modelRequest, nil
}

func (r *Router) getPodsAndServer(modelServerName types.NamespacedName) ([]*datastore.PodInfo, *v1alpha1.ModelServer, error) {
	pods, err := r.store.GetPodsByModelServer(modelServerName)
	if err != nil || len(pods) == 0 {
		return nil, nil, fmt.Errorf("can't find target pods of model server: %v, err: %v", modelServerName, err)
	}
	modelServer := r.store.GetModelServer(modelServerName)
	if modelServer == nil {
		return nil, nil, fmt.Errorf("can't find model server: %v", modelServerName)
	}
	return pods, modelServer, nil
}

func (r *Router) proxy(
	c *gin.Context,
	req *http.Request,
	ctx *framework.Context,
	stream bool,
	port int32,
) error {
	for i := 0; i < len(ctx.BestPods); i++ {
		// Request dispatched to the pod.
		if err := proxyDecodePod(c, req, ctx.BestPods[i].Pod.Status.PodIP, port, stream); err != nil {
			klog.Errorf(" pod request error: %v", err)
			continue
		}
		// record in prefix cache
		r.scheduler.RunPostHooks(ctx, i)
		return nil

	}
	c.AbortWithStatusJSON(http.StatusNotFound, "request to all pods failed")
	return fmt.Errorf("request to all pods failed")
}

func (r *Router) proxyModelEndpoint(
	c *gin.Context,
	req *http.Request,
	ctx *framework.Context,
	modelRequest ModelRequest,
	port int32,
) error {
	// build request
	var decodeRequest, prefillRequest *http.Request
	var err error

	stream := isStreaming(modelRequest)

	decodeRequest, err = buildDecodeRequest(req, modelRequest)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("failed to build request of decode: %v", err))
		return fmt.Errorf("failed to build request of decode: %v", err)
	}

	// proxy to pd aggregated pod
	if ctx.BestPods != nil {
		return r.proxy(c, decodeRequest, ctx, stream, port)
	}

	// build prefill request
	prefillRequest, err = buildPrefillRequest(req, modelRequest)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("failed to build request of prefill: %v", err))
		return fmt.Errorf("failed to build request of prefill: %v", err)
	}

	maxRetry := len(ctx.DecodePods)
	if len(ctx.PrefillPods) < maxRetry {
		maxRetry = len(ctx.PrefillPods)
	}
	for i := 0; i < maxRetry; i++ {
		// Dispatch to prefill pod first before dispatching to decode pod.
		klog.V(4).Infof("prefill pod is %v", ctx.PrefillPods[i].Pod.Name)
		if err := proxyPrefillPod(prefillRequest, ctx.PrefillPods[i].Pod.Status.PodIP, port); err != nil {
			klog.Errorf("prefill pod request error: %v", err)
			continue
		}

		// Request dispatched to the decode pod.
		if err := proxyDecodePod(c, decodeRequest, ctx.DecodePods[i].Pod.Status.PodIP, port, stream); err != nil {
			klog.Errorf("decode pod request error: %v", err)
			continue
		}
		// record in prefix cache
		r.scheduler.RunPostHooks(ctx, i)
		return nil
	}
	c.AbortWithStatusJSON(http.StatusNotFound, "request to all pods failed")
	return fmt.Errorf("request to all pods failed")
}

// proxyPrefillPod proxies a request to a prefill pod.
func proxyPrefillPod(
	req *http.Request,
	podIP string,
	port int32,
) error {
	resp, err := doRequest(req, podIP, port)
	if err != nil {
		return fmt.Errorf("prefill request error: %w", err)
	}
	resp.Body.Close()
	return nil
}

// proxyToDecodePods proxies the request to the decode pods, returns response to downstream.
func proxyDecodePod(
	c *gin.Context,
	req *http.Request,
	podIP string,
	port int32,
	stream bool,
) error {
	resp, err := doRequest(req, podIP, port)
	if err != nil {
		return fmt.Errorf("decode request error: %w", err)
	}
	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Header(k, v)
		}
	}
	defer resp.Body.Close()

	c.Status(resp.StatusCode)

	if stream {
		// If the request is a streaming request, we need to stream the response body.
		// Stream response: read and forward each event (line) one by one, and parse usage if present
		c.Status(resp.StatusCode)
		reader := bufio.NewReader(resp.Body)
		c.Stream(func(w io.Writer) bool {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				_, _ = w.Write(line)
				// Try to parse usage from this line, assuming it's a data line
				if bytes.HasPrefix(line, []byte("data:")) {
					parsed := handlers.ParseStreamRespForUsage(string(line))
					if parsed.Usage.TotalTokens > 0 {
						klog.V(4).Infof("Parsed usage: %+v", parsed.Usage)
					}
				}
			}
			if err != nil {
				if err != io.EOF {
					klog.Errorf("error reading stream body: %v", err)
				}
				return false
			}
			return true
		})
	} else {
		// Non-stream: efficiently stream response while capturing for parsing
		var buf bytes.Buffer
		teeReader := io.TeeReader(resp.Body, &buf)

		_, err := io.Copy(c.Writer, teeReader)
		if err != nil {
			klog.Errorf("copy response to downstream failed: %v", err)
			return nil
		}

		// Parse usage if present
		parsed, _ := handlers.ParseOpenAIResponseBody(buf.Bytes())
		if parsed != nil && parsed.Usage.TotalTokens > 0 {
			klog.V(4).Infof("Parsed usage: %+v", parsed.Usage)
		}
	}

	return nil
}

func doRequest(
	req *http.Request,
	podIP string,
	port int32,
) (*http.Response, error) {
	// step 1: change request URL to prefill pod URL.
	req.URL.Host = fmt.Sprintf("%s:%d", podIP, port)

	// step 2: use http.Transport to do request to prefill pod.
	transport := http.DefaultTransport
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http resp error, http code is %d", resp.StatusCode)
	}
	return resp, nil
}

// isStreaming checks if the given model request has streaming enabled
func isStreaming(modelRequest ModelRequest) bool {
	if v, ok := modelRequest["stream"]; ok {
		if stream, isBool := v.(bool); isBool && stream {
			return true
		}
	}
	return false
}

func buildPrefillRequest(req *http.Request, modelRequest ModelRequest) (*http.Request, error) {
	// In PD disaggregated mode, we need to send a prefill request to the prefill pod with non stream mode.
	delete(modelRequest, "stream")
	delete(modelRequest, "stream_options")

	body, err := json.Marshal(modelRequest)
	if err != nil {
		return nil, err
	}

	// build request
	reqCopy := req.Clone(req.Context())
	reqCopy.URL.Scheme = "http"
	reqCopy.Body = io.NopCloser(bytes.NewBuffer(body))
	reqCopy.ContentLength = int64(len(body))

	return reqCopy, nil
}

func buildDecodeRequest(req *http.Request, modelRequest ModelRequest) (*http.Request, error) {
	// Create a copy of the request to avoid modifying the original
	requestBody := make(ModelRequest)
	for k, v := range modelRequest {
		requestBody[k] = v
	}

	// Check if streaming is enabled
	if isStreaming(requestBody) {
		// For streaming requests, add stream_options to include token usage
		requestBody["stream_options"] = map[string]interface{}{
			"include_usage": true,
		}
	} else {
		// For non-streaming requests, ensure we request usage information
		// Most OpenAI-compatible APIs return usage by default for non-streaming,
		// but we can be explicit about it
		requestBody["include_usage"] = true
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	// build request
	reqCopy := req.Clone(req.Context())
	reqCopy.URL.Scheme = "http"
	reqCopy.Body = io.NopCloser(bytes.NewBuffer(body))
	reqCopy.ContentLength = int64(len(body))

	return reqCopy, nil
}
