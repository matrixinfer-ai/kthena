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

package datastore

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dto "github.com/prometheus/client_model/go"
	"istio.io/istio/pkg/util/sets"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/backend"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

var (
	log = logger.NewLogger("datastore")

	metricsName = []string{
		utils.GPUCacheUsage,
		utils.RequestWaitingNum,
		utils.RequestRunningNum,
		utils.TPOT,
		utils.TTFT,
	}

	histogramMetricsName = []string{
		utils.TPOT,
		utils.TTFT,
	}

	uppdateInterval = 1 * time.Second
)

// EventType represents different types of events that can trigger callbacks
type EventType string

const (
	EventAdd    EventType = "add"
	EventUpdate EventType = "update"
	EventDelete EventType = "delete"
	// Add more event types here as needed
)

// EventData contains information about the event that triggered the callback
type EventData struct {
	EventType EventType
	Pod       types.NamespacedName

	ModelName  string
	ModelRoute *aiv1alpha1.ModelRoute
	// Add more fields as needed for other event types
}

// CallbackFunc is the type of function that can be registered as a callback
type CallbackFunc func(data EventData)

// Store is an interface for storing and retrieving data
type Store interface {
	// Add modelServer which are selected by modelServer.Spec.WorkloadSelector
	AddOrUpdateModelServer(modelServer *aiv1alpha1.ModelServer, pods sets.Set[types.NamespacedName]) error
	// Delete modelServer
	DeleteModelServer(modelServer *aiv1alpha1.ModelServer) error
	// Get modelServer
	GetModelServer(name types.NamespacedName) *aiv1alpha1.ModelServer
	GetPodsByModelServer(name types.NamespacedName) ([]*PodInfo, error)

	// Refresh Store and ModelServer when add a new pod or update a pod
	AddOrUpdatePod(pod *corev1.Pod, modelServer []*aiv1alpha1.ModelServer) error
	// Refresh Store and ModelServer when delete a pod
	DeletePod(podName types.NamespacedName) error

	// New methods for routing functionality
	MatchModelServer(modelName string, request *http.Request) (types.NamespacedName, bool, error)

	// Model routing methods
	AddOrUpdateModelRoute(mr *aiv1alpha1.ModelRoute) error
	DeleteModelRoute(namespacedName string) error

	// New methods for callback management
	RegisterCallback(kind string, callback CallbackFunc)
	// Run to update pod info periodically
	Run(context.Context)
}

type PodInfo struct {
	Pod *corev1.Pod
	// Name of AI inference engine
	engine string
	// TODO: add metrics here
	GPUCacheUsage     float64 // GPU KV-cache usage.
	RequestWaitingNum float64 // Number of requests waiting to be processed.
	RequestRunningNum float64 // Number of requests running.
	// for calculating the average value over the time interval, need to store the results of the last query
	TimeToFirstToken   *dto.Histogram
	TimePerOutputToken *dto.Histogram
	TPOT               float64
	TTFT               float64

	mutex sync.RWMutex // Protects concurrent access to Models and modelServer fields
	// Protected fields - use accessor methods for thread-safe access
	models      sets.Set[string]               // running models. Including base model and lora adapaters.
	modelServer sets.Set[types.NamespacedName] // The modelservers this pod belongs to
}

// modelRouteInfo stores the mapping between a ModelRoute resource and its associated models.
// It maintains both the primary model and any LoRA adapters that are configured for this route.
type modelRouteInfo struct {
	// model is the primary model name that this route serves.
	// If empty, it means this route only serves LoRA adapters.
	model string

	// loras is a list of LoRA adapter names that this route serves.
	// These adapters can be used to modify the behavior of the primary model.
	loras []string
}

type store struct {
	mutex       sync.RWMutex
	modelServer map[types.NamespacedName]*modelServer
	pods        map[types.NamespacedName]*PodInfo

	routeMutex sync.RWMutex
	// Model routing fields
	routeInfo  map[string]*modelRouteInfo
	routes     map[string]*aiv1alpha1.ModelRoute
	loraRoutes map[string]*aiv1alpha1.ModelRoute

	// New fields for callback management
	callbacks map[string][]CallbackFunc
	initiated *atomic.Bool
}

func New() Store {
	return &store{
		modelServer: make(map[types.NamespacedName]*modelServer),
		pods:        make(map[types.NamespacedName]*PodInfo),
		routeInfo:   make(map[string]*modelRouteInfo),
		routes:      make(map[string]*aiv1alpha1.ModelRoute),
		loraRoutes:  make(map[string]*aiv1alpha1.ModelRoute),
		callbacks:   make(map[string][]CallbackFunc),
		initiated:   &atomic.Bool{},
	}
}

func (s *store) Run(ctx context.Context) {
	s.initiated.Store(true)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Only lock when copying pod list
			s.mutex.RLock()
			pods := make([]*PodInfo, 0, len(s.pods))
			for _, podInfo := range s.pods {
				pods = append(pods, podInfo)
			}
			s.mutex.RUnlock()

			// Unlock before updating pods
			for _, podInfo := range pods {
				s.updatePodMetrics(podInfo)
				s.updatePodModels(podInfo)
			}

			time.Sleep(uppdateInterval)
		}
	}
}

func (s *store) AddOrUpdateModelServer(ms *aiv1alpha1.ModelServer, pods sets.Set[types.NamespacedName]) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	name := utils.GetNamespaceName(ms)
	if _, ok := s.modelServer[name]; !ok {
		s.modelServer[name] = newModelServer(ms)
	} else {
		s.modelServer[name].modelServer = ms
	}
	// donot operate s.pods here, which are done within pod handler
	s.modelServer[name].pods = pods

	return nil
}

func (s *store) DeleteModelServer(ms *aiv1alpha1.ModelServer) error {
	name := utils.GetNamespaceName(ms)
	modelserver, ok := s.modelServer[name]
	if !ok {
		return nil
	}

	podNames := modelserver.getPods()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// delete the model server from the store
	delete(s.modelServer, name)
	// then delete the model server from all pod info
	for _, podName := range podNames {
		podInfo := s.pods[podName]
		if podInfo == nil {
			log.Warningf("pod %s not found", podName)
			continue
		}
		podInfo.RemoveModelServer(name)
		if podInfo.GetModelServerCount() == 0 {
			delete(s.pods, podName)
		}
	}

	return nil
}

func (s *store) GetModelServer(name types.NamespacedName) *aiv1alpha1.ModelServer {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if ms, ok := s.modelServer[name]; ok {
		return ms.modelServer
	}

	return nil
}

func (s *store) GetPodsByModelServer(name types.NamespacedName) ([]*PodInfo, error) {
	s.mutex.RLock()
	ms, ok := s.modelServer[name]
	s.mutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("model server not found: %v", name)
	}

	podNames := ms.getPods()
	pods := make([]*PodInfo, 0, len(podNames))

	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, podName := range podNames {
		pods = append(pods, s.pods[podName])
	}

	return pods, nil
}

func (s *store) AddOrUpdatePod(pod *corev1.Pod, modelServers []*aiv1alpha1.ModelServer) error {
	podName := utils.GetNamespaceName(pod)
	// TODO: check if pod is already in the store, use the existing PodInfo if exists
	newPodInfo := &PodInfo{
		Pod:         pod,
		modelServer: sets.Set[types.NamespacedName]{},
		models:      sets.New[string](),
	}

	for _, modelServer := range modelServers {
		modelServerName := utils.GetNamespaceName(modelServer)
		newPodInfo.AddModelServer(modelServerName)
		// NOTE: even if a pod belongs to multiple model servers, the backend should be the same
		newPodInfo.engine = string(modelServer.Spec.InferenceEngine)
	}

	s.mutex.Lock()
	s.pods[podName] = newPodInfo
	s.mutex.Unlock()

	s.updatePodMetrics(newPodInfo)
	s.updatePodModels(newPodInfo)

	return nil
}

func (s *store) DeletePod(podName types.NamespacedName) error {
	s.mutex.Lock()
	if pod, ok := s.pods[podName]; ok {
		modelServers := pod.GetModelServers()
		for modelServerName := range modelServers {
			s.modelServer[modelServerName].deletePod(podName)
		}
		delete(s.pods, podName)
	}
	s.mutex.Unlock()

	s.triggerCallbacks("Pod", EventData{
		EventType: EventDelete,
		Pod:       podName,
	})

	return nil
}

// Model routing methods
func (s *store) AddOrUpdateModelRoute(mr *aiv1alpha1.ModelRoute) error {
	s.routeMutex.Lock()
	key := mr.Namespace + "/" + mr.Name
	s.routeInfo[key] = &modelRouteInfo{
		model: mr.Spec.ModelName,
		loras: mr.Spec.LoraAdapters,
	}

	if mr.Spec.ModelName != "" {
		s.routes[mr.Spec.ModelName] = mr
	}

	for _, lora := range mr.Spec.LoraAdapters {
		s.loraRoutes[lora] = mr
	}
	s.routeMutex.Unlock()

	s.triggerCallbacks("ModelRoute", EventData{
		EventType:  EventUpdate,
		ModelName:  mr.Spec.ModelName,
		ModelRoute: mr,
	})
	return nil
}

func (s *store) DeleteModelRoute(namespacedName string) error {
	s.routeMutex.Lock()
	info := s.routeInfo[namespacedName]
	var modelName string
	if info != nil {
		modelName = info.model
		delete(s.routes, info.model)
		for _, lora := range info.loras {
			delete(s.loraRoutes, lora)
		}
	}
	delete(s.routeInfo, namespacedName)
	s.routeMutex.Unlock()

	s.triggerCallbacks("ModelRoute", EventData{
		EventType:  EventDelete,
		ModelName:  modelName,
		ModelRoute: nil,
	})
	return nil
}

func (s *store) MatchModelServer(model string, req *http.Request) (types.NamespacedName, bool, error) {
	s.routeMutex.RLock()
	defer s.routeMutex.RUnlock()

	var is_lora bool
	mr, ok := s.routes[model]
	if !ok {
		mr, ok = s.loraRoutes[model]
		if !ok {
			return types.NamespacedName{}, false, fmt.Errorf("not found route rules for model %s", model)
		}
		is_lora = true
	}

	rule, err := s.selectRule(req, mr.Spec.Rules)
	if err != nil {
		return types.NamespacedName{}, false, fmt.Errorf("failed to select route rule: %v", err)
	}

	dst, err := s.selectDestination(rule.TargetModels)
	if err != nil {
		return types.NamespacedName{}, false, fmt.Errorf("failed to select destination: %v", err)
	}

	return types.NamespacedName{Namespace: mr.Namespace, Name: dst.ModelServerName}, is_lora, nil
}

func (s *store) selectRule(req *http.Request, rules []*aiv1alpha1.Rule) (*aiv1alpha1.Rule, error) {
	for _, rule := range rules {
		if rule.ModelMatch == nil {
			return rule, nil
		}

		headersMatched := true
		for key, sm := range rule.ModelMatch.Headers {
			reqValue := req.Header.Get(key)
			if !matchString(sm, reqValue) {
				headersMatched = false
				break
			}
		}
		if !headersMatched {
			continue
		}

		uriMatched := true
		if uriMatch := rule.ModelMatch.Uri; uriMatch != nil {
			if !matchString(uriMatch, req.URL.Path) {
				uriMatched = false
			}
		}

		if !uriMatched {
			continue
		}

		return rule, nil
	}

	return nil, fmt.Errorf("failed to find a matching rule")
}

func matchString(sm *aiv1alpha1.StringMatch, value string) bool {
	switch {
	case sm.Exact != nil:
		return value == *sm.Exact
	case sm.Prefix != nil:
		return strings.HasPrefix(value, *sm.Prefix)
	case sm.Regex != nil:
		matched, _ := regexp.MatchString(*sm.Regex, value)
		return matched
	default:
		return true
	}
}

func (s *store) selectDestination(targets []*aiv1alpha1.TargetModel) (*aiv1alpha1.TargetModel, error) {
	weightedSlice, err := toWeightedSlice(targets)
	if err != nil {
		return nil, err
	}

	index := selectFromWeightedSlice(weightedSlice)

	return targets[index], nil
}

func toWeightedSlice(targets []*aiv1alpha1.TargetModel) ([]uint32, error) {
	var isWeighted bool
	if targets[0].Weight != nil {
		isWeighted = true
	}

	res := make([]uint32, len(targets))

	for i, target := range targets {
		if (isWeighted && target.Weight == nil) || (!isWeighted && target.Weight != nil) {
			return nil, fmt.Errorf("the weight field in targetModel must be either fully specified or not specified")
		}

		if isWeighted {
			res[i] = *target.Weight
		} else {
			// If weight is not specified, set to 1.
			res[i] = 1
		}
	}

	return res, nil
}

func selectFromWeightedSlice(weights []uint32) int {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	totalWeight := 0
	for _, weight := range weights {
		totalWeight += int(weight)
	}

	randomNum := rng.Intn(totalWeight)

	for i, weight := range weights {
		randomNum -= int(weight)
		if randomNum < 0 {
			return i
		}
	}

	return 0
}

func (s *store) updatePodMetrics(pod *PodInfo) {
	if pod.engine == "" {
		log.Error("failed to find backend in pod")
		return
	}

	previousHistogram := getPreviousHistogram(pod)
	gaugeMetrics, histogramMetrics := backend.GetPodMetrics(pod.engine, pod.Pod, previousHistogram)
	updateGaugeMetricsInfo(pod, gaugeMetrics)
	updateHistogramMetrics(pod, histogramMetrics)
}

func (s *store) updatePodModels(podInfo *PodInfo) {
	if podInfo.engine == "" {
		log.Error("failed to find backend in pod")
		return
	}

	models, err := backend.GetPodModels(podInfo.engine, podInfo.Pod)
	if err != nil {
		log.Errorf("failed to get models of pod %s/%s", podInfo.Pod.GetNamespace(), podInfo.Pod.GetName())
	}

	podInfo.UpdateModels(models)
}

func getPreviousHistogram(podinfo *PodInfo) map[string]*dto.Histogram {
	previousHistogram := make(map[string]*dto.Histogram)
	if podinfo.TimePerOutputToken != nil {
		previousHistogram[utils.TPOT] = podinfo.TimePerOutputToken
	}
	if podinfo.TimeToFirstToken != nil {
		previousHistogram[utils.TTFT] = podinfo.TimeToFirstToken
	}
	return previousHistogram
}

func updateGaugeMetricsInfo(podinfo *PodInfo, metricsInfo map[string]float64) {
	updateFuncs := map[string]func(float64){
		utils.GPUCacheUsage: func(f float64) {
			podinfo.GPUCacheUsage = f
		},
		utils.RequestWaitingNum: func(f float64) {
			podinfo.RequestWaitingNum = f
		},
		utils.RequestRunningNum: func(f float64) {
			podinfo.RequestRunningNum = f
		},
		utils.TPOT: func(f float64) {
			if f == float64(0.0) {
				return
			}
			podinfo.TPOT = f
		},
		utils.TTFT: func(f float64) {
			if f == float64(0.0) {
				return
			}
			podinfo.TTFT = f
		},
	}

	for _, name := range metricsName {
		if updateFunc, exist := updateFuncs[name]; exist {
			updateFunc(metricsInfo[name])
		} else {
			log.Debugf("Unknow metric: %s", name)
		}
	}
}

func updateHistogramMetrics(podinfo *PodInfo, histogramMetrics map[string]*dto.Histogram) {
	updateFuncs := map[string]func(*dto.Histogram){
		utils.TPOT: func(h *dto.Histogram) {
			podinfo.TimePerOutputToken = h
		},
		utils.TTFT: func(h *dto.Histogram) {
			podinfo.TimeToFirstToken = h
		},
	}

	for _, name := range histogramMetricsName {
		if updateFunc, exist := updateFuncs[name]; exist {
			updateFunc(histogramMetrics[name])
		} else {
			log.Debugf("Unknow histogram metric: %s", name)
		}
	}
}

// RegisterCallback registers a callback function for a specific resource
// Note this can only be called during bootstrapping.
func (s *store) RegisterCallback(kind string, callback CallbackFunc) {
	if s.initiated.Load() {
		log.Error("Cannot register callback after store is initiated")
		return
	}

	if _, exists := s.callbacks[kind]; !exists {
		s.callbacks[kind] = make([]CallbackFunc, 0)
	}
	s.callbacks[kind] = append(s.callbacks[kind], callback)
}

// triggerCallbacks executes all registered callbacks for a specific event type
func (s *store) triggerCallbacks(kind string, data EventData) {
	if callbacks, exists := s.callbacks[kind]; exists {
		for _, callback := range callbacks {
			go callback(data)
		}
	}
}

// PodInfo methods for thread-safe access to models and modelServer fields

// GetModels returns a copy of the models set
func (p *PodInfo) GetModels() sets.Set[string] {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	result := sets.New[string]()
	for model := range p.models {
		result.Insert(model)
	}
	return result
}

// GetModels returns a copy of the models set
func (p *PodInfo) Contains(model string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.models != nil && p.models.Contains(model)
}

// UpdateModels updates the models set with a new list of models
func (p *PodInfo) UpdateModels(models []string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.models = sets.New[string](models...)
}

// RemoveModel removes a model from the models set
func (p *PodInfo) RemoveModel(model string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.models != nil {
		p.models.Delete(model)
	}
}

// GetModelServers returns a copy of the modelServer set
func (p *PodInfo) GetModelServers() sets.Set[types.NamespacedName] {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	result := sets.New[types.NamespacedName]()
	for ms := range p.modelServer {
		result.Insert(ms)
	}
	return result
}

// AddModelServer adds a model server to the modelServer set
func (p *PodInfo) AddModelServer(ms types.NamespacedName) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.modelServer == nil {
		p.modelServer = sets.New[types.NamespacedName]()
	}
	p.modelServer.Insert(ms)
}

// RemoveModelServer removes a model server from the modelServer set
func (p *PodInfo) RemoveModelServer(ms types.NamespacedName) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.modelServer != nil {
		p.modelServer.Delete(ms)
	}
}

// HasModelServer checks if a model server exists in the modelServer set
func (p *PodInfo) HasModelServer(ms types.NamespacedName) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.modelServer != nil && p.modelServer.Contains(ms)
}

// GetModelServerCount returns the number of model servers
func (p *PodInfo) GetModelServerCount() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.modelServer == nil {
		return 0
	}
	return p.modelServer.Len()
}

// GetModelsList returns all models as a slice
func (p *PodInfo) GetModelsList() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.models == nil {
		return nil
	}
	return p.models.UnsortedList()
}

// GetModelServersList returns all model servers as a slice
func (p *PodInfo) GetModelServersList() []types.NamespacedName {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.modelServer == nil {
		return nil
	}
	return p.modelServer.UnsortedList()
}
