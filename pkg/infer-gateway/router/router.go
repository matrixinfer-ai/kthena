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

	// build request
	prefillRequest, err := buildPrefillRequest(req, modelRequest)
	if err != nil {
		return fmt.Errorf("failed to build request of prefill: %v", err)
	}
	decodeRequest, err := buildDecodeRequest(req, modelRequest)
	if err != nil {
		return fmt.Errorf("failed to build request of decode: %v", err)
	}

	for i := range ctxs {
		if ctxs[i].DecodePod != nil {
			if ctxs[i].PrefillPod != nil {
				// PD disaggregated if there is a perfill pod. Dispatch to perfill pod first before dispatching to decode pod.
				log.Debugf("prefill pod is %v", ctxs[i].PrefillPod.Pod.Name)
				if err := proxyPrefillPod(prefillRequest, ctxs[i].PrefillPod.Pod.Status.PodIP, port); err != nil {
					log.Errorf("prefill pod request error: %v", err)
					continue
				}
			}
			// Request dispatched to the decode pod.
			if err := proxyDecodePod(c, decodeRequest, ctxs[i].DecodePod.Pod.Status.PodIP, port); err != nil {
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

func buildPrefillRequest(req *http.Request, modelRequest ModelRequest) (*http.Request, error) {
	info := make(map[string]interface{})
	for k, v := range modelRequest {
		info[k] = v
	}
	info["max_tokens"] = 1
	delete(info, "steam")
	delete(info, "stream_options")

	body, err := json.Marshal(info)
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
