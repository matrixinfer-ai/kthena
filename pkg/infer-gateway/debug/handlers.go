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

package debug

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
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
	Name         string      `json:"name"`
	Namespace    string      `json:"namespace"`
	PodInfo      *PodInfo    `json:"podInfo,omitempty"`
	Engine       string      `json:"engine"`
	Metrics      *Metrics    `json:"metrics,omitempty"`
	Models       []string    `json:"models"`
	ModelServers []string    `json:"modelServers"`
	Containers   []Container `json:"containers,omitempty"`
}

type PodInfo struct {
	PodIP     string            `json:"podIP"`
	NodeName  string            `json:"nodeName"`
	Phase     string            `json:"phase"`
	StartTime string            `json:"startTime,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type Metrics struct {
	GPUCacheUsage               float64    `json:"gpuCacheUsage"`
	RequestWaitingNum           float64    `json:"requestWaitingNum"`
	RequestRunningNum           float64    `json:"requestRunningNum"`
	TPOT                        float64    `json:"tpot"`
	TTFT                        float64    `json:"ttft"`
	TimeToFirstTokenHistogram   *Histogram `json:"timeToFirstTokenHistogram,omitempty"`
	TimePerOutputTokenHistogram *Histogram `json:"timePerOutputTokenHistogram,omitempty"`
}

type Histogram struct {
	Buckets     []Bucket `json:"buckets"`
	SampleCount uint64   `json:"sampleCount"`
	SampleSum   float64  `json:"sampleSum"`
}

type Bucket struct {
	UpperBound      float64 `json:"upperBound"`
	CumulativeCount uint64  `json:"cumulativeCount"`
}

type Container struct {
	Name      string                `json:"name"`
	Image     string                `json:"image"`
	Ports     []ContainerPort       `json:"ports,omitempty"`
	Resources *ResourceRequirements `json:"resources,omitempty"`
}

type ContainerPort struct {
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

type ResourceRequirements struct {
	Requests map[string]string `json:"requests,omitempty"`
}

// List endpoints

// ListModelRoutes handles GET /debug/modelroutes
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

// ListModelServers handles GET /debug/modelservers
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
				podNames = append(podNames, pod.Pod.Namespace+"/"+pod.Pod.Name)
			}
			response.AssociatedPods = podNames
		}

		responses = append(responses, response)
	}

	c.JSON(http.StatusOK, gin.H{"modelservers": responses})
}

// ListPods handles GET /debug/pods
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

// GetModelRoute handles GET /debug/modelroute/{name}
func (h *DebugHandler) GetModelRoute(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name parameter is required"})
		return
	}

	// Try to find the ModelRoute in different namespaces
	// First try default namespace
	namespacedName := "default/" + name
	mr, info := h.store.GetModelRoute(namespacedName)

	if mr == nil {
		// Try to find in all namespaces
		allRoutes := h.store.GetAllModelRoutes()
		for nsName, route := range allRoutes {
			parts := strings.Split(nsName, "/")
			if len(parts) == 2 && parts[1] == name {
				mr = route
				namespacedName = nsName
				_, info = h.store.GetModelRoute(nsName)
				break
			}
		}
	}

	if mr == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ModelRoute not found"})
		return
	}

	parts := strings.Split(namespacedName, "/")
	response := ModelRouteResponse{
		Name:      parts[1],
		Namespace: parts[0],
		Spec:      mr.Spec,
	}

	if info != nil {
		response.RouteInfo = &RouteInfo{
			Model: info.Model,
			Loras: info.Loras,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetModelServer handles GET /debug/modelserver/{namespace}/{name}
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
			podNames = append(podNames, pod.Pod.Namespace+"/"+pod.Pod.Name)
		}
		response.AssociatedPods = podNames
	}

	c.JSON(http.StatusOK, response)
}

// GetPod handles GET /debug/pod/{namespace}/{name}
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

	// Add histogram metrics if available and details are requested
	if includeDetails {
		if podInfo.TimeToFirstToken != nil {
			response.Metrics.TimeToFirstTokenHistogram = convertHistogram(podInfo.TimeToFirstToken)
		}
		if podInfo.TimePerOutputToken != nil {
			response.Metrics.TimePerOutputTokenHistogram = convertHistogram(podInfo.TimePerOutputToken)
		}
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

		// Add container information
		response.Containers = h.convertContainers(podInfo.Pod.Spec.Containers)
	}

	return response
}

func (h *DebugHandler) convertContainers(containers []corev1.Container) []Container {
	var result []Container
	for _, container := range containers {
		c := Container{
			Name:  container.Name,
			Image: container.Image,
		}

		// Convert ports
		for _, port := range container.Ports {
			c.Ports = append(c.Ports, ContainerPort{
				ContainerPort: port.ContainerPort,
				Protocol:      string(port.Protocol),
			})
		}

		// Convert resources
		if len(container.Resources.Requests) > 0 {
			c.Resources = &ResourceRequirements{
				Requests: make(map[string]string),
			}
			for resource, quantity := range container.Resources.Requests {
				c.Resources.Requests[string(resource)] = quantity.String()
			}
		}

		result = append(result, c)
	}
	return result
}

func convertHistogram(hist *dto.Histogram) *Histogram {
	if hist == nil {
		return nil
	}

	result := &Histogram{
		SampleCount: hist.GetSampleCount(),
		SampleSum:   hist.GetSampleSum(),
	}

	for _, bucket := range hist.GetBucket() {
		result.Buckets = append(result.Buckets, Bucket{
			UpperBound:      bucket.GetUpperBound(),
			CumulativeCount: bucket.GetCumulativeCount(),
		})
	}

	return result
}
