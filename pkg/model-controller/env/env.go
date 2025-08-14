package env

import (
	"reflect"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

// Package env provides constants for environment variables used in the model controller.
const (
	Endpoint           = "ENDPOINT"
	RuntimePort        = "RUNTIME_PORT"
	RuntimeUrl         = "RUNTIME_URL"
	RuntimeMetricsPath = "RUNTIME_METRICS_PATH"
)

// GetEnvValueOrDefault gets value of specific env, if env does not exist, return default value
// Supports conversion to string, int, []corev1.EnvVar, etc.
func GetEnvValueOrDefault[T any](backend *registry.ModelBackend, name string, defaultValue T) T {
	for _, env := range backend.Env {
		if env.Name == name {
			// Use reflection to convert string value to type T
			var result T
			v := reflect.ValueOf(&result).Elem()
			// Convert based on the target type kind
			switch v.Kind() {
			case reflect.String:
				v.SetString(env.Value)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if val, err := strconv.ParseInt(env.Value, 10, 64); err == nil {
					v.SetInt(val)
				} else {
					return defaultValue
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if val, err := strconv.ParseUint(env.Value, 10, 64); err == nil {
					v.SetUint(val)
				} else {
					return defaultValue
				}
			case reflect.Float32, reflect.Float64:
				if val, err := strconv.ParseFloat(env.Value, 64); err == nil {
					v.SetFloat(val)
				} else {
					return defaultValue
				}
			case reflect.Bool:
				if val, err := strconv.ParseBool(env.Value); err == nil {
					v.SetBool(val)
				} else {
					return defaultValue
				}
			case reflect.TypeOf([]corev1.EnvVar{}).Kind():
				// Create a slice containing the matched env
				slice := []corev1.EnvVar{env}
				reflect.ValueOf(&result).Elem().Set(reflect.ValueOf(slice))
				return result
			default:
				// For unsupported types, return default value
				return defaultValue
			}
			return result
		}
	}
	return defaultValue
}
