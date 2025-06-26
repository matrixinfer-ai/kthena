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

		// step 2: Find pods and model server details
		modelName := modelRequest["model"].(string)
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

		// step 3: Overwrite model.
		if model != nil && !isLora {
			modelRequest["model"] = *model
		}

		// step 4: call scheduler.Schedule. Get top n decode pods and perfill pods
		dPods, pPods, err := r.scheduler.Schedule(modelRequest, pods, pdGroup)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("can't schedule to target pod: %v", err))
			return
		}

		// step 5: Generate request ID at the beginning
		req := c.Request
		requestID := uuid.New().String()
		if req.Header.Get("x-request-id") == "" {
			// Add x-request-id header to prefill request
			req.Header.Set("x-request-id", requestID)
		}

		// step 6: proxy to pods
		if !r.tryRequestPods(c, req, pPods, modelRequest, port, requestID, perfillModel) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, "prefill request error")
			log.Error("Perfill pods schedule all failed")
			return
		}

		if !r.tryRequestPods(c, req, dPods, modelRequest, port, requestID, decodeModel) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, "decode request error")
			log.Error("Decode pods schedule all failed")
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

func (r *Router) tryRequestPods(
	c *gin.Context,
	req *http.Request,
	Pods []*datastore.PodInfo,
	modelRequest ModelRequest,
	port int32,
	requestID string,
	model string,
) bool {
	if len(Pods) == 0 {
		return true
	}
	for i := range Pods {
		log.Debugf("prefill pod is %v", Pods[i].Pod.Name)
		var resp *http.Response
		var err error
		if model == perfillModel {
			resp, err = proxyPrefillPod(req, Pods[i].Pod.Status.PodIP, port, modelRequest)
			if err != nil {
				log.Errorf("prefill pod request error: %v", err)
				continue
			}
		} else {
			resp, err = proxyDecodePod(c, req, Pods[i].Pod.Status.PodIP, port, modelRequest)
			if err != nil {
				log.Errorf("decode pod request error: %v", err)
				continue
			}
		}
		defer resp.Body.Close()
		return true
	}
	return false
}

// proxyPrefillPod proxies a request to a prefill pod.
func proxyPrefillPod(
	req *http.Request,
	podIP string,
	port int32,
	modelRequest ModelRequest,
) (*http.Response, error) {
	// Prepare prefill body
	prefillBody := make(map[string]interface{})
	for k, v := range modelRequest {
		prefillBody[k] = v
	}
	prefillBody["max_tokens"] = 1
	delete(prefillBody, "stream")
	delete(prefillBody, "stream_options")

	resp, err := getResponse(req, podIP, port, prefillBody)
	if err != nil {
		return nil, fmt.Errorf("prefill request error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("prefill http resp error, http code is %d", resp.StatusCode)
	}
	return resp, nil
}

// proxyToDecodePods proxies the request to the decode pods, returns response to downstream.
func proxyDecodePod(
	c *gin.Context,
	req *http.Request,
	podIP string,
	port int32,
	modelRequest ModelRequest,
) (*http.Response, error) {
	resp, err := getResponse(req, podIP, port, modelRequest)
	if err != nil {
		return nil, fmt.Errorf("decode request error: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("decode http resp error, http code is %d", resp.StatusCode)
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
	return resp, nil
}

func getResponse(
	req *http.Request,
	podIP string,
	port int32,
	modelRequest ModelRequest) (*http.Response, error) {
	body, err := json.Marshal(modelRequest)
	if err != nil {
		return nil, fmt.Errorf("marshal body failed: %w", err)
	}

	// step 1: change request URL to prefill pod URL.
	prefillReq := req.Clone(req.Context())
	prefillReq.URL.Host = fmt.Sprintf("%s:%d", podIP, port)
	prefillReq.URL.Scheme = "http"
	prefillReq.Body = io.NopCloser(bytes.NewBuffer(body))
	prefillReq.ContentLength = int64(len(body))

	// step 2: use http.Transport to do request to prefill pod.
	transport := http.DefaultTransport
	return transport.RoundTrip(prefillReq)
}
