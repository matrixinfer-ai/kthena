package router

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

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
	hookPatch := *gomonkey.ApplyMethod(r.scheduler, "RunPostHooks", func(s scheduler.Scheduler, ctx *framework.Context, index int) {})

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
				return gomonkey.ApplyFunc(proxyDecodePod, func(c *gin.Context, req *http.Request, podIP string, port int32) error {
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
			name: "PrefillPods empty, only decode",
			ctx: &framework.Context{
				Model:      "test",
				Prompt:     "test",
				DecodePods: []*datastore.PodInfo{buildPodInfo("decode1", "1.1.1.1")},
			},
			decodePatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyDecodePod, func(c *gin.Context, req *http.Request, podIP string, port int32) error {
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
				return gomonkey.ApplyFunc(proxyDecodePod, func(c *gin.Context, req *http.Request, podIP string, port int32) error {
					return nil
				})
			},
			prefillPatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyPrefillPod, func(req *http.Request, podIP string, port int32) error {
					return nil
				})
			},
			wantErr: errors.New("no pod meets the requirements"),
		},
		{
			name: "proxyDecodePod returns error",
			ctx: &framework.Context{
				Model:      "test",
				Prompt:     "test",
				DecodePods: []*datastore.PodInfo{buildPodInfo("decode1", "1.1.1.1")},
			},
			decodePatch: func() *gomonkey.Patches {
				return gomonkey.ApplyFunc(proxyDecodePod, func(c *gin.Context, req *http.Request, podIP string, port int32) error {
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
			assert.Equal(t, tt.wantErr, err)
		})
	}
	defer hookPatch.Reset()
}
