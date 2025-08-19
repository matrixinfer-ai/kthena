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
	routeName := model.Name
	rules, loraAdapters := getRulesAndLoraAdapters(model)
	route := &networking.ModelRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       networking.ModelRouteKind,
			APIVersion: networking.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
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

// getRulesAndLoraAdapters generates routing rules and LoRA adapter names based on the model's backends and LoRA adapters.
func getRulesAndLoraAdapters(model *registry.Model) ([]*networking.Rule, []string) {

	targetModels, loraMap, loraMapNum := getTargetModelAndLoraMap(model)

	var rules []*networking.Rule
	var loraAdapters []string
	for loraName := range loraMap {
		loraAdapters = append(loraAdapters, loraName)
	}
	slices.Sort(loraAdapters)
	if len(model.Spec.Backends) == 1 {
		rules = append(rules, &networking.Rule{
			Name:         modelRouteRuleName,
			ModelMatch:   model.Spec.ModelMatch,
			TargetModels: targetModels,
		})
	} else {
		loraTarget := make(map[string][]*networking.TargetModel)
		modelMatchDefault := getModelMatchWithHeader(model, model.Name)
		rules = append(rules, &networking.Rule{
			Name:         modelRouteRuleName,
			ModelMatch:   modelMatchDefault,
			TargetModels: targetModels,
		})
		for _, loraName := range loraAdapters {
			for _, loraNum := range loraMapNum[loraName] {
				loraTarget[loraName] = append(loraTarget[loraName], targetModels[loraNum])
			}
			modelMatchLora := getModelMatchWithHeader(model, loraName)
			rules = append(rules, &networking.Rule{
				Name:         loraName,
				ModelMatch:   modelMatchLora,
				TargetModels: loraTarget[loraName],
			})
		}
	}
	return rules, loraAdapters
}

// getModelMatchWithHeader returns a ModelMatch with the "name" header set to the model name or lora adapter name.
func getModelMatchWithHeader(model *registry.Model, name string) *networking.ModelMatch {
	var modelMatch *networking.ModelMatch
	modelMatch = model.Spec.ModelMatch.DeepCopy()
	if modelMatch == nil {
		modelMatch = &networking.ModelMatch{}
	}
	if modelMatch.Headers == nil {
		modelMatch.Headers = make(map[string]*networking.StringMatch)
	}
	modelMatch.Headers["name"] = &networking.StringMatch{
		Exact: &name,
	}
	return modelMatch
}

// getTargetModelAndLoraMap returns the target models, a map of lora adapter names to backend names.
func getTargetModelAndLoraMap(model *registry.Model) ([]*networking.TargetModel, map[string]string, map[string][]int) {
	var targetModels []*networking.TargetModel
	// Use map to deduplicate lora adapters
	loraMap := make(map[string]string)
	loraMapNum := make(map[string][]int)
	for idx, backend := range model.Spec.Backends {
		for _, lora := range backend.LoraAdapters {
			loraMap[lora.Name] = backend.Name
			loraMapNum[lora.Name] = append(loraMapNum[lora.Name], idx)
		}
		targetModels = append(targetModels, &networking.TargetModel{
			ModelServerName: utils.GetBackendResourceName(model.Name, backend.Name),
			Weight:          backend.RouteWeight,
		})
	}
	return targetModels, loraMap, loraMapNum
}
