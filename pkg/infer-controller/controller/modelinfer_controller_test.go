package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/datastore"
)

func newTestControllerWithPods(pods []*corev1.Pod, namespace string) *ModelInferController {
	client := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	podInformer := informerFactory.Core().V1().Pods()

	// Create pods first
	for _, pod := range pods {
		_, _ = client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	}
	// Start informer and synchronize
	stopCh := make(chan struct{})
	defer close(stopCh)
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	return &ModelInferController{
		podsLister: podInformer.Lister(),
	}
}

func TestCheckNotUpdatedInferGroup(t *testing.T) {
	ns := "test-ns"
	groupName := "test-group"
	group := datastore.InferGroup{Name: groupName}
	mi := &v1alpha1.ModelInfer{ObjectMeta: metav1.ObjectMeta{Namespace: ns}}
	newHash := "hash123"

	t.Run("no pods", func(t *testing.T) {
		c := newTestControllerWithPods([]*corev1.Pod{}, ns)
		if !c.checkNotUpdatedInferGroup(group, mi, newHash) {
			t.Errorf("expected true when no pods")
		}
	})

	t.Run("no revision label", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Labels: map[string]string{
					v1alpha1.GroupNameLabelKey: groupName,
				},
				Name: "pod1",
			},
		}
		c := newTestControllerWithPods([]*corev1.Pod{pod}, ns)
		if !c.checkNotUpdatedInferGroup(group, mi, newHash) {
			t.Errorf("expected true when pod has no revision label")
		}
	})

	t.Run("revision not match", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Labels: map[string]string{
					v1alpha1.GroupNameLabelKey: groupName,
					v1alpha1.RevisionLabelKey:  "oldhash",
				},
				Name: "pod2",
			},
		}
		c := newTestControllerWithPods([]*corev1.Pod{pod}, ns)
		if !c.checkNotUpdatedInferGroup(group, mi, newHash) {
			t.Errorf("expected true when revision not match")
		}
	})

	t.Run("revision match", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Labels: map[string]string{
					v1alpha1.GroupNameLabelKey: groupName,
					v1alpha1.RevisionLabelKey:  newHash,
				},
				Name: "pod3",
			},
		}
		c := newTestControllerWithPods([]*corev1.Pod{pod}, ns)
		if c.checkNotUpdatedInferGroup(group, mi, newHash) {
			t.Errorf("expected false when revision matches")
		}
	})
}
