package utils

import (
	"hash/fnv"
	"testing"

	corev1 "k8s.io/api/core/v1"

	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
)

var (
	nginxPodTemplate = workloadv1alpha1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.14.2",
				},
			},
		},
	}
)
var replicas int32 = 1

func TestHashModelInferRevision(t *testing.T) {
	role1 := workloadv1alpha1.Role{
		Name:           "prefill",
		Replicas:       &replicas,
		EntryTemplate:  nginxPodTemplate,
		WorkerReplicas: 0,
		WorkerTemplate: nil,
	}
	role2 := workloadv1alpha1.Role{
		Name:           "decode",
		Replicas:       &replicas,
		EntryTemplate:  nginxPodTemplate,
		WorkerReplicas: 2,
		WorkerTemplate: &nginxPodTemplate,
	}
	role3 := workloadv1alpha1.Role{
		Name:           "prefill",
		Replicas:       &replicas,
		EntryTemplate:  nginxPodTemplate,
		WorkerReplicas: 0,
		WorkerTemplate: nil,
	}

	hash1 := HashModelInferRevision(role1)
	hash2 := HashModelInferRevision(role2)
	hash3 := HashModelInferRevision(role3)

	if hash1 == hash2 {
		t.Errorf("Hash should be different for different objects, got %s and %s", hash1, hash3)
	}
	if hash1 != hash3 {
		t.Errorf("Hash should be equal for identical objects, got %s and %s", hash1, hash2)
	}
}

func TestDeepHashObject(t *testing.T) {
	hasher := fnv.New32()
	role1 := workloadv1alpha1.Role{
		Name:           "prefill",
		Replicas:       &replicas,
		EntryTemplate:  nginxPodTemplate,
		WorkerReplicas: 0,
		WorkerTemplate: nil,
	}
	DeepHashObject(hasher, role1)
	firstHash := hasher.Sum32()

	hasher.Reset()
	DeepHashObject(hasher, role1)
	secondHash := hasher.Sum32()

	if firstHash != secondHash {
		t.Errorf("DeepHashObject should produce the same hash for the same object, got %v and %v", firstHash, secondHash)
	}
}
