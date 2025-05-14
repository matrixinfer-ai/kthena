package datastore

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

// Store is an interface for storing and retrieving data
type Store interface {
	AddOrUpdateModelServer(name types.NamespacedName, modelServer *aiv1alpha1.ModelServer, pods []*corev1.Pod) error
	DeleteModelServer(modelServer *aiv1alpha1.ModelServer) error
	// Get the real model name served by the model server, nil means the model name in the user request will be used.
	GetModelNameByModelServer(name types.NamespacedName) *string
	GetPodsByModelServer(name types.NamespacedName) []*PodInfo

	AddOrUpdatePod(pod *corev1.Pod, modelServers []*aiv1alpha1.ModelServer) error
	DeletePod(pod *corev1.Pod) error
}

type modelServer struct {
	mutex sync.RWMutex

	// real model name served
	model *string
	pods  map[types.NamespacedName]*PodInfo
}

type PodInfo struct {
	mu sync.RWMutex

	Pod *corev1.Pod
	// TODO: add metrics here
	GPUCacheUsage     float32                           // GPU KV-cache usage.
	RequestWaitingNum int                               // Number of requests waiting to be processed.
	Models            map[string]struct{}               // running model and lora adapaters.
	modelServer       map[types.NamespacedName]struct{} // The modelservers this pod belongs to
}

type store struct {
	mutex sync.RWMutex

	modelServer map[types.NamespacedName]*modelServer
	pods        map[types.NamespacedName]*PodInfo
}

func New() (Store, error) {
	return &store{
		modelServer: make(map[types.NamespacedName]*modelServer),
		pods:        make(map[types.NamespacedName]*PodInfo),
	}, nil
}

func (s *store) AddOrUpdateModelServer(name types.NamespacedName, ms *aiv1alpha1.ModelServer, pods []*corev1.Pod) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.modelServer[name]; !ok {
		s.modelServer[name] = &modelServer{
			pods: make(map[types.NamespacedName]*PodInfo),
		}
	}

	s.modelServer[name].mutex.Lock()
	defer s.modelServer[name].mutex.Unlock()

	s.modelServer[name].model = ms.Spec.Model

	podsMap := make(map[types.NamespacedName]*PodInfo)
	for _, pod := range pods {
		podName := utils.GetNamespaceName(pod)
		if podInfo, ok := s.pods[name]; ok {
			// If the pod was not belong to modelserver.
			if _, exist := podInfo.modelServer[name]; !exist {
				podInfo.modelServer[name] = struct{}{}
			}
			podsMap[podName] = podInfo
		} else {
			newPodInfo := &PodInfo{
				Pod:    pod,
				Models: make(map[string]struct{}),
				modelServer: map[types.NamespacedName]struct{}{
					name: struct{}{},
				},
			}
			podsMap[podName] = newPodInfo
			s.pods[podName] = newPodInfo
		}
		// TODO: use goroutine update new pod metrics
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

func (s *store) GetPodsByModelServer(name types.NamespacedName) []*PodInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if ms, ok := s.modelServer[name]; ok {
		ms.mutex.RLock()
		defer ms.mutex.RUnlock()

		pods := []*PodInfo{}

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
	newPodInfo := &PodInfo{
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
