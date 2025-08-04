package convert

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	networking "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
	"strings"
	"time"
)

// BuildModelServer creates arrays of ModelServer for the given model.
// Each model backend will create one model server.
func BuildModelServer(model *registry.Model) []*networking.ModelServer {
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
			return modelServers
		}
		var pdGroup *networking.PDGroup
		switch backend.Type {
		case registry.ModelBackendTypeVLLMDisaggregated, registry.ModelBackendTypeMindIEDisaggregated:
			pdGroup = &networking.PDGroup{
				GroupKey: workload.GroupName,
				PrefillLabels: map[string]string{
					workload.RoleLabelKey: "prefill",
				},
				DecodeLabels: map[string]string{
					workload.RoleLabelKey: "decode",
				},
			}
		}
		modelServer := networking.ModelServer{
			TypeMeta: metav1.TypeMeta{
				Kind:       networking.ModelServerKind,
				APIVersion: networking.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d-%s-server", model.Name, idx, strings.ToLower(string(backend.Type))),
				Namespace: model.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					utils.NewModelOwnerRef(model),
				},
			},
			Spec: networking.ModelServerSpec{
				Model:           &model.Name,
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
	return modelServers
}
