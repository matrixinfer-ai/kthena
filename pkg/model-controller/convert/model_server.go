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

package convert

import (
	"fmt"
	"time"

	networking "github.com/volcano-sh/kthena/pkg/apis/networking/v1alpha1"
	registry "github.com/volcano-sh/kthena/pkg/apis/registry/v1alpha1"
	workload "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	icUtils "github.com/volcano-sh/kthena/pkg/infer-controller/utils"
	"github.com/volcano-sh/kthena/pkg/model-controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var VLLMKvConnectorType = map[string]networking.KVConnectorType{
	"MooncakeConnectorV1": networking.ConnectorTypeMoonCake,
	"LMCacheConnectorV1":  networking.ConnectorTypeLMCache,
}

// BuildModelServer creates arrays of ModelServer for the given model.
// Each model backend will create one model server.
func BuildModelServer(model *registry.Model) ([]*networking.ModelServer, error) {
	var modelServers []*networking.ModelServer
	for _, backend := range model.Spec.Backends {
		var inferenceEngine networking.InferenceEngine
		switch backend.Type {
		case registry.ModelBackendTypeVLLM, registry.ModelBackendTypeVLLMDisaggregated:
			inferenceEngine = networking.VLLM
		default:
			return nil, fmt.Errorf("not support %s backend yet, please use vLLM backend", backend.Type)
		}
		servedModelName, err := getServedModelName(model, backend)
		if err != nil {
			return nil, err
		}
		pdGroup := getPdGroup(backend)
		kvConnector, err := getKvConnectorSpec(backend)
		if err != nil {
			return nil, err
		}
		modelServer := networking.ModelServer{
			TypeMeta: metav1.TypeMeta{
				Kind:       networking.ModelServerKind,
				APIVersion: networking.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.GetBackendResourceName(model.Name, backend.Name),
				Namespace: model.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					utils.NewModelOwnerRef(model),
				},
			},
			Spec: networking.ModelServerSpec{
				Model:           &servedModelName,
				InferenceEngine: inferenceEngine,
				WorkloadSelector: &networking.WorkloadSelector{
					MatchLabels: map[string]string{
						utils.OwnerUIDKey: string(model.UID),
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
				KVConnector: kvConnector,
			},
		}
		modelServer.Labels = utils.GetModelControllerLabels(model, backend.Name, icUtils.Revision(modelServer.Spec))
		modelServers = append(modelServers, &modelServer)
	}
	return modelServers, nil
}

func getKvConnectorSpec(backend registry.ModelBackend) (*networking.KVConnectorSpec, error) {
	var connectorType *networking.KVConnectorType
	foundConfig := false
	for _, worker := range backend.Workers {
		if worker.Type != registry.ModelWorkerTypePrefill && worker.Type != registry.ModelWorkerTypeDecode {
			continue
		}

		kvTransferConfig, err := utils.TryGetField(worker.Config.Raw, "kv-transfer-config")
		if err != nil {
			return nil, fmt.Errorf("failed to get kv-transfer-config for worker %s: %w", worker.Type, err)
		}
		if kvTransferConfig == nil {
			klog.Warningf("worker %s (backend %s) missing kv-transfer-config", worker.Type, backend.Name)
			continue
		}
		kvTransferConfigStr, ok := kvTransferConfig.(string)
		if !ok {
			klog.Warningf("invalid kv-transfer-config type %T for worker %s", kvTransferConfig, worker.Type)
			continue
		}

		kvTransferType, err := utils.TryGetField([]byte(kvTransferConfigStr), "kv_connector")
		if err != nil {
			klog.Warningf("invalid kv-transfer-config type %T for worker %s, str: %s", kvTransferConfig, worker.Type, kvTransferConfigStr)
			return nil, fmt.Errorf("failed to get kv_connector for worker %s: %w", worker.Type, err)
		}
		if kvTransferType == nil {
			klog.Warningf("worker %s (backend %s) missing kv_connector", worker.Type, backend.Name)
			continue
		}

		if converted, ok := kvTransferType.(string); ok {
			if ct, exists := VLLMKvConnectorType[converted]; exists {
				connectorType = &ct
				foundConfig = true
			} else {
				klog.Warningf("unknown kv_connector type %q for worker %s", converted, worker.Type)
			}
		} else {
			klog.Warningf("invalid kv_connector type %T for worker %s", kvTransferType, worker.Type)
		}
	}

	if foundConfig {
		return &networking.KVConnectorSpec{Type: *connectorType}, nil
	}
	return nil, nil
}

func getPdGroup(backend registry.ModelBackend) *networking.PDGroup {
	switch backend.Type {
	case registry.ModelBackendTypeVLLMDisaggregated, registry.ModelBackendTypeMindIEDisaggregated:
		return &networking.PDGroup{
			GroupKey: workload.GroupNameLabelKey,
			PrefillLabels: map[string]string{
				workload.RoleLabelKey: string(registry.ModelWorkerTypePrefill),
			},
			DecodeLabels: map[string]string{
				workload.RoleLabelKey: string(registry.ModelWorkerTypeDecode),
			},
		}
	}
	return nil
}

// getServedModelName gets served model name from the worker config. Default is the model name.
func getServedModelName(model *registry.Model, backend registry.ModelBackend) (string, error) {
	servedModelName := model.Name
	for _, worker := range backend.Workers {
		if worker.Type == registry.ModelWorkerTypeServer ||
			worker.Type == registry.ModelWorkerTypeDecode {
			valStr, err := utils.TryGetField(worker.Config.Raw, "served-model-name")
			if err != nil {
				return "", err
			}
			if valStr == nil {
				continue
			}
			if val, ok := valStr.(string); ok {
				servedModelName = val
				break
			}
		}
	}
	return servedModelName, nil
}
