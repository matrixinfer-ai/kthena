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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/convert"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
)

func (mc *ModelController) createOrUpdateAutoscalingPolicyAndBinding(ctx context.Context, model *registryv1alpha1.Model) error {
	if model.Spec.AutoscalingPolicy != nil {
		// Create autoscaling policy and optimize policy binding
		asp := convert.BuildAutoscalingPolicy(model.Spec.AutoscalingPolicy, model, "")
		aspBinding := convert.BuildOptimizePolicyBinding(model, utils.GetBackendResourceName(model.Name, ""))
		if err := mc.createOrUpdateAsp(ctx, asp); err != nil {
			return err
		}
		if err := mc.createOrUpdateAspBinding(ctx, aspBinding); err != nil {
			return err
		}
	} else {
		// Create autoscaling policy and scaling policy binding
		for _, backend := range model.Spec.Backends {
			if backend.AutoscalingPolicy == nil {
				continue
			}
			asp := convert.BuildAutoscalingPolicy(backend.AutoscalingPolicy, model, backend.Name)
			aspBinding := convert.BuildScalingPolicyBinding(model, &backend, utils.GetBackendResourceName(model.Name, backend.Name))
			if err := mc.createOrUpdateAsp(ctx, asp); err != nil {
				return err
			}
			if err := mc.createOrUpdateAspBinding(ctx, aspBinding); err != nil {
				return err
			}
		}
	}
	return nil
}

func (mc *ModelController) createOrUpdateAsp(ctx context.Context, policy *registryv1alpha1.AutoscalingPolicy) error {
	oldPolicy, err := mc.autoscalingPoliciesLister.AutoscalingPolicies(policy.Namespace).Get(policy.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicies(policy.Namespace).Create(ctx, policy, metav1.CreateOptions{}); err != nil {
				klog.Errorf("failed to create ASP %s,%v", klog.KObj(policy), err)
				return err
			}
			return nil
		}
		return err
	}
	if oldPolicy.Labels[utils.RevisionLabelKey] == policy.Labels[utils.RevisionLabelKey] {
		klog.Infof("Autoscaling policy %s does not need to update", policy.Name)
		return nil
	}
	if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicies(policy.Namespace).Update(ctx, policy, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

func (mc *ModelController) createOrUpdateAspBinding(ctx context.Context, binding *registryv1alpha1.AutoscalingPolicyBinding) error {
	oldPolicy, err := mc.autoscalingPolicyBindingsLister.AutoscalingPolicyBindings(binding.Namespace).Get(binding.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicyBindings(binding.Namespace).Create(ctx, binding, metav1.CreateOptions{}); err != nil {
				klog.Errorf("failed to create bindings %s,%v", klog.KObj(binding), err)
				return err
			}
			return nil
		}
		return err
	}
	if oldPolicy.Labels[utils.RevisionLabelKey] == binding.Labels[utils.RevisionLabelKey] {
		klog.Infof("Bindings %s does not need to update", binding.Name)
		return nil
	}
	if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicyBindings(binding.Namespace).Update(ctx, binding, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}
