package convert

import (
	"fmt"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networking "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

func BuildModelRoute(model *registry.Model) *networking.ModelRoute {
	var rules []*networking.Rule
	var loraAdapters []string
	var targetModels []*networking.TargetModel
	for idx, backend := range model.Spec.Backends {
		for _, lora := range backend.LoraAdapters {
			loraAdapters = append(loraAdapters, lora.Name)
		}
		targetModels = append(targetModels, &networking.TargetModel{
			ModelServerName: fmt.Sprintf("%s-%d-%s-server", model.Name, idx, strings.ToLower(string(backend.Type))),
			Weight:          backend.RouteWeight,
		})
	}
	// sort and then remove duplicate lora name
	slices.Sort(loraAdapters)
	loraAdapters = slices.Compact(loraAdapters)
	rules = append(rules, &networking.Rule{
		Name:         modelRouteRuleName,
		ModelMatch:   model.Spec.ModelMatch,
		TargetModels: targetModels,
	})
	route := &networking.ModelRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       networking.ModelRouteKind,
			APIVersion: networking.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-route", model.Name),
			Namespace: model.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: registry.GroupVersion.String(),
					Kind:       registry.ModelKind.Kind,
					Name:       model.Name,
					UID:        model.UID,
				},
			},
		},
		Spec: networking.ModelRouteSpec{
			ModelName:    model.Name,
			LoraAdapters: loraAdapters,
			Rules:        rules,
		},
	}
	return route
}
