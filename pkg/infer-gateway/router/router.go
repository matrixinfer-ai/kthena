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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/connectors"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/filters/ratelimit"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/handlers"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

const (
	tokenUsageKey = "token_usage"
)

type Router struct {
	scheduler       scheduler.Scheduler
	store           datastore.Store
	loadRateLimiter *ratelimit.TokenRateLimiter

	// KV Connector management
	connectorFactory *connectors.Factory
}

func NewRouter(store datastore.Store) *Router {
	loadRateLimiter := ratelimit.NewRateLimiter()
	store.RegisterCallback("ModelRoute", func(data datastore.EventData) {
		switch data.EventType {
		case datastore.EventAdd, datastore.EventUpdate:
			if data.ModelRoute == nil || data.ModelRoute.Spec.RateLimit == nil {
				return
			}
			klog.Infof("add or update rate limit for model %s", data.ModelName)
			loadRateLimiter.AddOrUpdateLimiter(data.ModelName, data.ModelRoute.Spec.RateLimit)
		case datastore.EventDelete:
			klog.Infof("delete rate limit for model %s", data.ModelName)
			loadRateLimiter.DeleteLimiter(data.ModelName)
		}
	})

	return &Router{
		store:            store,
		scheduler:        scheduler.NewScheduler(store),
		loadRateLimiter:  loadRateLimiter,
		connectorFactory: connectors.NewDefaultFactory(),
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
			return
		}
		userIdVal, ok := modelRequest["userId"]
		if !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, "missing userId in request body")
			return
		}
		userId, ok := userIdVal.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, "userId is not a string")
			return
		}
		requestID := uuid.New().String()
		userTokens, _ := r.store.GetTokenCount(userId, modelName)
		requestTime := time.Now()
		notifyChan := make(chan struct{})
		//
		queueReq := &datastore.Request{
			UserID:      userId,
			ReqID:       requestID,
			Payload:     modelRequest,
			Priority:    userTokens,
			RequestTime: requestTime,
			NotifyChan:  notifyChan,
		}
		if err := r.store.RequestWaitingQueuePush(queueReq); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("failed to enqueue request: %v", err))
			return
		}
		select {
		case <-notifyChan:
			// Received notification: request has been dequeued, execute subsequent logic (e.g., scheduling, proxying)
			klog.Infof("Request %s has been dequeued, starting processing", requestID)
			r.processAfterDequeue(c, queueReq) // Execute post-dequeue logic
		case <-time.After(60 * time.Second):
			// Timeout protection: avoid permanent blocking
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, "Request processing timed out")
			return
		}
	}
}

// Starts the queue processor and controls dequeue rate (e.g., maximum 5 requests processed per second)
func (r *Router) StartQueueProcessor(rate int, stopChan <-chan struct{}) {
	if rate <= 0 {
		klog.Errorf("Invalid rate: %d, must be > 0", rate)
		return
	}
	interval := time.Second / time.Duration(rate)

	go func() {
		ticker := time.NewTicker(interval)
		defer func() {
			ticker.Stop()
			klog.Info("Queue processor has stopped")
		}()
		var modelNames []string // model names for round-robin
		currentIndex := 0       // current index for round-robin
		for {
			select {
			case <-stopChan:
				// Received stop signal, terminate goroutine
				return
			case <-ticker.C:
				modelNames = r.store.GetRequestWaitingQueueModel()
				if len(modelNames) == 0 {
					klog.V(4).Infof("No non-empty queues, waiting (interval %v)", interval)
					continue
				}
				modelName := modelNames[currentIndex]
				currentIndex = (currentIndex + 1) % len(modelNames)
				// When timer triggers, retrieve one request from the queue (dequeue according to rate limit)
				req := r.store.RequestWaitingQueuePop(modelName)
				if req == nil {
					// Queue is empty, wait for next timer trigger
					klog.V(4).Infof("Queue is empty, waiting for next dequeue opportunity (interval %v)", interval)
					continue
				}

				// Process dequeue logic: notify blocked goroutine
				klog.Infof("Request %s has been dequeued (rate limit: one request every %v)", req.ReqID, interval)
				select {
				case <-req.NotifyChan:
					// already closed, do nothing
				default:
					close(req.NotifyChan)
				}
			}
		}
	}()
}

// router/router.go
func (r *Router) processAfterDequeue(c *gin.Context, queueReq *datastore.Request) {
	// Extract original request data from queue request
	modelRequest := queueReq.Payload.(ModelRequest)
	modelName := modelRequest["model"].(string)
	userId := queueReq.UserID
	// step 3: Find pods and model server details
	modelServerName, isLora, err := r.store.MatchModelServer(modelName, c.Request)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find corresponding model server: %v", err))
		return
	}
	klog.V(4).Infof("modelServer is %v, is_lora: %v", modelServerName, isLora)
	pods, modelServer, err := r.getPodsAndServer(modelServerName)
	if err != nil || len(pods) == 0 {
		klog.Errorf("failed to get pods and model server: %v, %v (user: %s)", modelServerName, err, userId)
		c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find model server: %v", modelServerName))
		return
	}

	model := modelServer.Spec.Model
	if model != nil && !isLora {
		modelRequest["model"] = *model
	}

	var pdGroup *v1alpha1.PDGroup
	if modelServer.Spec.WorkloadSelector != nil {
		pdGroup = modelServer.Spec.WorkloadSelector.PDGroup
	}
	prompt, err := utils.GetPrompt(modelRequest)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, "prompt not found")
		return
	}
	ctx := &framework.Context{
		Model:           modelName,
		Prompt:          prompt,
		ModelServerName: modelServerName,
		PDGroup:         pdGroup,
	}

	err = r.scheduler.Schedule(ctx, pods)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("can't schedule to target pod: %v", err))
		return
	}

	req := c.Request
	requestID := queueReq.ReqID
	if req.Header.Get("x-request-id") == "" {
		req.Header.Set("x-request-id", requestID)
	}

	if err := r.proxyModelEndpoint(c, req, ctx, modelRequest, modelServer.Spec.WorkloadPort.Port); err != nil {
		klog.Errorf("request failed (user: %s, reqID: %s): %v", userId, requestID, err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, "request processing failed")
		return
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
		if err := proxyRequest(c, req, ctx.BestPods[i].Pod.Status.PodIP, port, stream, r.loadRateLimiter, ctx.Model); err != nil {
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
	// proxy to pd aggregated pod
	if ctx.BestPods != nil {
		decodeRequest := connectors.BuildDecodeRequest(c, req, modelRequest)
		// build request
		stream := isStreaming(modelRequest)
		return r.proxy(c, decodeRequest, ctx, stream, port)
	}

	// Get appropriate connector for this model server
	kvConnector, err := r.getKVConnector(ctx.ModelServerName)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("failed to get KV connector: %v", err))
		return fmt.Errorf("failed to get KV connector: %w", err)
	}

	// PD disaggregated mode - use KV connector
	return r.proxyToPDDisaggregated(c, req, ctx, kvConnector, modelRequest, port)
}

// proxyRequest proxies the request to the model server pods, returns response to downstream.
func proxyRequest(
	c *gin.Context,
	req *http.Request,
	podIP string,
	port int32,
	stream bool,
	rateLimiter *ratelimit.TokenRateLimiter,
	modelName string,
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
				// Try to parse usage from this line, assuming it's a data line
				parsed := handlers.ParseStreamRespForUsage(string(line))
				if parsed.Usage.CompletionTokens > 0 {
					klog.V(4).Infof("Parsed usage: %+v", parsed.Usage)
					// Record output tokens for rate limiting
					if rateLimiter != nil {
						rateLimiter.RecordOutputTokens(modelName, parsed.Usage.CompletionTokens)
					}
					// The token usage is set by gateway, so remove it before sending to downstream
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
		if parsed != nil && parsed.Usage.CompletionTokens > 0 {
			klog.V(4).Infof("Parsed usage: %+v", parsed.Usage)
			// Record output tokens for rate limiting
			if rateLimiter != nil {
				rateLimiter.RecordOutputTokens(modelName, parsed.Usage.CompletionTokens)
			}
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

// getKVConnector gets the appropriate KV connector for a model server
func (r *Router) getKVConnector(modelServerName types.NamespacedName) (connectors.KVConnector, error) {
	modelServer := r.store.GetModelServer(modelServerName)
	if modelServer == nil {
		return nil, fmt.Errorf("model server %s not found", modelServerName)
	}

	// Determine connector type from ModelServer CRD
	connectorType := v1alpha1.ConnectorTypeHTTP
	if modelServer.Spec.KVConnector != nil && modelServer.Spec.KVConnector.Type != "" {
		connectorType = modelServer.Spec.KVConnector.Type
	}

	connector := r.connectorFactory.GetConnector(connectorType)
	if connector == nil {
		return nil, fmt.Errorf("failed to get connector %s", connectorType)
	}

	return connector, nil
}

// proxyToPDDisaggregated handles PD disaggregated routing using KV connectors
func (r *Router) proxyToPDDisaggregated(
	c *gin.Context,
	req *http.Request,
	ctx *framework.Context,
	kvConnector connectors.KVConnector,
	modelRequest ModelRequest,
	port int32,
) error {
	// Try multiple prefill/decode pairs
	maxRetry := len(ctx.DecodePods)
	if len(ctx.PrefillPods) < maxRetry {
		maxRetry = len(ctx.PrefillPods)
	}

	for i := 0; i < maxRetry; i++ {
		if ctx.PrefillPods[i] == nil || ctx.DecodePods[i] == nil {
			continue
		}

		// Build addresses for prefill and decode pods
		prefillAddr := fmt.Sprintf("%s:%d", ctx.PrefillPods[i].Pod.Status.PodIP, port)
		decodeAddr := fmt.Sprintf("%s:%d", ctx.DecodePods[i].Pod.Status.PodIP, port)

		klog.V(4).Infof("Attempting PD disaggregated request: prefill=%s, decode=%s", prefillAddr, decodeAddr)

		outputTokens, err := kvConnector.Proxy(c, modelRequest, prefillAddr, decodeAddr)
		if err != nil {
			klog.Errorf("proxy failed for prefill pod %s, decode pod %s: %v",
				ctx.PrefillPods[i].Pod.Name, ctx.DecodePods[i].Pod.Name, err)
			continue
		}

		// Record output tokens for rate limiting
		if outputTokens > 0 && r.loadRateLimiter != nil {
			r.loadRateLimiter.RecordOutputTokens(ctx.Model, outputTokens)
		}

		// Record successful operation in cache
		r.scheduler.RunPostHooks(ctx, i)

		klog.V(4).Infof("kv connector run successful for prefill pod %s, decode pod %s, output tokens: %d",
			ctx.PrefillPods[i].Pod.Name, ctx.DecodePods[i].Pod.Name, outputTokens)

		return nil
	}

	c.AbortWithStatusJSON(http.StatusInternalServerError, "all prefill/decode attempts failed")
	return fmt.Errorf("all prefill/decode attempts failed")
}
