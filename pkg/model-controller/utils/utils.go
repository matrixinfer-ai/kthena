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

package utils

import (
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
	networking "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/config"
)

const (
	ModelInferOwnerKey             = "model.uid"
	CacheURIPrefixPVC              = "pvc://"
	CacheURIPrefixHostPath         = "hostpath://"
	URIPrefixSeparator             = "://"
	VllmTemplatePath               = "templates/vllm.yaml"
	VllmDisaggregatedTemplatePath  = "templates/vllm-pd.yaml"
	VllmMultiNodeServingScriptPath = "/vllm-workspace/vllm/examples/online_serving/multi-node-serving.sh"
	inClusterNamespacePath         = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

//go:embed templates/*
var templateFS embed.FS

var XPUList = []corev1.ResourceName{"nvidia.com/gpu", "huawei.com/ascend-1980"}

// BuildModelInferCR creates ModelInfer objects based on the model's backends.
func BuildModelInferCR(model *registry.Model) ([]*workload.ModelInfer, error) {
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
	modelDownloadPath := getCachePath(backend.CacheURI) + getMountPath(backend.ModelURI)

	// Build an initial container list including model downloader container
	initContainers := []corev1.Container{
		{
			Name:  model.Name + "-model-downloader",
			Image: config.Config.GetModelInferDownloaderImage(),
			Args: []string{
				"--source", backend.ModelURI,
				"--output-dir", modelDownloadPath,
			},
			Env:     getEnvVarOrDefault(backend, "ENDPOINT", ""),
			EnvFrom: backend.EnvFrom,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      cacheVolume.Name,
				MountPath: getCachePath(backend.CacheURI),
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
	data := map[string]interface{}{
		"MODEL_INFER_TEMPLATE_METADATA": &metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d-%s-instance", model.Name, idx, strings.ToLower(string(backend.Type))),
			Namespace: model.Namespace,
			Labels: map[string]string{
				ModelInferOwnerKey: string(model.UID),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: registry.GroupVersion.String(),
					Kind:       registry.ModelKind,
					Name:       model.Name,
					UID:        model.UID,
				},
			},
		},
		"VOLUME_MOUNTS": []corev1.VolumeMount{{
			Name:      cacheVolume.Name,
			MountPath: getCachePath(backend.CacheURI),
		}},
		"VOLUMES": []*corev1.Volume{
			cacheVolume,
		},
		"MODEL_NAME":                 model.Name,
		"BACKEND_REPLICAS":           backend.MinReplicas, // todo: backend replicas
		"INIT_CONTAINERS":            initContainers,
		"ENGINE_PREFILL_COMMAND":     preFillCommand,
		"ENGINE_DECODE_COMMAND":      decodeCommand,
		"MODEL_INFER_RUNTIME_IMAGE":  config.Config.GetModelInferRuntimeImage(),
		"MODEL_INFER_RUNTIME_PORT":   getEnvValueOrDefault(backend, "RUNTIME_PORT", "8100"),
		"MODEL_INFER_RUNTIME_URL":    getEnvValueOrDefault(backend, "RUNTIME_URL", "http://localhost:8000/metrics"),
		"MODEL_INFER_RUNTIME_ENGINE": strings.ToLower(string(backend.Type)),
		"PREFILL_REPLICAS":           workersMap[registry.ModelWorkerTypePrefill].Replicas,
		"DECODE_REPLICAS":            workersMap[registry.ModelWorkerTypeDecode].Replicas,
		"ENGINE_DECODE_RESOURCES":    workersMap[registry.ModelWorkerTypeDecode].Resources,
		"ENGINE_DECODE_IMAGE":        workersMap[registry.ModelWorkerTypeDecode].Image,
		"ENGINE_PREFILL_RESOURCES":   workersMap[registry.ModelWorkerTypePrefill].Resources,
		"ENGINE_PREFILL_IMAGE":       workersMap[registry.ModelWorkerTypePrefill].Image,
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
	modelDownloadPath := getCachePath(backend.CacheURI) + getMountPath(backend.ModelURI)
	// only one worker in such circumstance so get the first worker's config as commands
	commands, err := buildCommands(&backend.Workers[0].Config, modelDownloadPath, workersMap)
	if err != nil {
		return nil, err
	}

	// Build an initial container list including model downloader container
	initContainers := []corev1.Container{
		{
			Name:  model.Name + "-model-downloader",
			Image: config.Config.GetModelInferDownloaderImage(),
			Args: []string{
				"--source", backend.ModelURI,
				"--output-dir", modelDownloadPath,
			},
			Env:     getEnvVarOrDefault(backend, "ENDPOINT", ""),
			EnvFrom: backend.EnvFrom,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      cacheVolume.Name,
				MountPath: getCachePath(backend.CacheURI),
			}},
		},
	}

	// Handle LoRA adapters
	if len(backend.LoraAdapters) > 0 {
		loraCommands, loraContainers := buildLoraComponents(model, backend, cacheVolume.Name)
		commands = append(commands, loraCommands...)
		initContainers = append(initContainers, loraContainers...)
	}

	data := map[string]interface{}{
		"MODEL_INFER_TEMPLATE_METADATA": &metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d-%s-instance", model.Name, idx, strings.ToLower(string(backend.Type))),
			Namespace: model.Namespace,
			Labels: map[string]string{
				ModelInferOwnerKey: string(model.UID),
			},
			// model owns model infer. ModelInfer will be deleted when the model is deleted
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: registry.GroupVersion.String(),
					Kind:       registry.ModelKind,
					Name:       model.Name,
					UID:        model.UID,
				},
			},
		},
		"MODEL_NAME":       model.Name,
		"BACKEND_NAME":     strings.ToLower(backend.Name),
		"BACKEND_REPLICAS": backend.MinReplicas, // todo: backend replicas
		"BACKEND_TYPE":     strings.ToLower(string(backend.Type)),
		"ENGINE_ENV":       getEnvVarOrDefault(backend, "ENDPOINT", ""),
		"WORKER_ENV":       getEnvVarOrDefault(backend, "ENDPOINT", ""),
		"SERVER_REPLICAS":  workersMap[registry.ModelWorkerTypeServer].Replicas,
		"SERVER_ENTRY_TEMPLATE_METADATA": &metav1.ObjectMeta{
			Labels: map[string]string{
				ModelInferOwnerKey: string(model.UID),
			},
		},
		"SERVER_WORKER_TEMPLATE_METADATA": nil,
		"VOLUMES": []*corev1.Volume{
			cacheVolume,
		},
		"VOLUME_MOUNTS": []corev1.VolumeMount{{
			Name:      cacheVolume.Name,
			MountPath: getCachePath(backend.CacheURI),
		}},
		"INIT_CONTAINERS":            initContainers,
		"MODEL_INFER_RUNTIME_IMAGE":  config.Config.GetModelInferRuntimeImage(),
		"MODEL_INFER_RUNTIME_PORT":   getEnvValueOrDefault(backend, "RUNTIME_PORT", "8100"),
		"MODEL_INFER_RUNTIME_URL":    getEnvValueOrDefault(backend, "RUNTIME_URL", "http://localhost:8000/metrics"),
		"MODEL_INFER_RUNTIME_ENGINE": strings.ToLower(string(backend.Type)),
		"ENGINE_SERVER_RESOURCES":    workersMap[registry.ModelWorkerTypeServer].Resources,
		"ENGINE_SERVER_IMAGE":        workersMap[registry.ModelWorkerTypeServer].Image,
		"ENGINE_SERVER_COMMAND":      commands,
		"WORKER_REPLICAS":            workersMap[registry.ModelWorkerTypeServer].Pods - 1,
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
func buildCommands(config *apiextensionsv1.JSON, modelDownloadPath string,
	workersMap map[registry.ModelWorkerType]*registry.ModelWorker) ([]string, error) {
	commands := []string{"python", "-m", "vllm.entrypoints.openai.api_server", "--model", modelDownloadPath}
	args, err := parseArgs(config)
	commands = append(commands, args...)
	if workersMap[registry.ModelWorkerTypeServer] != nil && workersMap[registry.ModelWorkerTypeServer].Pods > 1 {
		commands = append(commands, "--distributed_executor_backend", "ray")
		commands = []string{"bash", "-c", fmt.Sprintf("chmod u+x %s && %s leader --ray_cluster_size=%d --num-gpus=%d && %s", VllmMultiNodeServingScriptPath, VllmMultiNodeServingScriptPath, workersMap[registry.ModelWorkerTypeServer].Pods, getDeviceNum(workersMap[registry.ModelWorkerTypeServer]), strings.Join(commands, " "))}
	}
	return commands, err
}

// getMountPath returns the mount path for the given ModelBackend in the format "/<backend.Name>".
func getMountPath(modelURI string) string {
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
					ClaimName: getCachePath(backend.CacheURI),
				},
			},
		}, nil
	case strings.HasPrefix(backend.CacheURI, CacheURIPrefixHostPath):
		return &corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: getCachePath(backend.CacheURI),
					Type: func() *corev1.HostPathType { typ := corev1.HostPathDirectoryOrCreate; return &typ }(),
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("not support prefix in CacheURI: %s", backend.CacheURI)
}

func getCachePath(path string) string {
	if path == "" || !strings.Contains(path, URIPrefixSeparator) {
		return ""
	}
	return strings.Split(path, URIPrefixSeparator)[1]
}

func getVolumeName(backendName string) string {
	return backendName + "-weights"
}

// getEnvVarOrDefault gets EnvVar of specific env, if env does not exist, return default value
func getEnvVarOrDefault(backend *registry.ModelBackend, name string, defaultValue string) []corev1.EnvVar {
	for _, env := range backend.Env {
		if env.Name == name {
			return []corev1.EnvVar{env}
		}
	}
	return []corev1.EnvVar{
		{Name: name, Value: defaultValue},
	}
}

// getEnvValueOrDefault gets string value of specific env, if env does not exist, return default value
func getEnvValueOrDefault(backend *registry.ModelBackend, name string, defaultValue string) string {
	for _, env := range backend.Env {
		if env.Name == name {
			return env.Value
		}
	}
	return defaultValue
}

func getDeviceNum(worker *registry.ModelWorker) int64 {
	sum := int64(0)
	if worker.Resources.Requests != nil {
		for _, xpu := range XPUList {
			if val, exists := worker.Resources.Requests[xpu]; exists {
				sum += val.Value()
			}
		}
	}
	return sum
}

func replacePlaceholders(data *interface{}, values *map[string]interface{}) error {
	switch v := (*data).(type) {
	case map[string]interface{}:
		for key, val := range v {
			if err := replacePlaceholders(&val, values); err != nil {
				return err
			}
			v[key] = val
		}
	case []interface{}:
		for i := range v {
			if err := replacePlaceholders(&v[i], values); err != nil {
				return err
			}
		}
	case string:
		if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
			key := strings.TrimSuffix(strings.TrimPrefix(v, "${"), "}")
			if val, exists := (*values)[key]; exists {
				*data = deepCopyValue(val)
				return replacePlaceholders(data, values)
			}
			return fmt.Errorf("not found placeholder: %s", key)
		} else if strings.Contains(v, "${") {
			newStr, err := replaceEmbeddedPlaceholders(v, values)
			if err != nil {
				return err
			}
			*data = newStr
		}
	}
	return nil
}

func deepCopyValue(src interface{}) interface{} {
	if src == nil {
		return nil
	}

	switch src.(type) {
	case string, bool, int, int32, int64, float32, float64:
		return src
	}

	bytes, err := json.Marshal(src)
	if err != nil {
		return src
	}

	var dest interface{}
	if err := json.Unmarshal(bytes, &dest); err != nil {
		return src
	}

	return dest
}

func replaceEmbeddedPlaceholders(s string, values *map[string]interface{}) (string, error) {
	var result strings.Builder
	pos := 0

	for {
		start := strings.Index(s[pos:], "${")
		if start == -1 {
			result.WriteString(s[pos:])
			break
		}
		start += pos

		end := strings.Index(s[start:], "}")
		if end == -1 {
			return "", fmt.Errorf("not found end } in: %s", s[start:])
		}
		end += start

		result.WriteString(s[pos:start])

		key := s[start+2 : end]

		if val, exists := (*values)[key]; exists {
			switch v := val.(type) {
			case string:
				result.WriteString(v)
			case int, int32, int64, float32, float64:
				result.WriteString(fmt.Sprintf("%v", v))
			case bool:
				result.WriteString(strconv.FormatBool(v))
			default:
				jsonBytes, err := json.Marshal(val)
				if err != nil {
					return "", fmt.Errorf("failed to marshal value to JSON: %w", err)
				}
				result.WriteString(string(jsonBytes))
			}
		} else {
			return "", fmt.Errorf("key not found: %s", key)
		}

		pos = end + 1
	}

	return result.String(), nil
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
	if err := replacePlaceholders(&jsonObj, data); err != nil {
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

func parseArgs(config *apiextensionsv1.JSON) ([]string, error) {
	if config == nil || config.Raw == nil {
		return []string{}, nil
	}
	var configMap map[string]interface{}
	if err := json.Unmarshal(config.Raw, &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	keys := make([]string, 0, len(configMap))
	for k := range configMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(configMap)*2)
	for _, key := range keys {
		value := configMap[key]

		keyStr := fmt.Sprintf("--%s", strings.ReplaceAll(key, "_", "-"))

		var strValue string
		switch v := value.(type) {
		case string:
			strValue = v
		case bool:
			strValue = fmt.Sprintf("%t", v)
		case json.Number:
			strValue = value.(json.Number).String()
		default:
			strValue = fmt.Sprintf("%v", v)
		}
		args = append(args, keyStr)
		if strValue != "" {
			args = append(args, strValue)
		}
	}

	return args, nil
}

// GetInClusterNameSpace gets the namespace of model controller
func GetInClusterNameSpace() (string, error) {
	if _, err := os.Stat(inClusterNamespacePath); os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, please specify namespace")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %v", err)
	}
	// Load the namespace file and return its content
	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %v", err)
	}
	return string(namespace), nil
}

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
				GroupKey: "modelinfer.matrixinfer.ai/group-name",
				PrefillLabels: map[string]string{
					"modelinfer.matrixinfer.ai/role": "prefill",
				},
				DecodeLabels: map[string]string{
					"modelinfer.matrixinfer.ai/role": "decode",
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
					{
						APIVersion: registry.GroupVersion.String(),
						Kind:       registry.ModelKind,
						Name:       model.Name,
						UID:        model.UID,
					},
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

// buildDownloaderContainer builds downloader container to reduce code duplication
func buildDownloaderContainer(name, image, source, outputDir string, backend *registry.ModelBackend, cacheVolumeName string) corev1.Container {
	return corev1.Container{
		Name:  name,
		Image: image,
		Args: []string{
			"--source", source,
			"--output-dir", outputDir,
		},
		Env:     getEnvVarOrDefault(backend, "ENDPOINT", ""),
		EnvFrom: backend.EnvFrom,
		VolumeMounts: []corev1.VolumeMount{{
			Name:      cacheVolumeName,
			MountPath: getCachePath(backend.CacheURI),
		}},
	}
}

// buildLoraComponents builds LoRA related commands and containers
func buildLoraComponents(model *registry.Model, backend *registry.ModelBackend, cacheVolumeName string) ([]string, []corev1.Container) {
	adapterCount := len(backend.LoraAdapters)
	loras := make([]string, 0, adapterCount)
	loraContainers := make([]corev1.Container, 0, adapterCount)

	for i, adapter := range backend.LoraAdapters {
		// Create LoRA downloader container
		containerName := fmt.Sprintf("%s-lora-downloader-%d", model.Name, i)
		outputDir := getCachePath(backend.CacheURI) + getMountPath(adapter.ArtifactURL)

		// Build LoRA module string
		loraModule := fmt.Sprintf("%s=%s", adapter.Name, outputDir)
		loras = append(loras, loraModule)

		loraContainer := buildDownloaderContainer(
			containerName,
			config.Config.GetModelInferDownloaderImage(),
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
