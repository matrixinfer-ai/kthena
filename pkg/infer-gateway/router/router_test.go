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

package router

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"istio.io/istio/pkg/util/sets"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	klog.InitFlags(nil)
	// Set klog verbosity level to 4
	flag.Set("v", "4")
	flag.Parse() // Parse flags to apply the klog level

	pluginsWeight, plugins, pluginConfig, _ := utils.LoadSchedulerConfig("../scheduler/testdata/configmap.yaml")
	patch := gomonkey.ApplyFunc(utils.LoadSchedulerConfig, func() (map[string]int, []string, map[string]runtime.RawExtension, error) {
		return pluginsWeight, plugins, pluginConfig, nil
	})
	defer patch.Reset()

	// Run the tests
	exitCode := m.Run()
	// Exit with the appropriate code
	os.Exit(exitCode)
}

func buildPodInfo(name string, ip string) *datastore.PodInfo {
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Status: corev1.PodStatus{
			PodIP: ip,
		},
	}

	return &datastore.PodInfo{
		Pod: pod,
	}
}

func TestProxyModelEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req, _ := http.NewRequest("POST", "/", nil)
	modelReq := ModelRequest{"model": "test"}
	r := NewRouter(datastore.New())
	hookPatch := gomonkey.ApplyMethod(r.scheduler, "RunPostHooks", func(s scheduler.Scheduler, ctx *framework.Context, index int) {})
	defer hookPatch.Reset()

	tests := []struct {
		name         string
		ctx          *framework.Context
		decodePatch  func() *gomonkey.Patches
		prefillPatch func() *gomonkey.Patches
		wantErr      error
	}{
		{
			name: "PD Separation, request success",
			ctx: &framework.Context{
				Model:       "test",
				Prompt:      "it's test",
				DecodePods:  []*datastore.PodInfo{buildPodInfo("decode1", "1.1.1.1")},
				PrefillPods: []*datastore.PodInfo{buildPodInfo("prefill1", "1.1.1.2")},
			},
			decodePatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyDecodePod, func(c *gin.Context, req *http.Request, podIP string, port int32, stream bool) error {
					return nil
				})
			},
			prefillPatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyPrefillPod, func(req *http.Request, podIP string, port int32) error {
					return nil
				})
			},
			wantErr: nil,
		},
		{
			name: "BestPods are set, only decode",
			ctx: &framework.Context{
				Model:    "test",
				Prompt:   "test",
				BestPods: []*datastore.PodInfo{buildPodInfo("decode1", "1.1.1.1")},
			},
			decodePatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyDecodePod, func(c *gin.Context, req *http.Request, podIP string, port int32, stream bool) error {
					return nil
				})
			},
			prefillPatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyPrefillPod, func(req *http.Request, podIP string, port int32) error {
					return nil
				})
			},
			wantErr: nil,
		},
		{
			name: "DecodePods empty, only prefill",
			ctx: &framework.Context{
				Model:       "test",
				Prompt:      "test",
				PrefillPods: []*datastore.PodInfo{buildPodInfo("prefill1", "1.1.1.2")},
			},
			decodePatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyDecodePod, func(c *gin.Context, req *http.Request, podIP string, port int32, stream bool) error {
					return nil
				})
			},
			prefillPatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyPrefillPod, func(req *http.Request, podIP string, port int32) error {
					return nil
				})
			},
			wantErr: errors.New("request to all pods failed"),
		},
		{
			name: "proxyDecodePod returns error",
			ctx: &framework.Context{
				Model:    "test",
				Prompt:   "test",
				BestPods: []*datastore.PodInfo{buildPodInfo("decode1", "1.1.1.1")},
			},
			decodePatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyDecodePod, func(c *gin.Context, req *http.Request, podIP string, port int32, stream bool) error {
					return errors.New("decode error")
				})
			},
			prefillPatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyPrefillPod, func(req *http.Request, podIP string, port int32) error {
					return nil
				})
			},
			wantErr: errors.New("request to all pods failed"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patchDecode := tt.decodePatch()
			defer patchDecode.Reset()
			patchPrefill := tt.prefillPatch()
			defer patchPrefill.Reset()

			err := r.proxyModelEndpoint(c, req, tt.ctx, modelReq, int32(8080))
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// setupTestRouter initializes a router and its dependencies for testing.
func setupTestRouter(backendHandler http.Handler) (*Router, datastore.Store, *httptest.Server) {
	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(backendHandler)
	store := datastore.New()
	router := NewRouter(store)

	return router, store, backend
}

func TestRouter_HandlerFunc_AggregatedMode(t *testing.T) {
	// 1. Setup backend mock
	backendHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var reqBody ModelRequest
		json.Unmarshal(body, &reqBody)
		assert.Equal(t, "test-model-base", reqBody["model"]) // Model name overwritten
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"response-id"}`)
	})
	router, store, backend := setupTestRouter(backendHandler)
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	backendIP := backendURL.Hostname()
	backendPort, _ := strconv.Atoi(backendURL.Port())

	// 2. Populate store
	modelServer := &aiv1alpha1.ModelServer{
		ObjectMeta: v1.ObjectMeta{Name: "ms-1", Namespace: "default"},
		Spec: aiv1alpha1.ModelServerSpec{
			Model:           func(s string) *string { return &s }("test-model-base"),
			WorkloadPort:    aiv1alpha1.WorkloadPort{Port: int32(backendPort)},
			InferenceEngine: "vLLM",
			// No WorkloadSelector means aggregated mode
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{Name: "pod-1", Namespace: "default"},
		Status:     corev1.PodStatus{PodIP: backendIP, Phase: corev1.PodRunning},
	}
	modelRoute := &aiv1alpha1.ModelRoute{
		ObjectMeta: v1.ObjectMeta{Name: "mr-1", Namespace: "default"},
		Spec: aiv1alpha1.ModelRouteSpec{
			ModelName: "test-model",
			Rules: []*aiv1alpha1.Rule{
				{
					TargetModels: []*aiv1alpha1.TargetModel{
						{ModelServerName: "ms-1"},
					},
				},
			},
		},
	}

	store.AddOrUpdateModelServer(modelServer, sets.New(types.NamespacedName{Name: "pod-1", Namespace: "default"}))
	store.AddOrUpdatePod(pod1, []*aiv1alpha1.ModelServer{modelServer})
	store.AddOrUpdateModelRoute(modelRoute)

	// 3. Create request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "test-model", "prompt": "hello"}`
	c.Request, _ = http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	// 4. Execute handler
	router.HandlerFunc()(c)

	// 5. Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"response-id"`)
}

func TestRouter_HandlerFunc_DisaggregatedMode(t *testing.T) {
	// 1. Setup backend mock
	prefillReqs := 0
	decodeReqs := 0
	backendHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqBody ModelRequest
		json.Unmarshal(body, &reqBody)

		// Check if this is a prefill request (stream key removed) or decode request (stream key present)
		if _, hasStream := reqBody["stream"]; !hasStream {
			// Prefill request - stream key is deleted
			prefillReqs++
			assert.Equal(t, "test-model-base", reqBody["model"])
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"id":"prefill-resp"}`)
		} else {
			// Decode request - stream key is present
			decodeReqs++
			assert.Equal(t, "test-model-base", reqBody["model"])
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `data: {"id":"decode-resp"}`)
		}
	})
	router, store, backend := setupTestRouter(backendHandler)
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	backendIP := backendURL.Hostname()
	backendPort, _ := strconv.Atoi(backendURL.Port())

	// 2. Populate store
	modelServer := &aiv1alpha1.ModelServer{
		ObjectMeta: v1.ObjectMeta{Name: "ms-1", Namespace: "default"},
		Spec: aiv1alpha1.ModelServerSpec{
			Model:           func(s string) *string { return &s }("test-model-base"),
			WorkloadPort:    aiv1alpha1.WorkloadPort{Port: int32(backendPort)},
			InferenceEngine: "vLLM",
			WorkloadSelector: &aiv1alpha1.WorkloadSelector{
				PDGroup: &aiv1alpha1.PDGroup{
					GroupKey:      "group",
					DecodeLabels:  map[string]string{"app": "decode"},
					PrefillLabels: map[string]string{"app": "prefill"},
				},
			},
		},
	}
	decodePod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "decode-pod-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "decode", "group": "test-group"},
		},
		Status: corev1.PodStatus{PodIP: backendIP, Phase: corev1.PodRunning},
	}
	prefillPod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "prefill-pod-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "prefill", "group": "test-group"},
		},
		Status: corev1.PodStatus{PodIP: backendIP, Phase: corev1.PodRunning},
	}

	modelRoute := &aiv1alpha1.ModelRoute{
		ObjectMeta: v1.ObjectMeta{Name: "mr-1", Namespace: "default"},
		Spec: aiv1alpha1.ModelRouteSpec{
			ModelName: "test-model",
			Rules: []*aiv1alpha1.Rule{
				{
					TargetModels: []*aiv1alpha1.TargetModel{
						{ModelServerName: "ms-1"},
					},
				},
			},
		},
	}

	store.AddOrUpdateModelServer(modelServer, sets.New(
		types.NamespacedName{Name: "decode-pod-1", Namespace: "default"},
		types.NamespacedName{Name: "prefill-pod-1", Namespace: "default"},
	))
	store.AddOrUpdatePod(decodePod, []*aiv1alpha1.ModelServer{modelServer})
	store.AddOrUpdatePod(prefillPod, []*aiv1alpha1.ModelServer{modelServer})
	store.AddOrUpdateModelRoute(modelRoute)

	// 3. Create request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "test-model", "prompt": "hello", "stream": true}`
	c.Request, _ = http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	// 4. Execute handler
	router.HandlerFunc()(c)

	// 5. Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, prefillReqs)
	assert.Equal(t, 1, decodeReqs)
	assert.Contains(t, w.Body.String(), `data: {"id":"decode-resp"}`)
}

func TestRouter_HandlerFunc_ModelNotFound(t *testing.T) {
	router, _, backend := setupTestRouter(nil)
	defer backend.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "non-existent-model", "prompt": "hello"}`
	c.Request, _ = http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	router.HandlerFunc()(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "can't find corresponding model server")
}

func TestRouter_HandlerFunc_ScheduleFailure(t *testing.T) {
	backendHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This should not be called
		t.Error("backend should not be called on schedule failure")
	})
	router, store, backend := setupTestRouter(backendHandler)
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	backendIP := backendURL.Hostname()
	backendPort, _ := strconv.Atoi(backendURL.Port())

	// 2. Populate store
	modelServer := &aiv1alpha1.ModelServer{
		ObjectMeta: v1.ObjectMeta{Name: "ms-1", Namespace: "default"},
		Spec: aiv1alpha1.ModelServerSpec{
			Model:        func(s string) *string { return &s }("test-model-base"),
			WorkloadPort: aiv1alpha1.WorkloadPort{Port: int32(backendPort)},
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{Name: "pod-1", Namespace: "default"},
		Status:     corev1.PodStatus{PodIP: backendIP, Phase: corev1.PodRunning},
	}
	modelRoute := &aiv1alpha1.ModelRoute{
		ObjectMeta: v1.ObjectMeta{Name: "mr-1", Namespace: "default"},
		Spec: aiv1alpha1.ModelRouteSpec{
			ModelName: "test-model",
			Rules: []*aiv1alpha1.Rule{
				{
					TargetModels: []*aiv1alpha1.TargetModel{
						{ModelServerName: "ms-1"},
					},
				},
			},
		},
	}

	store.AddOrUpdateModelServer(modelServer, sets.New(types.NamespacedName{Name: "pod-1", Namespace: "default"}))
	store.AddOrUpdatePod(pod1, []*aiv1alpha1.ModelServer{modelServer})
	store.AddOrUpdateModelRoute(modelRoute)

	podInfo := store.GetPodInfo(types.NamespacedName{Name: "pod-1", Namespace: "default"})
	assert.NotNil(t, podInfo)
	podInfo.RequestWaitingNum = 20 // default max is 10, so this should be filtered out

	// 3. Create request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "test-model", "prompt": "hello"}`
	c.Request, _ = http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	// 4. Execute handler
	router.HandlerFunc()(c)

	// 5. Assertions
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "can't schedule to target pod")
}
func TestIsTokenUsageEnabled(t *testing.T) {
	tests := []struct {
		name         string
		modelRequest ModelRequest
		expected     bool
	}{
		{
			name:         "no stream_options",
			modelRequest: ModelRequest{"model": "test"},
			expected:     false,
		},
		{
			name: "stream_options exists but no include_usage",
			modelRequest: ModelRequest{
				"model":          "test",
				"stream_options": map[string]interface{}{},
			},
			expected: false,
		},
		{
			name: "include_usage is not a boolean",
			modelRequest: ModelRequest{
				"model": "test",
				"stream_options": map[string]interface{}{
					"include_usage": "true",
				},
			},
			expected: false,
		},
		{
			name: "include_usage is boolean false",
			modelRequest: ModelRequest{
				"model": "test",
				"stream_options": map[string]interface{}{
					"include_usage": false,
				},
			},
			expected: false,
		},
		{
			name: "include_usage is boolean true",
			modelRequest: ModelRequest{
				"model": "test",
				"stream_options": map[string]interface{}{
					"include_usage": true,
				},
			},
			expected: true,
		},
		{
			name: "stream_options is not a map",
			modelRequest: ModelRequest{
				"model":          "test",
				"stream_options": "invalid",
			},
			expected: false,
		},
		{
			name: "stream_options is not a map",
			modelRequest: ModelRequest{
				"model":          "test",
				"stream_options": nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTokenUsageEnabled(tt.modelRequest)
			assert.Equal(t, tt.expected, result)
		})
	}
}
