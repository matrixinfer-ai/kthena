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

	// 1.Delect exist key
	s.DeleteModelInfer(key1)
	_, exists := s.inferGroup[key1]
	assert.False(t, exists, "key1 should be deleted")

	// 2. Delect not exist key
	nonExistKey := types.NamespacedName{Namespace: "ns3", Name: "model3"}
	s.DeleteModelInfer(nonExistKey)
	// Delect another key
	_, exists = s.inferGroup[key2]
	assert.True(t, exists, "key2 should still exist")

	// 3. Delect same key twice
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

	// 1. Delect exist inferGroupName
	s.DeleteInferGroup(key1, "groupA")
	_, exists := s.inferGroup[key1]["groupA"]
	assert.False(t, exists, "groupA should be deleted from key1")

	// 2. Delect not exist inferGroupName
	s.DeleteInferGroup(key1, "groupX")
	_, exists = s.inferGroup[key1]["groupB"]
	assert.True(t, exists, "groupB should still exist in key1")

	// 3. modelInferName not exist
	nonExistKey := types.NamespacedName{Namespace: "ns3", Name: "model3"}
	s.DeleteInferGroup(nonExistKey, "groupY")

	// 4. Delect same inferGroupName twice
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
