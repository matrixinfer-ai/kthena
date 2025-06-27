package datastore

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/types"

	"matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
)

// Store is an interface for storing and retrieving data
type Store interface {
	GetInferGroupByModelInfer(modelInferName types.NamespacedName) ([]InferGroup, error)
	GetRunningPodByInferGroup(modelInferName types.NamespacedName, inferGroupName string) (map[string]struct{}, error)
	GetInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string) InferGroupStatus
	DeleteModelInfer(modelInferName types.NamespacedName) error
	DeleteInferGroup(modelInferName types.NamespacedName, inferGroupName string) error
	AddInferGroup(modelInferName types.NamespacedName, idx int) error
	AddRunningPodToInferGroup(modelInferName types.NamespacedName, inferGroupName string, pod string)
	DeleteRunningPodFromInferGroup(modelInferName types.NamespacedName, inferGroupName string, pod string)
	UpdateInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string, Status InferGroupStatus) error
}

type store struct {
	mutex sync.RWMutex

	// inferGroup is a map of model infer names to their infer groups
	// modelInfer -> group name-> InferGroup
	inferGroup map[types.NamespacedName]map[string]*InferGroup
}

type InferGroup struct {
	Name        string
	runningPods map[string]struct{} // Map of pod names in this infer group
	Status      InferGroupStatus
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
		inferGroup: make(map[types.NamespacedName]map[string]*InferGroup),
	}, nil
}

// GetInferGroupByModelInfer returns the list of inferGroups and errors
func (s *store) GetInferGroupByModelInfer(modelInferName types.NamespacedName) ([]InferGroup, error) {
	s.mutex.RLock()
	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		s.mutex.RUnlock()
		return nil, nil
	}
	// sort inferGroups by name
	inferGroupsSlice := make([]InferGroup, 0, len(inferGroups))
	for _, inferGroup := range inferGroups {
		// This is o clone to prevent r/w conflict later
		inferGroupsSlice = append(inferGroupsSlice, *inferGroup)
	}
	s.mutex.RUnlock()

	slices.SortFunc(inferGroupsSlice, func(a, b InferGroup) int {
		return strings.Compare(a.Name, b.Name)
	})

	return inferGroupsSlice, nil
}

// GetRunningPodByInferGroup returns the map of running pods name and errors
func (s *store) GetRunningPodByInferGroup(modelInferName types.NamespacedName, inferGroupName string) (map[string]struct{}, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	groups, ok := s.inferGroup[modelInferName]
	if !ok {
		return nil, fmt.Errorf("model infer %s not found", modelInferName)
	}

	group, ok := groups[inferGroupName]
	if !ok {
		return nil, nil
	}
	return group.runningPods, nil
}

// GetInferGroupStatus returns the status of inferGroup
func (s *store) GetInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string) InferGroupStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if inferGroups, exist := s.inferGroup[modelInferName]; exist {
		if group, ok := inferGroups[inferGroupName]; ok {
			return group.Status
		}
	}
	return InferGroupNotFound
}

// DeleteModelInfer delete modelInfer in inferGroup map
func (s *store) DeleteModelInfer(modelInferName types.NamespacedName) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.inferGroup, modelInferName)
	return nil
}

// DeleteInferGroup delete inferGroup in map
func (s *store) DeleteInferGroup(modelInferName types.NamespacedName, inferGroupName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if inferGroups, ok := s.inferGroup[modelInferName]; ok {
		delete(inferGroups, inferGroupName)
	}

	return nil
}

// AddInferGroup add inferGroup item of one modelInfer
func (s *store) AddInferGroup(modelInferName types.NamespacedName, idx int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newGroup := &InferGroup{
		Name:        utils.GenerateInferGroupName(modelInferName.Name, idx),
		runningPods: make(map[string]struct{}),
		Status:      InferGroupCreating,
	}

	if _, ok := s.inferGroup[modelInferName]; !ok {
		s.inferGroup[modelInferName] = make(map[string]*InferGroup)
	}
	s.inferGroup[modelInferName][newGroup.Name] = newGroup
	return nil
}

// AddRunningPodToInferGroup add inferGroup in runningPodOfInferGroup map
func (s *store) AddRunningPodToInferGroup(modelInferName types.NamespacedName, inferGroupName string, runningPodName string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, ok := s.inferGroup[modelInferName]; !ok {
		// If modelInferName not exist, create a new one
		s.inferGroup[modelInferName] = make(map[string]*InferGroup)
	}

	group, ok := s.inferGroup[modelInferName][inferGroupName]
	if !ok {
		// If inferGroupName not exist, create a new one
		group = &InferGroup{
			Name:        inferGroupName,
			runningPods: map[string]struct{}{runningPodName: {}},
			Status:      InferGroupCreating,
		}
		s.inferGroup[modelInferName][inferGroupName] = group
		return
	}

	group.runningPods[runningPodName] = struct{}{} // runningPods map has been initialized during AddInferGroup.
}

// DeleteRunningPodFromInferGroup delete runningPod in map
func (s *store) DeleteRunningPodFromInferGroup(modelInferName types.NamespacedName, inferGroupName string, pod string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if inferGroups, exist := s.inferGroup[modelInferName]; exist {
		if group, ok := inferGroups[inferGroupName]; ok {
			delete(group.runningPods, pod)
		}
	}
}

// UpdateInferGroupStatus update status of one inferGroup
func (s *store) UpdateInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string, status InferGroupStatus) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		return fmt.Errorf("failed to find modelInfer %s", modelInferName.Namespace+"/"+modelInferName.Name)
	}
	if group, ok := inferGroups[inferGroupName]; ok {
		group.Status = status
		inferGroups[inferGroupName] = group
	} else {
		return fmt.Errorf("failed to find inferGroup %s in modelInfer %s", inferGroupName, modelInferName.Namespace+"/"+modelInferName.Name)
	}
	return nil
}
