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
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
)

type podSpec struct {
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
		pods []podSpec
		want bool
	}{
		{
			name: "no pods",
			pods: nil,
			want: true,
		},
		{
			name: "no revision label",
			pods: []podSpec{
				{name: "pod1", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName}},
			},
			want: true,
		},
		{
			name: "revision not match",
			pods: []podSpec{
				{name: "pod2", labels: map[string]string{workloadv1alpha1.GroupNameLabelKey: groupName, workloadv1alpha1.RevisionLabelKey: "oldhash"}},
			},
			want: true,
		},
		{
			name: "revision match",
			pods: []podSpec{
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
		store.AddRunningPodToInferGroup(utils.GetNamespaceName(mi), groupName, pod.Name, newHash)
	}

	// Waiting for pod cache to sync
	sync := waitForObjectInCache(t, 2*time.Second, func() bool {
		pods, _ := controller.podsLister.Pods(ns).List(labels.SelectorFromSet(map[string]string{
			workloadv1alpha1.GroupNameLabelKey: groupName,
		}))
		return len(pods) == int(expectedPodNum)
	})
	assert.True(t, sync, "Pods should be found in cache")

	fmt.Println(store.GetInferGroupByModelInfer(utils.GetNamespaceName(mi)))
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
