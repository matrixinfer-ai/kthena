package datastore

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"

	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
)

// Store is an interface for storing and retrieving data
type Store interface {
	GetInferGroupByModelInfer(modelInferName types.NamespacedName) (int, []InferGroup, error)
	GetRunningPodByInferGroup(modelInferName types.NamespacedName, inferGroupName string) (int, []string, error)
	GetInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string) InferGroupStatus
	GetModelInferByInferGroup(name types.NamespacedName) *workloadv1alpha1.ModelInfer
	GetInferGroupNameByRunningPod(name types.NamespacedName) []string
	DeleteModelInfer(modelInferName types.NamespacedName) error
	DeleteInferGroupOfRunningPodMap(modelInferName types.NamespacedName, inferGroupName string) error
	UpdateInferGroupForModelInfer(modelInferName types.NamespacedName, inferGroupList []InferGroup) error
	UpdateRunningPodForInferGroup(inferGroupName types.NamespacedName, runningPodList []string) error
	UpdateInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string, Status InferGroupStatus) error
}

type store struct {
	mutex sync.RWMutex

	inferGroup              map[types.NamespacedName][]InferGroup
	runningPodOfInferGroups map[types.NamespacedName][]string
	inferGroupOfModelInfer  map[types.NamespacedName]*workloadv1alpha1.ModelInfer
}

type InferGroup struct {
	Name      string
	Namespace string
	Status    InferGroupStatus
}

type InferGroupStatus string

const (
	InferGroupRunning  InferGroupStatus = "Running"
	InferGroupCreating InferGroupStatus = "Creating"
	InferGroupDeleting InferGroupStatus = "Deleting"
	InferGroupNotFound InferGroupStatus = "NotFound"
)

func New() (Store, error) {
	return &store{
		inferGroup:              make(map[types.NamespacedName][]InferGroup),
		runningPodOfInferGroups: make(map[types.NamespacedName][]string),
		inferGroupOfModelInfer:  make(map[types.NamespacedName]*workloadv1alpha1.ModelInfer),
	}, nil
}

func (s *store) GetInferGroupByModelInfer(modelInferName types.NamespacedName) (int, []InferGroup, error) {
	// todo Returns the number of infergroups, the list of infergroup, and errors for a modelinfer
	return 0, nil, nil
}

func (s *store) GetRunningPodByInferGroup(modelInferName types.NamespacedName, inferGroupName string) (int, []string, error) {
	// todo Returns the number of running pods, the list of running pod names, and errors for an infergroup
	return 0, nil, nil
}

func (s *store) GetInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string) InferGroupStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if inferGroups, exist := s.inferGroup[modelInferName]; exist {
		for _, inferGroup := range inferGroups {
			if inferGroup.Name == inferGroupName {
				return inferGroup.Status
			}
		}
	}
	return InferGroupNotFound
}

func (s *store) DeleteModelInfer(modelInferName types.NamespacedName) error {
	// delete modelInfer in inferGroup map
	return nil
}

func (s *store) DeleteInferGroupOfRunningPodMap(modelInferName types.NamespacedName, inferGroupName string) error {
	// delete inferGroup in runningPodOfInferGroup
	s.mutex.Lock()
	defer s.mutex.RLock()
	groupNamedName := types.NamespacedName{
		Namespace: modelInferName.Namespace,
		Name:      inferGroupName,
	}
	delete(s.runningPodOfInferGroups, groupNamedName)
	for index, inferGroup := range s.inferGroup[modelInferName] {
		if inferGroup.Name == inferGroupName {
			s.inferGroup[modelInferName] = append(s.inferGroup[modelInferName][:index], s.inferGroup[modelInferName][index+1:]...)
		}
	}
	return nil
}

func (s *store) UpdateInferGroupForModelInfer(modelInferName types.NamespacedName, inferGroupList []InferGroup) error {
	return nil
}

func (s *store) UpdateRunningPodForInferGroup(inferGroupName types.NamespacedName, runningPodList []string) error {
	return nil
}

func (s *store) UpdateInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string, Status InferGroupStatus) error {
	return nil
}

func (s *store) GetModelInferByInferGroup(name types.NamespacedName) *workloadv1alpha1.ModelInfer {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for groupName, modelInfer := range s.inferGroupOfModelInfer {
		if groupName == name {
			return modelInfer
		}
	}
	return nil
}

func (s *store) GetInferGroupNameByRunningPod(name types.NamespacedName) []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.runningPodOfInferGroups[name]
}
