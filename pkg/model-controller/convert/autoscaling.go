package convert

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	icUtils "matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
)

func BuildAutoscalingPolicy(autoscalingConfig *registry.AutoscalingPolicySpec, model *registry.Model, backendName string) *registry.AutoscalingPolicy {
	return &registry.AutoscalingPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: registry.AutoscalingPolicyKind.GroupVersion().String(),
			Kind:       registry.AutoscalingPolicyKind.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   utils.GetBackendResourceName(model.Name, backendName),
			Labels: utils.GetModelControllerLabels(model.Name, backendName, icUtils.Revision(*autoscalingConfig)),
			OwnerReferences: []metav1.OwnerReference{
				utils.NewModelOwnerRef(model),
			},
			Namespace: model.Namespace,
		},
		Spec: *autoscalingConfig,
	}
}

func BuildScalingPolicyBindingSpec(backend *registry.ModelBackend, name string) *registry.AutoscalingPolicyBindingSpec {
	return &registry.AutoscalingPolicyBindingSpec{
		ScalingConfiguration: &registry.ScalingConfiguration{
			Target: registry.Target{
				Kind: "ModelInfer",
				Name: name,
				TargetRef: &corev1.LocalObjectReference{
					Name: name,
				},
			},
			MinReplicas: backend.MinReplicas,
			MaxReplicas: backend.MaxReplicas,
		},
		PolicyRef: &corev1.LocalObjectReference{
			Name: name,
		},
	}
}

func BuildPolicyBindingMeta(spec *registry.AutoscalingPolicyBindingSpec, model *registry.Model, backendName string, name string) *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:      name,
		Namespace: model.Namespace,
		Labels:    utils.GetModelControllerLabels(model.Name, backendName, icUtils.Revision(spec)),
		OwnerReferences: []metav1.OwnerReference{
			utils.NewModelOwnerRef(model),
		},
	}
}

func BuildScalingPolicyBinding(model *registry.Model, backend *registry.ModelBackend, name string) *registry.AutoscalingPolicyBinding {
	spec := BuildScalingPolicyBindingSpec(backend, name)
	return &registry.AutoscalingPolicyBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: registry.AutoscalingPolicyBindingKind.GroupVersion().String(),
			Kind:       registry.AutoscalingPolicyBindingKind.Kind,
		},
		ObjectMeta: *BuildPolicyBindingMeta(spec, model, backend.Name, name),
		Spec:       *spec,
	}
}

func BuildOptimizePolicyBindingSpec(model *registry.Model, name string) *registry.AutoscalingPolicyBindingSpec {
	params := make([]registry.OptimizerParam, 0, len(model.Spec.Backends))
	for _, backend := range model.Spec.Backends {
		targetName := utils.GetBackendResourceName(model.Name, backend.Name)
		params = append(params, registry.OptimizerParam{
			Target: registry.Target{
				Kind: registry.ModelInferenceTargetType,
				Name: targetName,
				TargetRef: &corev1.LocalObjectReference{
					Name: targetName,
				},
			},
			Cost: backend.ScalingCost,
		})
	}
	return &registry.AutoscalingPolicyBindingSpec{
		OptimizerConfiguration: &registry.OptimizerConfiguration{
			Params:                   params,
			CostExpansionRatePercent: model.Spec.CostExpansionRatePercent,
		},
		PolicyRef: &corev1.LocalObjectReference{
			Name: name,
		},
	}
}

func BuildOptimizePolicyBinding(model *registry.Model, name string) *registry.AutoscalingPolicyBinding {
	spec := BuildOptimizePolicyBindingSpec(model, name)
	return &registry.AutoscalingPolicyBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: registry.AutoscalingPolicyBindingKind.GroupVersion().String(),
			Kind:       registry.AutoscalingPolicyBindingKind.Kind,
		},
		ObjectMeta: *BuildPolicyBindingMeta(spec, model, "", name),
		Spec:       *spec,
	}
}
