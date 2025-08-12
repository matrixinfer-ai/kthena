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
	GetRoleList(modelInferName types.NamespacedName, groupName, roleName string) ([]Role, error)
	GetRoleStatus(modelInferName types.NamespacedName, groupName, roleName, roleID string) RoleStatus
	UpdateRoleStatus(modelInferName types.NamespacedName, groupName, roleName, roleID string, status RoleStatus) error
	DeleteRole(modelInferName types.NamespacedName, groupName, roleName, roleID string)
	DeleteModelInfer(modelInferName types.NamespacedName)
	DeleteInferGroup(modelInferName types.NamespacedName, inferGroupName string)
	AddInferGroup(modelInferName types.NamespacedName, idx int, revision string)
	AddRole(modelInferName types.NamespacedName, groupName, roleName, roleID, revision string)
	AddRunningPodToInferGroup(modelInferName types.NamespacedName, inferGroupName, pod, revision, roleName, roleID string)
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
	roles       map[string]map[string]*Role // roleName -> roleID -> *Role, like prefill -> prefill-0 -> *Role
}

type Role struct {
	Name     string
	Revision string
	Status   RoleStatus
}

type InferGroupStatus string

const (
	InferGroupRunning  InferGroupStatus = "Running"
	InferGroupCreating InferGroupStatus = "Creating"
	InferGroupDeleting InferGroupStatus = "Deleting"
	InferGroupScaling  InferGroupStatus = "Scaling"
	// InferGroupUpdating InferGroupStatus = "Updating"
	InferGroupNotFound InferGroupStatus = "NotFound"
)

type RoleStatus string

const (
	RoleCreating RoleStatus = "Creating"
	RoleDeleting RoleStatus = "Deleting"
	RoleNotFound RoleStatus = "NotFound"
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

// GetRoleList returns the list of roles and errors
func (s *store) GetRoleList(modelInferName types.NamespacedName, groupName, roleName string) ([]Role, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		return nil, fmt.Errorf("cannot list inferGroup of modelInfer %s", modelInferName.Name)
	}
	inferGroup, ok := inferGroups[groupName]
	if !ok {
		return nil, ErrInferGroupNotFound
	}
	roleMap, ok := inferGroup.roles[roleName]
	if !ok {
		// If the roleName does not exist, return an empty list instead of an error
		return []Role{}, nil
	}
	//
	//Convert roles in map to a slice
	roleSlice := make([]Role, 0, len(roleMap))
	for _, role := range roleMap {
		roleSlice = append(roleSlice, *role)
	}

	slices.SortFunc(roleSlice, func(a, b Role) int {
		return strings.Compare(a.Name, b.Name)
	})

	return roleSlice, nil
}

// UpdateRoleStatus updates the status of a specific role
func (s *store) UpdateRoleStatus(modelInferName types.NamespacedName, groupName, roleName, roleID string, status RoleStatus) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		return fmt.Errorf("cannot find modelInfer %s", modelInferName.Name)
	}

	inferGroup, ok := inferGroups[groupName]
	if !ok {
		return ErrInferGroupNotFound
	}

	roleMap, ok := inferGroup.roles[roleName]
	if !ok {
		return fmt.Errorf("roleName %s not found in group %s", roleName, groupName)
	}

	role, ok := roleMap[roleID]
	if !ok {
		return fmt.Errorf("role %s not found in roleName %s of group %s", roleID, roleName, groupName)
	}

	role.Status = status
	return nil
}

// GetRoleStatus returns the status of a specific role
func (s *store) GetRoleStatus(modelInferName types.NamespacedName, groupName, roleName, roleID string) RoleStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if inferGroups, exist := s.inferGroup[modelInferName]; exist {
		if group, ok := inferGroups[groupName]; ok {
			if roleMap, exists := group.roles[roleName]; exists {
				if role, found := roleMap[roleID]; found {
					return role.Status
				}
			}
		}
	}
	return RoleNotFound
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

// DeleteRole deletes a specific role from an infer group
func (s *store) DeleteRole(modelInferName types.NamespacedName, groupName, roleName, roleID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	inferGroups, ok := s.inferGroup[modelInferName]
	if !ok {
		return
	}

	inferGroup, ok := inferGroups[groupName]
	if !ok {
		return
	}

	roleMap, ok := inferGroup.roles[roleName]
	if !ok {
		return
	}
	delete(roleMap, roleID)
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
		roles:       make(map[string]map[string]*Role),
	}

	if _, ok := s.inferGroup[modelInferName]; !ok {
		s.inferGroup[modelInferName] = make(map[string]*InferGroup)
	}
	s.inferGroup[modelInferName][newGroup.Name] = newGroup
}

// AddRole adds a new role to an infer group
func (s *store) AddRole(modelInferName types.NamespacedName, groupName, roleName, roleID, revision string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newRole := &Role{
		Name:     roleID,
		Status:   RoleCreating,
		Revision: revision,
	}

	if _, ok := s.inferGroup[modelInferName]; !ok {
		s.inferGroup[modelInferName] = make(map[string]*InferGroup)
	}

	group, ok := s.inferGroup[modelInferName][groupName]
	if !ok {
		group = &InferGroup{
			Name:        groupName,
			runningPods: make(map[string]struct{}),
			Status:      InferGroupCreating,
			Revision:    revision,
			roles:       make(map[string]map[string]*Role),
		}
		s.inferGroup[modelInferName][groupName] = group
	}

	if _, exists := group.roles[roleName]; !exists {
		group.roles[roleName] = make(map[string]*Role)
	}

	group.roles[roleName][roleID] = newRole
}

// AddRunningPodToInferGroup add inferGroup in runningPodOfInferGroup map
func (s *store) AddRunningPodToInferGroup(modelInferName types.NamespacedName, inferGroupName, runningPodName, revision, roleName, roleID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	role := &Role{
		Name:     roleID,
		Status:   RoleCreating,
		Revision: revision,
	}
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
			roles:       make(map[string]map[string]*Role),
		}
		group.roles[roleName] = make(map[string]*Role)
		group.roles[roleName][roleID] = role

		s.inferGroup[modelInferName][inferGroupName] = group
		return
	}

	group.runningPods[runningPodName] = struct{}{} // runningPods map has been initialized during AddInferGroup.

	// Check if roleName exists, and initialize it if not
	if _, ok = group.roles[roleName]; !ok {
		group.roles[roleName] = make(map[string]*Role)
	}

	if _, ok = group.roles[roleName][roleID]; !ok {
		group.roles[roleName][roleID] = role
	}
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
