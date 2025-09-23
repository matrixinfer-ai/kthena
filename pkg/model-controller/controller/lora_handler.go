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

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	registryv1alpha1 "github.com/volcano-sh/kthena/pkg/apis/registry/v1alpha1"
	workload "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	"github.com/volcano-sh/kthena/pkg/model-controller/convert"
	"github.com/volcano-sh/kthena/pkg/model-controller/env"
	"github.com/volcano-sh/kthena/pkg/model-controller/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

// getPreviousModelVersion gets the previous version of the model from cache for comparison
func (mc *ModelController) getPreviousModelVersion(model *registryv1alpha1.Model) (*registryv1alpha1.Model, error) {
	cacheKey := fmt.Sprintf("%s/%s:%d", model.Namespace, model.Name, model.Generation)

	// Get the previous model from cache
	oldModel, exists := mc.loraUpdateCache[cacheKey]
	if !exists {
		klog.Warningf("Get previous Model version failed: %s", cacheKey)
		return nil, nil
	}

	return oldModel, nil
}

// hasOnlyLoraAdaptersChanged checks if only LoRA adapters have changed between old and new model
func (mc *ModelController) hasOnlyLoraAdaptersChanged(oldModel, newModel *registryv1alpha1.Model) bool {
	if len(oldModel.Spec.Backends) != len(newModel.Spec.Backends) {
		return false
	}

	hasLoraChanges := false

	for i, newBackend := range newModel.Spec.Backends {
		oldBackend := oldModel.Spec.Backends[i]

		// Create copies without LoraAdapters for comparison
		oldBackendCopy := oldBackend.DeepCopy()
		newBackendCopy := newBackend.DeepCopy()
		oldBackendCopy.LoraAdapters = nil
		newBackendCopy.LoraAdapters = nil

		// If anything other than LoraAdapters changed, return false
		if !reflect.DeepEqual(oldBackendCopy, newBackendCopy) {
			return false
		}

		// Check if LoraAdapters changed for VLLM backends
		if newBackend.Type == registryv1alpha1.ModelBackendTypeVLLM {
			if !reflect.DeepEqual(oldBackend.LoraAdapters, newBackend.LoraAdapters) {
				hasLoraChanges = true
			}
		}
	}

	return hasLoraChanges
}

// isModelInferReady checks if ModelInfer is ready for LoRA adapter updates
func (mc *ModelController) isModelInferReady(modelInfer *workload.ModelServing) bool {
	return modelInfer.Status.AvailableReplicas > 0
}

// LoraUpdateResult tracks the result of LoRA adapter updates across multiple replicas
type LoraUpdateResult struct {
	TotalReplicas   int
	SuccessReplicas int
	FailedReplicas  int
	PartialFailures []string // URLs that failed
	Errors          []error  // Corresponding errors
}

// updateLoraAdapters updates LoRA adapters for a specific backend across all replicas
func (mc *ModelController) updateLoraAdapters(ctx context.Context, newBackend, oldBackend *registryv1alpha1.ModelBackend, modelInfer *workload.ModelServing) error {
	// Get runtime service URLs for all replicas
	runtimeURLs, err := mc.getModelInferRuntimeURLs(modelInfer, newBackend)
	if err != nil {
		return fmt.Errorf("failed to get runtime URLs for ModelInfer %s: %v", modelInfer.Name, err)
	}

	klog.Infof("Updating LoRA adapters for ModelInfer %s across %d replicas", modelInfer.Name, len(runtimeURLs))

	// Prepare adapter maps for comparison
	oldAdapterMap := make(map[string]registryv1alpha1.LoraAdapter)
	for _, adapter := range oldBackend.LoraAdapters {
		oldAdapterMap[adapter.Name] = adapter
	}

	newAdapterMap := make(map[string]registryv1alpha1.LoraAdapter)
	for _, adapter := range newBackend.LoraAdapters {
		newAdapterMap[adapter.Name] = adapter
	}

	// Track overall results
	var overallResult LoraUpdateResult
	overallResult.TotalReplicas = len(runtimeURLs)

	// Phase 1: Unload adapters that are no longer needed
	adaptersToUnload := make([]string, 0)
	for adapterName := range oldAdapterMap {
		if _, exists := newAdapterMap[adapterName]; !exists {
			adaptersToUnload = append(adaptersToUnload, adapterName)
		}
	}

	if len(adaptersToUnload) > 0 {
		klog.Infof("Unloading %d LoRA adapters: %v", len(adaptersToUnload), adaptersToUnload)
		unloadResult := mc.unloadLoraAdaptersFromAllReplicas(ctx, runtimeURLs, adaptersToUnload)
		if unloadResult.FailedReplicas > 0 {
			klog.Warningf("Failed to unload LoRA adapters from %d/%d replicas", unloadResult.FailedReplicas, unloadResult.TotalReplicas)
			// Log errors but continue with loading new adapters
			for i, failedURL := range unloadResult.PartialFailures {
				klog.Errorf("Failed to unload from %s: %v", failedURL, unloadResult.Errors[i])
			}
		}
	}

	// Phase 2: Load new or updated adapters
	adaptersToLoad := make([]registryv1alpha1.LoraAdapter, 0)
	for _, adapter := range newBackend.LoraAdapters {
		oldAdapter, existed := oldAdapterMap[adapter.Name]
		// Load adapter if it's new or if the artifact URL changed
		if !existed || oldAdapter.ArtifactURL != adapter.ArtifactURL {
			adaptersToLoad = append(adaptersToLoad, adapter)
		}
	}

	if len(adaptersToLoad) > 0 {
		klog.Infof("Loading %d LoRA adapters", len(adaptersToLoad))
		loadResult := mc.loadLoraAdaptersToAllReplicas(ctx, runtimeURLs, adaptersToLoad, newBackend)
		overallResult = loadResult

		// Check if we have critical failures
		if loadResult.FailedReplicas > 0 {
			// Log warnings for partial failures
			klog.Warningf("Partial failure: LoRA adapter loading failed on %d/%d replicas",
				loadResult.FailedReplicas, loadResult.TotalReplicas)
			for i, failedURL := range loadResult.PartialFailures {
				klog.Errorf("Failed to load to %s: %v", failedURL, loadResult.Errors[i])
			}
		}
	}

	klog.Infof("LoRA adapter update completed for ModelInfer %s: %d/%d replicas successful",
		modelInfer.Name, overallResult.SuccessReplicas, overallResult.TotalReplicas)
	return nil
}

// unloadLoraAdaptersFromAllReplicas unloads LoRA adapters from all replicas
func (mc *ModelController) unloadLoraAdaptersFromAllReplicas(ctx context.Context, runtimeURLs []string, adapterNames []string) LoraUpdateResult {
	result := LoraUpdateResult{
		TotalReplicas:   len(runtimeURLs),
		PartialFailures: make([]string, 0),
		Errors:          make([]error, 0),
	}

	for _, runtimeURL := range runtimeURLs {
		replicaSuccess := true
		for _, adapterName := range adapterNames {
			if err := mc.unloadLoraAdapter(ctx, runtimeURL, adapterName); err != nil {
				klog.Errorf("Failed to unload LoRA adapter %s from %s: %v", adapterName, runtimeURL, err)
				if replicaSuccess {
					// Only record the replica as failed once
					result.PartialFailures = append(result.PartialFailures, runtimeURL)
					result.Errors = append(result.Errors, fmt.Errorf("failed to unload adapter %s: %v", adapterName, err))
					replicaSuccess = false
				}
			}
		}

		if replicaSuccess {
			result.SuccessReplicas++
		} else {
			result.FailedReplicas++
		}
	}

	return result
}

// loadLoraAdaptersToAllReplicas loads LoRA adapters to all replicas
func (mc *ModelController) loadLoraAdaptersToAllReplicas(ctx context.Context, runtimeURLs []string, adapters []registryv1alpha1.LoraAdapter, backend *registryv1alpha1.ModelBackend) LoraUpdateResult {
	result := LoraUpdateResult{
		TotalReplicas:   len(runtimeURLs),
		PartialFailures: make([]string, 0),
		Errors:          make([]error, 0),
	}

	for _, runtimeURL := range runtimeURLs {
		replicaSuccess := true
		for _, adapter := range adapters {
			if err := mc.loadLoraAdapter(ctx, runtimeURL, adapter, backend); err != nil {
				klog.Errorf("Failed to load LoRA adapter %s to %s: %v", adapter.Name, runtimeURL, err)
				if replicaSuccess {
					// Only record the replica as failed once
					result.PartialFailures = append(result.PartialFailures, runtimeURL)
					result.Errors = append(result.Errors, fmt.Errorf("failed to load adapter %s: %v", adapter.Name, err))
					replicaSuccess = false
				}
			}
		}

		if replicaSuccess {
			result.SuccessReplicas++
		} else {
			result.FailedReplicas++
		}
	}

	return result
}

// getModelInferRuntimeURLs constructs the runtime service URLs for all ModelInfer pods
func (mc *ModelController) getModelInferRuntimeURLs(modelInfer *workload.ModelServing, backend *registryv1alpha1.ModelBackend) ([]string, error) {
	// Get port from backend environment variables with default fallback to 8100
	port := env.GetEnvValueOrDefault[int32](backend, env.RuntimePort, 8100)

	// Get all available pod IPs for this ModelInfer
	podIPs, err := mc.getModelInferPodIPs(modelInfer)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod IPs for ModelInfer %s: %v", modelInfer.Name, err)
	}

	var runtimeURLs []string
	for _, podIP := range podIPs {
		runtimeURLs = append(runtimeURLs, fmt.Sprintf("http://%s:%d", podIP, port))
	}

	return runtimeURLs, nil
}

// getModelInferPodIPs gets the IPs of all available pods for a ModelInfer
func (mc *ModelController) getModelInferPodIPs(modelInfer *workload.ModelServing) ([]string, error) {
	// Use PodLister to get pods with the ModelInfer label
	labelSelector := labels.SelectorFromSet(labels.Set{
		workload.ModelServingNameLabelKey: modelInfer.Name,
	})

	podList, err := mc.podsLister.Pods(modelInfer.Namespace).List(labelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	if len(podList) == 0 {
		return nil, fmt.Errorf("no pods found for ModelInfer %s", modelInfer.Name)
	}

	// Collect all running pod IPs
	var podIPs []string
	for _, pod := range podList {
		if pod.Status.Phase == corev1.PodRunning && pod.Status.PodIP != "" {
			podIPs = append(podIPs, pod.Status.PodIP)
		}
	}

	if len(podIPs) == 0 {
		return nil, fmt.Errorf("no running pods with IP found for ModelInfer %s", modelInfer.Name)
	}

	return podIPs, nil
}

// loadLoraAdapter calls the load_lora_adapter API
func (mc *ModelController) loadLoraAdapter(ctx context.Context, runtimeURL string, adapter registryv1alpha1.LoraAdapter, backend *registryv1alpha1.ModelBackend) error {
	url := fmt.Sprintf("%s/v1/load_lora_adapter", runtimeURL)
	outputDir := convert.GetCachePath(backend.CacheURI) + convert.GetMountPath(adapter.ArtifactURL)

	requestBody := map[string]interface{}{
		"lora_name":      adapter.Name,
		"source":         adapter.ArtifactURL,
		"output_dir":     outputDir,
		"async_download": false, // Use synchronous download for better error handling
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	klog.Infof("Loading LoRA adapter %s from %s", adapter.Name, adapter.ArtifactURL)

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			klog.Errorf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		// Read response body to get detailed error message
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			klog.Errorf("Failed to read response body: %v", err)
			return fmt.Errorf("load LoRA adapter failed with status code: %d", resp.StatusCode)
		}

		errorDetail := string(bodyBytes)
		klog.Errorf("Load LoRA adapter failed - Status: %d, Response: %s", resp.StatusCode, errorDetail)
		return fmt.Errorf("load LoRA adapter failed with status code: %d, error: %s", resp.StatusCode, errorDetail)
	}

	klog.Infof("Successfully loaded LoRA adapter %s", adapter.Name)
	return nil
}

// unloadLoraAdapter calls the unload_lora_adapter API
func (mc *ModelController) unloadLoraAdapter(ctx context.Context, runtimeURL string, adapterName string) error {
	url := fmt.Sprintf("%s/v1/unload_lora_adapter", runtimeURL)

	requestBody := map[string]interface{}{
		"lora_name": adapterName,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	klog.Infof("Unloading LoRA adapter %s", adapterName)

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			klog.Errorf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		// Read response body to get detailed error message
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			klog.Errorf("Failed to read response body: %v", err)
			return fmt.Errorf("unload LoRA adapter failed with status code: %d", resp.StatusCode)
		}

		errorDetail := string(bodyBytes)
		klog.Errorf("Unload LoRA adapter failed - Status: %d, Response: %s", resp.StatusCode, errorDetail)
		return fmt.Errorf("unload LoRA adapter failed with status code: %d, error: %s", resp.StatusCode, errorDetail)
	}

	klog.Infof("Successfully unloaded LoRA adapter %s", adapterName)
	return nil
}

// cleanupOutdatedLoraUpdateCache removes all cache entries for the specified model
// that have a generation less than the current model generation
func (mc *ModelController) cleanupOutdatedLoraUpdateCache(model *registryv1alpha1.Model) {
	modelPrefix := fmt.Sprintf("%s/%s:", model.Namespace, model.Name)
	currentGeneration := model.Generation

	keysToDelete := make([]string, 0)
	for key := range mc.loraUpdateCache {
		if strings.HasPrefix(key, modelPrefix) {
			// Extract generation from the cache key "namespace/name:generation"
			parts := strings.Split(key, ":")
			if len(parts) >= 2 {
				generationStr := parts[len(parts)-1]
				if generation, err := strconv.ParseInt(generationStr, 10, 64); err == nil {
					if generation < currentGeneration {
						keysToDelete = append(keysToDelete, key)
					}
				}
			}
		}
	}

	for _, key := range keysToDelete {
		delete(mc.loraUpdateCache, key)
		klog.V(4).Infof("Cleaned up outdated LoRA update cache entry: %s", key)
	}
}

// getDynamicLoraUpdateBackends returns a list of backend names that can use dynamic LoRA updates
func (mc *ModelController) getDynamicLoraUpdateBackends(oldModel, newModel *registryv1alpha1.Model) []string {
	var dynamicUpdateBackends []string

	// Create maps for easier lookup by backend name
	oldBackendMap := make(map[string]*registryv1alpha1.ModelBackend)

	for i := range oldModel.Spec.Backends {
		backend := &oldModel.Spec.Backends[i]
		oldBackendMap[backend.Name] = backend
	}

	for i := range newModel.Spec.Backends {
		newBackend := &newModel.Spec.Backends[i]
		oldBackend, exists := oldBackendMap[newBackend.Name]
		if !exists {
			// New backend, skip dynamic update
			continue
		}

		// Check if only LoRA adapters changed and can use dynamic update
		if mc.canUseDynamicLoraUpdate(oldBackend, newBackend) {
			dynamicUpdateBackends = append(dynamicUpdateBackends, newBackend.Name)
		}
	}

	return dynamicUpdateBackends
}

// canUseDynamicLoraUpdate checks if a specific backend can use dynamic LoRA update
func (mc *ModelController) canUseDynamicLoraUpdate(oldBackend, newBackend *registryv1alpha1.ModelBackend) bool {
	// Only VLLM backends support dynamic LoRA updates
	if newBackend.Type != registryv1alpha1.ModelBackendTypeVLLM {
		return false
	}

	// Check if runtime LoRA update is enabled for this backend
	if !mc.isRuntimeLoraUpdateEnabled(newBackend) {
		return false
	}

	// Deep copy backends without LoRA adapters for comparison
	oldBackendCopy := oldBackend.DeepCopy()
	newBackendCopy := newBackend.DeepCopy()
	oldBackendCopy.LoraAdapters = nil
	newBackendCopy.LoraAdapters = nil

	// If anything other than LoRA adapters changed, can't use dynamic update
	if !reflect.DeepEqual(oldBackendCopy, newBackendCopy) {
		return false
	}

	// Check if LoRA adapters actually changed
	return !reflect.DeepEqual(oldBackend.LoraAdapters, newBackend.LoraAdapters)
}

// isRuntimeLoraUpdateEnabled checks if runtime LoRA update is enabled for a backend
func (mc *ModelController) isRuntimeLoraUpdateEnabled(backend *registryv1alpha1.ModelBackend) bool {
	for _, envVar := range backend.Env {
		if envVar.Name == "VLLM_ALLOW_RUNTIME_LORA_UPDATING" {
			return strings.ToLower(envVar.Value) == "true"
		}
	}
	return false
}

// handleDynamicLoraUpdates handles runtime LoRA adapter updates for specified backends
func (mc *ModelController) handleDynamicLoraUpdates(oldModel, newModel *registryv1alpha1.Model, backendNames []string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	var successfullyUpdated []string

	// Create maps for easier lookup by backend name
	oldBackendMap := make(map[string]*registryv1alpha1.ModelBackend)
	newBackendMap := make(map[string]*registryv1alpha1.ModelBackend)

	for i := range oldModel.Spec.Backends {
		backend := &oldModel.Spec.Backends[i]
		oldBackendMap[backend.Name] = backend
	}

	for i := range newModel.Spec.Backends {
		backend := &newModel.Spec.Backends[i]
		newBackendMap[backend.Name] = backend
	}

	// Update LoRA adapters for specified backends
	for _, backendName := range backendNames {
		newBackend, exists := newBackendMap[backendName]
		if !exists {
			klog.Warningf("Backend %s not found in new model", backendName)
			continue
		}

		oldBackend, exists := oldBackendMap[backendName]
		if !exists {
			klog.Warningf("Backend %s not found in old model", backendName)
			continue
		}

		klog.Infof("Updating LoRA adapters for backend %s", backendName)

		// Get ModelInfer for this backend using the correct naming convention
		modelInferName := utils.GetBackendResourceName(newModel.Name, newBackend.Name)
		modelInfer, err := mc.modelInfersLister.ModelServings(newModel.Namespace).Get(modelInferName)
		if err != nil {
			klog.Errorf("Failed to get ModelInfer %s: %v", modelInferName, err)
			continue
		}

		// Check if ModelInfer is ready
		if !mc.isModelInferReady(modelInfer) {
			klog.Warningf("ModelInfer %s is not ready, skipping LoRA adapter update", modelInferName)
			continue
		}

		// Handle LoRA adapter changes
		if err := mc.updateLoraAdapters(ctx, newBackend, oldBackend, modelInfer); err != nil {
			klog.Errorf("Failed to update LoRA adapters for backend %s: %v", newBackend.Name, err)
			continue
		}

		successfullyUpdated = append(successfullyUpdated, backendName)
	}
	return successfullyUpdated
}
