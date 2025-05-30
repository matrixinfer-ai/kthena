package datastore

import (
	"fmt"
	"slices"
	"sync"

	"k8s.io/apimachinery/pkg/types"

	"matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
)

// Store is an interface for storing and retrieving data
type Store interface {
	GetInferGroupByModelInfer(modelInferName types.NamespacedName) ([]InferGroup, error)
	GetRunningPodByInferGroup(modelInferName types.NamespacedName, inferGroupName string) ([]string, error)
	GetInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string) InferGroupStatus
	DeleteModelInfer(modelInferName types.NamespacedName) error
	DeleteInferGroupOfRunningPodMap(modelInferName types.NamespacedName, inferGroupName string) error
	InitInferGroupForModelInfer(modelInferName types.NamespacedName)
	InitRunningPodForInferGroup(inferGroupName types.NamespacedName)
	AddInferGroupForModelInfer(modelInferName types.NamespacedName, idx int) error
	AddRunningPodForInferGroup(inferGroupName types.NamespacedName, runningPodName string) error
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

// GetInferGroupByModelInfer returns the list of inferGroups and errors
func (s *store) GetInferGroupByModelInfer(modelInferName types.NamespacedName) ([]InferGroup, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		return nil, nil
	}
	return inferGroups, nil
}

// GetRunningPodByInferGroup returns the list of running pods name and errors
func (s *store) GetRunningPodByInferGroup(modelInferName types.NamespacedName, inferGroupName string) ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	runningPods, ok := s.runningPodOfInferGroups[types.NamespacedName{
		Namespace: modelInferName.Namespace,
		Name:      inferGroupName,
	}]
	if !ok {
		return nil, nil
	}
	return runningPods, nil
}

// GetInferGroupStatus returns the status of inferGroup
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

// DeleteModelInfer delete modelInfer in inferGroup map
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

// DeleteInferGroupOfRunningPodMap delete inferGroup in runningPodOfInferGroup map
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

// AddInferGroupForModelInfer add inferGroup item of one modelInfer
func (s *store) AddInferGroupForModelInfer(modelInferName types.NamespacedName, idx int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, ok := s.inferGroup[modelInferName]
	if !ok {
		return fmt.Errorf("modelinfer %s inferGroup list does not exist and needs to be initialized", modelInferName.Name)
	}
	newGroup := InferGroup{
		Name:      utils.GenerateInferGroupName(modelInferName.Name, idx),
		Namespace: modelInferName.Namespace,
		Status:    InferGroupCreating,
	}
	s.inferGroup[modelInferName] = slices.Insert(s.inferGroup[modelInferName], idx, newGroup)
	return nil
}

// AddRunningPodForInferGroup add inferGroup in runningPodOfInferGroup map
func (s *store) AddRunningPodForInferGroup(inferGroupName types.NamespacedName, runningPodName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, ok := s.runningPodOfInferGroups[inferGroupName]
	if !ok {
		return fmt.Errorf("inferGroup %s running pods listdoes not exist and needs to be initialized", inferGroupName.Name)
	}

	s.runningPodOfInferGroups[inferGroupName] = append(s.runningPodOfInferGroups[inferGroupName], runningPodName)
	return nil
}

// UpdateInferGroupStatus update status of one inferGroup
func (s *store) UpdateInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string, status InferGroupStatus) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		return fmt.Errorf("failed to find modelInfer %s", modelInferName.Namespace+"/"+modelInferName.Name)
	}
	for i := range inferGroups {
		if inferGroups[i].Name == inferGroupName {
			inferGroups[i].Status = status
		}
	}
	return nil
}

// InitInferGroupForModelInfer initialize a modelInfer into the inferGroup map
func (s *store) InitInferGroupForModelInfer(modelInferName types.NamespacedName) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.inferGroup[modelInferName] = nil
}

// InitRunningPodForInferGroup initialize an inferGroup into the runningPodOfInferGroup map
func (s *store) InitRunningPodForInferGroup(inferGroupName types.NamespacedName) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.runningPodOfInferGroups[inferGroupName] = nil
}
