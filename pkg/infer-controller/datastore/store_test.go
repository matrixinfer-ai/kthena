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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	"matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
)

func TestDeleteModelInfer(t *testing.T) {
	key1 := types.NamespacedName{Namespace: "ns1", Name: "model1"}
	key2 := types.NamespacedName{Namespace: "ns2", Name: "model2"}

	s := &store{
		mutex: sync.RWMutex{},
		inferGroup: map[types.NamespacedName]map[string]*InferGroup{
			key1: {},
			key2: {},
		},
	}

	// 1.Delete exist key
	s.DeleteModelInfer(key1)
	_, exists := s.inferGroup[key1]
	assert.False(t, exists, "key1 should be deleted")

	// 2. Delete not exist key
	nonExistKey := types.NamespacedName{Namespace: "ns3", Name: "model3"}
	s.DeleteModelInfer(nonExistKey)
	// Delete another key
	_, exists = s.inferGroup[key2]
	assert.True(t, exists, "key2 should still exist")

	// 3. Delete same key twice
	s.DeleteModelInfer(key2)
	s.DeleteModelInfer(key2)
	_, exists = s.inferGroup[key2]
	assert.False(t, exists, "key2 should be deleted after repeated deletes")
}

func TestDeleteInferGroup(t *testing.T) {
	key1 := types.NamespacedName{Namespace: "ns1", Name: "model1"}
	key2 := types.NamespacedName{Namespace: "ns2", Name: "model2"}

	s := &store{
		mutex: sync.RWMutex{},
		inferGroup: map[types.NamespacedName]map[string]*InferGroup{
			key1: {
				"groupA": &InferGroup{},
				"groupB": &InferGroup{},
			},
			key2: {
				"groupC": &InferGroup{},
			},
		},
	}

	// 1. Delete exist inferGroupName
	s.DeleteInferGroup(key1, "groupA")
	_, exists := s.inferGroup[key1]["groupA"]
	assert.False(t, exists, "groupA should be deleted from key1")

	// 2. Delete not exist inferGroupName
	s.DeleteInferGroup(key1, "groupX")
	_, exists = s.inferGroup[key1]["groupB"]
	assert.True(t, exists, "groupB should still exist in key1")

	// 3. modelInferName not exist
	nonExistKey := types.NamespacedName{Namespace: "ns3", Name: "model3"}
	s.DeleteInferGroup(nonExistKey, "groupY")

	// 4. Delete same inferGroupName twice
	s.DeleteInferGroup(key2, "groupC")
	s.DeleteInferGroup(key2, "groupC")
	_, exists = s.inferGroup[key2]["groupC"]
	assert.False(t, exists, "groupC should be deleted from key2 after repeated deletes")
}

func TestAddInferGroup(t *testing.T) {
	key := types.NamespacedName{Namespace: "ns1", Name: "model1"}

	s := &store{
		mutex:      sync.RWMutex{},
		inferGroup: make(map[types.NamespacedName]map[string]*InferGroup),
	}

	// 1. Add group to an empty modelInfer.
	s.AddInferGroup(key, 0, "9559767")
	groupName := utils.GenerateInferGroupName(key.Name, 0)
	group, exists := s.inferGroup[key][groupName]
	assert.True(t, exists, "group should exist after add")
	assert.Equal(t, groupName, group.Name)
	assert.Equal(t, InferGroupCreating, group.Status)
	assert.NotNil(t, group.runningPods)
	assert.Empty(t, group.runningPods)

	// 2. Adds a group to an existing modelInferName.
	s.AddInferGroup(key, 1, "9559767")
	groupName2 := utils.GenerateInferGroupName(key.Name, 1)
	group2, exists2 := s.inferGroup[key][groupName2]
	assert.True(t, exists2, "second group should exist after add")
	assert.Equal(t, groupName2, group2.Name)

	// 3. Multiple additions of the same idx, group is overwritten
	s.AddInferGroup(key, 1, "9559767")
	group3, exists3 := s.inferGroup[key][groupName2]
	assert.True(t, exists3, "group should still exist after overwrite")
	assert.Equal(t, groupName2, group3.Name)

	// 4. new modelInferName
	key2 := types.NamespacedName{Namespace: "ns2", Name: "model2"}
	s.AddInferGroup(key2, 0, "9559766")
	groupName4 := utils.GenerateInferGroupName(key2.Name, 0)
	group4, exists4 := s.inferGroup[key2][groupName4]
	assert.True(t, exists4, "group for new modelInferName should exist")
	assert.Equal(t, groupName4, group4.Name)
}

func TestGetRoleList(t *testing.T) {
	key := types.NamespacedName{Namespace: "ns1", Name: "model1"}

	s := &store{
		mutex: sync.RWMutex{},
		inferGroup: map[types.NamespacedName]map[string]*InferGroup{
			key: {
				"group0": &InferGroup{
					Name: "group0",
					roles: map[string]map[string]*Role{
						"prefill": {
							"prefill-0": &Role{Name: "prefill-0", Status: RoleCreating},
							"prefill-1": &Role{Name: "prefill-1", Status: RoleCreating},
						},
						"decode": {
							"decode-0": &Role{Name: "decode-0", Status: RoleCreating},
						},
					},
				},
			},
		},
	}

	// 1. Get existing roles
	roles, err := s.GetRoleList(key, "group0", "prefill")
	assert.NoError(t, err)
	assert.Len(t, roles, 2)
	// Check if sorted by name
	assert.Equal(t, "prefill-0", roles[0].Name)
	assert.Equal(t, "prefill-1", roles[1].Name)

	// 2. Get roles with non-existing roleLabel (should return empty list)
	roles, err = s.GetRoleList(key, "group0", "nonexist")
	assert.NoError(t, err)
	assert.Empty(t, roles)

	// 3. Get roles with non-existing modelInfer
	nonExistKey := types.NamespacedName{Namespace: "ns2", Name: "model2"}
	roles, err = s.GetRoleList(nonExistKey, "group0", "prefill")
	assert.Error(t, err)
	assert.Nil(t, roles)

	// 4. Get roles with non-existing group
	roles, err = s.GetRoleList(key, "nonexistgroup", "prefill")
	assert.Error(t, err)
	assert.Equal(t, ErrInferGroupNotFound, err)
	assert.Nil(t, roles)
}

func TestUpdateRoleStatus(t *testing.T) {
	key := types.NamespacedName{Namespace: "ns1", Name: "model1"}

	s := &store{
		mutex: sync.RWMutex{},
		inferGroup: map[types.NamespacedName]map[string]*InferGroup{
			key: {
				"group0": &InferGroup{
					Name: "group0",
					roles: map[string]map[string]*Role{
						"prefill": {
							"prefill-0": &Role{Name: "prefill-0", Status: RoleCreating},
						},
					},
				},
			},
		},
	}

	// 1. Update existing role status
	err := s.UpdateRoleStatus(key, "group0", "prefill", "prefill-0", RoleDeleting)
	assert.NoError(t, err)
	role := s.inferGroup[key]["group0"].roles["prefill"]["prefill-0"]
	assert.Equal(t, RoleDeleting, role.Status)

	// 2. Update non-existing modelInfer
	nonExistKey := types.NamespacedName{Namespace: "ns2", Name: "model2"}
	err = s.UpdateRoleStatus(nonExistKey, "group0", "prefill", "prefill-0", RoleDeleting)
	assert.Error(t, err)

	// 3. Update non-existing group
	err = s.UpdateRoleStatus(key, "nonexistgroup", "prefill", "prefill-0", RoleDeleting)
	assert.Error(t, err)
	assert.Equal(t, ErrInferGroupNotFound, err)

	// 4. Update non-existing roleLabel
	err = s.UpdateRoleStatus(key, "group0", "nonexistrole", "prefill-0", RoleDeleting)
	assert.Error(t, err)

	// 5. Update non-existing roleName
	err = s.UpdateRoleStatus(key, "group0", "prefill", "nonexistname", RoleDeleting)
	assert.Error(t, err)
}

func TestDeleteRole(t *testing.T) {
	key := types.NamespacedName{Namespace: "ns1", Name: "model1"}

	s := &store{
		mutex: sync.RWMutex{},
		inferGroup: map[types.NamespacedName]map[string]*InferGroup{
			key: {
				"group0": &InferGroup{
					Name: "group0",
					roles: map[string]map[string]*Role{
						"prefill": {
							"prefill-0": &Role{Name: "prefill-0"},
							"prefill-1": &Role{Name: "prefill-1"},
						},
					},
				},
			},
		},
	}

	// 1. Delete existing role
	s.DeleteRole(key, "group0", "prefill", "prefill-0")
	_, exists := s.inferGroup[key]["group0"].roles["prefill"]["prefill-0"]
	assert.False(t, exists, "role should be deleted")
	_, exists = s.inferGroup[key]["group0"].roles["prefill"]["prefill-1"]
	assert.True(t, exists, "other role should still exist")

	// 2. Delete non-existing role (should not panic)
	s.DeleteRole(key, "group0", "prefill", "nonexistrole")

	// 3. Delete role with non-existing roleLabel (should not panic)
	s.DeleteRole(key, "group0", "nonexistlabel", "prefill-1")

	// 4. Delete role with non-existing group (should not panic)
	s.DeleteRole(key, "nonexistgroup", "prefill", "prefill-1")

	// 5. Delete role with non-existing modelInfer (should not panic)
	nonExistKey := types.NamespacedName{Namespace: "ns2", Name: "model2"}
	s.DeleteRole(nonExistKey, "group0", "prefill", "prefill-1")
}

func TestAddRole(t *testing.T) {
	key := types.NamespacedName{Namespace: "ns1", Name: "model1"}

	s := &store{
		mutex:      sync.RWMutex{},
		inferGroup: make(map[types.NamespacedName]map[string]*InferGroup),
	}

	// 1. Add role to non-existing modelInfer and group
	s.AddRole(key, "group0", "prefill", "prefill-0", "revision1")
	role, exists := s.inferGroup[key]["group0"].roles["prefill"]["prefill-0"]
	assert.True(t, exists, "role should be created")
	assert.Equal(t, "prefill-0", role.Name)
	assert.Equal(t, RoleCreating, role.Status)
	assert.Equal(t, "revision1", role.Revision)

	// 2. Add another role to existing group
	s.AddRole(key, "group0", "prefill", "prefill-1", "revision2")
	role2, exists2 := s.inferGroup[key]["group0"].roles["prefill"]["prefill-1"]
	assert.True(t, exists2, "second role should be created")
	assert.Equal(t, "prefill-1", role2.Name)

	// 3. Add role with different roleLabel
	s.AddRole(key, "group0", "decode", "decode-0", "revision3")
	_, exists3 := s.inferGroup[key]["group0"].roles["decode"]["decode-0"]
	assert.True(t, exists3, "role with different label should be created")

	// 4. Overwrite existing role
	s.AddRole(key, "group0", "prefill", "prefill-0", "revision4")
	role4 := s.inferGroup[key]["group0"].roles["prefill"]["prefill-0"]
	assert.Equal(t, "revision4", role4.Revision, "role should be overwritten")
}
