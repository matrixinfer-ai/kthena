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

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"

	matrixinferfake "matrixinfer.ai/matrixinfer/client-go/clientset/versioned/fake"
	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
)

type resourceSpec struct {
	name   string
	labels map[string]string
}

func TestIsInferGroupOutdated(t *testing.T) {
	ns := "test-ns"
	groupName := "test-group"
	group := datastore.InferGroup{Name: groupName}
	mi := &workloadv1alpha1.ModelInfer{ObjectMeta: metav1.ObjectMeta{Namespace: ns}}
	newHash := "hash123"

	kubeClient := kubefake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	stopCh := make(chan struct{})
	defer close(stopCh)
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	c := &ModelInferController{
		podsLister: podInformer.Lister(),
	}

	cases := []struct {
		name string
		pods []resourceSpec
		want bool
	}{
		{
			name: "no pods",
			pods: nil,
			want: false,
		},
		{
			name: "no revision label",
			pods: []resourceSpec{
				{name: "pod1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
			},
			want: true,
		},
		{
			name: "revision not match",
			pods: []resourceSpec{
				{name: "pod2", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName, workloadv1alpha1.RevisionLabelKey: "oldhash"}},
			},
			want: true,
		},
		{
			name: "revision match",
			pods: []resourceSpec{
				{name: "pod3", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName, workloadv1alpha1.RevisionLabelKey: newHash}},
			},
			want: false,
		},
	}

	indexer := podInformer.Informer().GetIndexer()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// clean indexer
			for _, obj := range indexer.List() {
				err := indexer.Delete(obj)
				assert.NoError(t, err)
			}
			for _, p := range tc.pods {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      p.name,
						Labels:    p.labels,
					},
				}
				err := indexer.Add(pod)
				assert.NoError(t, err)
			}
			got := c.isInferGroupOutdated(group, mi.Namespace, newHash)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCheckInferGroupReady(t *testing.T) {
	ns := "default"
	groupName := "test-group"
	newHash := "hash123"
	roleLabel := "prefill"
	roleName := "prefill-0"

	kubeClient := kubefake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	store := datastore.New()
	// build controller
	controller := &ModelInferController{
		podsLister: podInformer.Lister(),
		store:      store,
	}
	stop := make(chan struct{})
	defer close(stop)
	kubeInformerFactory.Start(stop)
	kubeInformerFactory.WaitForCacheSync(stop)

	// build ModelInfer
	var expectedPodNum int32 = 2
	mi := &workloadv1alpha1.ModelInfer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "test-mi",
		},
		Spec: workloadv1alpha1.ModelInferSpec{
			Template: workloadv1alpha1.InferGroup{
				Roles: []workloadv1alpha1.Role{
					{
						Replicas: &expectedPodNum,
					},
				},
			},
		},
	}

	indexer := podInformer.Informer().GetIndexer()
	// Add 2 pods with labels matching group
	for i := 0; i < int(expectedPodNum); i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      fmt.Sprintf("pod-%d", i),
				Labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
				},
			},
		}
		err := indexer.Add(pod)
		assert.NoError(t, err)
		store.AddRunningPodToInferGroup(utils.GetNamespaceName(mi), groupName, pod.Name, newHash, roleLabel, roleName)
	}

	// Waiting for pod cache to sync
	sync := waitForObjectInCache(t, 2*time.Second, func() bool {
		pods, _ := controller.podsLister.Pods(ns).List(labels.SelectorFromSet(map[string]string{
			workloadv1alpha1.GroupNameLabelKey: groupName,
		}))
		return len(pods) == int(expectedPodNum)
	})
	assert.True(t, sync, "Pods should be found in cache")

	ok, err := controller.checkInferGroupReady(mi, groupName)
	assert.NoError(t, err)
	assert.True(t, ok)

	// case2: Pod quantity mismatch
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "pod-1",
			Labels: map[string]string{
				workloadv1alpha1.GroupNameLabelKey: groupName,
			},
		},
	}
	err = indexer.Delete(pod)
	assert.NoError(t, err)
	sync = waitForObjectInCache(t, 2*time.Second, func() bool {
		pods, _ := controller.podsLister.Pods(ns).List(labels.SelectorFromSet(map[string]string{
			workloadv1alpha1.GroupNameLabelKey: groupName,
		}))
		return len(pods) == int(expectedPodNum)-1
	})
	assert.True(t, sync, "Pods should be found in cache after deletion")
	store.DeleteRunningPodFromInferGroup(utils.GetNamespaceName(mi), groupName, "pod-1")

	ok, err = controller.checkInferGroupReady(mi, groupName)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestIsInferGroupDeleted(t *testing.T) {
	ns := "default"
	groupName := "test-mi-0"
	otherGroupName := "other-group"

	kubeClient := kubefake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	serviceInformer := kubeInformerFactory.Core().V1().Services()

	store := datastore.New()
	controller := &ModelInferController{
		podsLister:     podInformer.Lister(),
		servicesLister: serviceInformer.Lister(),
		store:          store,
	}

	stop := make(chan struct{})
	defer close(stop)
	kubeInformerFactory.Start(stop)
	kubeInformerFactory.WaitForCacheSync(stop)

	mi := &workloadv1alpha1.ModelInfer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "test-mi",
		},
	}

	cases := []struct {
		name             string
		pods             []resourceSpec
		services         []resourceSpec
		inferGroupStatus datastore.InferGroupStatus
		want             bool
	}{
		{
			name:             "inferGroup status is not Deleting - should return false",
			pods:             nil,
			services:         nil,
			inferGroupStatus: datastore.InferGroupCreating,
			want:             false,
		},
		{
			name:             "inferGroup status is Deleting - no resources - should return true",
			pods:             nil,
			services:         nil,
			inferGroupStatus: datastore.InferGroupDeleting,
			want:             true,
		},
		{
			name: "inferGroup status is Deleting - target group pods exist - should return false",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
			},
			services:         nil,
			inferGroupStatus: datastore.InferGroupDeleting,
			want:             false,
		},
		{
			name: "inferGroup status is Deleting - target group services exist - should return false",
			pods: nil,
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
			},
			inferGroupStatus: datastore.InferGroupDeleting,
			want:             false,
		},
		{
			name: "inferGroup status is Deleting - both target group resources exist - should return false",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
			},
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
			},
			inferGroupStatus: datastore.InferGroupDeleting,
			want:             false,
		},
		{
			name: "inferGroup status is Deleting - only other group resources exist - should return true",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: otherGroupName}},
			},
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: otherGroupName}},
			},
			inferGroupStatus: datastore.InferGroupDeleting,
			want:             true,
		},
		{
			name: "inferGroup status is Deleting - mixed group resources - target group exists - should return false",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
				{name: "pod-2", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: otherGroupName}},
			},
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: otherGroupName}},
			},
			inferGroupStatus: datastore.InferGroupDeleting,
			want:             false,
		},
		{
			name: "inferGroup status is Deleting - multiple target group resources - should return false",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
				{name: "pod-2", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
			},
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
				{name: "svc-2", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
			},
			inferGroupStatus: datastore.InferGroupDeleting,
			want:             false,
		},
	}

	podIndexer := podInformer.Informer().GetIndexer()
	serviceIndexer := serviceInformer.Informer().GetIndexer()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean indexers before each test
			for _, obj := range podIndexer.List() {
				err := podIndexer.Delete(obj)
				assert.NoError(t, err)
			}
			for _, obj := range serviceIndexer.List() {
				err := serviceIndexer.Delete(obj)
				assert.NoError(t, err)
			}

			store.AddInferGroup(utils.GetNamespaceName(mi), 0, "test-revision")
			err := store.UpdateInferGroupStatus(utils.GetNamespaceName(mi), groupName, tc.inferGroupStatus)
			assert.NoError(t, err)

			// Add test pods
			for _, p := range tc.pods {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      p.name,
						Labels:    p.labels,
					},
				}
				err := podIndexer.Add(pod)
				assert.NoError(t, err)
			}

			// Add test services
			for _, s := range tc.services {
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      s.name,
						Labels:    s.labels,
					},
				}
				err := serviceIndexer.Add(service)
				assert.NoError(t, err)
			}

			// Wait for cache to sync
			sync := waitForObjectInCache(t, 2*time.Second, func() bool {
				pods, _ := controller.podsLister.Pods(ns).List(labels.Everything())
				services, _ := controller.servicesLister.Services(ns).List(labels.Everything())
				return len(pods) == len(tc.pods) && len(services) == len(tc.services)
			})
			assert.True(t, sync, "Resources should be synced in cache")

			// Test the function
			got := controller.isInferGroupDeleted(mi, groupName)
			assert.Equal(t, tc.want, got, "isInferGroupDeleted result should match expected")

			store.DeleteInferGroup(utils.GetNamespaceName(mi), groupName)
		})
	}
}

func TestIsRoleDeleted(t *testing.T) {
	ns := "default"
	groupName := "test-mi-0"
	roleName := "prefill"
	roleID := "prefill-0"

	otherGroupName := "other-group"
	otherRoleName := "decode"
	otherRoleID := "decode-0"

	kubeClient := kubefake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	serviceInformer := kubeInformerFactory.Core().V1().Services()

	store := datastore.New()
	controller := &ModelInferController{
		podsLister:     podInformer.Lister(),
		servicesLister: serviceInformer.Lister(),
		store:          store,
	}

	stop := make(chan struct{})
	defer close(stop)
	kubeInformerFactory.Start(stop)
	kubeInformerFactory.WaitForCacheSync(stop)

	mi := &workloadv1alpha1.ModelInfer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "test-mi",
		},
	}

	cases := []struct {
		name       string
		pods       []resourceSpec
		services   []resourceSpec
		roleStatus datastore.RoleStatus
		want       bool
	}{
		{
			name:       "role status is not Deleting - should return false",
			pods:       nil,
			services:   nil,
			roleStatus: datastore.RoleCreating,
			want:       false,
		},
		{
			name:       "role status is Deleting - no resources - should return true",
			pods:       nil,
			services:   nil,
			roleStatus: datastore.RoleDeleting,
			want:       true,
		},
		{
			name: "role status is Deleting - target role pods exist - should return false",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
			},
			services:   nil,
			roleStatus: datastore.RoleDeleting,
			want:       false,
		},
		{
			name: "role status is Deleting - target role services exist - should return false",
			pods: nil,
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
			},
			roleStatus: datastore.RoleDeleting,
			want:       false,
		},
		{
			name: "role status is Deleting - both target role resources exist - should return false",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
			},
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
			},
			roleStatus: datastore.RoleDeleting,
			want:       false,
		},
		{
			name: "role status is Deleting - only other group resources exist - should return true",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: otherGroupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
			},
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: otherGroupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
			},
			roleStatus: datastore.RoleDeleting,
			want:       true,
		},
		{
			name: "role status is Deleting - only other role resources exist - should return true",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      otherRoleName,
					workloadv1alpha1.RoleIDKey:         otherRoleID,
				}},
			},
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      otherRoleName,
					workloadv1alpha1.RoleIDKey:         otherRoleID,
				}},
			},
			roleStatus: datastore.RoleDeleting,
			want:       true,
		},
		{
			name: "role status is Deleting - same group and roleName but different roleID - should return true",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         "prefill-1", // different roleID
				}},
			},
			services:   nil,
			roleStatus: datastore.RoleDeleting,
			want:       true,
		},
		{
			name: "role status is Deleting - mixed resources - target role exists - should return false",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
				{name: "pod-2", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      otherRoleName,
					workloadv1alpha1.RoleIDKey:         otherRoleID,
				}},
			},
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: otherGroupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
			},
			roleStatus: datastore.RoleDeleting,
			want:       false,
		},
		{
			name: "role status is Deleting - multiple target role resources - should return false",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
				{name: "pod-2", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
			},
			services: []resourceSpec{
				{name: "svc-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
				{name: "svc-2", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					workloadv1alpha1.RoleIDKey:         roleID,
				}},
			},
			roleStatus: datastore.RoleDeleting,
			want:       false,
		},
		{
			name: "role status is Deleting - incomplete label matching - missing RoleIDKey - should return true",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					workloadv1alpha1.RoleLabelKey:      roleName,
					// missing RoleIDKey
				}},
			},
			services:   nil,
			roleStatus: datastore.RoleDeleting,
			want:       true,
		},
		{
			name: "role status is Deleting - incomplete label matching - missing RoleLabelKey - should return true",
			pods: []resourceSpec{
				{name: "pod-1", labels: map[string]string{
					workloadv1alpha1.GroupNameLabelKey: groupName,
					// missing RoleLabelKey
					workloadv1alpha1.RoleIDKey: roleID,
				}},
			},
			services:   nil,
			roleStatus: datastore.RoleDeleting,
			want:       true,
		},
	}

	podIndexer := podInformer.Informer().GetIndexer()
	serviceIndexer := serviceInformer.Informer().GetIndexer()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean indexers before each test
			for _, obj := range podIndexer.List() {
				err := podIndexer.Delete(obj)
				assert.NoError(t, err)
			}
			for _, obj := range serviceIndexer.List() {
				err := serviceIndexer.Delete(obj)
				assert.NoError(t, err)
			}

			store.AddRole(utils.GetNamespaceName(mi), groupName, roleName, roleID, "test-revision")
			err := store.UpdateRoleStatus(utils.GetNamespaceName(mi), groupName, roleName, roleID, tc.roleStatus)
			assert.NoError(t, err)

			// Add test pods
			for _, p := range tc.pods {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      p.name,
						Labels:    p.labels,
					},
				}
				err := podIndexer.Add(pod)
				assert.NoError(t, err)
			}

			// Add test services
			for _, s := range tc.services {
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      s.name,
						Labels:    s.labels,
					},
				}
				err := serviceIndexer.Add(service)
				assert.NoError(t, err)
			}

			// Wait for cache to sync
			sync := waitForObjectInCache(t, 2*time.Second, func() bool {
				pods, _ := controller.podsLister.Pods(ns).List(labels.Everything())
				services, _ := controller.servicesLister.Services(ns).List(labels.Everything())
				return len(pods) == len(tc.pods) && len(services) == len(tc.services)
			})
			assert.True(t, sync, "Resources should be synced in cache")

			// Test the function
			got := controller.isRoleDeleted(mi, groupName, roleName, roleID)
			assert.Equal(t, tc.want, got, "isRoleDeleted result should match expected")

			store.DeleteInferGroup(utils.GetNamespaceName(mi), groupName)
		})
	}
}

func TestModelInferController_ModelInferLifecycle(t *testing.T) {
	// Create fake clients
	kubeClient := kubefake.NewSimpleClientset()
	matrixinferClient := matrixinferfake.NewSimpleClientset()

	// Create informer factories
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	matrixinferInformerFactory := informersv1alpha1.NewSharedInformerFactory(matrixinferClient, 0)

	// Create controller
	controller := NewModelInferController(kubeClient, matrixinferClient)

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(context.Background(), 5)

	// Start informers
	matrixinferInformerFactory.Start(stop)
	kubeInformerFactory.Start(stop)

	// Wait for cache sync
	cache.WaitForCacheSync(stop,
		controller.modelInfersInformer.HasSynced,
		controller.podsInformer.HasSynced,
		controller.servicesInformer.HasSynced,
	)

	// Test Case 1: ModelInfer Creation
	t.Run("ModelInferCreate", func(t *testing.T) {
		mi := createStandardModelInfer("test-mi", 2, 3)
		// Add ModelInfer to fake client
		_, err := matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Create(
			context.Background(), mi, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Wait for object to be available in cache
		found := waitForObjectInCache(t, 2*time.Second, func() bool {
			_, err := controller.modelInfersLister.ModelInfers("default").Get("test-mi")
			return err == nil
		})
		assert.True(t, found, "ModelInfer should be found in cache after creation")

		// Simulate controller processing the creation
		err = controller.syncModelInfer(context.Background(), "default/test-mi")
		assert.NoError(t, err)

		// Verify InferGroups were created in store
		verifyInferGroups(t, controller, mi, 2)
		// Verify each InferGroup has correct roles
		verifyRoles(t, controller, mi, 2)
		// Verify each InferGroup has correct pods
		verifyPodCount(t, controller, mi, 2)
	})

	// Test Case 2: ModelInfer Scale Up
	t.Run("ModelInferScaleUp", func(t *testing.T) {
		mi := createStandardModelInfer("test-mi-scale-up", 1, 2)
		// Create initial ModelInfer
		_, err := matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Create(
			context.Background(), mi, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Wait for object to be available in cache
		found := waitForObjectInCache(t, 2*time.Second, func() bool {
			_, err := controller.modelInfersLister.ModelInfers("default").Get("test-mi-scale-up")
			return err == nil
		})
		assert.True(t, found, "ModelInfer should be found in cache after creation")

		// Process initial creation
		err = controller.syncModelInfer(context.Background(), "default/test-mi-scale-up")
		assert.NoError(t, err)

		// Verify InferGroups initial state
		verifyInferGroups(t, controller, mi, 1)

		// Update ModelInfer to scale up
		updatedMI := mi.DeepCopy()
		updatedMI.Spec.Replicas = ptr.To[int32](3) // Scale up to 3 InferGroups

		_, err = matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Update(
			context.Background(), updatedMI, metav1.UpdateOptions{})
		assert.NoError(t, err)

		// Wait for update to be available in cache
		found = waitForObjectInCache(t, 2*time.Second, func() bool {
			mi, err := controller.modelInfersLister.ModelInfers("default").Get("test-mi-scale-up")
			return err == nil && *mi.Spec.Replicas == 3
		})
		assert.True(t, found, "Updated ModelInfer should be found in cache")

		// Process the update
		err = controller.syncModelInfer(context.Background(), "default/test-mi-scale-up")
		assert.NoError(t, err)

		// Verify InferGroups were created in store
		verifyInferGroups(t, controller, updatedMI, 3)
		// Verify each InferGroup has correct roles
		verifyRoles(t, controller, updatedMI, 3)
		// Verify each InferGroup has correct pods
		verifyPodCount(t, controller, updatedMI, 3)
	})

	// Test Case 3: ModelInfer Update - Scale Down Replicas
	t.Run("ModelInferUpdateScaleDown", func(t *testing.T) {
		mi := createStandardModelInfer("test-mi-scale-down", 3, 2)
		// Create initial ModelInfer
		_, err := matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Create(
			context.Background(), mi, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Wait for object to be available in cache
		found := waitForObjectInCache(t, 2*time.Second, func() bool {
			_, err := controller.modelInfersLister.ModelInfers("default").Get("test-mi-scale-down")
			return err == nil
		})
		assert.True(t, found, "ModelInfer should be found in cache after creation")

		// Process initial creation
		err = controller.syncModelInfer(context.Background(), "default/test-mi-scale-down")
		assert.NoError(t, err)

		// Verify InferGroups initial state
		verifyInferGroups(t, controller, mi, 3)

		// Update ModelInfer to scale down
		updatedMI := mi.DeepCopy()
		updatedMI.Spec.Replicas = ptr.To[int32](1) // Scale up to 1 InferGroups

		_, err = matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Update(
			context.Background(), updatedMI, metav1.UpdateOptions{})
		assert.NoError(t, err)

		// Wait for update to be available in cache
		found = waitForObjectInCache(t, 2*time.Second, func() bool {
			mi, err := controller.modelInfersLister.ModelInfers("default").Get("test-mi-scale-down")
			return err == nil && *mi.Spec.Replicas == 1
		})
		assert.True(t, found, "Updated ModelInfer should be found in cache")

		// Process the update
		err = controller.syncModelInfer(context.Background(), "default/test-mi-scale-down")
		assert.NoError(t, err)

		requirement, err := labels.NewRequirement(
			workloadv1alpha1.GroupNameLabelKey,
			selection.In,
			[]string{"test-mi-scale-down-1", "test-mi-scale-down-2"},
		)
		assert.NoError(t, err)

		selector := labels.NewSelector().Add(*requirement)
		podsToDelete, err := controller.podsLister.Pods("default").List(selector)
		assert.NoError(t, err)
		servicesToDelete, err := controller.servicesLister.Services("default").List(selector)
		assert.NoError(t, err)

		// Get the indexer of the Service Informer for simulating deletion
		svcIndexer := controller.servicesInformer.GetIndexer()

		// Simulate the deletion process of each Service
		for _, svc := range servicesToDelete {
			// Delete the Service from the indexer (simulating the Service disappearing from the cluster)
			err = svcIndexer.Delete(svc)
			assert.NoError(t, err)
		}

		// Get the indexer of the Pod Informer for simulating deletion
		podIndexer := controller.podsInformer.GetIndexer()

		// Simulate the deletion of each Pod
		for _, pod := range podsToDelete {
			// Delete the Pod from the indexer (simulating the Pod disappearing from the cluster)
			err = podIndexer.Delete(pod)
			assert.NoError(t, err)
			controller.deletePod(pod)
		}

		time.Sleep(100 * time.Millisecond)

		// Verify InferGroups were created in store
		verifyInferGroups(t, controller, updatedMI, 1)
		// Verify each InferGroup has correct roles
		verifyRoles(t, controller, updatedMI, 1)
		// Verify each InferGroup has correct pods
		verifyPodCount(t, controller, updatedMI, 1)
	})

	// Test Case 4: ModelInfer Update - Role Replicas Scale Up
	t.Run("ModelInferRoleReplicasScaleUp", func(t *testing.T) {
		mi := createStandardModelInfer("test-role-scale-up", 2, 1)
		// Create initial ModelInfer
		_, err := matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Create(
			context.Background(), mi, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Wait for object to be available in cache
		found := waitForObjectInCache(t, 2*time.Second, func() bool {
			_, err := controller.modelInfersLister.ModelInfers("default").Get("test-role-scale-up")
			return err == nil
		})
		assert.True(t, found, "ModelInfer should be found in cache after creation")

		// Process initial creation
		err = controller.syncModelInfer(context.Background(), "default/test-role-scale-up")
		assert.NoError(t, err)

		// Verify InferGroups initial state
		verifyInferGroups(t, controller, mi, 2)

		// Update ModelInfer to role scale down
		updatedMI := mi.DeepCopy()
		updatedMI.Spec.Template.Roles[0].Replicas = ptr.To[int32](3) // Scale up to 3 roles

		_, err = matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Update(
			context.Background(), updatedMI, metav1.UpdateOptions{})
		assert.NoError(t, err)

		// Wait for update to be available in cache
		found = waitForObjectInCache(t, 2*time.Second, func() bool {
			mi, err := controller.modelInfersLister.ModelInfers("default").Get("test-role-scale-up")
			return err == nil && *mi.Spec.Template.Roles[0].Replicas == 3
		})
		assert.True(t, found, "Updated ModelInfer should be found in cache")

		// Process the update
		err = controller.syncModelInfer(context.Background(), "default/test-role-scale-up")
		assert.NoError(t, err)

		// Verify InferGroups were created in store
		verifyInferGroups(t, controller, updatedMI, 2)
		// Verify each InferGroup has correct roles
		verifyRoles(t, controller, updatedMI, 2)
		// Verify each InferGroup has correct pods
		verifyPodCount(t, controller, updatedMI, 2)
	})

	// Test Case 5: ModelInfer Update - Role Replicas Scale Down
	t.Run("ModelInferRoleReplicasScaleDown", func(t *testing.T) {
		mi := createStandardModelInfer("test-role-scale-down", 2, 3)

		// Create initial ModelInfer
		_, err := matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Create(
			context.Background(), mi, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Wait for object to be available in cache
		found := waitForObjectInCache(t, 2*time.Second, func() bool {
			_, err := controller.modelInfersLister.ModelInfers("default").Get("test-role-scale-down")
			return err == nil
		})
		assert.True(t, found, "ModelInfer should be found in cache after creation")

		// Process initial creation
		err = controller.syncModelInfer(context.Background(), "default/test-role-scale-down")
		assert.NoError(t, err)

		// Verify InferGroups initial state
		verifyInferGroups(t, controller, mi, 2)

		// Update ModelInfer to role scale down
		updatedMI := mi.DeepCopy()
		updatedMI.Spec.Template.Roles[0].Replicas = ptr.To[int32](1) // Scale down to 1 role

		_, err = matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Update(
			context.Background(), updatedMI, metav1.UpdateOptions{})
		assert.NoError(t, err)

		// Wait for update to be available in cache
		found = waitForObjectInCache(t, 2*time.Second, func() bool {
			mi, err := controller.modelInfersLister.ModelInfers("default").Get("test-role-scale-down")
			return err == nil && *mi.Spec.Template.Roles[0].Replicas == 1
		})
		assert.True(t, found, "Updated ModelInfer should be found in cache")

		// Process the update
		err = controller.syncModelInfer(context.Background(), "default/test-role-scale-down")
		assert.NoError(t, err)

		requirement, err := labels.NewRequirement(
			workloadv1alpha1.RoleIDKey,
			selection.In,
			[]string{"prefill-1", "prefill-2"},
		)
		assert.NoError(t, err)

		selector := labels.NewSelector().Add(*requirement)
		podsToDelete, err := controller.podsLister.Pods("default").List(selector)
		assert.NoError(t, err)
		servicesToDelete, err := controller.servicesLister.Services("default").List(selector)
		assert.NoError(t, err)

		// Get the indexer of the Service Informer for simulating deletion
		svcIndexer := controller.servicesInformer.GetIndexer()

		// Simulate the deletion process of each Service
		for _, svc := range servicesToDelete {
			// Delete the Service from the indexer (simulating the Service disappearing from the cluster)
			err = svcIndexer.Delete(svc)
			assert.NoError(t, err)
		}

		// Get the indexer of the Pod Informer for simulating deletion
		podIndexer := controller.podsInformer.GetIndexer()

		// Simulate the deletion of each Pod
		for _, pod := range podsToDelete {
			// Delete the Pod from the indexer (simulating the Pod disappearing from the cluster)
			err = podIndexer.Delete(pod)
			assert.NoError(t, err)
			controller.deletePod(pod)
		}

		// Verify InferGroups were created in store
		verifyInferGroups(t, controller, updatedMI, 2)
		// Verify each InferGroup has correct roles
		verifyRoles(t, controller, updatedMI, 2)
		// Verify each InferGroup has correct pods
		verifyPodCount(t, controller, updatedMI, 2)
	})
}

// waitForObjectInCache waits for a specific object to appear in the cache
func waitForObjectInCache(t *testing.T, timeout time.Duration, checkFunc func() bool) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Logf("Object not found in cache after %v timeout", timeout)
			return false
		case <-ticker.C:
			if checkFunc() {
				return true
			}
		}
	}
}

// createStandardModelInfer Create a standard ModelInfer
func createStandardModelInfer(name string, replicas int32, roleReplicas int32) *workloadv1alpha1.ModelInfer {
	return &workloadv1alpha1.ModelInfer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: workloadv1alpha1.ModelInferSpec{
			Replicas:      ptr.To[int32](replicas),
			SchedulerName: "volcano",
			Template: workloadv1alpha1.InferGroup{
				Roles: []workloadv1alpha1.Role{
					{
						Name:     "prefill",
						Replicas: ptr.To[int32](roleReplicas),
						EntryTemplate: workloadv1alpha1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "prefill-container",
										Image: "test-image:latest",
									},
								},
							},
						},
					},
				},
			},
			RecoveryPolicy: workloadv1alpha1.RoleRecreate,
		},
	}
}

// verifyInferGroups Verify the number and name of InferGroup
func verifyInferGroups(t *testing.T, controller *ModelInferController, mi *workloadv1alpha1.ModelInfer, expectedCount int) {
	groups, err := controller.store.GetInferGroupByModelInfer(utils.GetNamespaceName(mi))
	assert.NoError(t, err)
	assert.Equal(t, expectedCount, len(groups), fmt.Sprintf("Should have %d InferGroups", expectedCount))

	// Verify that the InferGroup name follows the expected pattern
	expectedGroupNames := make([]string, expectedCount)
	for i := 0; i < expectedCount; i++ {
		expectedGroupNames[i] = fmt.Sprintf("%s-%d", mi.Name, i)
	}

	actualGroupNames := make([]string, len(groups))
	for i, group := range groups {
		actualGroupNames[i] = group.Name
	}
	assert.ElementsMatch(t, expectedGroupNames, actualGroupNames, "InferGroup names should follow expected pattern")
}

// verifyPodCount Verify the number of Pods in each InferGroup
func verifyPodCount(t *testing.T, controller *ModelInferController, mi *workloadv1alpha1.ModelInfer, expectedGroups int) {
	expectPodNum := utils.ExpectedPodNum(mi)
	for i := 0; i < expectedGroups; i++ {
		groupName := fmt.Sprintf("%s-%d", mi.Name, i)
		groupSelector := labels.SelectorFromSet(map[string]string{
			workloadv1alpha1.GroupNameLabelKey: groupName,
		})

		groupPods, err := controller.podsLister.Pods(mi.Namespace).List(groupSelector)
		assert.NoError(t, err)
		assert.Equal(t, expectPodNum, len(groupPods), fmt.Sprintf("InferGroup %s should have %d pods", groupName, expectPodNum))
	}
}

// verifyRoles Verify the number and name of Role
func verifyRoles(t *testing.T, controller *ModelInferController, mi *workloadv1alpha1.ModelInfer, expectedGroups int) {
	// Traverse each InferGroup
	for i := 0; i < expectedGroups; i++ {
		groupName := fmt.Sprintf("%s-%d", mi.Name, i)

		// Traverse each role defined in the ModelInfer spec
		for _, specRole := range mi.Spec.Template.Roles {
			roleName := specRole.Name
			expectedRoleReplicas := int(*specRole.Replicas)

			// Get all instances of the role from the store
			roles, err := controller.store.GetRoleList(utils.GetNamespaceName(mi), groupName, roleName)
			assert.NoError(t, err, fmt.Sprintf("Should be able to get role list for %s in group %s", roleName, groupName))

			// Verify the number of roles
			assert.Equal(t, expectedRoleReplicas, len(roles),
				fmt.Sprintf("Group %s should have %d replicas of role %s", groupName, expectedRoleReplicas, roleName))

			// Verify role ID naming conventions
			expectedRoleIDs := make([]string, expectedRoleReplicas)
			for j := 0; j < expectedRoleReplicas; j++ {
				expectedRoleIDs[j] = fmt.Sprintf("%s-%d", roleName, j)
			}

			actualRoleIDs := make([]string, len(roles))
			for j, role := range roles {
				actualRoleIDs[j] = role.Name
			}

			assert.ElementsMatch(t, expectedRoleIDs, actualRoleIDs,
				fmt.Sprintf("Role IDs in group %s for role %s should follow expected pattern", groupName, roleName))
		}
	}
}
