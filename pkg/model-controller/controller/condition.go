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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

const (
	ModelInitsReason      = "ModelCreating"
	ModelActiveReason     = "ModelAvailable"
	ModelProcessingReason = "ModelProcessing"
	ModelFailedReason     = "ModelAbnormal"
)

// setModelInitCondition sets model condition to initialized
func (mc *ModelController) setModelInitCondition(ctx context.Context, model *registryv1alpha1.Model) error {
	model, getError := mc.client.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
	if getError != nil {
		return getError
	}
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeInitialized),
		metav1.ConditionTrue, ModelInitsReason, "Model initialized"))
	if err := mc.updateModelStatus(ctx, model); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}
	return nil
}

// setModelProcessingCondition sets model condition to processing
func (mc *ModelController) setModelProcessingCondition(ctx context.Context, model *registryv1alpha1.Model) error {
	model, getError := mc.client.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
	if getError != nil {
		return getError
	}
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeActive),
		metav1.ConditionFalse, ModelProcessingReason, "Model not ready yet"))
	if err := mc.updateModelStatus(ctx, model); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}
	return nil
}

// setModelFailedCondition sets model condition to failed
func (mc *ModelController) setModelFailedCondition(ctx context.Context, model *registryv1alpha1.Model, err error) {
	model, getError := mc.client.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
	if getError != nil {
		klog.Errorf("get model failed: %v", getError)
		return
	}
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeFailed),
		metav1.ConditionTrue, ModelFailedReason, err.Error()))
	if err := mc.updateModelStatus(ctx, model); err != nil {
		klog.Errorf("update model status failed: %v", err)
	}
}

// setModelActiveCondition sets model conditions when all Model Infers are active.
func (mc *ModelController) setModelActiveCondition(ctx context.Context, model *registryv1alpha1.Model) error {
	model, getError := mc.client.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
	if getError != nil {
		return getError
	}
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeActive),
		metav1.ConditionTrue, ModelActiveReason, "Model is ready"))
	if err := mc.updateModelStatus(ctx, model); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}
	return nil
}

// newCondition returns a condition
func newCondition(conditionType string, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}
