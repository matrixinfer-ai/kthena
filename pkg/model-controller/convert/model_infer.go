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
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/utils/ptr"
	icUtils "matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/env"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/config"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
)

const (
	CacheURIPrefixPVC              = "pvc://"
	CacheURIPrefixHostPath         = "hostpath://"
	URIPrefixSeparator             = "://"
	VllmTemplatePath               = "templates/vllm.yaml"
	VllmDisaggregatedTemplatePath  = "templates/vllm-pd.yaml"
	VllmMultiNodeServingScriptPath = "/vllm-workspace/vllm/examples/online_serving/multi-node-serving.sh"
	modelRouteRuleName             = "default"
)

//go:embed templates/*
var templateFS embed.FS

// BuildModelInfer creates ModelInfer objects based on the model's backends.
func BuildModelInfer(model *registry.Model) ([]*workload.ModelInfer, error) {
	var infers []*workload.ModelInfer
	for idx, backend := range model.Spec.Backends {
		var infer *workload.ModelInfer
		var err error
		switch backend.Type {
		case registry.ModelBackendTypeVLLM:
			infer, err = buildVllmModelInfer(model, idx)
		case registry.ModelBackendTypeVLLMDisaggregated:
			infer, err = buildVllmDisaggregatedModelInfer(model, idx)
		default:
			return nil, fmt.Errorf("not support model backend type: %s", backend.Type)
		}
		if err != nil {
			return nil, err
		}
		infers = append(infers, infer)
	}
	return infers, nil
}

// buildVllmDisaggregatedModelInfer handles VLLM disaggregated backend creation.
func buildVllmDisaggregatedModelInfer(model *registry.Model, idx int) (*workload.ModelInfer, error) {
	backend := &model.Spec.Backends[idx]
	workersMap := mapWorkers(backend.Workers)
	if workersMap[registry.ModelWorkerTypePrefill] == nil {
		return nil, fmt.Errorf("prefill worker not found in backend: %s", backend.Name)
	}
	if workersMap[registry.ModelWorkerTypeDecode] == nil {
		return nil, fmt.Errorf("decode worker not found in backend: %s", backend.Name)
	}
	cacheVolume, err := buildCacheVolume(backend)
	if err != nil {
		return nil, err
	}
	modelDownloadPath := GetCachePath(backend.CacheURI) + GetMountPath(backend.ModelURI)

	// Build an initial container list including model downloader container
	initContainers := []corev1.Container{
		{
			Name:  model.Name + "-model-downloader",
			Image: config.Config.ModelInferDownloaderImage(),
			Args: []string{
				"--source", backend.ModelURI,
				"--output-dir", modelDownloadPath,
			},
			Env: env.GetEnvValueOrDefault[[]corev1.EnvVar](backend, env.Endpoint, []corev1.EnvVar{
				{Name: env.Endpoint, Value: ""},
			}),
			EnvFrom: backend.EnvFrom,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      cacheVolume.Name,
				MountPath: GetCachePath(backend.CacheURI),
			}},
		},
	}

	var preFillCommand []string
	var decodeCommand []string
	for _, worker := range backend.Workers {
		if worker.Type == registry.ModelWorkerTypePrefill {
			preFillCommand, err = buildCommands(&worker.Config, modelDownloadPath, workersMap)
			if err != nil {
				return nil, err
			}
		} else if worker.Type == registry.ModelWorkerTypeDecode {
			decodeCommand, err = buildCommands(&worker.Config, modelDownloadPath, workersMap)
			if err != nil {
				return nil, err
			}
		}
	}

	// Handle LoRA adapters
	if len(backend.LoraAdapters) > 0 {
		loraCommands, loraContainers := buildLoraComponents(model, backend, cacheVolume.Name)
		preFillCommand = append(preFillCommand, loraCommands...)
		decodeCommand = append(decodeCommand, loraCommands...)
		initContainers = append(initContainers, loraContainers...)
	}

	prefillEngineEnv := buildEngineEnvVars(backend,
		corev1.EnvVar{Name: "HF_HUB_OFFLINE", Value: "1"},
		corev1.EnvVar{Name: "HCCL_IF_IP", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
		}},
	)

	decodeEngineEnv := buildEngineEnvVars(backend,
		corev1.EnvVar{Name: "HF_HUB_OFFLINE", Value: "1"},
		corev1.EnvVar{Name: "GLOO_SOCKET_IFNAME", Value: "eth0"},
		corev1.EnvVar{Name: "TP_SOCKET_IFNAME", Value: "eth0"},
		corev1.EnvVar{Name: "HCCL_SOCKET_IFNAME", Value: "eth0"},
	)

	data := map[string]interface{}{
		"MODEL_INFER_TEMPLATE_METADATA": &metav1.ObjectMeta{
			Name:      utils.GetBackendResourceName(model.Name, backend.Name),
			Namespace: model.Namespace,
			Labels:    utils.GetModelControllerLabels(model, backend.Name, icUtils.Revision(backend)),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: registry.GroupVersion.String(),
					Kind:       registry.ModelKind.Kind,
					Name:       model.Name,
					UID:        model.UID,
				},
			},
		},
		"VOLUME_MOUNTS": []corev1.VolumeMount{{
			Name:      cacheVolume.Name,
			MountPath: GetCachePath(backend.CacheURI),
		}},
		"VOLUMES": []*corev1.Volume{
			cacheVolume,
		},
		"MODEL_NAME":                       model.Name,
		"BACKEND_REPLICAS":                 backend.MinReplicas, // todo: backend replicas
		"INIT_CONTAINERS":                  initContainers,
		"MODEL_DOWNLOAD_ENVFROM":           backend.EnvFrom,
		"ENGINE_PREFILL_COMMAND":           preFillCommand,
		"ENGINE_DECODE_COMMAND":            decodeCommand,
		"MODEL_INFER_RUNTIME_IMAGE":        config.Config.ModelInferRuntimeImage(),
		"MODEL_INFER_RUNTIME_PORT":         env.GetEnvValueOrDefault[int32](backend, env.RuntimePort, 8100),
		"MODEL_INFER_RUNTIME_URL":          env.GetEnvValueOrDefault[string](backend, env.RuntimeUrl, "http://localhost:8000"),
		"MODEL_INFER_RUNTIME_METRICS_PATH": env.GetEnvValueOrDefault[string](backend, env.RuntimeMetricsPath, "/metrics"),
		"ENGINE_PREFILL_ENV":               prefillEngineEnv,
		"ENGINE_DECODE_ENV":                decodeEngineEnv,
		"MODEL_INFER_RUNTIME_ENGINE":       strings.ToLower(string(backend.Type)),
		"MODEL_INFER_RUNTIME_POD":          "$(POD_NAME).$(NAMESPACE).svc.cluster.local",
		"PREFILL_REPLICAS":                 workersMap[registry.ModelWorkerTypePrefill].Replicas,
		"DECODE_REPLICAS":                  workersMap[registry.ModelWorkerTypeDecode].Replicas,
		"ENGINE_DECODE_RESOURCES":          workersMap[registry.ModelWorkerTypeDecode].Resources,
		"ENGINE_DECODE_IMAGE":              workersMap[registry.ModelWorkerTypeDecode].Image,
		"ENGINE_PREFILL_RESOURCES":         workersMap[registry.ModelWorkerTypePrefill].Resources,
		"ENGINE_PREFILL_IMAGE":             workersMap[registry.ModelWorkerTypePrefill].Image,
	}
	return loadModelInferTemplate(VllmDisaggregatedTemplatePath, &data)
}

// buildVllmModelInfer handles VLLM backend creation.
func buildVllmModelInfer(model *registry.Model, idx int) (*workload.ModelInfer, error) {
	backend := &model.Spec.Backends[idx]
	workersMap := mapWorkers(backend.Workers)
	if workersMap[registry.ModelWorkerTypeServer] == nil {
		return nil, fmt.Errorf("server worker not found in backend: %s", backend.Name)
	}
	cacheVolume, err := buildCacheVolume(backend)
	if err != nil {
		return nil, err
	}
	modelDownloadPath := GetCachePath(backend.CacheURI) + GetMountPath(backend.ModelURI)
	// only one worker in such circumstance so get the first worker's config as commands
	commands, err := buildCommands(&backend.Workers[0].Config, modelDownloadPath, workersMap)
	if err != nil {
		return nil, err
	}

	// Build an initial container list including model downloader container
	initContainers := []corev1.Container{
		{
			Name:  model.Name + "-model-downloader",
			Image: config.Config.ModelInferDownloaderImage(),
			Args: []string{
				"--source", backend.ModelURI,
				"--output-dir", modelDownloadPath,
			},
			Env: env.GetEnvValueOrDefault[[]corev1.EnvVar](backend, env.Endpoint, []corev1.EnvVar{
				{Name: env.Endpoint, Value: ""},
			}),
			EnvFrom: backend.EnvFrom,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      cacheVolume.Name,
				MountPath: GetCachePath(backend.CacheURI),
			}},
		},
	}

	// Handle LoRA adapters
	if len(backend.LoraAdapters) > 0 {
		loraCommands, loraContainers := buildLoraComponents(model, backend, cacheVolume.Name)
		commands = append(commands, loraCommands...)
		initContainers = append(initContainers, loraContainers...)
	}
	engineEnv := buildEngineEnvVars(backend)
	data := map[string]interface{}{
		"MODEL_INFER_TEMPLATE_METADATA": &metav1.ObjectMeta{
			Name:      utils.GetBackendResourceName(model.Name, backend.Name),
			Namespace: model.Namespace,
			Labels:    utils.GetModelControllerLabels(model, backend.Name, icUtils.Revision(backend)),
			// model owns model infer. ModelInfer will be deleted when the model is deleted
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: registry.GroupVersion.String(),
					Kind:       registry.ModelKind.Kind,
					Name:       model.Name,
					UID:        model.UID,
				},
			},
		},
		"MODEL_NAME":       model.Name,
		"BACKEND_NAME":     strings.ToLower(backend.Name),
		"BACKEND_REPLICAS": backend.MinReplicas, // todo: backend replicas
		"BACKEND_TYPE":     strings.ToLower(string(backend.Type)),
		"ENGINE_ENV":       engineEnv,
		"WORKER_ENV":       backend.Env,
		"SERVER_REPLICAS":  workersMap[registry.ModelWorkerTypeServer].Replicas,
		"SERVER_ENTRY_TEMPLATE_METADATA": &metav1.ObjectMeta{
			Labels: utils.GetModelControllerLabels(model, backend.Name, icUtils.Revision(backend)),
		},
		"SERVER_WORKER_TEMPLATE_METADATA": nil,
		"VOLUMES": []*corev1.Volume{
			cacheVolume,
		},
		"VOLUME_MOUNTS": []corev1.VolumeMount{{
			Name:      cacheVolume.Name,
			MountPath: GetCachePath(backend.CacheURI),
		}},
		"INIT_CONTAINERS":                  initContainers,
		"MODEL_DOWNLOAD_ENVFROM":           backend.EnvFrom,
		"MODEL_INFER_RUNTIME_IMAGE":        config.Config.ModelInferRuntimeImage(),
		"MODEL_INFER_RUNTIME_PORT":         env.GetEnvValueOrDefault[int32](backend, env.RuntimePort, 8100),
		"MODEL_INFER_RUNTIME_URL":          env.GetEnvValueOrDefault[string](backend, env.RuntimeUrl, "http://localhost:8000"),
		"MODEL_INFER_RUNTIME_METRICS_PATH": env.GetEnvValueOrDefault[string](backend, env.RuntimeMetricsPath, "/metrics"),
		"MODEL_INFER_RUNTIME_ENGINE":       strings.ToLower(string(backend.Type)),
		"MODEL_INFER_RUNTIME_POD":          "$(POD_NAME).$(NAMESPACE).svc.cluster.local",
		"ENGINE_SERVER_RESOURCES":          workersMap[registry.ModelWorkerTypeServer].Resources,
		"ENGINE_SERVER_IMAGE":              workersMap[registry.ModelWorkerTypeServer].Image,
		"ENGINE_SERVER_COMMAND":            commands,
		"WORKER_REPLICAS":                  workersMap[registry.ModelWorkerTypeServer].Pods - 1,
	}
	return loadModelInferTemplate(VllmTemplatePath, &data)
}

// mapWorkers creates a map of workers by type.
func mapWorkers(workers []registry.ModelWorker) map[registry.ModelWorkerType]*registry.ModelWorker {
	workersMap := make(map[registry.ModelWorkerType]*registry.ModelWorker, len(workers))
	for _, worker := range workers {
		workersMap[worker.Type] = &worker
	}
	return workersMap
}

// buildCommands constructs the command list for the backend.
func buildCommands(workerConfig *apiextensionsv1.JSON, modelDownloadPath string,
	workersMap map[registry.ModelWorkerType]*registry.ModelWorker) ([]string, error) {
	commands := []string{"python", "-m", "vllm.entrypoints.openai.api_server", "--model", modelDownloadPath, "--enable-lora"}
	args, err := utils.ParseArgs(workerConfig)
	commands = append(commands, args...)
	if workersMap[registry.ModelWorkerTypeServer] != nil && workersMap[registry.ModelWorkerTypeServer].Pods > 1 {
		commands = append(commands, "--distributed_executor_backend", "ray")
		commands = []string{"bash", "-c", fmt.Sprintf("chmod u+x %s && %s leader --ray_cluster_size=%d --num-gpus=%d && %s", VllmMultiNodeServingScriptPath, VllmMultiNodeServingScriptPath, workersMap[registry.ModelWorkerTypeServer].Pods, utils.GetDeviceNum(workersMap[registry.ModelWorkerTypeServer]), strings.Join(commands, " "))}
	}
	commands = append(commands, "--kv-events-config", config.GetDefaultKVEventsConfig())
	commands = append(commands, "--enforce-eager")
	return commands, err
}

// GetMountPath returns the mount path for the given ModelBackend in the format "/<backend.Name>".
func GetMountPath(modelURI string) string {
	h := md5.New()
	h.Write([]byte(modelURI))
	hashBytes := h.Sum(nil)
	hashHex := hex.EncodeToString(hashBytes)
	return "/" + hashHex
}

func buildCacheVolume(backend *registry.ModelBackend) (*corev1.Volume, error) {
	volumeName := getVolumeName(backend.Name)
	switch {
	case backend.CacheURI == "":
		return &corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}, nil
	case strings.HasPrefix(backend.CacheURI, CacheURIPrefixPVC):
		return &corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: GetCachePath(backend.CacheURI),
				},
			},
		}, nil
	case strings.HasPrefix(backend.CacheURI, CacheURIPrefixHostPath):
		return &corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: GetCachePath(backend.CacheURI),
					Type: ptr.To(corev1.HostPathDirectoryOrCreate),
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("not support prefix in CacheURI: %s", backend.CacheURI)
}

func GetCachePath(path string) string {
	if path == "" || !strings.Contains(path, URIPrefixSeparator) {
		return ""
	}
	return strings.Split(path, URIPrefixSeparator)[1]
}

func getVolumeName(backendName string) string {
	return backendName + "-weights"
}

// loadModelInferTemplate loads and processes the template file.
func loadModelInferTemplate(templatePath string, data *map[string]interface{}) (*workload.ModelInfer, error) {
	templateBytes, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}

	var jsonObj interface{}
	if err := yaml.Unmarshal(templateBytes, &jsonObj); err != nil {
		return nil, fmt.Errorf("YAML template parse failed: %w", err)
	}
	if err := utils.ReplacePlaceholders(&jsonObj, data); err != nil {
		return nil, fmt.Errorf("replace placeholders failed: %v", err)
	}

	replacedJsonBytes, err := json.Marshal(jsonObj)
	if err != nil {
		return nil, fmt.Errorf("JSON parse failed with replaced json bytes: %w", err)
	}

	modelInfer := &workload.ModelInfer{}
	reader := bytes.NewReader(replacedJsonBytes)
	decoder := yaml.NewYAMLOrJSONDecoder(reader, 1024)
	if err := decoder.Decode(modelInfer); err != nil {
		return nil, fmt.Errorf("model infer parse json failed : %w", err)
	}

	return modelInfer, nil
}

// buildDownloaderContainer builds downloader container to reduce code duplication
func buildDownloaderContainer(name, image, source, outputDir string, backend *registry.ModelBackend, cacheVolumeName string) corev1.Container {
	return corev1.Container{
		Name:  name,
		Image: image,
		Args: []string{
			"--source", source,
			"--output-dir", outputDir,
		},
		Env:     backend.Env,
		EnvFrom: backend.EnvFrom,
		VolumeMounts: []corev1.VolumeMount{{
			Name:      cacheVolumeName,
			MountPath: GetCachePath(backend.CacheURI),
		}},
	}
}

func buildEngineEnvVars(backend *registry.ModelBackend, additionalEnvs ...corev1.EnvVar) []corev1.EnvVar {
	standardEnvs := []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
			},
		},
		{Name: "VLLM_USE_V1", Value: "1"},
	}
	return append(append(append([]corev1.EnvVar(nil), backend.Env...), standardEnvs...), additionalEnvs...)
}

// buildLoraComponents builds LoRA related commands and containers
func buildLoraComponents(model *registry.Model, backend *registry.ModelBackend, cacheVolumeName string) ([]string, []corev1.Container) {
	adapterCount := len(backend.LoraAdapters)
	loras := make([]string, 0, adapterCount)
	loraContainers := make([]corev1.Container, 0, adapterCount)

	for i, adapter := range backend.LoraAdapters {
		// Create LoRA downloader container
		containerName := fmt.Sprintf("%s-lora-downloader-%d", model.Name, i)
		outputDir := GetCachePath(backend.CacheURI) + GetMountPath(adapter.ArtifactURL)

		// Build LoRA module string
		loraModule := fmt.Sprintf("%s=%s", adapter.Name, outputDir)
		loras = append(loras, loraModule)

		loraContainer := buildDownloaderContainer(
			containerName,
			config.Config.ModelInferDownloaderImage(),
			adapter.ArtifactURL,
			outputDir,
			backend,
			cacheVolumeName,
		)
		loraContainers = append(loraContainers, loraContainer)
	}

	// Build LoRA command arguments
	loraCommands := []string{"--enable-lora", "--lora-modules", strings.Join(loras, " ")}

	return loraCommands, loraContainers
}
