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
	"errors"
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
	GetInferGroup(modelInferName types.NamespacedName, inferGroupName string) *InferGroup
	GetRunningPodNumByInferGroup(modelInferName types.NamespacedName, inferGroupName string) (int, error)
	GetInferGroupStatus(modelInferName types.NamespacedName, inferGroupName string) InferGroupStatus
	DeleteModelInfer(modelInferName types.NamespacedName)
	DeleteInferGroup(modelInferName types.NamespacedName, inferGroupName string)
	AddInferGroup(modelInferName types.NamespacedName, idx int, revision string)
	AddRunningPodToInferGroup(modelInferName types.NamespacedName, inferGroupName string, pod string, revision string)
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
	Revision    string
	Status      InferGroupStatus
}

type InferGroupStatus string

const (
	InferGroupRunning  InferGroupStatus = "Running"
	InferGroupCreating InferGroupStatus = "Creating"
	InferGroupDeleting InferGroupStatus = "Deleting"
	InferGroupError    InferGroupStatus = "Error"
	// InferGroupUpdating InferGroupStatus = "Updating"
	InferGroupNotFound InferGroupStatus = "NotFound"
)

var ErrInferGroupNotFound = errors.New("infer group not found")

func New() Store {
	return &store{
		inferGroup: make(map[types.NamespacedName]map[string]*InferGroup),
	}
}

// GetInferGroupByModelInfer returns the list of inferGroups and errors
func (s *store) GetInferGroupByModelInfer(modelInferName types.NamespacedName) ([]InferGroup, error) {
	s.mutex.RLock()
	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		s.mutex.RUnlock()
		return nil, ErrInferGroupNotFound
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

// GetRunningPodNumByInferGroup returns the number of running pods and errors
func (s *store) GetRunningPodNumByInferGroup(modelInferName types.NamespacedName, inferGroupName string) (int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	groups, ok := s.inferGroup[modelInferName]
	if !ok {
		return 0, fmt.Errorf("model infer %s not found", modelInferName)
	}

	group, ok := groups[inferGroupName]
	if !ok {
		return 0, nil
	}
	return len(group.runningPods), nil
}

// GetInferGroup returns the GetInferGroup
func (s *store) GetInferGroup(modelInferName types.NamespacedName, inferGroupName string) *InferGroup {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	groups, ok := s.inferGroup[modelInferName]
	if !ok {
		return nil
	}

	return groups[inferGroupName]
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
func (s *store) DeleteModelInfer(modelInferName types.NamespacedName) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.inferGroup, modelInferName)
}

// DeleteInferGroup delete inferGroup in map
func (s *store) DeleteInferGroup(modelInferName types.NamespacedName, inferGroupName string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if inferGroups, ok := s.inferGroup[modelInferName]; ok {
		delete(inferGroups, inferGroupName)
	}
}

// AddInferGroup add inferGroup item of one modelInfer
func (s *store) AddInferGroup(modelInferName types.NamespacedName, idx int, revision string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newGroup := &InferGroup{
		Name:        utils.GenerateInferGroupName(modelInferName.Name, idx),
		runningPods: make(map[string]struct{}),
		Status:      InferGroupCreating,
		Revision:    revision,
	}

	if _, ok := s.inferGroup[modelInferName]; !ok {
		s.inferGroup[modelInferName] = make(map[string]*InferGroup)
	}
	s.inferGroup[modelInferName][newGroup.Name] = newGroup
}

// AddRunningPodToInferGroup add inferGroup in runningPodOfInferGroup map
func (s *store) AddRunningPodToInferGroup(modelInferName types.NamespacedName, inferGroupName string, runningPodName string, revision string) {
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
			Revision:    revision,
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
