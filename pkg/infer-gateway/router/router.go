package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"

	"matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/filters/ratelimit"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

const (
	decodeModel  = "decode"
	perfillModel = "perfill"
)

var (
	log = logger.NewLogger("router")
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
			log.Infof("add or update rate limit for model %s", data.ModelName)
			loadRateLimiter.AddOrUpdateLimiter(data.ModelName, data.ModelRoute.Spec.RateLimit)
		} else if data.EventType == datastore.EventDelete {
			log.Infof("delete rate limit for model %s", data.ModelName)
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
		log.Debugf("modelServer is %v, is_lora: %v", modelServerName, isLora)

		pods, modelServer, err := r.getPodsAndServer(modelServerName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find model server: %v", modelServerName))
			return
		}
		pdGroup := modelServer.Spec.WorkloadSelector.PDGroup
		model := modelServer.Spec.Model
		port := modelServer.Spec.WorkloadPort.Port

		// step 4: Overwrite model.
		if model != nil && !isLora {
			modelRequest["model"] = *model
		}

		// step 5: call scheduler.Schedule. Get top n decode pods and perfill pods
		ctxs, err := r.scheduler.Schedule(modelRequest, pods, pdGroup)
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
		if err := r.proxyModelEndpoint(c, req, ctxs, modelRequest, port); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, "model request error")
			log.Errorf("request failed: %v", err)
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
	log.Debugf("model name is %v", modelName)

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

func (r *Router) proxyModelEndpoint(
	c *gin.Context,
	req *http.Request,
	ctxs []*framework.Context,
	modelRequest ModelRequest,
	port int32,
) error {
	if len(ctxs) == 0 {
		return fmt.Errorf("no pod meets the requirements")
	}
	for i := range ctxs {
		if ctxs[i].DecodePod != nil {
			if ctxs[i].PrefillPod != nil {
				// PD disaggregated if there is a perfill pod. Dispatch to perfill pod first before dispatching to decode pod.
				log.Debugf("prefill pod is %v", ctxs[i].PrefillPod.Pod.Name)
				if err := proxyPrefillPod(req, ctxs[i].PrefillPod.Pod.Status.PodIP, port, modelRequest); err != nil {
					log.Errorf("prefill pod request error: %v", err)
					continue
				}
			}
			// Request dispatched to the decode pod.
			if err := proxyDecodePod(c, req, ctxs[i].DecodePod.Pod.Status.PodIP, port, modelRequest); err != nil {
				log.Errorf("decode pod request error: %v", err)
				continue
			}
			return nil
		}
	}
	return fmt.Errorf("request to all pods failed")
}

// proxyPrefillPod proxies a request to a prefill pod.
func proxyPrefillPod(
	req *http.Request,
	podIP string,
	port int32,
	modelRequest ModelRequest,
) error {
	// Prepare prefill body
	prefillBody := make(map[string]interface{})
	for k, v := range modelRequest {
		prefillBody[k] = v
	}
	prefillBody["max_tokens"] = 1
	delete(prefillBody, "stream")
	delete(prefillBody, "stream_options")

	resp, err := doRequest(req, podIP, port, prefillBody)
	if err != nil {
		return fmt.Errorf("prefill request error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("prefill http resp error, http code is %d", resp.StatusCode)
	}
	return nil
}

// proxyToDecodePods proxies the request to the decode pods, returns response to downstream.
func proxyDecodePod(
	c *gin.Context,
	req *http.Request,
	podIP string,
	port int32,
	modelRequest ModelRequest,
) error {
	resp, err := doRequest(req, podIP, port, modelRequest)
	if err != nil {
		return fmt.Errorf("decode request error: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("decode http resp error, http code is %d", resp.StatusCode)
	}
	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Header(k, v)
		}
	}
	defer resp.Body.Close()
	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 512)
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		return err != io.EOF
	})
	return nil
}

func doRequest(
	req *http.Request,
	podIP string,
	port int32,
	modelRequest ModelRequest) (*http.Response, error) {
	body, err := json.Marshal(modelRequest)
	if err != nil {
		return nil, fmt.Errorf("marshal body failed: %w", err)
	}

	// step 1: change request URL to prefill pod URL.
	Req := req.Clone(req.Context())
	Req.URL.Host = fmt.Sprintf("%s:%d", podIP, port)
	Req.URL.Scheme = "http"
	Req.Body = io.NopCloser(bytes.NewBuffer(body))
	Req.ContentLength = int64(len(body))

	// step 2: use http.Transport to do request to prefill pod.
	transport := http.DefaultTransport
	return transport.RoundTrip(Req)
}
