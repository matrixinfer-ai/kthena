/*
Copyright 2025.

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

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

var _ = Describe("Model Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		model := &registryv1.Model{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Model")
			err := k8sClient.Get(ctx, typeNamespacedName, model)
			if err != nil && errors.IsNotFound(err) {
				resource := &registryv1.Model{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &registryv1.Model{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Model")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ModelReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})

func TestModelController_CacheVolume_HuggingFace_HostPath(t *testing.T) {
	model := &registryv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
		Spec: registryv1.ModelSpec{
			Backends: []registryv1.ModelBackend{
				{
					Name:        "backend1",
					Type:        registryv1.ModelBackendTypeVLLM,
					MinReplicas: 1,
					ModelURI:    "hf://test/dummy",
					CacheURI:    "hostPath://tmp/test",
					Workers: []registryv1.ModelWorker{
						{
							Type:  registryv1.ModelWorkerTypeServer,
							Pods:  1,
							Image: "vllm-server:latest",
						},
					},
				},
			},
		},
	}

	infers, err := buildModelInferCR(model)
	assert.Nil(t, err)
	assert.Len(t, infers, 1)
	assert.Len(t, infers[0].Spec.Template.Spec.Roles, 1)

	// check volumes
	assert.Len(t, infers[0].Spec.Template.Spec.Roles[0].EntryTemplate.Spec.Volumes, 1)
	diff := cmp.Diff(corev1.Volume{
		Name: "backend1-server-weights",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/tmp/test",
			},
		},
	}, infers[0].Spec.Template.Spec.Roles[0].EntryTemplate.Spec.Volumes[0])
	assert.Empty(t, diff, "Differences found:\n%s", diff)
	// check init containers
	assert.Len(t, infers[0].Spec.Template.Spec.Roles[0].EntryTemplate.Spec.InitContainers, 1)
	diff = cmp.Diff(corev1.Container{
		Name:  "backend1-server-downloader",
		Image: "matrixinfer/downloader:latest",
		Args: []string{
			"-s", "hf://test/dummy",
			"-o", "/backend1",
			"-e", "vllm",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "backend1-server-weights",
				MountPath: "/backend1",
			},
		},
	}, infers[0].Spec.Template.Spec.Roles[0].EntryTemplate.Spec.InitContainers[0])
	assert.Empty(t, diff, "Differences found:\n%s", diff)
}
