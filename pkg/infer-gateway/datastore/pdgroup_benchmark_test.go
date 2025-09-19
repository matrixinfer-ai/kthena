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

/*
Copyright Kthena-AI Authors.

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

/*
Copyright Kthena-AI Authors.

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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	aiv1alpha1 "github.com/volcano-sh/kthena/pkg/apis/networking/v1alpha1"
)

// BenchmarkPDGroup benchmarks the new optimized approach
func BenchmarkPDGroup(b *testing.B) {
	store := New()

	// Create ModelServer with PDGroup
	modelServer := &aiv1alpha1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
		Spec: aiv1alpha1.ModelServerSpec{
			WorkloadSelector: &aiv1alpha1.WorkloadSelector{
				PDGroup: &aiv1alpha1.PDGroup{
					GroupKey: "pd-group",
					DecodeLabels: map[string]string{
						"role": "decode",
					},
					PrefillLabels: map[string]string{
						"role": "prefill",
					},
				},
			},
		},
	}

	modelServerName := types.NamespacedName{
		Namespace: "default",
		Name:      "test-model",
	}

	store.AddOrUpdateModelServer(modelServer, nil)

	// Add pods to store (this pre-categorizes them)
	pods := createBenchmarkTestPods(1000)
	for _, pod := range pods {
		store.AddOrUpdatePod(pod, []*aiv1alpha1.ModelServer{modelServer})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate new optimized approach: direct lookup
		decodePods, _ := store.GetDecodePods(modelServerName)

		// For each decode pod, get matching prefill pods
		if len(decodePods) > 0 {
			decodePodName := types.NamespacedName{
				Namespace: decodePods[0].Pod.Namespace,
				Name:      decodePods[0].Pod.Name,
			}
			_, _ = store.GetPrefillPodsForDecodeGroup(modelServerName, decodePodName)
		}
	}
}

// BenchmarkPDGroupLookupOnly benchmarks just the lookup performance
func BenchmarkPDGroupLookupOnly(b *testing.B) {
	store := New()

	// Create ModelServer with PDGroup
	modelServer := &aiv1alpha1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
		Spec: aiv1alpha1.ModelServerSpec{
			WorkloadSelector: &aiv1alpha1.WorkloadSelector{
				PDGroup: &aiv1alpha1.PDGroup{
					GroupKey: "pd-group",
					DecodeLabels: map[string]string{
						"role": "decode",
					},
					PrefillLabels: map[string]string{
						"role": "prefill",
					},
				},
			},
		},
	}

	modelServerName := types.NamespacedName{
		Namespace: "default",
		Name:      "test-model",
	}

	store.AddOrUpdateModelServer(modelServer, nil)

	// Add pods to store
	pods := createBenchmarkTestPods(10000) // Test with 10k pods
	for _, pod := range pods {
		store.AddOrUpdatePod(pod, []*aiv1alpha1.ModelServer{modelServer})
	}

	// Get a decode pod for testing
	decodePods, _ := store.GetDecodePods(modelServerName)
	if len(decodePods) == 0 {
		b.Fatal("No decode pods found")
	}

	decodePodName := types.NamespacedName{
		Namespace: decodePods[0].Pod.Namespace,
		Name:      decodePods[0].Pod.Name,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This should be O(1) lookup with our optimization
		_, _ = store.GetPrefillPodsForDecodeGroup(modelServerName, decodePodName)
	}
}

// createBenchmarkTestPods creates test pods for benchmarking
func createBenchmarkTestPods(count int) []*corev1.Pod {
	pods := make([]*corev1.Pod, count)

	for i := 0; i < count; i++ {
		var role string
		var groupValue string

		// Create mix of decode and prefill pods across different groups
		if i%2 == 0 {
			role = "decode"
		} else {
			role = "prefill"
		}

		groupValue = fmt.Sprintf("group-%d", i%10) // 10 different groups

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: "default",
				Labels: map[string]string{
					"pd-group": groupValue,
					"role":     role,
				},
			},
			Status: corev1.PodStatus{
				PodIP: fmt.Sprintf("10.0.%d.%d", i/256, i%256),
			},
		}

		pods[i] = pod
	}

	return pods
}
