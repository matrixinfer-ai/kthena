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
	DeleteInferGroup(modelInferName types.NamespacedName, inferGroupName string) error
	AddInferGroup(modelInferName types.NamespacedName, idx int) error
	AddRunningPodToInferGroup(inferGroupName types.NamespacedName, runningPodName string)
	DeleteRunningPodFromInferGroup(inferGroupName types.NamespacedName, deletePodName string)
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
	InferGroupUpdating InferGroupStatus = "Updating"
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

// DeleteInferGroup delete inferGroup in runningPodOfInferGroup map
func (s *store) DeleteInferGroup(modelInferName types.NamespacedName, inferGroupName string) error {
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

// AddInferGroup add inferGroup item of one modelInfer
func (s *store) AddInferGroup(modelInferName types.NamespacedName, idx int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newGroup := InferGroup{
		Name:      utils.GenerateInferGroupName(modelInferName.Name, idx),
		Namespace: modelInferName.Namespace,
		Status:    InferGroupCreating,
	}
	group := s.inferGroup[modelInferName]
	if idx < 0 || idx > len(group) {
		return fmt.Errorf("infer group index %d out of range", idx)
	}
	s.inferGroup[modelInferName] = slices.Insert(group, idx, newGroup)
	return nil
}

// AddRunningPodToInferGroup add inferGroup in runningPodOfInferGroup map
func (s *store) AddRunningPodToInferGroup(inferGroupName types.NamespacedName, runningPodName string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.runningPodOfInferGroups[inferGroupName] = append(s.runningPodOfInferGroups[inferGroupName], runningPodName)
}

// DeleteRunningPodFromInferGroup delete runningPod in map
func (s *store) DeleteRunningPodFromInferGroup(inferGroupName types.NamespacedName, deletePodName string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.runningPodOfInferGroups[inferGroupName] = slices.DeleteFunc(s.runningPodOfInferGroups[inferGroupName], func(podName string) bool {
		return podName == deletePodName
	})
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
