package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
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
	r := &Router{}

	tests := []struct {
		name         string
		ctxs         []*framework.Context
		decodePatch  gomonkey.Patches
		prefillPatch gomonkey.Patches
		wantErr      error
	}{
		{
			name: "PD Separation, request success",
			ctxs: []*framework.Context{
				{
					DecodePod:  buildPodInfo("decode1", "1.1.1.1"),
					PrefillPod: buildPodInfo("prefill1", "1.1.1.2"),
				},
			},
			decodePatch: *gomonkey.ApplyFunc(proxyDecodePod, func(c *gin.Context, req *http.Request, podIP string, port int32, modelRequest ModelRequest) error {
				return nil
			}),
			prefillPatch: *gomonkey.ApplyFunc(proxyPrefillPod, func(req *http.Request, podIP string, port int32, modelRequest ModelRequest) error {
				return nil
			}),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.proxyModelEndpoint(c, req, tt.ctxs, modelReq, int32(8080))
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
