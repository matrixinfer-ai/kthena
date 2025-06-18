package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler"
)

var (
	log = logger.NewLogger("router")
)

type Router struct {
	scheduler scheduler.Scheduler
	store     datastore.Store
}

func NewRouter(store datastore.Store) *Router {
	return &Router{
		store:     store,
		scheduler: scheduler.NewScheduler(store),
	}
}

type ModelRequest map[string]interface{}

func (r *Router) HandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate request ID at the beginning
		requestID := uuid.New().String()

		// implement gin request body reading here
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, err)
			return
		}
		var modelRequest ModelRequest
		if err := json.Unmarshal(bodyBytes, &modelRequest); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, err)
			return
		}

		modelName, ok := modelRequest["model"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusNotFound, "model not found")
			return
		}

		log.Debugf("model name is %v", modelName)

		// Use datastore to find the corresponding model server
		modelServerName, is_lora, err := r.store.MatchModelServer(modelName, c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find corresponding model server: %v", err))
			return
		}

		log.Debugf("modelServer is %v, is_lora: %v", modelServerName, is_lora)

		// Get endpoints from datastore
		pods, err := r.store.GetPodsByModelServer(modelServerName)
		if err != nil || len(pods) == 0 {
			c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find target pods of model server: %v, err: %v", modelServerName, err))
			return
		}

		// Get PDGroup from datastore
		modelServer := r.store.GetModelServer(modelServerName)
		if modelServer == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find model server: %v", modelServerName))
			return
		}
		pdGroup := modelServer.Spec.WorkloadSelector.PDGroup
		model := modelServer.Spec.Model
		port := modelServer.Spec.WorkloadPort.Port

		// Overwrite model.
		if model != nil && !is_lora {
			modelRequest["model"] = *model
		}

		// call scheduler.Schedule
		targetPods, err := r.scheduler.Schedule(modelRequest, pods, pdGroup)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("can't schedule to target pod: %v", err))
			return
		}

		req := c.Request

		if targetPods.PrefillPod != nil {
			log.Debugf("prefill pod is %v", targetPods.PrefillPod.Pod.Name)

			// First request to prefill pod
			prefillReq := req.Clone(req.Context())
			prefillBody := make(map[string]interface{})
			for k, v := range modelRequest {
				prefillBody[k] = v
			}
			prefillBody["max_tokens"] = 1

			body, err := json.Marshal(prefillBody)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("marshal prefill body failed: %v", err))
				return
			}

			// step 1: change request URL to prefill pod URL.
			prefillReq.URL.Host = fmt.Sprintf("%s:%d", targetPods.PrefillPod.Pod.Status.PodIP, port)
			prefillReq.URL.Scheme = "http"
			prefillReq.Body = io.NopCloser(bytes.NewBuffer(body))
			prefillReq.ContentLength = int64(len(body))

			if prefillReq.Header.Get("x-request-id") == "" {
				// Add x-request-id header to prefill request
				prefillReq.Header.Set("x-request-id", requestID)
			}

			// step 2: use http.Transport to do request to prefill pod.
			transport := http.DefaultTransport
			resp, err := transport.RoundTrip(prefillReq)
			if err != nil {
				log.Errorf("prefill request error: %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, "prefill request error")
				return
			}
			resp.Body.Close()
		}

		body, err := json.Marshal(modelRequest)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("marshal http body failed: %v", err))
			return
		}

		log.Debugf("target/decode pod is %v", targetPods.DecodePod.Pod.Name)

		// step 1: change request URL to real server URL.
		req.URL.Host = fmt.Sprintf("%s:%d", targetPods.DecodePod.Pod.Status.PodIP, port)
		req.URL.Scheme = "http"
		req.Body = io.NopCloser(bytes.NewBuffer(body))
		req.ContentLength = int64(len(body))

		if req.Header.Get("x-request-id") == "" {
			// Add x-request-id header to decode request
			req.Header.Set("x-request-id", requestID)
		}

		// step 2: use http.Transport to do request to real server.
		transport := http.DefaultTransport
		resp, err := transport.RoundTrip(req)
		if err != nil {
			log.Errorf("error: %v", err)
			c.String(http.StatusInternalServerError, "error")
			return
		}

		// step 3: return real server response to downstream.
		for k, vv := range resp.Header {
			for _, v := range vv {
				c.Header(k, v)
			}
		}
		defer resp.Body.Close()

		// Maybe we need to read the response to get the tokens for ratelimiting later
		c.Stream(func(w io.Writer) bool {
			buf := make([]byte, 512)
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
			}
			return err != io.EOF
		})
	}
}
