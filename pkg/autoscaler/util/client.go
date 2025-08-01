package util

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"time"
)

func GetModelInferTarget(ctx context.Context, client clientset.Interface, namespace string, name string) (*workload.ModelInfer, error) {
	ctx, cancel := context.WithTimeout(ctx, AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()
	if instance, err := client.WorkloadV1alpha1().ModelInfers(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		return nil, err
	} else {
		return instance, nil
	}
}

func GetMetricPods(ctx context.Context, client kubernetes.Interface, namespace string, matchLabels map[string]string) (*corev1.PodList, error) {
	listPodCtx, cancel := context.WithTimeout(ctx, AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()

	if podList, err := client.CoreV1().Pods(namespace).List(listPodCtx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(matchLabels).String(),
	}); err != nil {
		return podList, nil
	} else {
		return nil, err
	}
}

func UpdateModelInfer(ctx context.Context, client clientset.Interface, modelInfer *workload.ModelInfer) error {
	modelInferCtx, cancel := context.WithTimeout(ctx, AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()
	if oldModelInfer, err := client.WorkloadV1alpha1().ModelInfers(modelInfer.Namespace).Get(modelInferCtx, modelInfer.Name, metav1.GetOptions{}); err == nil {
		modelInfer.ResourceVersion = oldModelInfer.ResourceVersion
		if _, updateErr := client.WorkloadV1alpha1().ModelInfers(modelInfer.Namespace).Update(modelInferCtx, modelInfer, metav1.UpdateOptions{}); updateErr != nil {
			klog.Errorf("failed to update modelInfer,err: %v", updateErr)
			return updateErr
		}
	} else {
		klog.Errorf("failed to get old modelInfer,err: %v", err)
		return err
	}
	return nil
}
