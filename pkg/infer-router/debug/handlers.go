/*
Copyright The Volcano Authors.

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

package debug

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/types"

	aiv1alpha1 "github.com/volcano-sh/kthena/pkg/apis/networking/v1alpha1"
	"github.com/volcano-sh/kthena/pkg/infer-router/datastore"
)

// DebugHandler provides debug endpoints for the gateway
type DebugHandler struct {
	store datastore.Store
}

// NewDebugHandler creates a new debug handler
func NewDebugHandler(store datastore.Store) *DebugHandler {
	return &DebugHandler{
		store: store,
	}
}

// Response structures matching the specification

type ModelRouteResponse struct {
	Name      string                    `json:"name"`
	Namespace string                    `json:"namespace"`
	Spec      aiv1alpha1.ModelRouteSpec `json:"spec"`
	RouteInfo *RouteInfo                `json:"routeInfo,omitempty"`
}

type RouteInfo struct {
	Model string   `json:"model"`
	Loras []string `json:"loras"`
}

type ModelServerResponse struct {
	Name           string                     `json:"name"`
	Namespace      string                     `json:"namespace"`
	Spec           aiv1alpha1.ModelServerSpec `json:"spec"`
	AssociatedPods []string                   `json:"associatedPods,omitempty"`
}

type PodResponse struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	PodInfo      *PodInfo `json:"podInfo,omitempty"`
	Engine       string   `json:"engine"`
	Metrics      *Metrics `json:"metrics,omitempty"`
	Models       []string `json:"models"`
	ModelServers []string `json:"modelServers"`
}

type PodInfo struct {
	PodIP     string            `json:"podIP"`
	NodeName  string            `json:"nodeName"`
	Phase     string            `json:"phase"`
	StartTime string            `json:"startTime,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type Metrics struct {
	GPUCacheUsage     float64 `json:"gpuCacheUsage"`
	RequestWaitingNum float64 `json:"requestWaitingNum"`
	RequestRunningNum float64 `json:"requestRunningNum"`
	TPOT              float64 `json:"tpot"`
	TTFT              float64 `json:"ttft"`
}

// List endpoints

// ListModelRoutes handles GET /debug/config_dump/modelroutes
func (h *DebugHandler) ListModelRoutes(c *gin.Context) {
	modelRoutes := h.store.GetAllModelRoutes()

	var responses []ModelRouteResponse
	for namespacedName, mr := range modelRoutes {
		parts := strings.Split(namespacedName, "/")
		if len(parts) != 2 {
			continue
		}

		response := ModelRouteResponse{
			Name:      parts[1],
			Namespace: parts[0],
			Spec:      mr.Spec,
		}

		responses = append(responses, response)
	}

	c.JSON(http.StatusOK, gin.H{"modelroutes": responses})
}

// ListModelServers handles GET /debug/config_dump/modelservers
func (h *DebugHandler) ListModelServers(c *gin.Context) {
	modelServers := h.store.GetAllModelServers()

	var responses []ModelServerResponse
	for namespacedName, ms := range modelServers {
		response := ModelServerResponse{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Spec:      ms.Spec,
		}

		// Get associated pods
		if pods, err := h.store.GetPodsByModelServer(namespacedName); err == nil {
			var podNames []string
			for _, pod := range pods {
				if pod.Pod != nil {
					podNames = append(podNames, pod.Pod.Namespace+"/"+pod.Pod.Name)
				}
			}
			response.AssociatedPods = podNames
		}

		responses = append(responses, response)
	}

	c.JSON(http.StatusOK, gin.H{"modelservers": responses})
}

// ListPods handles GET /debug/config_dump/pods
func (h *DebugHandler) ListPods(c *gin.Context) {
	pods := h.store.GetAllPods()

	var responses []PodResponse
	for namespacedName, podInfo := range pods {
		response := h.convertPodInfoToResponse(namespacedName, podInfo, false)
		responses = append(responses, response)
	}

	c.JSON(http.StatusOK, gin.H{"pods": responses})
}

// Get specific resource endpoints

// GetModelRoute handles GET /debug/config_dump/namespaces/{namespace}/modelroutes/{name}
func (h *DebugHandler) GetModelRoute(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name parameters are required"})
		return
	}

	namespacedName := namespace + "/" + name
	mr := h.store.GetModelRoute(namespacedName)

	if mr == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ModelRoute not found"})
		return
	}

	response := ModelRouteResponse{
		Name:      name,
		Namespace: namespace,
		Spec:      mr.Spec,
	}

	c.JSON(http.StatusOK, response)
}

// GetModelServer handles GET /debug/config_dump/namespaces/{namespace}/modelservers/{name}
func (h *DebugHandler) GetModelServer(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name parameters are required"})
		return
	}

	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	ms := h.store.GetModelServer(namespacedName)
	if ms == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ModelServer not found"})
		return
	}

	response := ModelServerResponse{
		Name:      name,
		Namespace: namespace,
		Spec:      ms.Spec,
	}

	// Get associated pods
	if pods, err := h.store.GetPodsByModelServer(namespacedName); err == nil {
		var podNames []string
		for _, pod := range pods {
			if pod.Pod != nil {
				podNames = append(podNames, pod.Pod.Namespace+"/"+pod.Pod.Name)
			}
		}
		response.AssociatedPods = podNames
	}

	c.JSON(http.StatusOK, response)
}

// GetPod handles GET /debug/config_dump/namespaces/{namespace}/pods/{name}
func (h *DebugHandler) GetPod(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if namespace == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and name parameters are required"})
		return
	}

	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	podInfo := h.store.GetPodInfo(namespacedName)
	if podInfo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	response := h.convertPodInfoToResponse(namespacedName, podInfo, true)
	c.JSON(http.StatusOK, response)
}

// Helper methods

func (h *DebugHandler) convertPodInfoToResponse(namespacedName types.NamespacedName, podInfo *datastore.PodInfo, includeDetails bool) PodResponse {
	response := PodResponse{
		Name:      namespacedName.Name,
		Namespace: namespacedName.Namespace,
		Engine:    podInfo.GetEngine(),
		Models:    podInfo.GetModelsList(),
	}

	// Convert model servers
	modelServers := podInfo.GetModelServersList()
	var msNames []string
	for _, ms := range modelServers {
		msNames = append(msNames, ms.Namespace+"/"+ms.Name)
	}
	response.ModelServers = msNames

	// Add metrics
	response.Metrics = &Metrics{
		GPUCacheUsage:     podInfo.GPUCacheUsage,
		RequestWaitingNum: podInfo.RequestWaitingNum,
		RequestRunningNum: podInfo.RequestRunningNum,
		TPOT:              podInfo.TPOT,
		TTFT:              podInfo.TTFT,
	}

	// Add pod info if details are requested
	if includeDetails && podInfo.Pod != nil {
		response.PodInfo = &PodInfo{
			PodIP:    podInfo.Pod.Status.PodIP,
			NodeName: podInfo.Pod.Spec.NodeName,
			Phase:    string(podInfo.Pod.Status.Phase),
			Labels:   podInfo.Pod.Labels,
		}

		if podInfo.Pod.Status.StartTime != nil {
			response.PodInfo.StartTime = podInfo.Pod.Status.StartTime.Format("2006-01-02T15:04:05Z")
		}
	}

	return response
}
