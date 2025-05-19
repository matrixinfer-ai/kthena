package datastore

import (
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/backend"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/backend/sglang"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/backend/vllm"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

var (
	log = logger.NewLogger("datastore")

	uppdateInterval = 1 * time.Second
)

// Store is an interface for storing and retrieving data
type Store interface {
	// Add modelServer and pods which are selected by modelServer.Spec.WorkloadSelector
	AddOrUpdateModelServer(name types.NamespacedName, modelServer *aiv1alpha1.ModelServer, pods []*corev1.Pod) error
	// Delete modelServer
	DeleteModelServer(modelServer *aiv1alpha1.ModelServer) error
	// Get modelServer name. This name is as same as modelServer.Spec.Model
	GetModelNameByModelServer(name types.NamespacedName) *string
	GetPodsByModelServer(name types.NamespacedName) []PodInfo

	// Refresh Store and ModelServer when add a new pod or update a pod
	AddOrUpdatePod(pod *corev1.Pod, modelServer []*aiv1alpha1.ModelServer) error
	// Refresh Store and ModelServer when delete a pod
	DeletePod(pod *corev1.Pod) error

	// New methods for routing functionality
	MatchModelServer(modelName string, request *http.Request) (types.NamespacedName, bool, error)
	GetModelServerEndpoints(name types.NamespacedName) ([]PodInfo, *string, error)

	// Model routing methods
	AddOrUpdateModelRoute(mr *aiv1alpha1.ModelRoute) error
	DeleteModelRoute(namespacedName string) error
}

type modelServer struct {
	mutex sync.RWMutex

	model *string
	pods  map[types.NamespacedName]PodInfo
}

type PodInfo struct {
	Pod *corev1.Pod

	// TODO: add metrics here
	GPUCacheUsage     float64 // GPU KV-cache usage.
	RequestWaitingNum float64 // Number of requests waiting to be processed.
	// for calculating the average value over the time interval, need to store the results of the last query
	TimeToFirstToken   *dto.Histogram
	TimePerOutputToken *dto.Histogram
	TPOT               float64
	TTFT               float64
	Models             map[string]struct{}               // running lora adapaters.
	modelServer        map[types.NamespacedName]struct{} // The modelservers this pod belongs to
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
	mutex sync.RWMutex

	modelServer map[types.NamespacedName]*modelServer
	pods        map[types.NamespacedName]PodInfo

	// Model routing fields
	routeInfo  map[string]*modelRouteInfo
	routes     map[string]*aiv1alpha1.ModelRoute
	loraRoutes map[string]*aiv1alpha1.ModelRoute
}

func New() Store {
	return &store{
		modelServer: make(map[types.NamespacedName]*modelServer),
		pods:        make(map[types.NamespacedName]PodInfo),
		routeInfo:   make(map[string]*modelRouteInfo),
		routes:      make(map[string]*aiv1alpha1.ModelRoute),
		loraRoutes:  make(map[string]*aiv1alpha1.ModelRoute),
	}
}

func (s *store) Run(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
			for _, ms := range s.modelServer {
				for _, podInfo := range ms.pods {
					s.updatePodMetrics(podInfo.Pod)
				}
			}
		}
	}
}

func (s *store) AddOrUpdateModelServer(name types.NamespacedName, ms *aiv1alpha1.ModelServer, pods []*corev1.Pod) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.modelServer[name]; !ok {
		s.modelServer[name] = &modelServer{
			pods: make(map[types.NamespacedName]PodInfo),
		}
	}

	s.modelServer[name].model = ms.Spec.Model

	podsMap := make(map[types.NamespacedName]PodInfo)
	for _, pod := range pods {
		podName := utils.GetNamespaceName(pod)
		if podInfo, ok := s.pods[name]; ok {
			// If the pod was not belong to modelserver.
			if _, exist := podInfo.modelServer[name]; !exist {
				podInfo.modelServer[name] = struct{}{}
			}
			podsMap[podName] = podInfo
		} else {
			newPodInfo := PodInfo{
				Pod:    pod,
				Models: make(map[string]struct{}),
				modelServer: map[types.NamespacedName]struct{}{
					name: struct{}{},
				},
			}
			podsMap[podName] = newPodInfo
			s.pods[podName] = newPodInfo
		}
		s.updatePodMetrics(pod)
	}

	s.modelServer[name].pods = podsMap

	return nil
}

func (s *store) DeleteModelServer(ms *aiv1alpha1.ModelServer) error {
	name := utils.GetNamespaceName(ms)
	s.PodHandlerWhenDeleteModelServer(name)
	s.mutex.Unlock()
	delete(s.modelServer, name)
	s.mutex.Unlock()
	return nil
}

func (s *store) GetModelNameByModelServer(name types.NamespacedName) *string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if ms, ok := s.modelServer[name]; ok {
		return ms.model
	}

	return nil
}

func (s *store) GetPodsByModelServer(name types.NamespacedName) []PodInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if ms, ok := s.modelServer[name]; ok {
		ms.mutex.RLock()
		defer ms.mutex.RUnlock()

		pods := []PodInfo{}

		for _, pod := range ms.pods {
			pods = append(pods, pod)
		}

		return pods
	}

	return nil
}

func (s *store) AddOrUpdatePod(pod *corev1.Pod, modelServers []*aiv1alpha1.ModelServer) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	podName := utils.GetNamespaceName(pod)
	newPodInfo := PodInfo{
		Pod:         pod,
		modelServer: make(map[types.NamespacedName]struct{}),
	}

	modelServerNames := []types.NamespacedName{}
	for _, modelServer := range modelServers {
		modelServerName := utils.GetNamespaceName(modelServer)
		modelServerNames = append(modelServerNames, modelServerName)
		newPodInfo.modelServer[modelServerName] = struct{}{}
	}

	// if already have podinfo, need to delete old pod in modelserver
	if podInfo, ok := s.pods[podName]; ok {
		for name, _ := range podInfo.modelServer {
			delete(s.modelServer[name].pods, podName)
		}
	}

	s.pods[podName] = newPodInfo
	for _, modelServerName := range modelServerNames {
		s.modelServer[modelServerName].pods[podName] = newPodInfo
	}

	//TODO update metrics of new pod
	return nil
}

func (s *store) PodHandlerWhenDeleteModelServer(modelServerName types.NamespacedName) error {
	pods := s.modelServer[modelServerName].pods
	for podName := range pods {
		podInfo := s.pods[podName]
		s.mutex.Lock()
		delete(podInfo.modelServer, modelServerName)
		// if modelServer is nil, pod will delete
		if len(podInfo.modelServer) == 0 {
			delete(s.pods, podName)
		}
		s.mutex.Unlock()
	}

	return nil
}

func (s *store) DeletePod(pod *corev1.Pod) error {
	podName := utils.GetNamespaceName(pod)
	s.mutex.Lock()
	modelServers := s.pods[podName].modelServer
	for modelServerName := range modelServers {
		delete(s.modelServer[modelServerName].pods, podName)
	}
	delete(s.pods, podName)
	s.mutex.Unlock()
	return nil
}

// Model routing methods
func (s *store) AddOrUpdateModelRoute(mr *aiv1alpha1.ModelRoute) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.routeInfo[mr.Namespace+"/"+mr.Name] = &modelRouteInfo{
		model: mr.Spec.ModelName,
		loras: mr.Spec.LoraAdapters,
	}

	if mr.Spec.ModelName != "" {
		s.routes[mr.Spec.ModelName] = mr
	}

	for _, lora := range mr.Spec.LoraAdapters {
		s.loraRoutes[lora] = mr
	}

	return nil
}

func (s *store) DeleteModelRoute(namespacedName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	info := s.routeInfo[namespacedName]
	if info != nil {
		delete(s.routes, info.model)
		for _, lora := range info.loras {
			delete(s.loraRoutes, lora)
		}
	}

	delete(s.routeInfo, namespacedName)

	return nil
}

func (s *store) MatchModelServer(model string, req *http.Request) (types.NamespacedName, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

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

func (s *store) GetModelServerEndpoints(name types.NamespacedName) ([]PodInfo, *string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if ms, ok := s.modelServer[name]; ok {
		ms.mutex.RLock()
		defer ms.mutex.RUnlock()

		pods := []PodInfo{}
		for _, pod := range ms.pods {
			pods = append(pods, pod)
		}

		return pods, ms.model, nil
	}

	return nil, nil, fmt.Errorf("model server not found: %v", name)
}

func (s *store) updatePodMetrics(pod *corev1.Pod) {
	podName := utils.GetNamespaceName(pod)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	podInfo, exist := s.pods[podName]
	if !exist {
		log.Errorf("failed to get podInfo of pod %s/%s", pod.GetNamespace(), pod.GetName())
		return
	}

	countMetricsInfo, histogramMetricsInfo := backend.GetPodMetrics(pod)

	updatePodInfoCount(&podInfo, countMetricsInfo)
	updatePodInfoHistogram(&podInfo, histogramMetricsInfo)

	for ms := range podInfo.modelServer {
		s.modelServer[ms].pods[podName] = podInfo
	}
}

func updatePodInfoCount(podInfo *PodInfo, countMap map[string]float64) *PodInfo {
	for name, value := range countMap {
		switch name {
		case vllm.GPUCacheUsage:
			podInfo.GPUCacheUsage = value
		case vllm.RequestWaitingNum:
			podInfo.RequestWaitingNum = value
		case sglang.GPUCacheUsage:
			podInfo.GPUCacheUsage = value
		case sglang.RequestWaitingNum:
			podInfo.RequestWaitingNum = value
		}
	}

	return podInfo
}

func updatePodInfoHistogram(podInfo *PodInfo, histogram map[string]*dto.Histogram) *PodInfo {
	lastTimeToFirstToken := podInfo.TimeToFirstToken
	lastTimePerOutputToken := podInfo.TimePerOutputToken

	for name, value := range histogram {
		switch name {
		case vllm.TPOT:
			podInfo.TPOT = lastPeriodAvg(lastTimePerOutputToken, value)
			podInfo.TimePerOutputToken = value
		case vllm.TTFT:
			podInfo.TTFT = lastPeriodAvg(lastTimeToFirstToken, value)
			podInfo.TimeToFirstToken = value
		case sglang.TPOT:
			podInfo.TPOT = lastPeriodAvg(lastTimePerOutputToken, value)
			podInfo.TimePerOutputToken = value
		case sglang.TTFT:
			podInfo.TTFT = lastPeriodAvg(lastTimeToFirstToken, value)
			podInfo.TimeToFirstToken = value
		}
	}
	return podInfo
}

func lastPeriodAvg(previous, current *dto.Histogram) float64 {
	previousSum := previous.GetSampleSum()
	previousCount := previous.GetSampleCount()

	currentSum := current.GetSampleSum()
	currentCount := current.GetSampleCount()

	deltaSum := currentSum - previousSum
	deltaCount := currentCount - previousCount

	if deltaCount == 0 {
		return 0
	}

	return deltaSum / float64(deltaCount)
}
