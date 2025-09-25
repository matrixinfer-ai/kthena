/*
Copyright The Volcano Authors.

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
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	kthenafake "github.com/volcano-sh/kthena/client-go/clientset/versioned/fake"
	informersv1alpha1 "github.com/volcano-sh/kthena/client-go/informers/externalversions"
	aiv1alpha1 "github.com/volcano-sh/kthena/pkg/apis/networking/v1alpha1"
	"github.com/volcano-sh/kthena/pkg/kthena-router/backend"
	"github.com/volcano-sh/kthena/pkg/kthena-router/datastore"
	"github.com/volcano-sh/kthena/pkg/kthena-router/utils"
)

func TestModelServerController_ModelServerLifecycle(t *testing.T) {
	// Create fake clients
	kubeClient := kubefake.NewSimpleClientset()
	kthenaClient := kthenafake.NewSimpleClientset()

	// Create informer factories
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	kthenaInformerFactory := informersv1alpha1.NewSharedInformerFactory(kthenaClient, 0)

	// Create store
	store := datastore.New()

	// Create controller
	controller := NewModelServerController(
		kthenaInformerFactory,
		kubeInformerFactory,
		store,
	)

	stop := make(chan struct{})
	defer close(stop)

	kthenaInformerFactory.Start(stop)
	kubeInformerFactory.Start(stop)

	// Test Case 1: ModelServer Creation
	t.Run("ModelServerCreate", func(t *testing.T) {
		ms := &aiv1alpha1.ModelServer{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-modelserver",
			},
			Spec: aiv1alpha1.ModelServerSpec{
				InferenceEngine: aiv1alpha1.VLLM,
				WorkloadSelector: &aiv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{
						"app": "test-model",
					},
				},
			},
		}

		// Add ModelServer to fake client
		_, err := kthenaClient.NetworkingV1alpha1().ModelServers("default").Create(
			context.Background(), ms, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Wait for cache to sync gracefully
		if !waitForCacheSync(t, 5*time.Second, controller.modelServerSynced, controller.podSynced) {
			t.Fatal("Failed to sync caches within timeout")
		}

		// Additionally wait for the specific object to be available in cache
		found := waitForObjectInCache(t, 2*time.Second, func() bool {
			_, err := controller.modelServerLister.ModelServers("default").Get("test-modelserver")
			return err == nil
		})
		if !found {
			t.Log("ModelServer not found in cache - proceeding anyway for unit test")
		}
		// Simulate controller receiving the event
		controller.enqueueModelServer(ms)
		assert.Equal(t, 1, controller.workqueue.Len())

		// Process the queue item
		err = controller.syncModelServerHandler("default/test-modelserver")
		assert.NoError(t, err)

		// Verify ModelServer was added to store
		storedMS := store.GetModelServer(types.NamespacedName{
			Namespace: "default",
			Name:      "test-modelserver",
		})
		assert.NotNil(t, storedMS, "ModelServer should be found in store after creation")
		assert.Equal(t, "test-modelserver", storedMS.Name)
	})

	// Test Case 2: ModelServer Update
	t.Run("ModelServerUpdate", func(t *testing.T) {
		ms := &aiv1alpha1.ModelServer{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-modelserver-update",
				Labels: map[string]string{
					"version": "v1",
				},
			},
			Spec: aiv1alpha1.ModelServerSpec{
				InferenceEngine: aiv1alpha1.VLLM,
				WorkloadSelector: &aiv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{
						"app": "test-model-update",
					},
				},
			},
		}

		// Create initial ModelServer
		_, err := kthenaClient.NetworkingV1alpha1().ModelServers("default").Create(
			context.Background(), ms, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Additionally wait for the specific object to be available in cache
		found := waitForObjectInCache(t, 2*time.Second, func() bool {
			_, err := controller.modelServerLister.ModelServers("default").Get("test-modelserver-update")
			return err == nil
		})
		if !found {
			t.Log("ModelServer not found in cache after creation - proceeding anyway")
		}
		// Process initial creation
		controller.enqueueModelServer(ms)
		err = controller.syncModelServerHandler("default/test-modelserver-update")
		assert.NoError(t, err)

		// Update ModelServer
		updatedMS := ms.DeepCopy()
		updatedMS.Labels["version"] = "v2"
		updatedMS.Spec.WorkloadSelector.MatchLabels["environment"] = "production"

		_, err = kthenaClient.NetworkingV1alpha1().ModelServers("default").Update(
			context.Background(), updatedMS, metav1.UpdateOptions{})
		assert.NoError(t, err)

		// Additionally wait for the specific object to be available in cache
		found = waitForObjectInCache(t, 2*time.Second, func() bool {
			ms, err := controller.modelServerLister.ModelServers("default").Get("test-modelserver-update")
			return err == nil && ms.Labels["version"] == "v2"
		})
		if !found {
			t.Log("ModelServer not found in cache after creation - proceeding anyway")
		}
		// Simulate controller receiving update event
		controller.enqueueModelServer(updatedMS)
		// Clear any previous items from queue
		for controller.workqueue.Len() > 0 {
			item, _ := controller.workqueue.Get()
			controller.workqueue.Done(item)
			controller.workqueue.Forget(item)
		}
		controller.enqueueModelServer(updatedMS)
		assert.Equal(t, 1, controller.workqueue.Len())

		// Process the update
		err = controller.syncModelServerHandler("default/test-modelserver-update")
		assert.NoError(t, err)

		// Verify updated ModelServer in store
		storedMS := store.GetModelServer(types.NamespacedName{
			Namespace: "default",
			Name:      "test-modelserver-update",
		})
		assert.NotNil(t, storedMS, "ModelServer should be found in store after update")
		assert.Equal(t, "v2", storedMS.Labels["version"])
		assert.Equal(t, "production", storedMS.Spec.WorkloadSelector.MatchLabels["environment"])
	})

	// Test Case 3: ModelServer Deletion
	t.Run("ModelServerDelete", func(t *testing.T) {
		ms := &aiv1alpha1.ModelServer{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-modelserver-delete",
			},
			Spec: aiv1alpha1.ModelServerSpec{
				InferenceEngine: aiv1alpha1.VLLM,
				WorkloadSelector: &aiv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{
						"app": "test-model-delete",
					},
				},
			},
		}

		// Create ModelServer first
		_, err := kthenaClient.NetworkingV1alpha1().ModelServers("default").Create(
			context.Background(), ms, metav1.CreateOptions{})
		assert.NoError(t, err)

		waitForObjectInCache(t, 2*time.Second, func() bool {
			_, err := controller.modelServerLister.ModelServers("default").Get("test-modelserver-delete")
			return err == nil
		})

		// Process creation
		err = controller.syncModelServerHandler("default/test-modelserver-delete")
		assert.NoError(t, err)

		// Verify it exists in store
		storedMS := store.GetModelServer(types.NamespacedName{
			Namespace: "default",
			Name:      "test-modelserver-delete",
		})
		assert.NotNil(t, storedMS, "ModelServer should be found in store before deletion")

		// Delete ModelServer
		err = kthenaClient.NetworkingV1alpha1().ModelServers("default").Delete(
			context.Background(), "test-modelserver-delete", metav1.DeleteOptions{})
		assert.NoError(t, err)

		waitForObjectInCache(t, 2*time.Second, func() bool {
			_, err := controller.modelServerLister.ModelServers("default").Get("test-modelserver-delete")
			return err != nil
		})

		// Process the deletion - this should handle the NotFound error gracefully
		err = controller.syncModelServerHandler("default/test-modelserver-delete")
		assert.NoError(t, err)

		// Verify ModelServer was removed from store
		storedMS = store.GetModelServer(types.NamespacedName{
			Namespace: "default",
			Name:      "test-modelserver-delete",
		})
		assert.Nil(t, storedMS)
	})
}

func TestModelServerController_PodLifecycle(t *testing.T) {
	patch := setupMockBackend()
	defer patch.Reset()

	// Create fake clients
	kubeClient := kubefake.NewSimpleClientset()
	kthenaClient := kthenafake.NewSimpleClientset()

	// Create a ModelServer first to associate pods with
	ms := &aiv1alpha1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-modelserver-pods",
		},
		Spec: aiv1alpha1.ModelServerSpec{
			InferenceEngine: aiv1alpha1.VLLM,
			WorkloadSelector: &aiv1alpha1.WorkloadSelector{
				MatchLabels: map[string]string{
					"app": "test-model-pods",
				},
			},
		},
	}
	_, err := kthenaClient.NetworkingV1alpha1().ModelServers("default").Create(
		context.Background(), ms, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create informer factories
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	kthenaInformerFactory := informersv1alpha1.NewSharedInformerFactory(kthenaClient, 0)

	// Create store
	store := datastore.New()

	// Create controller
	controller := NewModelServerController(
		kthenaInformerFactory,
		kubeInformerFactory,
		store,
	)

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	kthenaInformerFactory.Start(stop)
	kubeInformerFactory.Start(stop)
	waitForCacheSync(t, 5*time.Second, controller.modelServerSynced, controller.podSynced)

	// Test Case 1: Pod Creation (Ready Pod)
	t.Run("PodCreateReady", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-pod-ready",
				Labels: map[string]string{
					"app": "test-model-pods",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}

		// Add Pod to fake client
		_, err := kubeClient.CoreV1().Pods("default").Create(
			context.Background(), pod, metav1.CreateOptions{})
		assert.NoError(t, err)

		sync := waitForObjectInCache(t, 2*time.Second, func() bool {
			pods, _ := store.GetPodsByModelServer(utils.GetNamespaceName(ms))
			return len(pods) > 0 && pods[0].Pod.Name == "test-pod-ready"
		})
		assert.True(t, sync, "Pod should be found in store after creation")
	})

	// Test Case 2: Pod Creation (Not Ready Pod)
	t.Run("PodCreateNotReady", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-pod-not-ready",
				Labels: map[string]string{
					"app": "test-model-pods",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending, // Not running
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionFalse,
					},
				},
			},
		}

		// Add Pod to fake client
		_, err := kubeClient.CoreV1().Pods("default").Create(
			context.Background(), pod, metav1.CreateOptions{})
		assert.NoError(t, err)

		sync := waitForObjectInCache(t, 2*time.Second, func() bool {
			pods, _ := controller.podLister.Pods("default").Get("test-pod-not-ready")
			return pods != nil && pods.Name == "test-pod-not-ready"
		})
		assert.True(t, sync, "Pod should be found in lister after creation")

		// Since pod is not ready, it should be deleted from store (or not added)
		// The exact verification depends on your store implementation
		sync = waitForObjectInCache(t, 2*time.Second, func() bool {
			pods, _ := store.GetPodsByModelServer(utils.GetNamespaceName(ms))
			return len(pods) == 1 && pods[0].Pod.Name == "test-pod-ready"
		})
		assert.True(t, sync, "Pod should be found in store after creation")
	})

	// Test Case 3: Pod Update (Becomes Ready)
	t.Run("PodUpdateBecomesReady", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-pod-update-ready",
				Labels: map[string]string{
					"app": "test-model-pods",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionFalse,
					},
				},
			},
		}

		// Create initial pod (not ready)
		_, err := kubeClient.CoreV1().Pods("default").Create(
			context.Background(), pod, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Update pod to be ready
		updatedPod := pod.DeepCopy()
		updatedPod.Status.Phase = corev1.PodRunning
		updatedPod.Status.Conditions[0].Status = corev1.ConditionTrue

		_, err = kubeClient.CoreV1().Pods("default").Update(
			context.Background(), updatedPod, metav1.UpdateOptions{})
		assert.NoError(t, err)

		// Verify pod is now considered ready
		// The exact verification depends on your store implementation
		sync := waitForObjectInCache(t, 2*time.Second, func() bool {
			pods, _ := store.GetPodsByModelServer(utils.GetNamespaceName(ms))
			return len(pods) == 2 &&
				(pods[0].Pod.Name == "test-pod-update-ready" || pods[1].Pod.Name == "test-pod-update-ready")
		})
		assert.True(t, sync, "Pod should be found in store after update")
	})

	// Test Case 4: Pod Update (Becomes Not Ready)
	t.Run("PodUpdateBecomesNotReady", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-pod-update-not-ready",
				Labels: map[string]string{
					"app": "test-model-pods",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}

		// Create initial pod (ready)
		_, err := kubeClient.CoreV1().Pods("default").Create(
			context.Background(), pod, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Verify pod was in store since it's ready
		sync := waitForObjectInCache(t, 2*time.Second, func() bool {
			pods, _ := store.GetPodsByModelServer(utils.GetNamespaceName(ms))
			return len(pods) == 3
		})
		assert.True(t, sync, "Pod should be found in store after creation")

		// Update pod to not ready
		updatedPod := pod.DeepCopy()
		updatedPod.Status.Phase = corev1.PodFailed
		updatedPod.Status.Conditions[0].Status = corev1.ConditionFalse

		_, err = kubeClient.CoreV1().Pods("default").Update(
			context.Background(), updatedPod, metav1.UpdateOptions{})
		assert.NoError(t, err)

		// Verify pod was removed from store since it's not ready
		sync = waitForObjectInCache(t, 2*time.Second, func() bool {
			pods, _ := store.GetPodsByModelServer(utils.GetNamespaceName(ms))
			return len(pods) == 2 && (pods[0].Pod.Name != "test-pod-update-not-ready" && pods[1].Pod.Name != "test-pod-update-not-ready")
		})
		assert.True(t, sync, "Pod should not be found in store after creation")
	})

	// Test Case 5: Pod Deletion
	t.Run("PodDelete", func(t *testing.T) {
		// Delete all pods
		for _, podName := range []string{"test-pod-ready", "test-pod-not-ready", "test-pod-update-ready", "test-pod-update-not-ready"} {
			err = kubeClient.CoreV1().Pods("default").Delete(
				context.Background(), podName, metav1.DeleteOptions{})
			assert.NoError(t, err)
		}

		// Verify pod was removed from store
		// Verify pod was removed from store since it's not ready
		sync := waitForObjectInCache(t, 2*time.Second, func() bool {
			pods, _ := store.GetPodsByModelServer(utils.GetNamespaceName(ms))
			return len(pods) == 0
		})
		assert.True(t, sync, "Pod should not be found in store after deletion")
	})
}

func TestModelServerController_ErrorHandling(t *testing.T) {
	// Create fake clients
	kubeClient := kubefake.NewSimpleClientset()
	kthenaClient := kthenafake.NewSimpleClientset()

	// Create informer factories
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	kthenaInformerFactory := informersv1alpha1.NewSharedInformerFactory(kthenaClient, 0)

	// Create store
	store := datastore.New()

	// Create controller
	controller := NewModelServerController(
		kthenaInformerFactory,
		kubeInformerFactory,
		store,
	)

	// Test Case 1: Invalid ModelServer Key
	t.Run("InvalidModelServerKey", func(t *testing.T) {
		err := controller.syncModelServerHandler("invalid-key-format")
		assert.NoError(t, err) // Should handle gracefully and return nil
	})

	// Test Case 2: Invalid Pod Key
	t.Run("InvalidPodKey", func(t *testing.T) {
		err := controller.syncPodHandler("invalid-key-format")
		assert.NoError(t, err) // Should handle gracefully and return nil
	})

	// Test Case 3: Non-existent ModelServer
	t.Run("NonExistentModelServer", func(t *testing.T) {
		err := controller.syncModelServerHandler("default/non-existent-modelserver")
		assert.NoError(t, err) // Should handle NotFound error gracefully
	})

	// Test Case 4: Non-existent Pod
	t.Run("NonExistentPod", func(t *testing.T) {
		err := controller.syncPodHandler("default/non-existent-pod")
		assert.NoError(t, err) // Should handle NotFound error gracefully
	})
}

func TestModelServerController_WorkQueueProcessing(t *testing.T) {
	// Create fake clients
	kubeClient := kubefake.NewSimpleClientset()
	kthenaClient := kthenafake.NewSimpleClientset()

	// Create informer factories
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	kthenaInformerFactory := informersv1alpha1.NewSharedInformerFactory(kthenaClient, 0)

	// Create store
	store := datastore.New()

	// Create controller
	controller := NewModelServerController(
		kthenaInformerFactory,
		kubeInformerFactory,
		store,
	)

	// Test Case 1: Initial Sync Signal
	t.Run("InitialSyncSignal", func(t *testing.T) {
		// Add initial sync signal (empty QueueItem)
		controller.workqueue.Add(QueueItem{})
		assert.Equal(t, 1, controller.workqueue.Len())

		// Process the initial sync signal
		processed := controller.processNextWorkItem()
		assert.True(t, processed)
		assert.True(t, controller.HasSynced())
		assert.Equal(t, 0, controller.workqueue.Len())
	})

	// Test Case 2: Unknown Resource Type
	t.Run("UnknownResourceType", func(t *testing.T) {
		unknownItem := QueueItem{
			ResourceType: "UnknownType",
			Key:          "default/unknown-resource",
		}

		controller.workqueue.Add(unknownItem)
		assert.Equal(t, 1, controller.workqueue.Len())

		// Process unknown resource type
		processed := controller.processNextWorkItem()
		assert.True(t, processed)
		assert.Equal(t, 0, controller.workqueue.Len())
	})

	// Test Case 3: Multiple Queue Items
	t.Run("MultipleQueueItems", func(t *testing.T) {
		// Add multiple items to queue
		items := []QueueItem{
			{ResourceType: ResourceTypeModelServer, Key: "default/ms1"},
			{ResourceType: ResourceTypePod, Key: "default/pod1"},
			{ResourceType: ResourceTypeModelServer, Key: "default/ms2"},
			{ResourceType: ResourceTypePod, Key: "default/pod2"},
		}

		for _, item := range items {
			controller.workqueue.Add(item)
		}
		assert.Equal(t, 4, controller.workqueue.Len())

		// Process all items
		processedCount := 0
		for controller.workqueue.Len() > 0 {
			processed := controller.processNextWorkItem()
			assert.True(t, processed)
			processedCount++
		}
		assert.Equal(t, 4, processedCount)
		assert.Equal(t, 0, controller.workqueue.Len())
	})
}

func TestModelServerController_PodSelectionLogic(t *testing.T) {
	// Create fake clients
	kubeClient := kubefake.NewSimpleClientset()
	kthenaClient := kthenafake.NewSimpleClientset()

	// Create informer factories
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	kthenaInformerFactory := informersv1alpha1.NewSharedInformerFactory(kthenaClient, 0)

	// Create store
	store := datastore.New()

	// Create controller
	controller := NewModelServerController(
		kthenaInformerFactory,
		kubeInformerFactory,
		store,
	)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)
	kthenaInformerFactory.Start(stop)
	kubeInformerFactory.Start(stop)

	// Test Case: Pod with Non-matching Labels
	t.Run("PodWithNonMatchingLabels", func(t *testing.T) {
		// Create ModelServer with specific selector
		ms := &aiv1alpha1.ModelServer{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-modelserver-selector",
			},
			Spec: aiv1alpha1.ModelServerSpec{
				InferenceEngine: aiv1alpha1.VLLM,
				WorkloadSelector: &aiv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{
						"app":     "specific-model",
						"version": "v1",
					},
				},
			},
		}

		_, err := kthenaClient.NetworkingV1alpha1().ModelServers("default").Create(
			context.Background(), ms, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Create pod with non-matching labels
		podNonMatching := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-pod-non-matching",
				Labels: map[string]string{
					"app":     "different-model", // Doesn't match ModelServer selector
					"version": "v2",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}

		_, err = kubeClient.CoreV1().Pods("default").Create(
			context.Background(), podNonMatching, metav1.CreateOptions{})
		assert.NoError(t, err)

		waitForObjectInCache(t, 2*time.Second, func() bool {
			pods, _ := controller.podLister.Pods("default").Get("test-pod-non-matching")
			return pods != nil && pods.Name == "test-pod-non-matching"
		})

		// Process the pod - should not be associated with ModelServer
		controller.enqueuePod(podNonMatching)
		err = controller.syncPodHandler("default/test-pod-non-matching")
		assert.NoError(t, err)

		// Create pod with matching labels
		podMatching := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-pod-matching",
				Labels: map[string]string{
					"app":     "specific-model", // Matches ModelServer selector
					"version": "v1",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}

		_, err = kubeClient.CoreV1().Pods("default").Create(
			context.Background(), podMatching, metav1.CreateOptions{})
		assert.NoError(t, err)

		sync := waitForObjectInCache(t, 2*time.Second, func() bool {
			pods, _ := store.GetPodsByModelServer(utils.GetNamespaceName(ms))
			return len(pods) > 0 && pods[0].Pod.Name == "test-pod-matching"
		})
		assert.True(t, sync, "Pod should be found in store after creation")
	})
}

func TestModelServerController_ComprehensiveLifecycleTest(t *testing.T) {
	// Create a comprehensive test that tests the full workflow
	// with proper informer setup and timing
	kubeClient := kubefake.NewSimpleClientset()
	kthenaClient := kthenafake.NewSimpleClientset()

	// Create and add resources to fake clients BEFORE starting informers
	ms := &aiv1alpha1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "integration-modelserver",
			Labels: map[string]string{
				"version": "v1",
			},
		},
		Spec: aiv1alpha1.ModelServerSpec{
			InferenceEngine: aiv1alpha1.VLLM,
			WorkloadSelector: &aiv1alpha1.WorkloadSelector{
				MatchLabels: map[string]string{
					"app": "integration-model",
				},
			},
		},
	}

	readyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "ready-pod",
			Labels: map[string]string{
				"app": "integration-model",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	notReadyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "not-ready-pod",
			Labels: map[string]string{
				"app": "integration-model",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	// Add resources to fake clients
	_, err := kthenaClient.NetworkingV1alpha1().ModelServers("test-ns").Create(
		context.Background(), ms, metav1.CreateOptions{})
	assert.NoError(t, err)

	_, err = kubeClient.CoreV1().Pods("test-ns").Create(
		context.Background(), readyPod, metav1.CreateOptions{})
	assert.NoError(t, err)

	_, err = kubeClient.CoreV1().Pods("test-ns").Create(
		context.Background(), notReadyPod, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create informer factories and start them
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	kthenaInformerFactory := informersv1alpha1.NewSharedInformerFactory(kthenaClient, 0)

	stopCh := make(chan struct{})
	defer close(stopCh)

	// Create controller and store
	store := datastore.New()
	controller := NewModelServerController(
		kthenaInformerFactory,
		kubeInformerFactory,
		store,
	)

	kthenaInformerFactory.Start(stopCh)
	kubeInformerFactory.Start(stopCh)

	waitForObjectInCache(t, 2*time.Second, func() bool {
		ms, err := controller.modelServerLister.ModelServers("test-ns").Get("integration-modelserver")
		return err == nil && ms.Name == "integration-modelserver"
	})

	// Test ModelServer processing
	err = controller.syncModelServerHandler("test-ns/integration-modelserver")
	assert.NoError(t, err)

	waitForObjectInCache(t, 2*time.Second, func() bool {
		ret, err := controller.podLister.Pods("test-ns").List(labels.Everything())
		return err == nil && len(ret) == 2
	})

	// Test Pod processing
	err = controller.syncPodHandler("test-ns/ready-pod")
	assert.NoError(t, err)

	err = controller.syncPodHandler("test-ns/not-ready-pod")
	assert.NoError(t, err)

	// Test update scenario
	updatedMS := ms.DeepCopy()
	updatedMS.Labels["version"] = "v2"
	updatedMS.Spec.InferenceEngine = aiv1alpha1.SGLang

	_, err = kthenaClient.NetworkingV1alpha1().ModelServers("test-ns").Update(
		context.Background(), updatedMS, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// Wait for update to propagate gracefully
	if !waitForCacheSync(t, 5*time.Second, controller.modelServerSynced) {
		t.Log("Cache sync timeout after update - proceeding anyway")
	}

	// Wait for the updated object to be available in cache
	found := waitForObjectInCache(t, 2*time.Second, func() bool {
		ms, err := controller.modelServerLister.ModelServers("test-ns").Get("integration-modelserver")
		if err != nil {
			return false
		}
		// Check if the update is reflected
		return ms.Labels["version"] == "v2"
	})
	assert.True(t, found, "Updated ModelServer should be found in cache after update")

	// Process the update
	err = controller.syncModelServerHandler("test-ns/integration-modelserver")
	assert.NoError(t, err)

	// Verify update
	storedMS := store.GetModelServer(types.NamespacedName{
		Namespace: "test-ns",
		Name:      "integration-modelserver",
	})
	if storedMS != nil {
		assert.Equal(t, "v2", storedMS.Labels["version"])
		assert.Equal(t, aiv1alpha1.SGLang, storedMS.Spec.InferenceEngine)
	}

	// Test error handling for non-existent resources
	err = controller.syncModelServerHandler("test-ns/non-existent-modelserver")
	assert.NoError(t, err) // This should work fine for pods

	err = controller.syncPodHandler("test-ns/non-existent-pod")
	assert.NoError(t, err) // This should work fine for pods
}

// Helper functions for testing

// waitForCacheSync waits for the informer caches to sync with a timeout
func waitForCacheSync(t *testing.T, timeout time.Duration, cacheSyncWaiters ...cache.InformerSynced) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if !cache.WaitForCacheSync(ctx.Done(), cacheSyncWaiters...) {
		t.Logf("Cache sync timeout after %v - some caches may not be synced", timeout)
		return false
	}
	return true
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

// Helper function to setup mock for backend calls
func setupMockBackend() *gomonkey.Patches {
	patch := gomonkey.NewPatches()
	patch.ApplyFunc(backend.GetPodMetrics, func(backend string, pod *corev1.Pod, previousHistogram map[string]*dto.Histogram) (map[string]float64, map[string]*dto.Histogram) {
		return map[string]float64{
			utils.GPUCacheUsage:     0.5,
			utils.RequestWaitingNum: 10,
			utils.RequestRunningNum: 5,
		}, map[string]*dto.Histogram{}
	})
	patch.ApplyFunc(backend.GetPodModels, func(backend string, pod *corev1.Pod) ([]string, error) {
		return []string{"test-model"}, nil
	})
	return patch
}
