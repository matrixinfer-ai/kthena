package datastore

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

// Store is an interface for storing and retrieving data
type Store interface {
	GetInferGroupByModelInfer(modelInferName types.NamespacedName) (int, []InferGroup, error)
	GetRunningPodByInferGroup(modelInferName types.NamespacedName, inferGroupName string) (int, []string, error)
	GetInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string) InferGroupStatus
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
	}, nil
}

// Returns the number of infergroups, the list of infergroup, and errors for a modelinfer
func (s *store) GetInferGroupByModelInfer(modelInferName types.NamespacedName) (int, []InferGroup, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		return 0, nil, nil
	}
	return len(inferGroups), inferGroups, nil
}

// Returns the number of running pods, the list of running pod names, and errors for an infergroup
func (s *store) GetRunningPodByInferGroup(modelInferName types.NamespacedName, inferGroupName string) (int, []string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	runningPods, ok := s.runningPodOfInferGroups[types.NamespacedName{
		Namespace: modelInferName.Namespace,
		Name:      inferGroupName,
	}]
	if !ok {
		return 0, nil, nil
	}
	return len(runningPods), runningPods, nil
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

// delete modelInfer in inferGroup map
func (s *store) DeleteModelInfer(modelInferName types.NamespacedName) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		return nil
	}
	for _, groupName := range inferGroups {
		groupNamedName := types.NamespacedName{
			Namespace: groupName.Namespace,
			Name:      groupName.Name,
		}
		delete(s.runningPodOfInferGroups, groupNamedName)
	}
	delete(s.inferGroup, modelInferName)
	return nil
}

// delete inferGroup in runningPodOfInferGroup
func (s *store) DeleteInferGroupOfRunningPodMap(modelInferName types.NamespacedName, inferGroupName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.inferGroup[modelInferName] = inferGroupList
	return nil
}

func (s *store) UpdateRunningPodForInferGroup(inferGroupName types.NamespacedName, runningPodList []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.runningPodOfInferGroups[inferGroupName] = runningPodList
	return nil
}

func (s *store) UpdateInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string, status InferGroupStatus) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		return fmt.Errorf("Failed to find modelInfer %s", modelInferName.Namespace+"/"+modelInferName.Name)
	}
	for i := range inferGroups {
		if inferGroups[i].Name == inferGroupName {
			inferGroups[i].Status = status
		}
	}
	return nil
}
