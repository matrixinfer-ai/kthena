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
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	networking "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	icUtils "matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
)

// BuildModelServer creates arrays of ModelServer for the given model.
// Each model backend will create one model server.
func BuildModelServer(model *registry.Model) ([]*networking.ModelServer, error) {
	var modelServers []*networking.ModelServer
	for idx, backend := range model.Spec.Backends {
		var inferenceEngine networking.InferenceEngine
		switch backend.Type {
		case registry.ModelBackendTypeVLLM, registry.ModelBackendTypeVLLMDisaggregated:
			inferenceEngine = networking.VLLM
		case registry.ModelBackendTypeSGLang:
			inferenceEngine = networking.SGLang
		case registry.ModelBackendTypeMindIE, registry.ModelBackendTypeMindIEDisaggregated:
			klog.Warning("Not support MindIE backend yet, please use vLLM or SGLang backend")
			return modelServers, nil
		}
		var pdGroup *networking.PDGroup
		switch backend.Type {
		case registry.ModelBackendTypeVLLMDisaggregated, registry.ModelBackendTypeMindIEDisaggregated:
			pdGroup = &networking.PDGroup{
				GroupKey: workload.GroupNameLabelKey,
				PrefillLabels: map[string]string{
					workload.RoleLabelKey: "prefill",
				},
				DecodeLabels: map[string]string{
					workload.RoleLabelKey: "decode",
				},
			}
		}
		servedModelName, err := getServedModelName(model, backend)
		if err != nil {
			return nil, err
		}
		modelServer := networking.ModelServer{
			TypeMeta: metav1.TypeMeta{
				Kind:       networking.ModelServerKind,
				APIVersion: networking.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d-%s-server", model.Name, idx, strings.ToLower(string(backend.Type))),
				Namespace: model.Namespace,
				Labels:    utils.GetModelControllerLabels(model, backend.Name, icUtils.Revision(model.Spec)),
				OwnerReferences: []metav1.OwnerReference{
					utils.NewModelOwnerRef(model),
				},
			},
			Spec: networking.ModelServerSpec{
				Model:           &servedModelName,
				InferenceEngine: inferenceEngine,
				WorkloadSelector: &networking.WorkloadSelector{
					MatchLabels: map[string]string{
						"model.uid": string(model.UID),
					},
					PDGroup: pdGroup,
				},
				WorkloadPort: networking.WorkloadPort{
					Port: 8000, // todo: get port from config
				},
				TrafficPolicy: &networking.TrafficPolicy{
					Retry: &networking.Retry{
						Attempts:      5,
						RetryInterval: &metav1.Duration{Duration: time.Duration(0) * time.Second},
					},
				},
			},
		}
		modelServers = append(modelServers, &modelServer)
	}
	return modelServers, nil
}

// getServedModelName gets served model name from the worker config. Default is the model name.
func getServedModelName(model *registry.Model, backend registry.ModelBackend) (string, error) {
	servedModelName := model.Name
	for _, worker := range backend.Workers {
		args, err := utils.ParseArgs(&worker.Config)
		if err != nil {
			return "", err
		}
		for i, str := range args {
			if str == "--served-model-name" && i+1 < len(args) {
				servedModelName = args[i+1]
				break
			}
		}
	}
	return servedModelName, nil
}
