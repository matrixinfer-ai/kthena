package controller

import (
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
	"testing"
)

// todo fix it
func TestModelController_CacheVolume_HuggingFace_HostPath(t *testing.T) {
	model := &registryv1alpha1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
		Spec: registryv1alpha1.ModelSpec{
			Backends: []registryv1alpha1.ModelBackend{
				{
					Name: "backend1",
					Type: registryv1alpha1.ModelBackendTypeVLLM,
					Config: apiextensionsv1.JSON{
						Raw: []byte(`{"max-model-len": 32768, "block-size": 128, "trust-remote-code": "", "tensor-parallel-size": 2, "gpu-memory-utilization": 0.9}`),
					},
					MinReplicas: 1,
					ModelURI:    "s3://aios_models/deepseek-ai/DeepSeek-V3-W8A8/vllm-ascend",
					CacheURI:    "hostpath:///tmp/test",
					Workers: []registryv1alpha1.ModelWorker{
						{
							Type:  registryv1alpha1.ModelWorkerTypeServer,
							Pods:  1,
							Image: "vllm-server:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:                            resource.MustParse("100m"),
									corev1.ResourceMemory:                         resource.MustParse("1Gi"),
									corev1.ResourceName("huawei.com/ascend-1980"): resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
		},
	}

	infers, err := utils.BuildModelInferCR(model)
	assert.Nil(t, err)

	assert.Len(t, infers, 1)
	assert.Len(t, infers[0].Spec.Template.Roles, 1)
	diff := cmp.Diff(infers[0].Name, "test-model-0-vllm-instance")
	assert.Empty(t, diff, "Differences found:\n%s", diff)

	// check volumes
	assert.Len(t, infers[0].Spec.Template.Roles[0].EntryTemplate.Spec.Volumes, 1)
	diff = cmp.Diff(infers[0].Spec.Template.Roles[0].EntryTemplate.Spec.Volumes, []corev1.Volume{
		{
			Name: "backend1-weights",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/tmp/test",
				},
			},
		},
	})
	assert.Empty(t, diff, "Differences found:\n%s", diff)

	diff = cmp.Diff(infers[0].Spec.Template.Roles[0].EntryTemplate.Spec.InitContainers, []corev1.Container{
		{
			Name:  "test-model-downloader",
			Image: "matrixinfer/downloader:latest",
			Args: []string{
				"--source",
				"s3://aios_models/deepseek-ai/DeepSeek-V3-W8A8/vllm-ascend",
				"--output-dir",
				"/tmp/test/13a7c4d58031cdc502ca1bb4a592f2b",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "backend1-weights",
					MountPath: "/tmp/test/13a7c4d58031cdc502ca1bb4a592f2b",
				},
			},
		},
	})
	assert.Empty(t, diff, "Differences found:\n%s", diff)

	diff = cmp.Diff(infers[0].Spec.Template.Roles[0], nil)
	assert.Empty(t, diff, "Differences found:\n%s", diff)

	assert.Len(t, infers[0].Spec.Template.Roles[0].EntryTemplate.Spec.Containers, 2)
	diff = cmp.Diff(infers[0].Spec.Template.Roles[0].EntryTemplate.Spec.Containers[1].Command, []string{
		"python",
		"-m",
		"vllm.entrypoints.openai.api_server",
		"--model",
		"/tmp/test/946c462680900e02ffd22bfe455cda99",
		"--block-size",
		"128",
		"--gpu-memory-utilization",
		"0.9",
		"--max-model-len",
		"32768",
		"--tensor-parallel-size",
		"2",
		"--trust-remote-code"})
	assert.Empty(t, diff, "Differences found:\n%s", diff)

	diff = cmp.Diff(infers[0].Spec.Template.Roles[0].EntryTemplate.Spec.Containers[1].VolumeMounts, []corev1.VolumeMount{
		{
			Name:      "backend1-weights",
			MountPath: "/tmp/test/946c462680900e02ffd22bfe455cda99",
		},
	})
	assert.Empty(t, diff, "Differences found:\n%s", diff)
}
