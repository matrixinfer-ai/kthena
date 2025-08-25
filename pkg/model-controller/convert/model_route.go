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

package convert

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networking "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	icUtils "matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
)

func BuildModelRoute(model *registry.Model) *networking.ModelRoute {
	var rules []*networking.Rule
	var targetModels []*networking.TargetModel
	// Use map to deduplicate lora adapters
	loraMap := make(map[string]struct{})
	for _, backend := range model.Spec.Backends {
		for _, lora := range backend.LoraAdapters {
			loraMap[lora.Name] = struct{}{}
		}
		targetModels = append(targetModels, &networking.TargetModel{
			ModelServerName: utils.GetBackendResourceName(model.Name, backend.Name),
			Weight:          backend.RouteWeight,
		})
	}
	var loraAdapters []string
	for loraName := range loraMap {
		loraAdapters = append(loraAdapters, loraName)
	}
	slices.Sort(loraAdapters)
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
			Name:      model.Name,
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
	route.Labels = utils.GetModelControllerLabels(model, "", icUtils.Revision(route.Spec))
	return route
}
