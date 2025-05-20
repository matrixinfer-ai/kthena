package router

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

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
		pods, model, err := r.store.GetModelServerEndpoints(modelServerName)
		if err != nil || len(pods) == 0 {
			c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find target pods of model server: %v, err: %v", modelServerName, err))
			return
		}
		// Overwrite model.
		if model != nil && !is_lora {
			modelRequest["model"] = *model
		}

		// call scheduler.Schedule
		targetPod, err := r.scheduler.Schedule(modelRequest, pods)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("can't schedule to target pod: %v", err))
			return
		}

		req := c.Request

		body, err := json.Marshal(modelRequest)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("marshal http body failed: %v", err))
			return
		}

		// step 1: change request URL to real server URL.
		// TODO: the target port need to be defined within modelServer
		req.URL.Host = fmt.Sprintf("%s:%d", targetPod.Pod.Status.PodIP, 8000)
		req.URL.Scheme = "http"
		req.Body = io.NopCloser(bytes.NewBuffer(body))
		req.ContentLength = int64(len(body))

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
		bufio.NewReader(resp.Body).WriteTo(c.Writer)
	}
}
