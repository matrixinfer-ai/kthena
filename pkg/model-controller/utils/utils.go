package utils

import (
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/config"
)

const (
	ModelInferOwnerKey = "model.uid"

	CacheURIPrefixPVC      = "pvc://"
	CacheURIPrefixHostPath = "hostpath://"
	URIPrefixSeparator     = "://"

	VllmTemplatePath              = "templates/vllm.yaml"
	VllmDisaggregatedTemplatePath = "templates/vllm-pd.yaml"

	VllmMultiNodeServingScriptPath = "/vllm-workspace/vllm/examples/online_serving/multi-node-serving.sh"
)

func BuildModelInferCR(model *registry.Model) ([]*workload.ModelInfer, error) {
	infers := make([]*workload.ModelInfer, 0, len(model.Spec.Backends))
	for backendIdx, backend := range model.Spec.Backends {
		switch backend.Type {
		case registry.ModelBackendTypeVLLM:
			infer, err := buildVllmModelInfer(model, backendIdx)
			if err != nil {
				return nil, err
			}
			infers = append(infers, infer)
		case registry.ModelBackendTypeVLLMDisaggregated:
			infer, err := buildVllmDisaggregatedModelInfer(model, backendIdx)
			if err != nil {
				return nil, err
			}
			infers = append(infers, infer)
		default:
			return nil, fmt.Errorf("not support model backend type: %s", backend.Type)
		}
	}
	return infers, nil
}

func buildVllmDisaggregatedModelInfer(model *registry.Model, backendIdx int) (*workload.ModelInfer, error) {
	backend := &model.Spec.Backends[backendIdx]
	workersMap := make(map[registry.ModelWorkerType]*registry.ModelWorker, len(backend.Workers))
	for _, worker := range backend.Workers {
		workersMap[worker.Type] = &worker
	}

	data := map[string]interface{}{}
	// TODO: insert params into data map
	modelInfer, err := loadModelInferTemplate(VllmDisaggregatedTemplatePath, &data)
	if err != nil {
		return nil, err
	}
	return modelInfer, nil
}

func buildVllmModelInfer(model *registry.Model, backendIdx int) (*workload.ModelInfer, error) {
	backend := &model.Spec.Backends[backendIdx]
	workersMap := make(map[registry.ModelWorkerType]*registry.ModelWorker, len(backend.Workers))
	for _, worker := range backend.Workers {
		workersMap[worker.Type] = &worker
	}
	if workersMap[registry.ModelWorkerTypeServer] == nil {
		return nil, fmt.Errorf("not found server worker in backend: %s", backend.Name)
	}

	cacheVolume, err := buildCacheVolume(backend)
	if err != nil {
		return nil, err
	}

	weightsPath := getCachePath(backend.CacheURI) + getMountPath(backend.ModelURI)
	commands := []string{"python", "-m", "vllm.entrypoints.openai.api_server", "--model", weightsPath}
	args, err := parseArgs(&backend.Config)
	if err != nil {
		return nil, err
	}
	commands = append(commands, args...)

	if workersMap[registry.ModelWorkerTypeServer].Pods > 1 {
		commands = append(commands, "--distributed_executor_backend", "ray")
		commands = []string{"bash", "-c", fmt.Sprintf("chmod u+x %s && %s leader --ray_cluster_size=%d --num-gpus=%d && %s", VllmMultiNodeServingScriptPath, VllmMultiNodeServingScriptPath, workersMap[registry.ModelWorkerTypeServer].Pods, getDeviceNum(workersMap[registry.ModelWorkerTypeServer]), strings.Join(commands, " "))}
	}

	data := map[string]interface{}{
		"MODEL_INFER_TEMPLATE_METADATA": &metav1.ObjectMeta{
			Name:      model.Name + "-" + strconv.Itoa(backendIdx) + "-" + strings.ToLower(string(backend.Type)) + "-instance",
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
		"ENGINE_ENV":       backend.GetEnvVarOrDefault("ENDPOINT", ""),
		"WORKER_ENV":       backend.GetEnvVarOrDefault("ENDPOINT", ""),
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
		"MODEL_URL":                    backend.ModelURI,
		"MODEL_DOWNLOAD_PATH":          weightsPath,
		"MODEL_DOWNLOAD_ENV":           backend.GetEnvVarOrDefault("ENDPOINT", ""),
		"MODEL_DOWNLOAD_ENVFROM":       backend.EnvFrom,
		"MODEL_INFER_DOWNLOADER_IMAGE": config.Config.GetModelInferDownloaderImage(),
		"MODEL_INFER_RUNTIME_IMAGE":    config.Config.GetModelInferRuntimeImage(),
		"MODEL_INFER_RUNTIME_PORT":     backend.GetEnvValueOrDefault("RUNTIME_PORT", "8100"),
		"MODEL_INFER_RUNTIME_URL":      backend.GetEnvValueOrDefault("RUNTIME_URL", "http://localhost:8000/metrics"),
		"MODEL_INFER_RUNTIME_ENGINE":   strings.ToLower(string(backend.Type)),
		"ENGINE_SERVER_RESOURCES":      workersMap[registry.ModelWorkerTypeServer].Resources,
		"ENGINE_SERVER_IMAGE":          workersMap[registry.ModelWorkerTypeServer].Image,
		"ENGINE_SERVER_COMMAND":        commands,
		"WORKER_REPLICAS":              workersMap[registry.ModelWorkerTypeServer].Pods - 1,
	}

	modelInfer, err := loadModelInferTemplate(VllmTemplatePath, &data)
	if err != nil {
		return nil, err
	}
	return modelInfer, nil
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

//go:embed templates/*
var templateFS embed.FS

var XPUList = []corev1.ResourceName{"nvidia.com/gpu", "huawei.com/ascend-1980"}

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
