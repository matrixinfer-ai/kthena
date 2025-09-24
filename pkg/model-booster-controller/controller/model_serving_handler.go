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

	workload "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	"github.com/volcano-sh/kthena/pkg/model-booster-controller/convert"
	"github.com/volcano-sh/kthena/pkg/model-booster-controller/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

// createOrUpdateModelInfer attempts to create model infer if model infer does not exist, or update it if it is different from model.
// Meanwhile, delete model infer if it is not in the model spec anymore.
func (mc *ModelController) createOrUpdateModelInfer(ctx context.Context, model *workload.ModelBooster, excludedBackends []string) error {
	existingModelInfers, err := mc.listModelInferByLabel(model)
	if err != nil {
		return err
	}

	excludedSet := make(map[string]bool)
	for _, backendName := range excludedBackends {
		excludedSet[backendName] = true
	}

	var filteredBackends []workload.ModelBackend
	for _, backend := range model.Spec.Backends {
		if !excludedSet[backend.Name] {
			filteredBackends = append(filteredBackends, backend)
		}
	}

	// If all backends are excluded, skip ModelServing updates
	if len(filteredBackends) == 0 {
		klog.Infof("All backends excluded from ModelServing update for model %s", model.Name)
		return nil
	}

	// Create a temporary model with filtered backends for ModelServing creation
	tempModel := model.DeepCopy()
	tempModel.Spec.Backends = filteredBackends

	modelInfers, err := convert.BuildModelInfer(tempModel)
	if err != nil {
		klog.Errorf("failed to build model infer for model %s: %v", model.Name, err)
		return err
	}
	modelInfersToKeep := make(map[string]struct{})
	for _, modelInfer := range modelInfers {
		modelInfersToKeep[modelInfer.Name] = struct{}{}
		oldModelInfer, err := mc.modelServingLister.ModelServings(modelInfer.Namespace).Get(modelInfer.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.V(4).Infof("Create ModelBooster Infer %s", modelInfer.Name)
				if _, err := mc.client.WorkloadV1alpha1().ModelServings(model.Namespace).Create(ctx, modelInfer, metav1.CreateOptions{}); err != nil {
					klog.Errorf("failed to create ModelServing %s: %v", klog.KObj(modelInfer), err)
					return err
				}
				continue
			}
			klog.Errorf("failed to get ModelServing %s: %v", klog.KObj(modelInfer), err)
			return err
		}
		if oldModelInfer.Labels[utils.RevisionLabelKey] == modelInfer.Labels[utils.RevisionLabelKey] {
			klog.Infof("ModelBooster Infer %s of model %s does not need to update", modelInfer.Name, model.Name)
			continue
		}
		modelInfer.ResourceVersion = oldModelInfer.ResourceVersion
		if _, err := mc.client.WorkloadV1alpha1().ModelServings(model.Namespace).Update(ctx, modelInfer, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("failed to update ModelServing %s: %v", klog.KObj(modelInfer), err)
			return err
		}
		klog.V(4).Infof("Updated ModelBooster Infer %s for model %s", modelInfer.Name, model.Name)
	}
	for _, existingModelInfer := range existingModelInfers {
		// if not exist in modelInfersToKeep, delete it
		if _, ok := modelInfersToKeep[existingModelInfer.Name]; !ok {
			if err := mc.client.WorkloadV1alpha1().ModelServings(model.Namespace).Delete(ctx, existingModelInfer.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			klog.V(4).Infof("Delete ModelServing %s", existingModelInfer.Name)
		}
	}
	return nil
}

// listModelInferByLabel list all model infer which label key is "owner" and label value is model uid
func (mc *ModelController) listModelInferByLabel(model *workload.ModelBooster) ([]*workload.ModelServing, error) {
	if modelInfers, err := mc.modelServingLister.ModelServings(model.Namespace).List(labels.SelectorFromSet(map[string]string{
		utils.OwnerUIDKey: string(model.UID),
	})); err != nil {
		return nil, err
	} else {
		return modelInfers, nil
	}
}
